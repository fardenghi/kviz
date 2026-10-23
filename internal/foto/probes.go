package foto

// Estado de probe (módulo 3 de la spec): Kubernetes no guarda el resultado de cada ejecución de
// una probe, así que kviz lo deduce de señales indirectas: Ready para la readiness, Started para
// la startup, y los reinicios y los events Unhealthy/Killing para la liveness.

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
)

// Estados de probe.
const (
	ProbeOk        = "ok"
	ProbeFallando  = "fallando"
	ProbePendiente = "pendiente"
	SinProbe       = "sin probe"
)

// Probe es una probe de un container con su estado deducido.
type Probe struct {
	// Nombre es liveness, readiness o startup.
	Nombre string `json:"nombre"`
	// Estado es ok, fallando, pendiente o "sin probe".
	Estado string `json:"estado"`
	// Config es nil si el container no tiene esa probe.
	Config *ConfigDeProbe `json:"config"`
	// Senales son las señales de las que se dedujo el estado, en español ("Ready: false", …).
	Senales []string `json:"senales"`
	// Fallos son los últimos events de fallo de la probe, del más nuevo al más viejo.
	Fallos []FalloDeProbe `json:"fallos"`
}

// ConfigDeProbe es la configuración de una probe, con los valores por defecto de Kubernetes.
type ConfigDeProbe struct {
	// Tipo es httpGet, tcpSocket, exec o grpc.
	Tipo string `json:"tipo"`
	Path string `json:"path,omitempty"`
	// Puerto es un número o el nombre de un puerto del container.
	Puerto  string   `json:"puerto,omitempty"`
	Host    string   `json:"host,omitempty"`
	Esquema string   `json:"esquema,omitempty"`
	Comando []string `json:"comando,omitempty"`

	InitialDelaySeconds int32 `json:"initialDelaySeconds"`
	PeriodSeconds       int32 `json:"periodSeconds"`
	TimeoutSeconds      int32 `json:"timeoutSeconds"`
	SuccessThreshold    int32 `json:"successThreshold"`
	FailureThreshold    int32 `json:"failureThreshold"`
}

// FalloDeProbe es un event Unhealthy o Killing de la probe.
type FalloDeProbe struct {
	// Motivo es la razón del event: Unhealthy (la probe falló) o Killing (el kubelet reinició el
	// container porque la probe falló).
	Motivo  string `json:"motivo"`
	Mensaje string `json:"mensaje"`
	// Veces es cuántas veces se repitió el event.
	Veces int32 `json:"veces"`
	// Cuando es la última vez, en RFC 3339 (UTC); "" si no se sabe.
	Cuando string `json:"cuando"`
}

// ultimosFallos es cuántos fallos de cada probe trae la foto.
const ultimosFallos = 5

