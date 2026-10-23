package api_test

// Logs de un container (ticket 11): GET /api/logs abre un stream de Server-Sent Events con los
// logs actuales, que siguen en vivo, o con los del reinicio anterior.

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

func TestLosLogsDeUnContainerLleganPorElStreamYSiguenEnVivo(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), podConDosContainers(0))
	pedidos := kviz.logsDelCluster(t, map[bool]string{false: "GET / 200\nGET /salud 200\n"})

	stream := kviz.abrirLogs(t, "default", "web-7d4b9-x2k", "web", false)

	if got := stream.lineasHastaElFin(t); strings.Join(got, "|") != "GET / 200|GET /salud 200" {
		t.Fatalf("líneas = %q, want las del container web", got)
	}
	p := pedidos.unico(t)
	if p.Container != "web" || !p.Follow || p.Previous {
		t.Fatalf("pedido de logs = %s, want container web, siguiendo en vivo (follow) y no el anterior", mostrar(p))
	}
}

func TestLosLogsDelReinicioAnteriorSePidenConPrevious(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), podConDosContainers(3))
	pedidos := kviz.logsDelCluster(t, map[bool]string{
		false: "arrancando…\n",
		true:  "conectando a la base\npanic: connection refused\n",
	})

	stream := kviz.abrirLogs(t, "default", "web-7d4b9-x2k", "web", true)

	if got := stream.lineasHastaElFin(t); strings.Join(got, "|") != "conectando a la base|panic: connection refused" {
		t.Fatalf("líneas = %q, want las del reinicio anterior", got)
	}
	p := pedidos.unico(t)
	if p.Container != "web" || !p.Previous || p.Follow {
		t.Fatalf("pedido de logs = %s, want container web, el reinicio anterior (previous) y sin seguir", mostrar(p))
	}
}

func TestCadaContainerTieneSusPropiosLogs(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), podConDosContainers(0))
	pedidos := kviz.logsDelCluster(t, map[bool]string{false: "una línea\n"})

	kviz.abrirLogs(t, "default", "web-7d4b9-x2k", "log-shipper", false).lineasHastaElFin(t)

	if p := pedidos.unico(t); p.Container != "log-shipper" {
		t.Fatalf("container pedido = %q, want log-shipper", p.Container)
	}
}

func TestUnaLineaSinSaltoFinalTambienLlega(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), podConDosContainers(0))
	kviz.logsDelCluster(t, map[bool]string{false: "primera\nsegunda sin salto"})

	got := kviz.abrirLogs(t, "default", "web-7d4b9-x2k", "web", false).lineasHastaElFin(t)

	if strings.Join(got, "|") != "primera|segunda sin salto" {
		t.Fatalf("líneas = %q, want las dos", got)
	}
}

func TestSinReiniciosNoHayLogsAnteriores(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), podConDosContainers(0))

	if status := kviz.statusDeLogs(t, "default", "web-7d4b9-x2k", "web", true); status != http.StatusNotFound {
		t.Fatalf("GET /api/logs?anterior: status %d, want 404 (el container no se reinició)", status)
	}
}

func TestLogsDeUnPodOContainerQueNoEstaEnLaFotoDa404(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), podConDosContainers(0))

	if status := kviz.statusDeLogs(t, "default", "otro-pod", "web", false); status != http.StatusNotFound {
		t.Errorf("pod inexistente: status %d, want 404", status)
	}
	if status := kviz.statusDeLogs(t, "default", "web-7d4b9-x2k", "fantasma", false); status != http.StatusNotFound {
		t.Errorf("container inexistente: status %d, want 404", status)
	}
	if status := kviz.statusDeLogs(t, "kube-system", "web-7d4b9-x2k", "web", false); status != http.StatusNotFound {
		t.Errorf("otro namespace: status %d, want 404", status)
	}
}

func TestSiKubernetesNoDaLosLogsLlegaElMotivoPorElStream(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), podConDosContainers(0))
	motivo := `container "web" in pod "web-7d4b9-x2k" is waiting to start: ContainerCreating`
	kviz.cliente.PrependReactor("get", "pods", func(a k8stesting.Action) (bool, runtime.Object, error) {
		if a.GetSubresource() != "log" {
			return false, nil, nil
		}
		return true, nil, errorDeKubernetes(motivo)
	})

	m := kviz.abrirLogs(t, "default", "web-7d4b9-x2k", "web", false).esperarFin(t)

	if m.Tipo != "error" || !strings.Contains(m.Error, "waiting to start") {
		t.Fatalf("último mensaje = %s, want un error con el motivo de Kubernetes", mostrar(m))
	}
}

// ---------- arnés de los logs ----------

