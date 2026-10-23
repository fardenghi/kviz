package api_test

// Request de prueba (POST /api/pruebas): kviz la origina desde el pod de sondeo, devuelve la
// respuesta y el recorrido básico (Service → endpoints → pod). El pod de sondeo se reemplaza por un
// doble de su interfaz con respuestas guionadas; su ciclo de vida (creado, huérfano reutilizado o
// limpiado, borrado al cerrar) se observa en el cliente falso.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stesting "k8s.io/client-go/testing"

	"kviz/internal/sondeo"
)

// Contrato JSON de POST /api/pruebas, tal como lo lee la interfaz (web/src/prueba.ts).
type pedidoDePrueba struct {
	Tipo    string           `json:"tipo,omitempty"`
	Destino string           `json:"destino"`
	Metodo  string           `json:"metodo,omitempty"`
	Path    string           `json:"path,omitempty"`
	Headers []headerDePrueba `json:"headers,omitempty"`
	Body    string           `json:"body,omitempty"`
}

type headerDePrueba struct {
	Nombre string `json:"nombre"`
	Valor  string `json:"valor"`
}

type pruebaJSON struct {
	ID        int                `json:"id"`
	Tipo      string             `json:"tipo"`
	URL       string             `json:"url"`
	Metodo    string             `json:"metodo"`
	Origen    string             `json:"origen"`
	Respuesta *respuestaDePrueba `json:"respuesta"`
	Error     string             `json:"error"`
	Recorrido recorridoJSON      `json:"recorrido"`
}

type respuestaDePrueba struct {
	Status  int              `json:"status"`
	Texto   string           `json:"texto"`
	Headers []headerDePrueba `json:"headers"`
	Body    string           `json:"body"`
	Ms      float64          `json:"ms"`
}

type recorridoJSON struct {
	Pasos []pasoJSON `json:"pasos"`
}

type pasoJSON struct {
	N       int       `json:"n"`
	Tipo    string    `json:"tipo"`
	Titulo  string    `json:"titulo"`
	Detalle string    `json:"detalle"`
	Objetos []refJSON `json:"objetos"`
}

type refJSON struct {
	Tipo string `json:"tipo"`
	ID   string `json:"id"`
}

const (
	podK2x8p = "backend-79f9ffc666-k2x8p" // en lab-worker, 10.244.1.5
	pod7hw4q = "backend-79f9ffc666-7hw4q" // en lab-worker2, 10.244.2.4
)

func TestUnaRequestDePruebaAUnServiceDevuelveLaRespuestaDelPodDeSondeo(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	kviz.sondeo.responder(sondeo.RespuestaHTTP{
		Status:   200,
		Headers:  []sondeo.Header{{Nombre: "Content-Type", Valor: "text/plain"}},
		Body:     "Hello, " + podK2x8p + "!",
		Duracion: 14 * time.Millisecond,
	})

	status, got := kviz.mandarPrueba(t, pedidoDePrueba{
		Destino: "backend.default.svc.cluster.local:80",
		Metodo:  "POST",
		Path:    "/hello",
		Headers: []headerDePrueba{{Nombre: "X-Curso", Valor: "redes"}},
		Body:    `{"hola":1}`,
	})

	if status != http.StatusOK {
		t.Fatalf("POST /api/pruebas: status %d", status)
	}
	wantRespuesta := &respuestaDePrueba{
		Status: 200, Texto: "OK", Headers: []headerDePrueba{{Nombre: "Content-Type", Valor: "text/plain"}},
		Body: "Hello, " + podK2x8p + "!", Ms: 14,
	}
	if got.Tipo != "http" || got.Metodo != "POST" || got.URL != "http://backend.default.svc.cluster.local:80/hello" ||
		!reflect.DeepEqual(got.Respuesta, wantRespuesta) || got.Error != "" {
		t.Fatalf("prueba = %s, want la respuesta guionada de POST http://backend.default.svc.cluster.local:80/hello", mostrar(got))
	}
	ejecutados := kviz.sondeo.ejecutados()
	wantPedido := sondeo.PedidoHTTP{
		Metodo: "POST", URL: "http://backend.default.svc.cluster.local:80/hello",
		Headers: []sondeo.Header{{Nombre: "X-Curso", Valor: "redes"}}, Body: `{"hola":1}`,
	}
	if len(ejecutados) != 1 || !reflect.DeepEqual(ejecutados[0].pedido, wantPedido) {
		t.Fatalf("el pod de sondeo ejecutó %s, want %s", mostrar(ejecutados), mostrar(wantPedido))
	}
}

