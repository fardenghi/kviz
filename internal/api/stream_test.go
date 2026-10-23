package api_test

// Tiempo real por el canal de streaming (GET /api/stream): al conectarse llega la foto completa
// y después, a medida que cambia el cluster, las actualizaciones.

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAlConectarseElStreamMandaLaFotoCompleta(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab",
		nodo("lab-worker", "172.23.0.4"),
		pod("default", "nginx", "lab-worker", "10.244.1.5", corev1.PodRunning),
	)

	stream := kviz.abrirStream(t)
	m := stream.siguiente(t)

	if m.Tipo != "foto" || m.Foto == nil {
		t.Fatalf("primer mensaje = %s, want la foto completa", mostrar(m))
	}
	if !m.Foto.Conectado || len(m.Foto.Nodos) != 1 || len(m.Foto.Pods) != 1 || m.Foto.Pods[0].Nombre != "nginx" {
		t.Fatalf("foto = %s, want el nodo lab-worker y el pod nginx", mostrar(m.Foto))
	}
}

func TestUnPodCreadoLlegaComoActualizacion(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"))
	stream := kviz.abrirStream(t)
	stream.siguiente(t) // la foto completa

	kviz.crear(t, pod("default", "web-7d4b9-x2k", "lab-worker", "10.244.1.9", corev1.PodRunning))

	c := stream.esperarCambio(t, "pods", "default/web-7d4b9-x2k")
	got := podDelCambio(t, c)
	want := podEnFoto{Nombre: "web-7d4b9-x2k", Namespace: "default", Nodo: ptr("lab-worker"), IP: ptr("10.244.1.9"), Fase: "Running"}
	if c.Borrado || got.podEnFoto.Nombre != want.Nombre || *got.Nodo != *want.Nodo || *got.IP != *want.IP || got.Fase != want.Fase {
		t.Fatalf("cambio = %s, want el pod %s", mostrar(c), mostrar(want))
	}
}

func TestUnPodModificadoLlegaComoActualizacion(t *testing.T) {
	pendiente := pod("default", "web-7d4b9-x2k", "lab-worker", "", corev1.PodPending)
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), pendiente)
	stream := kviz.abrirStream(t)
	stream.siguiente(t)

	corriendo := pendiente.DeepCopy()
	corriendo.Status.Phase, corriendo.Status.PodIP = corev1.PodRunning, "10.244.1.9"
	kviz.modificar(t, corriendo)

	got := podDelCambio(t, stream.esperarCambio(t, "pods", "default/web-7d4b9-x2k"))
	if got.Fase != "Running" || got.IP == nil || *got.IP != "10.244.1.9" {
		t.Fatalf("pod = %s, want Running con IP 10.244.1.9", mostrar(got))
	}
}

func TestUnPodBorradoLlegaComoBaja(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab",
		nodo("lab-worker", "172.23.0.4"),
		pod("default", "nginx", "lab-worker", "10.244.1.5", corev1.PodRunning),
	)
	stream := kviz.abrirStream(t)
	stream.siguiente(t)

	kviz.borrar(t, "default", "nginx")

	c := stream.esperarCambio(t, "pods", "default/nginx")
	if !c.Borrado {
		t.Fatalf("cambio = %s, want la baja del pod", mostrar(c))
	}
}

func TestUnPodQueSeEstaBorrandoQuedaTerminando(t *testing.T) {
	nginx := pod("default", "nginx", "lab-worker", "10.244.1.5", corev1.PodRunning)
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), nginx)
	stream := kviz.abrirStream(t)
	stream.siguiente(t)

	// lo que hace la API de Kubernetes con `kubectl delete` mientras corre el período de gracia
	terminando := nginx.DeepCopy()
	ahora := metav1.Now()
	terminando.DeletionTimestamp = &ahora
	kviz.modificar(t, terminando)

	got := podDelCambio(t, stream.esperarCambio(t, "pods", "default/nginx"))
	if got.Estado != "Terminating" {
		t.Fatalf("pod = %s, want estado Terminating", mostrar(got))
	}
}

