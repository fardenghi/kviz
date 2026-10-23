package foto

// Datos de la foto que solo usa el panel de detalle (ticket 08): labels y edad del pod, y
// capacidad del nodo. Las réplicas y la estrategia de los workloads están en rollout.go.

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Capacidad es status.capacity de un nodo, en las unidades de Kubernetes ("8", "16365916Ki",
// "110"); "" si el nodo no la informa.
type Capacidad struct {
	CPU     string `json:"cpu"`
	Memoria string `json:"memoria"`
	Pods    string `json:"pods"`
}

func capacidadDe(n *corev1.Node) Capacidad {
	texto := func(r corev1.ResourceName) string {
		if q, ok := n.Status.Capacity[r]; ok {
			return q.String()
		}
		return ""
	}
	return Capacidad{CPU: texto(corev1.ResourceCPU), Memoria: texto(corev1.ResourceMemory), Pods: texto(corev1.ResourcePods)}
}

func etiquetas(l map[string]string) map[string]string {
	if l == nil {
		return map[string]string{}
	}
	return l
}

func creado(m metav1.ObjectMeta) string {
	if m.CreationTimestamp.IsZero() {
		return ""
	}
	return m.CreationTimestamp.UTC().Format(time.RFC3339)
}