func TestElRecorridoAUnNombreDNSVaDelDNSAlServiceASusEndpointsYAlPodQueRespondio(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	kviz.sondeo.responder(sondeo.RespuestaHTTP{Status: 200, Body: "Hello, " + podK2x8p + "!"})

	_, got := kviz.mandarPrueba(t, pedidoDePrueba{Destino: "backend.default.svc.cluster.local:80", Path: "/hello"})

	want := []pasoJSON{
		{N: 1, Tipo: "dns", Titulo: "backend.default.svc.cluster.local → 10.96.11.121", Detalle: "CoreDNS resuelve el nombre del Service a su ClusterIP",
			Objetos: []refJSON{{Tipo: "service", ID: "default/backend"}}},
		{N: 2, Tipo: "service", Titulo: "backend 10.96.11.121:80 → :8081", Detalle: "NodePort, targetPort 8081",
			Objetos: []refJSON{{Tipo: "service", ID: "default/backend"}}},
		{N: 3, Tipo: "endpoints", Titulo: "10.244.1.5:8081, 10.244.2.4:8081", Detalle: "2 endpoints listos; kube-proxy elige uno",
			Objetos: []refJSON{{Tipo: "pod", ID: "default/" + podK2x8p}, {Tipo: "pod", ID: "default/" + pod7hw4q}}},
		{N: 4, Tipo: "pod", Titulo: podK2x8p + " 10.244.1.5", Detalle: "Conocido: el body de la respuesta trae el nombre del pod",
			Objetos: []refJSON{{Tipo: "pod", ID: "default/" + podK2x8p}}},
	}
	if !reflect.DeepEqual(got.Recorrido.Pasos, want) {
		t.Fatalf("recorrido = %s\nwant %s", mostrar(got.Recorrido.Pasos), mostrar(want))
	}
}

func TestSinSaberQuePodRespondioElRecorridoTerminaEnLosEndpointsListos(t *testing.T) {
	objetos := append(clusterDeLaGuia(),
		pod("default", "backend-79f9ffc666-zz9zz", "lab-worker", "10.244.1.9", corev1.PodRunning),
		endpointSlice("default", "backend-otra", "backend", endpoint("10.244.1.9", "backend-79f9ffc666-zz9zz", "lab-worker", ptrBool(false))),
	)
	kviz := nuevoKviz(t, "kind-lab", objetos...)
	kviz.sondeo.responder(sondeo.RespuestaHTTP{Status: 200, Body: "Hello!"})

	// por ClusterIP y sin puerto: va al 80
	_, got := kviz.mandarPrueba(t, pedidoDePrueba{Destino: "10.96.11.121"})

	if got.URL != "http://10.96.11.121/" {
		t.Errorf("url = %q, want http://10.96.11.121/", got.URL)
	}
	tipos := []string{}
	for _, p := range got.Recorrido.Pasos {
		tipos = append(tipos, p.Tipo)
	}
	if !reflect.DeepEqual(tipos, []string{"service", "endpoints"}) {
		t.Fatalf("pasos = %s, want service y endpoints, sin pod", mostrar(got.Recorrido.Pasos))
	}
	endpoints := got.Recorrido.Pasos[1]
	wantObjetos := []refJSON{{Tipo: "pod", ID: "default/" + podK2x8p}, {Tipo: "pod", ID: "default/" + pod7hw4q}}
	if !reflect.DeepEqual(endpoints.Objetos, wantObjetos) || endpoints.Detalle != "2 endpoints listos; kube-proxy elige uno; 1 no listo, que no recibe tráfico" {
		t.Fatalf("paso de endpoints = %s, want solo los 2 listos y el no listo aparte", mostrar(endpoints))
	}
}

