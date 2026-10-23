package api_test

// Events del cluster (ticket 12): la foto y el stream traen los events, y cada uno dice a qué
// objeto de la foto se refiere, para seleccionarlo con un clic y mostrarlo en su panel de detalle.

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type eventEnFoto struct {
	ID      string        `json:"id"`
	Tipo    string        `json:"tipo"`
	Motivo  string        `json:"motivo"`
	Mensaje string        `json:"mensaje"`
	Objeto  objetoDeEvent `json:"objeto"`
	EnFoto  *objetoEnFoto `json:"enFoto"`
	Ultima  string        `json:"ultimaVez"`
	Veces   int32         `json:"veces"`
}

type objetoDeEvent struct {
	Tipo      string `json:"tipo"`
	Namespace string `json:"namespace"`
	Nombre    string `json:"nombre"`
}

// objetoEnFoto es la sección de la foto (pods, nodos, services, ingress, workloads) y el id del
// objeto en esa sección.
type objetoEnFoto struct {
	Seccion string `json:"seccion"`
	ID      string `json:"id"`
}

func (k *kvizDePrueba) events(t *testing.T) []eventEnFoto {
	t.Helper()
	var f struct {
		Events []eventEnFoto `json:"events"`
	}
	k.getJSON(t, "/api/foto", &f)
	return f.Events
}

var horaDePrueba = time.Date(2026, 10, 23, 15, 10, 31, 0, time.UTC)

// event arma un Event de core/v1 como los que escribe el kubelet o un controller: en el
// namespace del objeto (o en default si el objeto no tiene namespace).
func event(nombre string, objeto corev1.ObjectReference, tipo, motivo, mensaje string, ultima time.Time, veces int32) *corev1.Event {
	ns := objeto.Namespace
	if ns == "" {
		ns = "default"
	}
	return &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Namespace: ns, Name: nombre},
		InvolvedObject: objeto,
		Type:           tipo,
		Reason:         motivo,
		Message:        mensaje,
		FirstTimestamp: metav1.NewTime(ultima.Add(-time.Minute)),
		LastTimestamp:  metav1.NewTime(ultima),
		Count:          veces,
	}
}

func objetoRef(tipo, namespace, nombre string) corev1.ObjectReference {
	return corev1.ObjectReference{Kind: tipo, Namespace: namespace, Name: nombre}
}

func TestLaFotoTraeLosEventsDelCluster(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab",
		nodo("lab-worker", "172.23.0.4"),
		pod("default", "api-6f9b8d7c5-zt5rq", "lab-worker", "10.244.1.10", corev1.PodRunning),
		event("api-6f9b8d7c5-zt5rq.17a9b", objetoRef("Pod", "default", "api-6f9b8d7c5-zt5rq"),
			"Warning", "BackOff", "Back-off restarting failed container api", horaDePrueba, 7),
	)

	got := kviz.events(t)

	want := []eventEnFoto{{
		ID:      "default/api-6f9b8d7c5-zt5rq.17a9b",
		Tipo:    "Warning",
		Motivo:  "BackOff",
		Mensaje: "Back-off restarting failed container api",
		Objeto:  objetoDeEvent{Tipo: "Pod", Namespace: "default", Nombre: "api-6f9b8d7c5-zt5rq"},
		EnFoto:  &objetoEnFoto{Seccion: "pods", ID: "default/api-6f9b8d7c5-zt5rq"},
		Ultima:  "2026-10-23T15:10:31Z",
		Veces:   7,
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("events = %s\nwant %s", mostrar(got), mostrar(want))
	}
}