// probesDe deduce el estado de la liveness, readiness y startup del container (en ese orden), con
// los events del pod (ver eventosDeProbes).
func probesDe(spec *corev1.Container, c Container, estado *corev1.ContainerStatus, eventos []corev1.Event) []Probe {
	s := senalesDe(c, estado, eventos)
	return []Probe{
		deducir("liveness", spec.LivenessProbe, s, func(p *Probe, f fallos) {
			switch {
			case !s.corriendo && c.Reinicios > 0 && f.reinicios > 0:
				p.Estado = ProbeFallando
				p.Senales = append(p.Senales, noCorre(c))
			case !s.corriendo:
				p.Estado = ProbePendiente
				p.Senales = append(p.Senales, noCorre(c))
			case !c.Started:
				p.Estado = ProbePendiente
				p.Senales = append(p.Senales, "Started: false (la liveness corre recién cuando pasa la startup)")
			case f.desdeQueArranco > 0:
				p.Estado = ProbeFallando
				p.Senales = append(p.Senales, f.texto())
			default:
				p.Estado = ProbeOk
				p.Senales = append(p.Senales, sinFallos)
			}
			if c.Reinicios > 0 {
				p.Senales = append(p.Senales, reinicios(c.Reinicios, f.reinicios, "liveness"))
			}
		}),
		deducir("readiness", spec.ReadinessProbe, s, func(p *Probe, f fallos) {
			p.Senales = append(p.Senales, "Ready: "+strconv.FormatBool(c.Ready))
			switch {
			case c.Ready:
				p.Estado = ProbeOk
			case !s.corriendo:
				p.Estado = ProbePendiente
				p.Senales = append(p.Senales, noCorre(c))
			case !c.Started:
				p.Estado = ProbePendiente
				p.Senales = append(p.Senales, "Started: false (la readiness corre recién cuando pasa la startup)")
			case f.desdeQueArranco > 0:
				p.Estado = ProbeFallando
				p.Senales = append(p.Senales, f.texto())
			default:
				p.Estado = ProbePendiente
				p.Senales = append(p.Senales, todaviaNoPaso(p.Config))
			}
		}),
		deducir("startup", spec.StartupProbe, s, func(p *Probe, f fallos) {
			p.Senales = append(p.Senales, "Started: "+strconv.FormatBool(c.Started))
			switch {
			case c.Started:
				p.Estado = ProbeOk
			case !s.corriendo && c.Reinicios > 0 && f.reinicios > 0:
				p.Estado = ProbeFallando
				p.Senales = append(p.Senales, noCorre(c), reinicios(c.Reinicios, f.reinicios, "startup"))
			case !s.corriendo:
				p.Estado = ProbePendiente
				p.Senales = append(p.Senales, noCorre(c))
			case f.desdeQueArranco > 0:
				p.Estado = ProbeFallando
				p.Senales = append(p.Senales, f.texto())
			default:
				p.Estado = ProbePendiente
				p.Senales = append(p.Senales, todaviaNoPaso(p.Config))
			}
		}),
	}
}

// senales son las señales de un container de las que se deducen sus probes.
type senales struct {
	corriendo bool
	// arranco es cuándo arrancó la ejecución actual (o la última) del container; cero si no se sabe.
	arranco time.Time
	// eventos son los events Unhealthy y Killing del container.
	eventos []corev1.Event
}

func senalesDe(c Container, estado *corev1.ContainerStatus, eventos []corev1.Event) senales {
	s := senales{}
	if estado != nil {
		switch st := estado.State; {
		case st.Running != nil:
			s.corriendo, s.arranco = true, st.Running.StartedAt.Time
		case st.Terminated != nil:
			s.arranco = st.Terminated.StartedAt.Time
		case estado.LastTerminationState.Terminated != nil:
			s.arranco = estado.LastTerminationState.Terminated.StartedAt.Time
		}
	}
	for _, e := range eventos {
		if strings.HasSuffix(e.InvolvedObject.FieldPath, "{"+c.Nombre+"}") {
			s.eventos = append(s.eventos, e)
		}
	}
	return s
}

// fallos resume los events de fallo de una probe.
type fallos struct {
	// desdeQueArranco son los fallos (events Unhealthy, contando sus repeticiones) desde que
	// arrancó el container.
	desdeQueArranco int32
	// reinicios son los reinicios que hizo el kubelet porque la probe falló (events Killing).
	reinicios int32
}

func (f fallos) texto() string {
	return fmt.Sprintf("%d %s (events Unhealthy) desde que arrancó el container", f.desdeQueArranco, plural(f.desdeQueArranco, "fallo", "fallos"))
}

const sinFallos = "sin fallos (events Unhealthy) desde que arrancó el container"

func todaviaNoPaso(c *ConfigDeProbe) string {
	return fmt.Sprintf("sin fallos desde que arrancó el container: todavía no pasó ninguna vez (initialDelaySeconds %d)", c.InitialDelaySeconds)
}

func reinicios(total, porLaProbe int32, probe string) string {
	t := fmt.Sprintf("%d %s", total, plural(total, "reinicio", "reinicios"))
	if porLaProbe > 0 {
		t += fmt.Sprintf(", %d por la %s (events Killing)", porLaProbe, probe)
	}
	return t
}

func plural(n int32, uno, varios string) string {
	if n == 1 {
		return uno
	}
	return varios
}

