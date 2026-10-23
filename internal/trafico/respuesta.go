package trafico

// Qué pod respondió, solo cuando kviz lo puede saber: porque el body trae el nombre de un pod de
// los endpoints, o porque el log del Ingress Controller dice a qué endpoint mandó la request. Con
// el pod se suman el salto entre nodos (si el pod está en otro nodo que el de entrada: el de la
// puerta, el del Ingress Controller o, si no, el del pod de sondeo) y el paso pod.

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/fardenghi/kviz/internal/foto"
)

// HeaderDeRequest es el header con el que kviz encuentra la request en el log de ingress-nginx:
// el Ingress Controller usa su valor como $req_id, el último campo de cada línea del log.
const HeaderDeRequest = "X-Request-ID"

// Respuesta es lo que kviz sabe de una respuesta para saber qué pod la dio.
type Respuesta struct {
	Body string
	// Upstream es el endpoint (`ip:puerto`) al que el Ingress Controller mandó la request, según
	// su log; "" si no pasó por él o la línea no apareció.
	Upstream string
}

// PodQueRespondio es el pod de los endpoints del recorrido que dio la respuesta, si kviz lo puede
// saber: por el log del Ingress Controller (el upstream) o porque el body trae su nombre. Devuelve
// también cómo se supo, para el detalle del paso; nil si no se sabe.
func PodQueRespondio(f foto.Foto, r Recorrido, resp Respuesta) (*foto.Pod, string) {
	if p := podDelUpstream(f, r, resp.Upstream); p != nil {
		return p, "Conocido: el log del Ingress Controller dice que la mandó a " + resp.Upstream
	}
	var elegido *foto.Pod
	for _, p := range podsDeLosEndpoints(f, r) {
		// el nombre más largo gana: un pod no se confunde con otro cuyo nombre es un prefijo del suyo
		if strings.Contains(resp.Body, p.Nombre) && (elegido == nil || len(p.Nombre) > len(elegido.Nombre)) {
			elegido = p
		}
	}
	if elegido == nil {
		return nil, ""
	}
	return elegido, "Conocido: el body de la respuesta trae el nombre del pod"
}

// podDelUpstream es el pod de los endpoints del recorrido con la IP del upstream; nil si no es ninguno.
func podDelUpstream(f foto.Foto, r Recorrido, upstream string) *foto.Pod {
	ip, _, err := net.SplitHostPort(upstream)
	if err != nil {
		return nil
	}
	for _, p := range podsDeLosEndpoints(f, r) {
		if p.IP != nil && *p.IP == ip {
			return p
		}
	}
	return nil
}

// ConRespuesta completa el recorrido con la respuesta: si se sabe qué pod la dio, suma el salto
// entre nodos (si corresponde) y el pod. Si no, el recorrido termina en los endpoints posibles.
func ConRespuesta(f foto.Foto, r Recorrido, resp Respuesta) Recorrido {
	p, como := PodQueRespondio(f, r, resp)
	if p == nil {
		return r
	}
	return conPod(f, r, p, como)
}

func conPod(f foto.Foto, r Recorrido, p *foto.Pod, como string) Recorrido {
	r = r.conSalto(f, p)
	ip := "sin IP"
	if p.IP != nil {
		ip = *p.IP
	}
	return r.con(Paso{Tipo: PasoPod, Titulo: p.Nombre + " " + ip, Detalle: como, Objetos: []Ref{refPod(p)}})
}

// conSalto suma el salto entre nodos si el pod corre en otro nodo que el de entrada.
func (r Recorrido) conSalto(f foto.Foto, p *foto.Pod) Recorrido {
	desde, delSondeo := nodoDeEntrada(f, r)
	if desde == nil || p.Nodo == nil || *p.Nodo == desde.Nombre {
		return r
	}
	hasta := nodoPorNombre(f, *p.Nodo)
	if hasta == nil {
		return r
	}
	sale := "del nodo " + desde.InternalIP
	if delSondeo {
		sale = fmt.Sprintf("del nodo del pod de sondeo (%s)", desde.InternalIP)
	}
	return r.con(Paso{
		Tipo:    PasoSalto,
		Titulo:  desde.Nombre + " → " + hasta.Nombre,
		Detalle: fmt.Sprintf("El paquete sale %s hacia %s, donde corre el pod", sale, hasta.InternalIP),
		Objetos: []Ref{refNodo(desde.Nombre), refNodo(hasta.Nombre)},
	})
}