func TestCadaEventDiceAQueObjetoDeLaFotoSeRefiere(t *testing.T) {
	web := deployment("default", "web")
	rsNuevo := replicaSet(web, "web-6c7d9f8b5", 1, 2)
	rsViejo := replicaSet(web, "web-5d8f7c9b4", 0, 1) // ya escaló a 0: no aparece en la cadena
	report := cronJob("default", "report")
	jobReport := job("default", "report-28745160", report)
	db := statefulSet("default", "db")
	agente := daemonSet("kube-system", "log-agent")

	casos := []struct {
		objeto corev1.ObjectReference
		want   *objetoEnFoto
	}{
		{objetoRef("Pod", "default", "web-6c7d9f8b5-mn3vt"), &objetoEnFoto{"pods", "default/web-6c7d9f8b5-mn3vt"}},
		{objetoRef("Node", "", "lab-worker"), &objetoEnFoto{"nodos", "lab-worker"}},
		{objetoRef("Service", "default", "web"), &objetoEnFoto{"services", "default/web"}},
		{objetoRef("Ingress", "default", "web"), &objetoEnFoto{"ingress", "default/web"}},
		{objetoRef("Deployment", "default", "web"), &objetoEnFoto{"workloads", "Deployment/default/web"}},
		// los events de un ReplicaSet o un Job van al workload de arriba de su cadena
		{objetoRef("ReplicaSet", "default", "web-6c7d9f8b5"), &objetoEnFoto{"workloads", "Deployment/default/web"}},
		{objetoRef("ReplicaSet", "default", "web-5d8f7c9b4"), &objetoEnFoto{"workloads", "Deployment/default/web"}},
		{objetoRef("Job", "default", "report-28745160"), &objetoEnFoto{"workloads", "CronJob/default/report"}},
		{objetoRef("CronJob", "default", "report"), &objetoEnFoto{"workloads", "CronJob/default/report"}},
		{objetoRef("StatefulSet", "default", "db"), &objetoEnFoto{"workloads", "StatefulSet/default/db"}},
		{objetoRef("DaemonSet", "kube-system", "log-agent"), &objetoEnFoto{"workloads", "DaemonSet/kube-system/log-agent"}},
		// un objeto que ya no está (el pod se borró) o que kviz no dibuja (un PVC): sin objeto
		{objetoRef("Pod", "default", "web-5d8f7c9b4-qz7lm"), nil},
		{objetoRef("PersistentVolumeClaim", "default", "data-db-0"), nil},
		// mismo nombre en otro namespace: no es el mismo objeto
		{objetoRef("Service", "otro", "web"), nil},
	}
	objetos := []runtime.Object{
		nodo("lab-worker", "172.23.0.4"),
		podDe(rsNuevo, "web-6c7d9f8b5-mn3vt", "lab-worker"),
		service("default", "web", corev1.ServiceTypeClusterIP, "10.96.0.20", map[string]string{"app": "web"},
			corev1.ServicePort{Protocol: corev1.ProtocolTCP, Port: 80}),
		&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "web"}},
		web, rsNuevo, rsViejo, report, jobReport, db, agente,
	}
	for i, c := range casos {
		objetos = append(objetos, event(fmt.Sprintf("e%02d", i), c.objeto, "Normal", "Prueba", "", horaDePrueba, 1))
	}
	kviz := nuevoKviz(t, "kind-lab", objetos...)

	got := map[string]*objetoEnFoto{}
	for _, e := range kviz.events(t) {
		got[e.Objeto.Tipo+" "+e.Objeto.Namespace+"/"+e.Objeto.Nombre] = e.EnFoto
	}
	for _, c := range casos {
		clave := c.objeto.Kind + " " + c.objeto.Namespace + "/" + c.objeto.Name
		if g, ok := got[clave]; !ok || !reflect.DeepEqual(g, c.want) {
			t.Errorf("event de %s: enFoto = %s, want %s", clave, mostrar(g), mostrar(c.want))
		}
	}
}

func TestUnEventNuevoLlegaPorElStream(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"),
		pod("default", "ml-train", "", "", corev1.PodPending))
	stream := kviz.abrirStream(t)
	stream.siguiente(t) // la foto completa

	e := event("ml-train.17a9c", objetoRef("Pod", "default", "ml-train"),
		"Warning", "FailedScheduling", "0/3 nodes are available: 3 Insufficient cpu", horaDePrueba, 1)
	if _, err := kviz.cliente.CoreV1().Events("default").Create(context.Background(), e, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	got := eventDelCambio(t, stream.esperarCambio(t, "events", "default/ml-train.17a9c"))
	if got.Motivo != "FailedScheduling" || got.EnFoto == nil || *got.EnFoto != (objetoEnFoto{"pods", "default/ml-train"}) {
		t.Fatalf("event = %s, want FailedScheduling del pod ml-train", mostrar(got))
	}
}