func TestUnaRequestSinRespuestaDiceElMotivoYMuestraElRecorrido(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	kviz.sondeo.guionar(func(sondeo.PedidoHTTP) (sondeo.RespuestaHTTP, error) {
		return sondeo.RespuestaHTTP{}, sondeo.SinRespuesta{Motivo: "Failed to connect to backend.default port 80: Connection refused"}
	})

	status, got := kviz.mandarPrueba(t, pedidoDePrueba{Destino: "backend.default"})

	if status != http.StatusOK || got.Respuesta != nil || got.Error != "Failed to connect to backend.default port 80: Connection refused" {
		t.Fatalf("status %d, prueba = %s, want sin respuesta y con el motivo", status, mostrar(got))
	}
	if len(got.Recorrido.Pasos) != 3 {
		t.Fatalf("recorrido = %s, want DNS, Service y endpoints", mostrar(got.Recorrido))
	}
}

func TestUnDestinoQueNoEstaEnLaFotoSePruebaIgualSinRecorrido(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	kviz.sondeo.responder(sondeo.RespuestaHTTP{Status: 404, Body: "no"})

	status, got := kviz.mandarPrueba(t, pedidoDePrueba{Destino: "http://example.internal:8080/x?y=1"})

	if status != http.StatusOK || got.Respuesta == nil || got.Respuesta.Status != 404 || got.Respuesta.Texto != "Not Found" {
		t.Fatalf("status %d, prueba = %s, want la respuesta 404", status, mostrar(got))
	}
	if got.URL != "http://example.internal:8080/x?y=1" || got.Recorrido.Pasos == nil || len(got.Recorrido.Pasos) != 0 {
		t.Fatalf("prueba = %s, want la URL tal cual y un recorrido sin pasos", mostrar(got))
	}
}

func TestUnPedidoSinDestinoEsUnError(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)

	if status, _ := kviz.mandarPrueba(t, pedidoDePrueba{Destino: " "}); status != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", status)
	}
	if len(kviz.sondeo.ejecutados()) != 0 {
		t.Fatal("el pod de sondeo no debería haber ejecutado nada")
	}
}

func TestSinClusterNoSePuedeMandarUnaRequestDePrueba(t *testing.T) {
	kc := nuevoKubeconfig("kind-lab")
	lab := kc.crearCluster("kind-lab", clusterDeLaGuia()...)
	kviz := nuevoKvizConKubeconfig(t, kc)
	kviz.esperarConectado(t)
	lab.caido.Store(true)
	kviz.fotoHastaQue(t, func(f fotoJSON) bool { return !f.Conectado })

	if status, _ := kviz.mandarPrueba(t, pedidoDePrueba{Destino: "backend.default"}); status != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503", status)
	}
}

// ---------- ciclo de vida del pod de sondeo ----------

func TestElPodDeSondeoSeCreaEnKvizSystemRecienConLaPrimeraRequest(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	if pods := kviz.podsDeSondeo(t); len(pods) != 0 {
		t.Fatalf("antes de la primera request hay %d pods en kviz-system, want ninguno", len(pods))
	}

	_, primera := kviz.mandarPrueba(t, pedidoDePrueba{Destino: "backend.default"})
	kviz.mandarPrueba(t, pedidoDePrueba{Destino: "backend.default"})

	pods := kviz.podsDeSondeo(t)
	if len(pods) != 1 {
		t.Fatalf("hay %d pods en kviz-system, want uno solo para las dos requests", len(pods))
	}
	p := pods[0]
	if len(p.Spec.Containers) != 1 || !strings.HasPrefix(p.Spec.Containers[0].Image, "curlimages/curl:") {
		t.Errorf("containers = %s, want uno con la imagen de curl", mostrar(p.Spec.Containers))
	}
	for _, e := range kviz.sondeo.ejecutados() {
		if e.pod.Namespace != "kviz-system" || e.pod.Nombre != p.Name {
			t.Errorf("la request salió de %s/%s, want del pod de sondeo %s", e.pod.Namespace, e.pod.Nombre, p.Name)
		}
	}
	if want := "pod de sondeo " + p.Name + " (kviz-system, lab-control-plane)"; primera.Origen != want {
		t.Errorf("origen = %q, want %q", primera.Origen, want)
	}
}