// nodoDeEntrada es el nodo desde el que sale la request hacia el pod: el del Ingress Controller
// que la mandó, o el de la puerta por la que entró; si no pasó por ninguno (un Service por nombre o
// ClusterIP, la IP de un pod), el del pod de sondeo, y entonces delSondeo es true. nil si no se
// sabe (varios pods del Ingress Controller, por ejemplo).
func nodoDeEntrada(f foto.Foto, r Recorrido) (nodo *foto.Nodo, delSondeo bool) {
	directo := true
	for _, paso := range r.Pasos {
		switch paso.Tipo {
		case PasoPuerta:
			directo = false
			for _, o := range paso.Objetos {
				if pu := puertaPorID(f, o.ID); o.Tipo == RefPuerta && pu != nil {
					nodo = nodoPorNombre(f, pu.Nodo)
				}
			}
		case PasoIngress:
			directo = false
			if ics := PodsDelIngressController(Recorrido{Pasos: []Paso{paso}}); len(ics) == 1 {
				if p := podDe(f, ics[0].ID); p != nil && p.Nodo != nil {
					nodo = nodoPorNombre(f, *p.Nodo)
				}
			}
		}
	}
	if directo && r.sondeo != "" {
		return nodoPorNombre(f, r.sondeo), true
	}
	return nodo, false
}

// PodsDelIngressController son los pods del Ingress Controller por los que pasa el recorrido: los
// que marca el paso de la regla de Ingress.
func PodsDelIngressController(r Recorrido) []Ref {
	out := []Ref{}
	for _, paso := range r.Pasos {
		if paso.Tipo != PasoIngress {
			continue
		}
		for _, o := range paso.Objetos {
			if o.Tipo == RefPod {
				out = append(out, o)
			}
		}
	}
	return out
}

// upstreamDelLog toma, de una línea de log de ingress-nginx (log-format-upstream por defecto), lo
// que sigue a `[upstream] [alternativo]`: $upstream_addr, una lista si hubo reintentos.
var upstreamDelLog = regexp.MustCompile(`\[[^\]]*\] \[[^\]]*\] ((?:\S+:\d+)(?:, \S+:\d+)*) `)

// UpstreamDelLog busca en las líneas del log del Ingress Controller la de la request con ese
// X-Request-ID y devuelve el endpoint (`ip:puerto`) que respondió: el último, si hubo reintentos.
// "" si la línea no está.
func UpstreamDelLog(lineas []string, idDeRequest string) string {
	for i := len(lineas) - 1; i >= 0; i-- {
		campos := strings.Fields(lineas[i])
		if len(campos) == 0 || campos[len(campos)-1] != idDeRequest {
			continue
		}
		m := upstreamDelLog.FindStringSubmatch(lineas[i])
		if m == nil {
			return ""
		}
		todos := strings.Split(m[1], ", ")
		return todos[len(todos)-1]
	}
	return ""
}

func podsDeLosEndpoints(f foto.Foto, r Recorrido) []*foto.Pod {
	var out []*foto.Pod
	for _, paso := range r.Pasos {
		if paso.Tipo != PasoEndpoints {
			continue
		}
		for _, o := range paso.Objetos {
			if p := podDe(f, o.ID); p != nil {
				out = append(out, p)
			}
		}
	}
	return out
}

func puertaPorID(f foto.Foto, id string) *foto.Puerta {
	for i := range f.Puertas {
		if f.Puertas[i].ID == id {
			return &f.Puertas[i]
		}
	}
	return nil
}