// deducir arma la probe: "sin probe" si no está configurada, y si no, su config, sus últimos
// fallos y el estado que calcula estado.
func deducir(nombre string, spec *corev1.Probe, s senales, estado func(*Probe, fallos)) Probe {
	p := Probe{Nombre: nombre, Estado: SinProbe, Senales: []string{}, Fallos: []FalloDeProbe{}}
	if spec == nil {
		return p
	}
	p.Config = configDe(spec)

	var f fallos
	var deLaProbe []corev1.Event
	for _, e := range s.eventos {
		mensaje := strings.ToLower(e.Message)
		switch {
		case e.Reason == "Unhealthy" && strings.HasPrefix(mensaje, nombre+" probe"):
			if !cuandoFue(e).Before(s.arranco) {
				f.desdeQueArranco += veces(e)
			}
		case e.Reason == "Killing" && strings.Contains(mensaje, "failed "+nombre+" probe"):
			f.reinicios += veces(e)
		default:
			continue
		}
		deLaProbe = append(deLaProbe, e)
	}
	sort.SliceStable(deLaProbe, func(i, j int) bool { return cuandoFue(deLaProbe[i]).After(cuandoFue(deLaProbe[j])) })
	for i, e := range deLaProbe {
		if i == ultimosFallos {
			break
		}
		p.Fallos = append(p.Fallos, FalloDeProbe{Motivo: e.Reason, Mensaje: e.Message, Veces: veces(e), Cuando: ultimaVez(e)})
	}
	estado(&p, f)
	return p
}

// eventosDeProbes agrupa por pod ("<namespace>/<nombre>") los events Unhealthy y Killing de pods,
// que son los que deja el kubelet cuando una probe falla y cuando reinicia un container. Un
// event de otro pod con el mismo nombre (otro UID) no cuenta.
func eventosDeProbes(eventos []corev1.Event, pods []*corev1.Pod) map[string][]corev1.Event {
	uid := map[string]string{}
	for _, p := range pods {
		uid[p.Namespace+"/"+p.Name] = string(p.UID)
	}
	out := map[string][]corev1.Event{}
	for _, e := range eventos {
		o := e.InvolvedObject
		if o.Kind != "Pod" || (e.Reason != "Unhealthy" && e.Reason != "Killing") {
			continue
		}
		clave := o.Namespace + "/" + o.Name
		if u := uid[clave]; u != "" && o.UID != "" && string(o.UID) != u {
			continue
		}
		out[clave] = append(out[clave], e)
	}
	return out
}

func noCorre(c Container) string {
	return "el container no está corriendo (" + c.Estado + ")"
}

func configDe(p *corev1.Probe) *ConfigDeProbe {
	c := &ConfigDeProbe{
		InitialDelaySeconds: p.InitialDelaySeconds,
		PeriodSeconds:       porDefecto(p.PeriodSeconds, 10),
		TimeoutSeconds:      porDefecto(p.TimeoutSeconds, 1),
		SuccessThreshold:    porDefecto(p.SuccessThreshold, 1),
		FailureThreshold:    porDefecto(p.FailureThreshold, 3),
	}
	switch h := p.ProbeHandler; {
	case h.HTTPGet != nil:
		c.Tipo, c.Path, c.Puerto, c.Host = "httpGet", h.HTTPGet.Path, h.HTTPGet.Port.String(), h.HTTPGet.Host
		c.Esquema = string(h.HTTPGet.Scheme)
		if c.Esquema == "" {
			c.Esquema = string(corev1.URISchemeHTTP)
		}
		if c.Path == "" {
			c.Path = "/"
		}
	case h.TCPSocket != nil:
		c.Tipo, c.Puerto, c.Host = "tcpSocket", h.TCPSocket.Port.String(), h.TCPSocket.Host
	case h.Exec != nil:
		c.Tipo, c.Comando = "exec", h.Exec.Command
	case h.GRPC != nil:
		c.Tipo = "grpc"
		c.Puerto = strconv.Itoa(int(h.GRPC.Port))
	}
	return c
}

func porDefecto(v, d int32) int32 {
	if v == 0 {
		return d
	}
	return v
}
