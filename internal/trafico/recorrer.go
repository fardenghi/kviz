package trafico

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/fardenghi/kviz/internal/foto"
)

// Opcion ajusta el cálculo del recorrido con lo que trae el pedido además de la URL.
type Opcion func(*opciones)

type opciones struct {
	// host es el header Host de la request: elige la regla de Ingress.
	host string
	// sondeo es el nodo del pod de sondeo.
	sondeo string
}

// ConHost dice el header Host que lleva la request, si el pedido lo pone; si no, vale el host de
// la URL. El Ingress Controller elige la regla por ese header.
func ConHost(host string) Opcion {
	return func(o *opciones) {
		if host = strings.TrimSpace(host); host != "" {
			o.host = host
		}
	}
}

// DesdeNodo dice en qué nodo corre el pod de sondeo del que sale la request: es el nodo de
// entrada cuando la request no entra por una puerta ni por el Ingress Controller (un Service por
// nombre o ClusterIP, la IP de un pod), y de ahí sale el salto hacia el nodo del pod.
func DesdeNodo(nodo string) Opcion {
	return func(o *opciones) { o.sondeo = nodo }
}

// Recorrer calcula el recorrido de una request a la URL según la foto. El destino puede ser:
//   - un Service, por nombre DNS o ClusterIP: [DNS →] Service → endpoints;
//   - un nodo, por nombre o InternalIP, y un puerto: [DNS →] puerta → … lo que haya detrás de la
//     puerta: el Service de un NodePort, o el pod de un hostPort; si ese pod es el Ingress
//     Controller (una ruta de Ingress), → regla de Ingress → Service → endpoints;
//   - la IP de un pod: pod.
//
// Fuera de la foto, el recorrido no tiene pasos.
func Recorrer(f foto.Foto, u *url.URL, ops ...Opcion) Recorrido {
	host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
	o := opciones{host: host}
	for _, op := range ops {
		op(&o)
	}
	c := calculo{f: f, http: u.Scheme != "tcp", host: o.host, path: u.EscapedPath(), r: Recorrido{Pasos: []Paso{}, sondeo: o.sondeo}}
	if c.path == "" {
		c.path = "/"
	}
	puerto := puertoDe(u)
	esIP := net.ParseIP(host) != nil
	if s := serviceDe(f, host); s != nil {
		if !esIP {
			c.dnsDelService(host, s)
		}
		c.desdeService(s, intstr.FromInt32(puerto), nil)
	} else if n := nodoDe(f, host); n != nil {
		if !esIP {
			c.con(Paso{Tipo: PasoDNS, Titulo: host + " → " + n.InternalIP,
				Detalle: "Nombre de un nodo: el DNS lo resuelve a su InternalIP", Objetos: []Ref{refNodo(n.Nombre)}})
		}
		c.desdeNodo(n, puerto)
	} else if p := f.PodPorIP(host); p != nil && esIP {
		c.r = c.r.conSalto(f, p)
		c.con(Paso{Tipo: PasoPod, Titulo: fmt.Sprintf("%s %s:%d", p.Nombre, host, puerto),
			Detalle: "IP de un pod: la request va directo al pod, sin Service ni kube-proxy", Objetos: []Ref{refPod(p)}})
	}
	return c.r
}

// calculo es un recorrido a medio armar, con lo que hace falta para seguirlo.
type calculo struct {
	f foto.Foto
	// http es falso en una prueba de conexión (`tcp://`): sin path ni Host, no hay regla de
	// Ingress, y la conexión termina en el pod del Ingress Controller.
	http bool
	// host y path con los que el Ingress Controller elige la regla.
	host, path string
	r          Recorrido
}

func (c *calculo) con(p Paso) { c.r = c.r.con(p) }

// dnsDelService es el paso DNS de un nombre de Service: la ClusterIP, o las IPs de los pods si
// el Service es headless.
func (c *calculo) dnsDelService(nombre string, s *foto.Service) {
	paso := Paso{Tipo: PasoDNS, Titulo: nombre + " → " + s.ClusterIP,
		Detalle: "CoreDNS resuelve el nombre del Service a su ClusterIP", Objetos: []Ref{refService(s)}}
	if s.ClusterIP == "" || s.ClusterIP == "None" {
		var ips []string
		for _, e := range s.Endpoints {
			if e.Listo {
				ips = append(ips, e.IP)
			}
		}
		paso.Titulo = nombre + " → " + strings.Join(ips, ", ")
		if len(ips) == 0 {
			paso.Titulo = nombre + " → sin IPs"
		}
		paso.Detalle = "Service headless: CoreDNS devuelve directo las IPs de los pods listos"
	}
	c.con(paso)
}