func TestSiBorranElPodDeSondeoLaRequestSiguienteCreaOtro(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	kviz.mandarPrueba(t, pedidoDePrueba{Destino: "backend.default"})
	kviz.borrar(t, "kviz-system", kviz.podsDeSondeo(t)[0].Name)

	if status, _ := kviz.mandarPrueba(t, pedidoDePrueba{Destino: "backend.default"}); status != http.StatusOK {
		t.Fatalf("status %d, want 200", status)
	}
	if pods := kviz.podsDeSondeo(t); len(pods) != 1 {
		t.Fatalf("hay %d pods de sondeo, want uno nuevo", len(pods))
	}
}

func TestAlCerrarKvizSeBorraElPodDeSondeo(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	kviz.mandarPrueba(t, pedidoDePrueba{Destino: "backend.default"})

	kviz.cerrar(t)

	if pods := kviz.podsDeSondeo(t); len(pods) != 0 {
		t.Fatalf("después de cerrar quedan %s en kviz-system", nombres(pods))
	}
}

func TestAlArrancarSeReutilizaUnPodDeSondeoHuerfanoQueSigueCorriendo(t *testing.T) {
	huerfano := podDeSondeoHuerfano("kviz-sondeo-viejo", corev1.PodRunning)
	kviz := nuevoKviz(t, "kind-lab", append(clusterDeLaGuia(), huerfano)...)

	kviz.mandarPrueba(t, pedidoDePrueba{Destino: "backend.default"})

	if pods := kviz.podsDeSondeo(t); len(pods) != 1 || pods[0].Name != "kviz-sondeo-viejo" {
		t.Fatalf("pods en kviz-system = %s, want solo el huérfano reutilizado", nombres(pods))
	}
	if e := kviz.sondeo.ejecutados(); len(e) != 1 || e[0].pod.Nombre != "kviz-sondeo-viejo" {
		t.Fatalf("ejecutados = %s, want la request desde el huérfano", mostrar(e))
	}
	kviz.cerrar(t)
	if pods := kviz.podsDeSondeo(t); len(pods) != 0 {
		t.Fatalf("después de cerrar quedan %s, want el huérfano adoptado también borrado", nombres(pods))
	}
}

func TestAlArrancarSeBorranLosPodsDeSondeoHuerfanosQueNoSirven(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", append(clusterDeLaGuia(),
		podDeSondeoHuerfano("kviz-sondeo-viejo", corev1.PodFailed),
		podDeSondeoHuerfano("kviz-sondeo-otro", corev1.PodRunning),
		podDeSondeoHuerfano("kviz-sondeo-tercero", corev1.PodRunning),
		pod("kviz-system", "no-es-de-kviz", "lab-worker", "10.244.1.8", corev1.PodRunning),
	)...)

	// al conectarse ya limpió: queda uno de los que corren (para reutilizarlo) y el pod ajeno
	pods := kviz.podsDeSondeo(t)
	if len(pods) != 2 || (pods[0].Name != "kviz-sondeo-otro" && pods[0].Name != "kviz-sondeo-tercero") || pods[1].Name != "no-es-de-kviz" {
		t.Fatalf("pods en kviz-system = %s, want uno de los huérfanos que corren y el pod ajeno", nombres(pods))
	}
}