func TestUnNodoNuevoLlegaComoActualizacion(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"))
	stream := kviz.abrirStream(t)
	stream.siguiente(t)

	if _, err := kviz.cliente.CoreV1().Nodes().Create(context.Background(), nodo("lab-worker2", "172.23.0.3"), metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	c := stream.esperarCambio(t, "nodos", "lab-worker2")
	var got nodoEnFoto
	if err := json.Unmarshal(c.Valor, &got); err != nil || got != (nodoEnFoto{Nombre: "lab-worker2", InternalIP: "172.23.0.3"}) {
		t.Fatalf("cambio = %s, want el nodo lab-worker2 con su InternalIP", mostrar(c))
	}
}

func TestUnEndpointQueEntraLlegaComoCambioDeSuService(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab",
		service("default", "web", corev1.ServiceTypeClusterIP, "10.96.0.20", map[string]string{"app": "web"},
			corev1.ServicePort{Protocol: corev1.ProtocolTCP, Port: 80}),
	)
	stream := kviz.abrirStream(t)
	stream.siguiente(t)

	es := endpointSlice("default", "web-abc12", "web", endpoint("10.244.1.8", "web-5d8f-a1", "lab-worker", ptrBool(true)))
	if _, err := kviz.cliente.DiscoveryV1().EndpointSlices("default").Create(context.Background(), es, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	c := stream.esperarCambio(t, "services", "default/web")
	var got serviceEnFoto
	if err := json.Unmarshal(c.Valor, &got); err != nil || len(got.Endpoints) != 1 || got.Endpoints[0].IP != "10.244.1.8" {
		t.Fatalf("cambio = %s, want el Service web con el endpoint 10.244.1.8", mostrar(c))
	}
}

func TestCadaPestanaRecibeLasActualizaciones(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"))
	a, b := kviz.abrirStream(t), kviz.abrirStream(t)
	a.siguiente(t)
	b.siguiente(t)

	kviz.crear(t, pod("default", "nginx", "lab-worker", "10.244.1.5", corev1.PodRunning))

	a.esperarCambio(t, "pods", "default/nginx")
	b.esperarCambio(t, "pods", "default/nginx")
}

// ---------- arnés del stream ----------

// crear, modificar y borrar cambian el cluster falso en vivo, como lo haría kubectl.
func (k *kvizDePrueba) crear(t *testing.T, p *corev1.Pod) {
	t.Helper()
	if _, err := k.cliente.CoreV1().Pods(p.Namespace).Create(context.Background(), p, metav1.CreateOptions{}); err != nil {
		t.Fatalf("crear pod %s: %v", p.Name, err)
	}
}

func (k *kvizDePrueba) modificar(t *testing.T, p *corev1.Pod) {
	t.Helper()
	if _, err := k.cliente.CoreV1().Pods(p.Namespace).Update(context.Background(), p, metav1.UpdateOptions{}); err != nil {
		t.Fatalf("modificar pod %s: %v", p.Name, err)
	}
}

func (k *kvizDePrueba) borrar(t *testing.T, namespace, nombre string) {
	t.Helper()
	if err := k.cliente.CoreV1().Pods(namespace).Delete(context.Background(), nombre, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("borrar pod %s: %v", nombre, err)
	}
}

// mensajeDelStream es el contrato JSON de cada mensaje del stream (web/src/tiempoReal.ts).
type mensajeDelStream struct {
	Tipo    string         `json:"tipo"`
	Foto    *fotoJSON      `json:"foto"`
	Cambios []cambioEnFoto `json:"cambios"`
}

type cambioEnFoto struct {
	Seccion string          `json:"seccion"`
	ID      string          `json:"id"`
	Valor   json.RawMessage `json:"valor"`
	Borrado bool            `json:"borrado"`
}

type streamDePrueba struct {
	mensajes <-chan mensajeDelStream
}

// abrirStream se conecta a GET /api/stream como lo hace la interfaz (EventSource) y decodifica
// cada mensaje `data:` del stream de Server-Sent Events.
func (k *kvizDePrueba) abrirStream(t *testing.T) *streamDePrueba {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, k.srv.URL+"/api/stream", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/stream: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/stream: status %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("GET /api/stream: Content-Type = %q, want text/event-stream", ct)
	}

	mensajes := make(chan mensajeDelStream, 100)
	go func() {
		defer res.Body.Close()
		defer close(mensajes)
		lector := bufio.NewScanner(res.Body)
		lector.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
		for lector.Scan() {
			datos, ok := strings.CutPrefix(lector.Text(), "data: ")
			if !ok {
				continue
			}
			var m mensajeDelStream
			if err := json.Unmarshal([]byte(datos), &m); err != nil {
				t.Errorf("mensaje del stream que no es JSON: %q: %v", datos, err)
				return
			}
			mensajes <- m
		}
	}()
	return &streamDePrueba{mensajes: mensajes}
}

const esperaStream = 5 * time.Second

func (s *streamDePrueba) siguiente(t *testing.T) mensajeDelStream {
	t.Helper()
	select {
	case m, ok := <-s.mensajes:
		if !ok {
			t.Fatal("el stream se cerró")
		}
		return m
	case <-time.After(esperaStream):
		t.Fatal("no llegó ningún mensaje por el stream")
	}
	return mensajeDelStream{}
}

// esperarCambio lee actualizaciones hasta encontrar un cambio de la sección e id pedidos, y lo
// devuelve. Falla si no llega a tiempo.
func (s *streamDePrueba) esperarCambio(t *testing.T, seccion, id string) cambioEnFoto {
	t.Helper()
	limite := time.After(esperaStream)
	var vistos []cambioEnFoto
	for {
		select {
		case m, ok := <-s.mensajes:
			if !ok {
				t.Fatal("el stream se cerró")
			}
			if m.Tipo != "cambios" {
				t.Fatalf("mensaje %s, want solo actualizaciones después de la foto", mostrar(m))
			}
			for _, c := range m.Cambios {
				if c.Seccion == seccion && c.ID == id {
					return c
				}
				vistos = append(vistos, c)
			}
		case <-limite:
			t.Fatalf("no llegó un cambio de %s %q; llegaron %s", seccion, id, mostrar(vistos))
		}
	}
}

// podDelCambio decodifica el valor de un cambio de la sección pods.
func podDelCambio(t *testing.T, c cambioEnFoto) podEnFotoEnVivo {
	t.Helper()
	var p podEnFotoEnVivo
	if err := json.Unmarshal(c.Valor, &p); err != nil {
		t.Fatalf("valor del cambio %s: %v", mostrar(c), err)
	}
	return p
}

type podEnFotoEnVivo struct {
	podEnFoto
	ID     string `json:"id"`
	Estado string `json:"estado"`
}
