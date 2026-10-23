package api_test

// Arnés de la costura 1: los tests le hablan a kviz por su API HTTP, igual que la interfaz.
// Kubernetes se reemplaza por el cliente falso de client-go cargado con los objetos del caso.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	"kviz/internal/api"
	"kviz/internal/conexion"
)

// Contrato JSON de la foto, tal como lo lee la interfaz (web/src/foto.ts).
// Se decodifica a tipos propios del test para que un cambio en los nombres JSON rompa el test.
type fotoJSON struct {
	Contexto  string           `json:"contexto"`
	Conectado bool             `json:"conectado"`
	Motivo    string           `json:"motivo"`
	Nodos     []nodoEnFoto     `json:"nodos"`
	Pods      []podEnFoto      `json:"pods"`
	Workloads []workloadEnFoto `json:"workloads"`
}

type nodoEnFoto struct {
	Nombre     string `json:"nombre"`
	InternalIP string `json:"internalIP"`
}

type podEnFoto struct {
	Nombre    string  `json:"nombre"`
	Namespace string  `json:"namespace"`
	Nodo      *string `json:"nodo"`
	IP        *string `json:"ip"`
	Fase      string  `json:"fase"`
	// Workload es el id del workload dueño del pod; null si es un Pod suelto.
	Workload *string `json:"workload"`
}

type workloadEnFoto struct {
	ID        string          `json:"id"`
	Tipo      string          `json:"tipo"`
	Nombre    string          `json:"nombre"`
	Namespace string          `json:"namespace"`
	Color     string          `json:"color"`
	Cadena    []eslabonEnFoto `json:"cadena"`
}

type eslabonEnFoto struct {
	Tipo   string   `json:"tipo"`
	Nombre string   `json:"nombre"`
	Pods   []string `json:"pods"`
}

type kvizDePrueba struct {
	srv     *httptest.Server
	cliente *fake.Clientset
}

func nuevoKviz(t *testing.T, contexto string, objetos ...runtime.Object) *kvizDePrueba {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	cliente := fake.NewClientset(objetos...)
	c := conexion.Nueva(cliente, contexto)
	if err := c.Iniciar(ctx); err != nil {
		t.Fatalf("iniciar conexión: %v", err)
	}
	srv := httptest.NewServer(api.Nuevo(c, nil))
	t.Cleanup(srv.Close)
	return &kvizDePrueba{srv: srv, cliente: cliente}
}

// nuevoKvizDesdeKubeconfig arranca kviz como el binario: con el contexto actual del kubeconfig
// que diga KUBECONFIG. Sirve para los casos en que no hay cluster al que conectarse.
func nuevoKvizDesdeKubeconfig(t *testing.T) *kvizDePrueba {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	c := conexion.DesdeKubeconfig()
	go c.Iniciar(ctx) //nolint:errcheck // sin cluster no hay nada que esperar
	srv := httptest.NewServer(api.Nuevo(c, nil))
	t.Cleanup(srv.Close)
	return &kvizDePrueba{srv: srv}
}

func (k *kvizDePrueba) foto(t *testing.T) fotoJSON {
	t.Helper()
	var f fotoJSON
	k.getJSON(t, "/api/foto", &f)
	return f
}

// fotoCruda devuelve la foto campo por campo, sin decodificar, para revisar el JSON tal cual.
func (k *kvizDePrueba) fotoCruda(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	var f map[string]json.RawMessage
	k.getJSON(t, "/api/foto", &f)
	return f
}

func (k *kvizDePrueba) getJSON(t *testing.T, ruta string, destino any) {
	t.Helper()
	res, err := http.Get(k.srv.URL + ruta)
	if err != nil {
		t.Fatalf("GET %s: %v", ruta, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d", ruta, res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(destino); err != nil {
		t.Fatalf("GET %s: decodificar: %v", ruta, err)
	}
}

// ---------- objetos de Kubernetes para armar los casos ----------

func nodo(nombre, internalIP string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: nombre},
		Status: corev1.NodeStatus{Addresses: []corev1.NodeAddress{
			{Type: corev1.NodeHostName, Address: nombre},
			{Type: corev1.NodeInternalIP, Address: internalIP},
		}},
	}
}