// desdeNodo sigue una request que entra por un puerto de un nodo: la puerta que abre ese puerto,
// si hay, y lo que hay detrás.
func (c *calculo) desdeNodo(n *foto.Nodo, puerto int32) {
	pu := puertaDe(c.f, n.Nombre, puerto)
	if pu == nil {
		c.con(Paso{Tipo: PasoPuerta, Titulo: fmt.Sprintf("%s :%d", n.Nombre, puerto),
			Detalle: fmt.Sprintf("Ningún NodePort ni hostPort abre el puerto %d en este nodo: la request no llega a ningún pod", puerto),
			Objetos: []Ref{refNodo(n.Nombre)}})
		return
	}
	paso := Paso{Tipo: PasoPuerta, Titulo: fmt.Sprintf("%s :%d (%s)", n.Nombre, puerto, pu.Tipo), Objetos: []Ref{refPuerta(pu)}}
	if pu.Service != nil {
		s := c.f.ServicePorID(foto.IDDe(pu.Namespace, *pu.Service))
		paso.Detalle = fmt.Sprintf("kube-proxy del nodo la manda al Service %s", *pu.Service)
		c.con(paso)
		if s != nil {
			c.desdeService(s, intstr.FromInt32(pu.HaciaPuerto), nil)
		}
		return
	}
	p := c.f.PodPorID(foto.IDDe(pu.Namespace, deref(pu.Pod)))
	if p == nil {
		paso.Detalle = "El pod del hostPort ya no está"
		c.con(paso)
		return
	}
	if c.http && p.EsIngressController() {
		paso.Detalle = "Entra al Ingress Controller " + p.Nombre
		c.con(paso)
		c.desdeIngressController([]*foto.Pod{p})
		return
	}
	paso.Detalle = fmt.Sprintf("Entra directo al pod %s (hostPort %d → :%d)", p.Nombre, puerto, pu.HaciaPuerto)
	c.con(paso)
	c.con(Paso{Tipo: PasoPod, Titulo: fmt.Sprintf("%s %s:%d", p.Nombre, deref(p.IP), pu.HaciaPuerto),
		Detalle: "Conocido: un hostPort lleva a un solo pod", Objetos: []Ref{refPod(p)}})
}

// desdeService sigue una request que llega a un puerto (número o nombre) del Service: Service →
// endpoints listos. ics son los pods del Ingress Controller que la mandan, o nil si llega por
// kube-proxy. Si los endpoints son el Ingress Controller, sigue por la regla de Ingress.
func (c *calculo) desdeService(s *foto.Service, puerto intstr.IntOrString, ics []*foto.Pod) {
	ref := refService(s)
	p := puertoDelService(s, puerto)
	if p == nil {
		c.con(Paso{
			Tipo:    PasoService,
			Titulo:  fmt.Sprintf("%s %s:%s", s.Nombre, s.ClusterIP, puerto.String()),
			Detalle: fmt.Sprintf("el Service no expone el puerto %s: la request no llega a ningún pod", puerto.String()),
			Objetos: []Ref{ref},
		})
		return
	}
	quien := "kube-proxy"
	paso := Paso{
		Tipo:    PasoService,
		Titulo:  fmt.Sprintf("%s %s:%d → :%s", s.Nombre, s.ClusterIP, p.Puerto, p.TargetPort.String()),
		Detalle: fmt.Sprintf("%s, targetPort %s", s.Tipo, p.TargetPort.String()),
		Objetos: []Ref{ref},
	}
	if ics != nil {
		quien = "el Ingress Controller"
		paso.Detalle += "; el Ingress Controller manda directo a los endpoints"
	}
	c.con(paso)

	var direcciones []string
	pods := []Ref{}
	var listos []*foto.Pod
	noListos := 0
	for _, e := range s.Endpoints {
		if !e.Listo {
			noListos++
			continue
		}
		direcciones = append(direcciones, e.IP+":"+p.TargetPort.String())
		if e.Pod != nil {
			id := foto.IDDe(s.Namespace, *e.Pod)
			pods = append(pods, Ref{Tipo: RefPod, ID: id})
			if pod := c.f.PodPorID(id); pod != nil {
				listos = append(listos, pod)
			}
		}
	}
	endpoints := Paso{Tipo: PasoEndpoints, Titulo: strings.Join(direcciones, ", "), Objetos: pods}
	switch {
	case len(direcciones) == 0:
		endpoints.Titulo = "sin endpoints listos"
		endpoints.Detalle = "ningún pod listo coincide con el selector: la request no tiene a dónde ir"
	case len(direcciones) == 1:
		endpoints.Detalle = "1 endpoint listo"
	default:
		endpoints.Detalle = fmt.Sprintf("%d endpoints listos; %s elige uno", len(direcciones), quien)
	}
	if noListos > 0 {
		if noListos == 1 {
			endpoints.Detalle += "; 1 no listo, que no recibe tráfico"
		} else {
			endpoints.Detalle += fmt.Sprintf("; %d no listos, que no reciben tráfico", noListos)
		}
	}
	c.con(endpoints)

	// el Service del Ingress Controller: la request sigue por una regla de Ingress
	if c.http && ics == nil && len(listos) > 0 && len(listos) == len(direcciones) {
		for _, l := range listos {
			if !l.EsIngressController() {
				return
			}
		}
		c.desdeIngressController(listos)
	}
}

// ---------- búsquedas en la foto ----------

// nodoDe busca el nodo por nombre o InternalIP.
func nodoDe(f foto.Foto, host string) *foto.Nodo {
	for i := range f.Nodos {
		n := &f.Nodos[i]
		if host == strings.ToLower(n.Nombre) || (n.InternalIP != "" && host == n.InternalIP) {
			return n
		}
	}
	return nil
}

// puertaDe es la puerta TCP de un nodo en un puerto, si la hay.
func puertaDe(f foto.Foto, nodo string, puerto int32) *foto.Puerta {
	for i := range f.Puertas {
		p := &f.Puertas[i]
		if p.Nodo == nodo && p.Puerto == puerto && (p.Protocolo == "" || p.Protocolo == "TCP") {
			return p
		}
	}
	return nil
}

func refNodo(nombre string) Ref      { return Ref{Tipo: RefNodo, ID: nombre} }
func refPod(p *foto.Pod) Ref         { return Ref{Tipo: RefPod, ID: p.ID} }
func refService(s *foto.Service) Ref { return Ref{Tipo: RefService, ID: s.ID} }
func refIngress(i *foto.Ingress) Ref { return Ref{Tipo: RefIngress, ID: i.ID} }
func refPuerta(p *foto.Puerta) Ref   { return Ref{Tipo: RefPuerta, ID: p.ID} }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
