package foto

// Recursos de un container (módulo 4 de la spec): requests y limits de CPU y memoria, y el
// consumo real que mide metrics-server, si el cluster lo tiene (Foto.Metricas).

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
)

// Recursos son la CPU y la memoria de un container.
type Recursos struct {
	CPU     Recurso `json:"cpu"`
	Memoria Recurso `json:"memoria"`
}

// Recurso es lo que el container pide (request), su tope (limit) y lo que usa (consumo). Cada uno
// es nil si no está: sin request o sin limit, o sin consumo medido (no hay metrics-server, o
// todavía no midió ese container).
type Recurso struct {
	Request *Cantidad `json:"request"`
	Limit   *Cantidad `json:"limit"`
	Consumo *Cantidad `json:"consumo"`
}

// Cantidad es una cantidad de CPU o de memoria.
type Cantidad struct {
	// Texto es como la escribe Kubernetes ("500m", "1Gi"); el consumo, en milicores o en Mi.
	Texto string `json:"texto"`
	// Valor es en milicores (CPU) o en bytes (memoria), para comparar consumo, request y limit.
	Valor int64 `json:"valor"`
}

// consumoPorPod indexa lo que midió metrics-server por pod ("<namespace>/<nombre>") y container.
func consumoPorPod(pods []metricsv1beta1.PodMetrics) map[string]map[string]corev1.ResourceList {
	out := map[string]map[string]corev1.ResourceList{}
	for _, p := range pods {
		porContainer := map[string]corev1.ResourceList{}
		for _, c := range p.Containers {
			porContainer[c.Name] = c.Usage
		}
		out[p.Namespace+"/"+p.Name] = porContainer
	}
	return out
}

func recursosDe(spec corev1.ResourceRequirements, consumo corev1.ResourceList) Recursos {
	recurso := func(nombre corev1.ResourceName, valor func(*resource.Quantity) int64, texto func(int64) string) Recurso {
		var r Recurso
		if q, ok := spec.Requests[nombre]; ok {
			r.Request = &Cantidad{Texto: q.String(), Valor: valor(&q)}
		}
		if q, ok := spec.Limits[nombre]; ok {
			r.Limit = &Cantidad{Texto: q.String(), Valor: valor(&q)}
		}
		if q, ok := consumo[nombre]; ok {
			v := valor(&q)
			r.Consumo = &Cantidad{Texto: texto(v), Valor: v}
		}
		return r
	}
	return Recursos{
		CPU:     recurso(corev1.ResourceCPU, (*resource.Quantity).MilliValue, func(m int64) string { return fmt.Sprintf("%dm", m) }),
		Memoria: recurso(corev1.ResourceMemory, (*resource.Quantity).Value, textoDeMemoria),
	}
}

// textoDeMemoria escribe bytes en Mi (o en Ki si es menos de 1 Mi), redondeado.
func textoDeMemoria(b int64) string {
	const ki, mi = 1024, 1024 * 1024
	if b < mi {
		return fmt.Sprintf("%dKi", (b+ki/2)/ki)
	}
	return fmt.Sprintf("%dMi", (b+mi/2)/mi)
}