// pod arma un pod ubicado en un nodo, con su IP y fase. Con nodo "" queda sin nodo asignado.
func pod(namespace, nombre, nodo, ip string, fase corev1.PodPhase) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: nombre},
		Spec:       corev1.PodSpec{NodeName: nodo},
		Status:     corev1.PodStatus{Phase: fase, PodIP: ip},
	}
}

// ---------- workloads: dueños armados con ownerReferences ----------

var (
	tipoDeployment  = metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"}
	tipoReplicaSet  = metav1.TypeMeta{APIVersion: "apps/v1", Kind: "ReplicaSet"}
	tipoStatefulSet = metav1.TypeMeta{APIVersion: "apps/v1", Kind: "StatefulSet"}
	tipoDaemonSet   = metav1.TypeMeta{APIVersion: "apps/v1", Kind: "DaemonSet"}
	tipoJob         = metav1.TypeMeta{APIVersion: "batch/v1", Kind: "Job"}
	tipoCronJob     = metav1.TypeMeta{APIVersion: "batch/v1", Kind: "CronJob"}
)

// meta arma la metadata de un objeto. Si se pasa un dueño, queda como su controller en las
// ownerReferences, igual que lo hacen los controllers de Kubernetes.
func meta(namespace, nombre string, dueno ...metav1.Object) metav1.ObjectMeta {
	m := metav1.ObjectMeta{Namespace: namespace, Name: nombre, UID: types.UID(namespace + "/" + nombre)}
	for _, d := range dueno {
		tm := d.(interface{ GetObjectKind() schema.ObjectKind }).GetObjectKind().GroupVersionKind()
		m.OwnerReferences = append(m.OwnerReferences, metav1.OwnerReference{
			APIVersion: tm.GroupVersion().String(), Kind: tm.Kind, Name: d.GetName(), UID: d.GetUID(),
			Controller: ptrBool(true),
		})
	}
	return m
}

func deployment(namespace, nombre string) *appsv1.Deployment {
	return &appsv1.Deployment{TypeMeta: tipoDeployment, ObjectMeta: meta(namespace, nombre)}
}

// replicaSet arma un ReplicaSet de un Deployment con sus réplicas deseadas; creado indica el
// orden de creación (más alto = más nuevo).
func replicaSet(d *appsv1.Deployment, nombre string, replicas int32, creado int) *appsv1.ReplicaSet {
	rs := &appsv1.ReplicaSet{TypeMeta: tipoReplicaSet, ObjectMeta: meta(d.Namespace, nombre, d)}
	rs.CreationTimestamp = metav1.NewTime(time.Date(2026, 10, 1, 0, creado, 0, 0, time.UTC))
	rs.Spec.Replicas = &replicas
	return rs
}

func statefulSet(namespace, nombre string) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{TypeMeta: tipoStatefulSet, ObjectMeta: meta(namespace, nombre)}
}

func daemonSet(namespace, nombre string) *appsv1.DaemonSet {
	return &appsv1.DaemonSet{TypeMeta: tipoDaemonSet, ObjectMeta: meta(namespace, nombre)}
}

func cronJob(namespace, nombre string) *batchv1.CronJob {
	return &batchv1.CronJob{TypeMeta: tipoCronJob, ObjectMeta: meta(namespace, nombre)}
}

// job arma un Job; con dueño (un CronJob) queda como uno de sus Jobs.
func job(namespace, nombre string, dueno ...metav1.Object) *batchv1.Job {
	return &batchv1.Job{TypeMeta: tipoJob, ObjectMeta: meta(namespace, nombre, dueno...)}
}

// podDe arma un pod corriendo en un nodo cuyo dueño es el objeto dado.
func podDe(dueno metav1.Object, nombre, nodo string) *corev1.Pod {
	p := pod(dueno.GetNamespace(), nombre, nodo, "10.244.1.1", corev1.PodRunning)
	p.ObjectMeta = meta(dueno.GetNamespace(), nombre, dueno)
	return p
}

func ptrBool(b bool) *bool { return &b }

func ptr(s string) *string { return &s }

// mostrar imprime un valor como JSON, para que los mensajes de error muestren los punteros.
func mostrar(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
