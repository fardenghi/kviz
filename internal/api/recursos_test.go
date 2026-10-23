package api_test

// Recursos (ticket 09): requests y limits de CPU y memoria de cada container, con el consumo real
// si el cluster tiene metrics-server (metrics.k8s.io). Sin metrics-server la foto dice que no hay
// métricas, y kviz sugiere el comando para instalarlo en kind (con --kubelet-insecure-tls).

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"

	"kviz/internal/conexion"
)

type cantidadEnFoto struct {
	Texto string `json:"texto"`
	// Valor es en milicores (CPU) o en bytes (memoria).
	Valor int64 `json:"valor"`
}

type recursoEnFoto struct {
	Request *cantidadEnFoto `json:"request"`
	Limit   *cantidadEnFoto `json:"limit"`
	Consumo *cantidadEnFoto `json:"consumo"`
}

type recursosEnFoto struct {
	CPU     recursoEnFoto `json:"cpu"`
	Memoria recursoEnFoto `json:"memoria"`
}

type fotoConRecursos struct {
	Metricas bool `json:"metricas"`
	Pods     []struct {
		Nombre     string `json:"nombre"`
		Containers []struct {
			Nombre   string         `json:"nombre"`
			Recursos recursosEnFoto `json:"recursos"`
		} `json:"containers"`
	} `json:"pods"`
}

// recursos devuelve si la foto dice que hay métricas, y los recursos de cada container por
// "<pod>/<container>".
func (k *kvizDePrueba) recursos(t *testing.T) (metricas bool, porContainer map[string]recursosEnFoto) {
	t.Helper()
	var f fotoConRecursos
	k.getJSON(t, "/api/foto", &f)
	porContainer = map[string]recursosEnFoto{}
	for _, p := range f.Pods {
		for _, c := range p.Containers {
			porContainer[p.Nombre+"/"+c.Nombre] = c.Recursos
		}
	}
	return f.Metricas, porContainer
}

// ---------- doble de metrics-server ----------

// metricsServerDePrueba hace de metrics-server de un cluster falso. Sin instalar, la API
// metrics.k8s.io no existe (como en un cluster de kind recién creado).
type metricsServerDePrueba struct {
	mu        sync.Mutex
	instalado bool
	pods      []metricsv1beta1.PodMetrics
}

// medir instala metrics-server (si no estaba) y deja el consumo de los pods.
func (m *metricsServerDePrueba) medir(pods ...metricsv1beta1.PodMetrics) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.instalado, m.pods = true, pods
}

func (m *metricsServerDePrueba) Pods(context.Context) ([]metricsv1beta1.PodMetrics, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.instalado {
		return nil, errors.New("the server could not find the requested resource (get pods.metrics.k8s.io)")
	}
	return append([]metricsv1beta1.PodMetrics(nil), m.pods...), nil
}

var _ conexion.Metricas = (*metricsServerDePrueba)(nil)

// consumo es lo que mide metrics-server de un pod: cpu y memoria por container, de a pares
// (container, cpu, memoria).
func consumo(p *corev1.Pod, porContainer ...[3]string) metricsv1beta1.PodMetrics {
	m := metricsv1beta1.PodMetrics{
		ObjectMeta: metav1.ObjectMeta{Namespace: p.Namespace, Name: p.Name},
		Timestamp:  metav1.NewTime(arranque.Add(time.Minute)),
		Window:     metav1.Duration{Duration: 15 * time.Second},
	}
	for _, c := range porContainer {
		m.Containers = append(m.Containers, metricsv1beta1.ContainerMetrics{Name: c[0], Usage: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse(c[1]), corev1.ResourceMemory: resource.MustParse(c[2]),
		}})
	}
	return m
}

// nuevoKvizConMetricsServer arranca kviz sobre un cluster falso con el metrics-server dado.
func nuevoKvizConMetricsServer(t *testing.T, ms *metricsServerDePrueba, objetos ...runtime.Object) *kvizDePrueba {
	t.Helper()
	kc := nuevoKubeconfig("kind-lab")
	cluster := kc.crearCluster("kind-lab", objetos...)
	cluster.metricsServer = ms
	k := nuevoKvizConKubeconfig(t, kc)
	k.cliente = cluster.cliente
	k.esperarConectado(t)
	return k
}

func medida(texto string, valor int64) *cantidadEnFoto {
	return &cantidadEnFoto{Texto: texto, Valor: valor}
}

const mi = 1024 * 1024