func TestUnEventQueSeRepiteLlegaConLasVecesYLaUltimaVez(t *testing.T) {
	e := event("api.17a9d", objetoRef("Pod", "default", "api"), "Warning", "BackOff", "Back-off restarting failed container api", horaDePrueba, 3)
	kviz := nuevoKviz(t, "kind-lab", e)
	stream := kviz.abrirStream(t)
	stream.siguiente(t)

	otraVez := e.DeepCopy()
	otraVez.Count = 4
	otraVez.LastTimestamp = metav1.NewTime(horaDePrueba.Add(30 * time.Second))
	if _, err := kviz.cliente.CoreV1().Events("default").Update(context.Background(), otraVez, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}

	got := eventDelCambio(t, stream.esperarCambio(t, "events", "default/api.17a9d"))
	if got.Veces != 4 || got.Ultima != "2026-10-23T15:11:01Z" {
		t.Fatalf("event = %s, want 4 veces, la última a las 15:11:01", mostrar(got))
	}
}

func TestUnEventBorradoLlegaComoBaja(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", event("api.17a9d", objetoRef("Pod", "default", "api"), "Normal", "Pulled", "", horaDePrueba, 1))
	stream := kviz.abrirStream(t)
	stream.siguiente(t)

	// Kubernetes borra los events viejos (por defecto, a la hora)
	if err := kviz.cliente.CoreV1().Events("default").Delete(context.Background(), "api.17a9d", metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}

	if c := stream.esperarCambio(t, "events", "default/api.17a9d"); !c.Borrado {
		t.Fatalf("cambio = %s, want la baja del event", mostrar(c))
	}
}

func TestUnEventDeEventsK8sIoTraeLaUltimaVezDeSuSerie(t *testing.T) {
	// los events que se escriben con la API events.k8s.io no llevan lastTimestamp ni count: la
	// hora va en eventTime y, si se repite, en series
	unaVez := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Namespace: "default", Name: "nginx.1"},
		InvolvedObject: objetoRef("Pod", "default", "nginx"),
		Type:           "Normal", Reason: "Scheduled",
		EventTime: metav1.NewMicroTime(horaDePrueba),
	}
	repetido := &corev1.Event{
		ObjectMeta:     metav1.ObjectMeta{Namespace: "default", Name: "nginx.2"},
		InvolvedObject: objetoRef("Pod", "default", "nginx"),
		Type:           "Warning", Reason: "Unhealthy",
		EventTime: metav1.NewMicroTime(horaDePrueba),
		Series:    &corev1.EventSeries{Count: 5, LastObservedTime: metav1.NewMicroTime(horaDePrueba.Add(2 * time.Minute))},
	}
	kviz := nuevoKviz(t, "kind-lab", unaVez, repetido)

	got := kviz.events(t)

	if len(got) != 2 || got[0].Ultima != "2026-10-23T15:10:31Z" || got[0].Veces != 1 ||
		got[1].Ultima != "2026-10-23T15:12:31Z" || got[1].Veces != 5 {
		t.Fatalf("events = %s, want Scheduled 1 vez a las 15:10:31 y Unhealthy 5 veces, la última a las 15:12:31", mostrar(got))
	}
}

func eventDelCambio(t *testing.T, c cambioEnFoto) eventEnFoto {
	t.Helper()
	if c.Borrado {
		t.Fatalf("cambio = %s, want un alta o modificación", mostrar(c))
	}
	var e eventEnFoto
	if err := json.Unmarshal(c.Valor, &e); err != nil {
		t.Fatalf("valor del cambio %s: %v", mostrar(c), err)
	}
	return e
}