// podConDosContainers es un pod con el container web (con los reinicios dados) y un sidecar.
func podConDosContainers(reinicios int32) *corev1.Pod {
	p := pod("default", "web-7d4b9-x2k", "lab-worker", "10.244.1.9", corev1.PodRunning)
	p.Spec.Containers = []corev1.Container{{Name: "web", Image: "nginx:1.27"}, {Name: "log-shipper", Image: "fluent-bit:3"}}
	p.Status.ContainerStatuses = []corev1.ContainerStatus{
		{Name: "web", RestartCount: reinicios, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
		{Name: "log-shipper", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
	}
	return p
}

// pedidosDeLogs anota los pedidos de logs que le llegan al cluster falso.
type pedidosDeLogs struct {
	mu    sync.Mutex
	todos []corev1.PodLogOptions
}

func (p *pedidosDeLogs) unico(t *testing.T) corev1.PodLogOptions {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.todos) != 1 {
		t.Fatalf("pedidos de logs al cluster = %s, want uno", mostrar(p.todos))
	}
	return p.todos[0]
}

// logsDelCluster hace que el cluster falso responda los logs dados: los actuales (false) o los
// del reinicio anterior (true). Devuelve los pedidos que le llegan.
func (k *kvizDePrueba) logsDelCluster(t *testing.T, logs map[bool]string) *pedidosDeLogs {
	t.Helper()
	pedidos := &pedidosDeLogs{}
	k.cliente.PrependReactor("get", "pods", func(a k8stesting.Action) (bool, runtime.Object, error) {
		if a.GetSubresource() != "log" {
			return false, nil, nil
		}
		opts := a.(k8stesting.GenericAction).GetValue().(*corev1.PodLogOptions)
		pedidos.mu.Lock()
		pedidos.todos = append(pedidos.todos, *opts)
		pedidos.mu.Unlock()
		return true, &runtime.Unknown{Raw: []byte(logs[opts.Previous])}, nil
	})
	return pedidos
}

func logsURL(namespace, pod, container string, anterior bool) string {
	q := url.Values{"namespace": {namespace}, "pod": {pod}, "container": {container}}
	if anterior {
		q.Set("anterior", "true")
	}
	return "/api/logs?" + q.Encode()
}

// mensajeDeLogs es el contrato JSON de cada mensaje de GET /api/logs (web/src/logs.ts).
type mensajeDeLogs struct {
	Tipo   string   `json:"tipo"`
	Lineas []string `json:"lineas"`
	Error  string   `json:"error"`
}

type streamDeLogs struct {
	mensajes <-chan mensajeDeLogs
}

// abrirLogs se conecta a GET /api/logs como lo hace la interfaz (EventSource).
func (k *kvizDePrueba) abrirLogs(t *testing.T, namespace, pod, container string, anterior bool) *streamDeLogs {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ruta := logsURL(namespace, pod, container, anterior)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, k.srv.URL+ruta, nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", ruta, err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: status %d", ruta, res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("GET %s: Content-Type = %q, want text/event-stream", ruta, ct)
	}
	mensajes := make(chan mensajeDeLogs, 100)
	go func() {
		defer res.Body.Close()
		defer close(mensajes)
		lector := bufio.NewScanner(res.Body)
		for lector.Scan() {
			datos, ok := strings.CutPrefix(lector.Text(), "data: ")
			if !ok {
				continue
			}
			var m mensajeDeLogs
			if err := json.Unmarshal([]byte(datos), &m); err != nil {
				t.Errorf("mensaje de logs que no es JSON: %q: %v", datos, err)
				return
			}
			mensajes <- m
		}
	}()
	return &streamDeLogs{mensajes: mensajes}
}

// esperarFin lee el stream hasta el mensaje que lo termina (fin o error) y lo devuelve.
func (s *streamDeLogs) esperarFin(t *testing.T) mensajeDeLogs {
	t.Helper()
	m, _ := s.leerHastaElFin(t)
	return m
}

// lineasHastaElFin junta las líneas de logs hasta que el stream termina bien.
func (s *streamDeLogs) lineasHastaElFin(t *testing.T) []string {
	t.Helper()
	m, lineas := s.leerHastaElFin(t)
	if m.Tipo != "fin" {
		t.Fatalf("el stream terminó con %s, want fin", mostrar(m))
	}
	return lineas
}

func (s *streamDeLogs) leerHastaElFin(t *testing.T) (mensajeDeLogs, []string) {
	t.Helper()
	limite := time.After(esperaStream)
	var lineas []string
	for {
		select {
		case m, ok := <-s.mensajes:
			if !ok {
				t.Fatalf("el stream se cerró sin fin; llegaron %q", lineas)
			}
			switch m.Tipo {
			case "lineas":
				lineas = append(lineas, m.Lineas...)
			case "fin", "error":
				return m, lineas
			default:
				t.Fatalf("mensaje de logs desconocido: %s", mostrar(m))
			}
		case <-limite:
			t.Fatalf("el stream no terminó; llegaron %q", lineas)
		}
	}
}

func (k *kvizDePrueba) statusDeLogs(t *testing.T, namespace, pod, container string, anterior bool) int {
	t.Helper()
	res, err := http.Get(k.srv.URL + logsURL(namespace, pod, container, anterior))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	return res.StatusCode
}

type errorDeKubernetes string

func (e errorDeKubernetes) Error() string { return string(e) }