func TestConMetricsServerCadaContainerTraeSuConsumoContraRequestsYLimits(t *testing.T) {
	backend := podBackend()
	ms := &metricsServerDePrueba{}
	ms.medir(consumo(backend, [3]string{"application", "2873163n", "46080Ki"}))
	kviz := nuevoKvizConMetricsServer(t, ms, nodo("lab-worker", "172.23.0.4"), backend)

	metricas, recursos := kviz.recursos(t)

	want := recursosEnFoto{
		CPU:     recursoEnFoto{Request: medida("500m", 500), Limit: medida("1", 1000), Consumo: medida("3m", 3)},
		Memoria: recursoEnFoto{Request: medida("1Gi", 1024*mi), Limit: medida("2Gi", 2048*mi), Consumo: medida("45Mi", 45*mi)},
	}
	if !metricas || !reflect.DeepEqual(recursos["backend/application"], want) {
		t.Errorf("metricas = %v, recursos = %s\nwant metricas = true y %s", metricas, mostrar(recursos["backend/application"]), mostrar(want))
	}
}

func TestUnContainerSinRequestsNiLimitsSoloTraeSuConsumo(t *testing.T) {
	shell := pod("default", "shell", "lab-worker", "10.244.1.9", corev1.PodRunning)
	shell.Spec.Containers = []corev1.Container{{Name: "shell", Image: "busybox:latest"}}
	ms := &metricsServerDePrueba{}
	ms.medir(consumo(shell, [3]string{"shell", "0", "300Ki"}))
	kviz := nuevoKvizConMetricsServer(t, ms, nodo("lab-worker", "172.23.0.4"), shell)

	_, recursos := kviz.recursos(t)

	want := recursosEnFoto{
		CPU:     recursoEnFoto{Consumo: medida("0m", 0)},
		Memoria: recursoEnFoto{Consumo: medida("300Ki", 300*1024)},
	}
	if !reflect.DeepEqual(recursos["shell/shell"], want) {
		t.Errorf("recursos = %s, want %s", mostrar(recursos["shell/shell"]), mostrar(want))
	}
}

// Un cluster de kind recién creado no trae metrics-server.
func TestSinMetricsServerLaFotoDiceQueNoHayMetricasYSeSugiereInstalarlo(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), podBackend())

	metricas, recursos := kviz.recursos(t)

	want := recursosEnFoto{
		CPU:     recursoEnFoto{Request: medida("500m", 500), Limit: medida("1", 1000)},
		Memoria: recursoEnFoto{Request: medida("1Gi", 1024*mi), Limit: medida("2Gi", 2048*mi)},
	}
	if metricas || !reflect.DeepEqual(recursos["backend/application"], want) {
		t.Errorf("metricas = %v, recursos = %s\nwant sin métricas: requests y limits sin consumo %s", metricas, mostrar(recursos["backend/application"]), mostrar(want))
	}
	mismosComandos(t, kviz.comandos(t, "metrics-server", "", ""), []string{
		"kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml --context kind-lab",
		`kubectl patch deployment metrics-server --type=json -p '[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]' -n kube-system --context kind-lab`,
		"kubectl rollout status deployment/metrics-server -n kube-system --context kind-lab",
	})
}

func TestCuandoSeInstalaMetricsServerElConsumoLlegaEnVivo(t *testing.T) {
	backend := podBackend()
	ms := &metricsServerDePrueba{}
	kviz := nuevoKvizConMetricsServer(t, ms, nodo("lab-worker", "172.23.0.4"), backend)
	stream := kviz.abrirStream(t)
	if m := stream.siguiente(t); m.Foto == nil {
		t.Fatalf("primer mensaje = %s, want la foto", mostrar(m))
	}

	ms.medir(consumo(backend, [3]string{"application", "250m", "512Mi"}))

	var p struct {
		Containers []struct {
			Recursos recursosEnFoto `json:"recursos"`
		} `json:"containers"`
	}
	c := stream.esperarCambio(t, "pods", "default/backend")
	if err := json.Unmarshal(c.Valor, &p); err != nil || len(p.Containers) != 1 {
		t.Fatalf("cambio = %s (%v), want el pod backend con su container", mostrar(c), err)
	}
	r := p.Containers[0].Recursos
	if !reflect.DeepEqual(r.CPU.Consumo, medida("250m", 250)) || !reflect.DeepEqual(r.Memoria.Consumo, medida("512Mi", 512*mi)) {
		t.Errorf("recursos = %s, want consumo 250m y 512Mi", mostrar(r))
	}
	if metricas, _ := kviz.recursos(t); !metricas {
		t.Errorf("metricas = false, want true: ya hay metrics-server")
	}
}