func TestCrearYBorrarElPodDeSondeoEsLaUnicaEscrituraEnElCluster(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kviz-system"}}
	kviz := nuevoKviz(t, "kind-lab", append(clusterDeLaGuia(), ns)...)
	kviz.abrirStream(t).siguiente(t)
	kviz.getJSON(t, "/api/comandos?tipo=Service&namespace=default&nombre=backend", new([]any))
	kviz.mandarPrueba(t, pedidoDePrueba{Destino: "backend.default"})
	kviz.mandarPrueba(t, pedidoDePrueba{Destino: "10.96.11.121"})

	kviz.cerrar(t)

	want := []string{"create pods kviz-system", "delete pods kviz-system"}
	if got := escrituras(kviz.cliente.Actions()); !reflect.DeepEqual(got, want) {
		t.Fatalf("escrituras en el cluster = %s, want %s", mostrar(got), mostrar(want))
	}
}

func TestSiNoExisteKvizSystemSeCreaParaElPodDeSondeo(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)

	kviz.mandarPrueba(t, pedidoDePrueba{Destino: "backend.default"})

	want := []string{"create namespaces ", "create pods kviz-system"}
	if got := escrituras(kviz.cliente.Actions()); !reflect.DeepEqual(got, want) {
		t.Fatalf("escrituras en el cluster = %s, want %s", mostrar(got), mostrar(want))
	}
}

