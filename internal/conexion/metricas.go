package conexion

// Métricas (módulo 4 de la spec): el consumo real de CPU y memoria de cada container sale de
// metrics-server (la API metrics.k8s.io). No se puede observar con un informer, así que la
// conexión lo consulta cada tanto y avisa cuando cambia.

import (
	"context"
	"errors"
	"reflect"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
)

// Metricas es metrics-server de un cluster. En los tests, un doble con o sin metrics-server.
type Metricas interface {
	// Pods devuelve el consumo de los containers de todos los pods. Da error si el cluster no
	// tiene metrics-server (la API metrics.k8s.io no existe o no responde).
	Pods(ctx context.Context) ([]metricsv1beta1.PodMetrics, error)
}

// metricsServer es metrics-server de verdad, por la API metrics.k8s.io.
type metricsServer struct{ cliente metricsclient.Interface }

func metricasDe(config *rest.Config) (Metricas, error) {
	c, err := metricsclient.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return metricsServer{c}, nil
}

func (m metricsServer) Pods(ctx context.Context) ([]metricsv1beta1.PodMetrics, error) {
	l, err := m.cliente.MetricsV1beta1().PodMetricses(metav1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return l.Items, nil
}

// medicion es lo último que se le preguntó a metrics-server.
type medicion struct {
	mu sync.Mutex
	// hay dice si el cluster tiene metrics-server.
	hay  bool
	pods []metricsv1beta1.PodMetrics
}

// medir le pregunta el consumo a metrics-server (sin metrics-server, o si no contesta, no hay
// métricas) y dice si cambió desde la medición anterior.
func (m *medicion) medir(ctx context.Context, fuente Metricas) (cambio bool) {
	var pods []metricsv1beta1.PodMetrics
	err := errors.New("no hay cliente de metrics-server")
	if fuente != nil {
		ctx, cancelar := context.WithTimeout(ctx, esperaRespuesta)
		pods, err = fuente.Pods(ctx)
		cancelar()
	}
	hay := err == nil
	if !hay {
		pods = nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	cambio = hay != m.hay || !reflect.DeepEqual(pods, m.pods)
	m.hay, m.pods = hay, pods
	return cambio
}

func (m *medicion) ultima() (hay bool, pods []metricsv1beta1.PodMetrics) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.hay, m.pods
}

// vigilarMetricas vuelve a medir cada intervalo, y avisa cuando el consumo cambia, hasta que se
// cancela ctx.
func (c *Conexion) vigilarMetricas(ctx context.Context) {
	for esperar(ctx, c.intervalo) {
		if c.medicion.medir(ctx, c.metricas) {
			c.avisos.avisar()
		}
	}
}
