package api_test

// Arnés de la costura 1: los tests le hablan a kviz por su API HTTP, igual que la interfaz.
// Kubernetes se reemplaza por el cliente falso de client-go cargado con los objetos del caso.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"kviz/internal/api"
	"kviz/internal/conexion"
)

// Contrato JSON de la foto, tal como lo lee la interfaz (web/src/foto.ts).
// Se decodifica a tipos propios del test para que un cambio en los nombres JSON rompa el test.
type fotoJSON struct {
	Contexto  string       `json:"contexto"`
	Conectado bool         `json:"conectado"`
	Motivo    string       `json:"motivo"`
	Nodos     []nodoEnFoto `json:"nodos"`
	Pods      []podEnFoto  `json:"pods"`
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

func ptr(s string) *string { return &s }

// mostrar imprime un valor como JSON, para que los mensajes de error muestren los punteros.
func mostrar(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
