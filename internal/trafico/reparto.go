package trafico

import (
	"sort"

	"kviz/internal/foto"
)

// Reparto es cómo se repartieron entre los pods las respuestas de una request de prueba repetida
// N veces: el balanceo de carga visto desde afuera.
type Reparto struct {
	// Total es la cantidad de requests que se mandaron.
	Total int `json:"total"`
	// Pods cuenta las respuestas de cada pod identificado, de más a menos (y por id).
	Pods []PorPod `json:"pods"`
	// SinIdentificar son las respuestas de las que no se sabe qué pod las dio.
	SinIdentificar int `json:"sinIdentificar"`
	// SinRespuesta son las requests que no recibieron respuesta.
	SinRespuesta int `json:"sinRespuesta"`
}

// PorPod es la cantidad de respuestas que dio un pod.
type PorPod struct {
	// Pod es el id del pod en la foto (`<namespace>/<nombre>`).
	Pod string `json:"pod"`
	N   int    `json:"n"`
}

// Repartir cuenta las respuestas por pod con la misma identificación que el recorrido
// (PodQueRespondio): cuerpos son los bodies de las respuestas que llegaron y sinRespuesta las
// requests que no recibieron respuesta.
func Repartir(f foto.Foto, r Recorrido, cuerpos []string, sinRespuesta int) Reparto {
	out := Reparto{Total: len(cuerpos) + sinRespuesta, Pods: []PorPod{}, SinRespuesta: sinRespuesta}
	cuenta := map[string]int{}
	for _, b := range cuerpos {
		if p := PodQueRespondio(f, r, b); p != nil {
			cuenta[p.ID]++
		} else {
			out.SinIdentificar++
		}
	}
	for id, n := range cuenta {
		out.Pods = append(out.Pods, PorPod{Pod: id, N: n})
	}
	sort.Slice(out.Pods, func(i, j int) bool {
		a, b := out.Pods[i], out.Pods[j]
		return a.N > b.N || (a.N == b.N && a.Pod < b.Pod)
	})
	return out
}
