package foto

import (
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// HostPort es un puerto de un container que se abre en el nodo del pod.
type HostPort struct {
	// Puerto es el hostPort: el número que se abre en el nodo.
	Puerto        int32  `json:"puerto"`
	Protocolo     string `json:"protocolo"`
	Container     string `json:"container"`
	ContainerPort int32  `json:"containerPort"`
}

// Puerta es un puerto abierto en un nodo hacia el cluster: un NodePort, que se
// abre en todos los nodos, o un hostPort, que se abre solo en el nodo de su pod.
type Puerta struct {
	// ID es "<nodo>/<puerto>/<protocolo>": en un nodo, un puerto y protocolo abre una sola puerta.
	ID        string `json:"id"`
	Nodo      string `json:"nodo"`
	Puerto    int32  `json:"puerto"`
	Protocolo string `json:"protocolo"`
	// Tipo es "NodePort" o "hostPort".
	Tipo string `json:"tipo"`
	// Namespace del Service o del pod al que lleva la puerta.
	Namespace string `json:"namespace"`
	// Service es el Service al que lleva una puerta NodePort; nil en las de hostPort.
	Service *string `json:"service"`
	// Pod es el pod al que lleva una puerta de hostPort; nil en las NodePort.
	Pod *string `json:"pod"`
	// HaciaPuerto es el puerto del Service (NodePort) o del container (hostPort) al que llega.
	HaciaPuerto int32 `json:"haciaPuerto"`
	// IngressController marca un hostPort del Ingress Controller: la entrada al cluster, que la
	// vista muestra aunque los namespaces del sistema estén ocultos. Los demás hostPorts del
	// sistema (por ejemplo los de los pods estáticos con hostNetwork) se ocultan con su namespace.
	IngressController bool `json:"ingressController"`
}

// esIngressController reconoce el pod de ingress-nginx por sus etiquetas o por la imagen de alguno
// de sus containers.
func esIngressController(labels map[string]string, imagenes []string) bool {
	if labels["app.kubernetes.io/name"] == "ingress-nginx" && labels["app.kubernetes.io/component"] == "controller" {
		return true
	}
	for _, i := range imagenes {
		if strings.Contains(i, "ingress-nginx/controller") {
			return true
		}
	}
	return false
}

// EsIngressController dice si el pod es el Ingress Controller (ingress-nginx).
func (p Pod) EsIngressController() bool {
	imagenes := make([]string, 0, len(p.Containers))
	for _, c := range p.Containers {
		imagenes = append(imagenes, c.Imagen)
	}
	return esIngressController(p.Labels, imagenes)
}

const (
	PuertaNodePort = "NodePort"
	PuertaHostPort = "hostPort"
)

// hostPorts junta los hostPorts de los containers (y de los init containers, que pueden ser
// sidecars) de un pod.
func hostPorts(p *corev1.Pod) []HostPort {
	out := []HostPort{}
	for _, c := range append(append([]corev1.Container{}, p.Spec.InitContainers...), p.Spec.Containers...) {
		for _, cp := range c.Ports {
			if cp.HostPort == 0 {
				continue
			}
			protocolo := string(cp.Protocol)
			if protocolo == "" {
				protocolo = string(corev1.ProtocolTCP)
			}
			out = append(out, HostPort{Puerto: cp.HostPort, Protocolo: protocolo, Container: c.Name, ContainerPort: cp.ContainerPort})
		}
	}
	return out
}

// puertas deriva las puertas: cada nodePort abre una puerta en cada nodo, y cada hostPort una
// puerta solo en el nodo de su pod, mientras el pod no terminó. Quedan ordenadas por id; si dos
// chocan en el mismo nodo, puerto y protocolo (un cluster mal armado), queda la NodePort, o el
// pod de menor id.
func puertas(nodos []Nodo, services []Service, o Objetos) []Puerta {
	out := []Puerta{}
	for _, s := range services {
		for _, p := range s.Puertos {
			if p.NodePort == 0 {
				continue
			}
			for _, n := range nodos {
				out = append(out, Puerta{Nodo: n.Nombre, Puerto: p.NodePort, Protocolo: p.Protocolo, Tipo: PuertaNodePort,
					Namespace: s.Namespace, Service: opcional(s.Nombre), HaciaPuerto: p.Puerto})
			}
		}
	}
	for _, p := range o.Pods {
		if p.Spec.NodeName == "" || p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed {
			continue
		}
		imagenes := []string{}
		for _, c := range append(append([]corev1.Container{}, p.Spec.InitContainers...), p.Spec.Containers...) {
			imagenes = append(imagenes, c.Image)
		}
		ic := esIngressController(p.Labels, imagenes)
		for _, hp := range hostPorts(p) {
			out = append(out, Puerta{Nodo: p.Spec.NodeName, Puerto: hp.Puerto, Protocolo: hp.Protocolo, Tipo: PuertaHostPort,
				Namespace: p.Namespace, Pod: opcional(p.Name), HaciaPuerto: hp.ContainerPort, IngressController: ic})
		}
	}
	for i := range out {
		out[i].ID = fmt.Sprintf("%s/%d/%s", out[i].Nodo, out[i].Puerto, out[i].Protocolo)
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		if a.Tipo != b.Tipo {
			return a.Tipo == PuertaNodePort
		}
		return a.Namespace+"/"+deref(a.Pod) < b.Namespace+"/"+deref(b.Pod)
	})
	unicas := []Puerta{}
	for _, p := range out {
		if len(unicas) == 0 || unicas[len(unicas)-1].ID != p.ID {
			unicas = append(unicas, p)
		}
	}
	return unicas
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