func TestAlCambiarDeContextoSeBorraElPodDeSondeoDelClusterAnterior(t *testing.T) {
	kc := nuevoKubeconfig("kind-lab")
	lab := kc.crearCluster("kind-lab", clusterDeLaGuia()...)
	kc.crearCluster("kind-otro", nodo("otro-worker", "172.24.0.4"))
	kviz := nuevoKvizConKubeconfig(t, kc)
	kviz.cliente = lab.cliente
	kviz.esperarConectado(t)
	kviz.mandarPrueba(t, pedidoDePrueba{Destino: "backend.default"})

	kviz.cambiarContexto(t, "kind-otro")

	limite := time.Now().Add(5 * time.Second)
	for len(kviz.podsDeSondeo(t)) != 0 {
		if time.Now().After(limite) {
			t.Fatalf("el cluster de kind-lab sigue con %s", nombres(kviz.podsDeSondeo(t)))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// ---------- arnés: el doble del pod de sondeo ----------

// sondeoDePrueba reemplaza la ejecución dentro del pod de sondeo: devuelve respuestas guionadas y
// recuerda qué se le pidió y en qué pod.
type sondeoDePrueba struct {
	mu        sync.Mutex
	guion     func(sondeo.PedidoHTTP) (sondeo.RespuestaHTTP, error)
	historial []ejecucion
	// observador, si hay, ve cada request antes de que se responda (el Ingress Controller falso la anota en su log).
	observador func(sondeo.PedidoHTTP)
	// las pruebas de conexión TCP, aparte de las requests
	guionTCP   func(sondeo.PedidoTCP) (sondeo.ResultadoTCP, error)
	conexiones []sondeo.PedidoTCP
}

type ejecucion struct {
	pod    sondeo.Pod
	pedido sondeo.PedidoHTTP
}

func nuevoSondeoDePrueba() *sondeoDePrueba {
	return &sondeoDePrueba{
		guion: func(sondeo.PedidoHTTP) (sondeo.RespuestaHTTP, error) {
			return sondeo.RespuestaHTTP{Status: 200}, nil
		},
		guionTCP: func(sondeo.PedidoTCP) (sondeo.ResultadoTCP, error) {
			return sondeo.ResultadoTCP{Resultado: sondeo.Abre}, nil
		},
	}
}

// responder hace que todas las requests de prueba reciban esta respuesta.
func (s *sondeoDePrueba) responder(r sondeo.RespuestaHTTP) {
	s.guionar(func(sondeo.PedidoHTTP) (sondeo.RespuestaHTTP, error) { return r, nil })
}

func (s *sondeoDePrueba) guionar(g func(sondeo.PedidoHTTP) (sondeo.RespuestaHTTP, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.guion = g
}

func (s *sondeoDePrueba) HTTP(_ context.Context, pod sondeo.Pod, p sondeo.PedidoHTTP) (sondeo.RespuestaHTTP, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.historial = append(s.historial, ejecucion{pod: pod, pedido: p})
	if s.observador != nil {
		s.observador(p)
	}
	return s.guion(p)
}

// alEjecutar hace que o vea cada request de prueba que ejecuta el pod de sondeo.
func (s *sondeoDePrueba) alEjecutar(o func(sondeo.PedidoHTTP)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.observador = o
}

func (s *sondeoDePrueba) ejecutados() []ejecucion {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.historial)
}

// conectar hace que todas las pruebas de conexión TCP den este resultado.
func (s *sondeoDePrueba) conectar(r sondeo.ResultadoTCP) {
	s.guionarConexion(func(sondeo.PedidoTCP) (sondeo.ResultadoTCP, error) { return r, nil })
}

func (s *sondeoDePrueba) guionarConexion(g func(sondeo.PedidoTCP) (sondeo.ResultadoTCP, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.guionTCP = g
}

func (s *sondeoDePrueba) TCP(_ context.Context, _ sondeo.Pod, p sondeo.PedidoTCP) (sondeo.ResultadoTCP, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.conexiones = append(s.conexiones, p)
	return s.guionTCP(p)
}

// conexionesEjecutadas son las pruebas de conexión TCP que ejecutó el pod de sondeo.
func (s *sondeoDePrueba) conexionesEjecutadas() []sondeo.PedidoTCP {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.conexiones)
}

// mandarPrueba manda una request de prueba como el formulario de la interfaz.
func (k *kvizDePrueba) mandarPrueba(t *testing.T, p pedidoDePrueba) (int, pruebaJSON) {
	t.Helper()
	cuerpo, _ := json.Marshal(p)
	res, err := http.Post(k.srv.URL+"/api/pruebas", "application/json", bytes.NewReader(cuerpo))
	if err != nil {
		t.Fatalf("POST /api/pruebas: %v", err)
	}
	defer res.Body.Close()
	var got pruebaJSON
	if res.StatusCode == http.StatusOK {
		if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
			t.Fatalf("POST /api/pruebas: decodificar: %v", err)
		}
	}
	return res.StatusCode, got
}

// podsDeSondeo son los pods que hay en kviz-system, ordenados por nombre.
func (k *kvizDePrueba) podsDeSondeo(t *testing.T) []corev1.Pod {
	t.Helper()
	lista, err := k.cliente.CoreV1().Pods(sondeo.Namespace).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	sort.Slice(lista.Items, func(i, j int) bool { return lista.Items[i].Name < lista.Items[j].Name })
	return lista.Items
}

// cerrar cierra kviz como al cortarlo con Ctrl-C: borra el pod de sondeo.
func (k *kvizDePrueba) cerrar(t *testing.T) {
	t.Helper()
	if err := k.gestor.Cerrar(context.Background()); err != nil {
		t.Fatalf("cerrar: %v", err)
	}
}

// podDeSondeoHuerfano es un pod de sondeo que dejó una sesión anterior de kviz.
func podDeSondeoHuerfano(nombre string, fase corev1.PodPhase) *corev1.Pod {
	p := pod("kviz-system", nombre, "lab-worker", "10.244.1.7", fase)
	p.Labels = sondeo.Etiquetas
	return p
}

func nombres(pods []corev1.Pod) string {
	var n []string
	for _, p := range pods {
		n = append(n, p.Name)
	}
	return mostrar(n)
}

// escrituras son las acciones del cliente falso que modifican el cluster, como "create pods kviz-system/…".
func escrituras(acciones []k8stesting.Action) []string {
	var out []string
	for _, a := range acciones {
		switch a.GetVerb() {
		case "get", "list", "watch":
			continue
		}
		out = append(out, a.GetVerb()+" "+a.GetResource().Resource+" "+a.GetNamespace())
	}
	return out
}
