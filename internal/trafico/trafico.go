// Package trafico es la depuración de tráfico (módulo 11 de la spec): a partir del destino de una
// request de prueba y la foto, calcula el recorrido desde la configuración (no rastrea paquetes).
// Es puro: no habla con el cluster ni con el pod de sondeo.
//
// Hoy el recorrido es el básico: Service → endpoints → pod. Para sumar pasos (DNS, puerta, regla
// de Ingress, salto entre nodos) se agregan su tipo de paso y su cálculo en Recorrer; la interfaz
// los dibuja por los objetos que marca cada paso, sin conocer el tipo.
package trafico

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"kviz/internal/foto"
)

// Recorrido es el camino que siguió una request de prueba, en pasos numerados.
type Recorrido struct {
	// Pasos en orden, numerados desde 1. Vacío si el destino no es un objeto de la foto.
	Pasos []Paso `json:"pasos"`
}

// Tipos de paso.
const (
	PasoService   = "service"
	PasoEndpoints = "endpoints"
	PasoPod       = "pod"
)

// Paso es un paso del recorrido.
type Paso struct {
	N int `json:"n"`
	// Tipo: service, endpoints o pod (ver las constantes Paso…).
	Tipo string `json:"tipo"`
	// Titulo es la línea principal del paso: `backend 10.96.11.121:80 → :8081`.
	Titulo string `json:"titulo"`
	// Detalle explica el paso en español.
	Detalle string `json:"detalle"`
	// Objetos de la foto que el paso marca en la vista de topología.
	Objetos []Ref `json:"objetos"`
}

// Ref es un objeto de la foto: su tipo (service, pod, ingress, puerta, nodo) y su id en la lista
// de la foto (`<namespace>/<nombre>` para un Service o un pod).
type Ref struct {
	Tipo string `json:"tipo"`
	ID   string `json:"id"`
}

// URL arma la URL de una request de prueba: destino es `host[:puerto]` o una URL (con o sin
// esquema, que por defecto es http, y con o sin path); path, si viene, reemplaza al del destino.
func URL(destino, path string) (*url.URL, error) {
	destino = strings.TrimSpace(destino)
	if destino == "" {
		return nil, fmt.Errorf("falta el destino")
	}
	if !strings.Contains(destino, "://") {
		destino = "http://" + destino
	}
	u, err := url.Parse(destino)
	if err != nil {
		return nil, fmt.Errorf("el destino %q no es una URL: %w", destino, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("la request de prueba es HTTP: el destino no puede ser %s://", u.Scheme)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("el destino %q no tiene host", destino)
	}
	if path = strings.TrimSpace(path); path != "" {
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		p, err := url.Parse(path)
		if err != nil {
			return nil, fmt.Errorf("el path %q no sirve: %w", path, err)
		}
		u.Path, u.RawPath, u.RawQuery = p.Path, p.RawPath, p.RawQuery
	}
	if u.Path == "" {
		u.Path = "/"
	}
	return u, nil
}

// Recorrer calcula el recorrido de una request a la URL según la foto. Si el host es la ClusterIP
// o un nombre DNS de un Service, el recorrido es Service → endpoints listos.
func Recorrer(f foto.Foto, u *url.URL) Recorrido {
	r := Recorrido{Pasos: []Paso{}}
	host := strings.TrimSuffix(u.Hostname(), ".")
	puerto := puertoDe(u)
	s := serviceDe(f, host)
	if s == nil {
		return r
	}
	ref := Ref{Tipo: "service", ID: s.ID}
	p := puertoDelService(s, puerto)
	if p == nil {
		return r.con(Paso{
			Tipo:    PasoService,
			Titulo:  fmt.Sprintf("%s %s:%d", s.Nombre, s.ClusterIP, puerto),
			Detalle: fmt.Sprintf("el Service no expone el puerto %d: la request no llega a ningún pod", puerto),
			Objetos: []Ref{ref},
		})
	}
	r = r.con(Paso{
		Tipo:    PasoService,
		Titulo:  fmt.Sprintf("%s %s:%d → :%s", s.Nombre, s.ClusterIP, puerto, p.TargetPort.String()),
		Detalle: fmt.Sprintf("%s, targetPort %s", s.Tipo, p.TargetPort.String()),
		Objetos: []Ref{ref},
	})

	var direcciones []string
	pods := []Ref{}
	noListos := 0
	for _, e := range s.Endpoints {
		if !e.Listo {
			noListos++
			continue
		}
		direcciones = append(direcciones, e.IP+":"+p.TargetPort.String())
		if e.Pod != nil {
			pods = append(pods, Ref{Tipo: "pod", ID: s.Namespace + "/" + *e.Pod})
		}
	}
	paso := Paso{Tipo: PasoEndpoints, Titulo: strings.Join(direcciones, ", "), Objetos: pods}
	switch {
	case len(direcciones) == 0:
		paso.Titulo = "sin endpoints listos"
		paso.Detalle = "ningún pod listo coincide con el selector: la request no tiene a dónde ir"
	case len(direcciones) == 1:
		paso.Detalle = "1 endpoint listo"
	default:
		paso.Detalle = fmt.Sprintf("%d endpoints listos; kube-proxy elige uno", len(direcciones))
	}
	if noListos > 0 {
		if noListos == 1 {
			paso.Detalle += "; 1 no listo, que no recibe tráfico"
		} else {
			paso.Detalle += fmt.Sprintf("; %d no listos, que no reciben tráfico", noListos)
		}
	}
	return r.con(paso)
}

// PodQueRespondio es el pod de los endpoints del recorrido que respondió, si kviz lo puede saber:
// cuando el body de la respuesta trae su nombre. nil si no se sabe.
func PodQueRespondio(f foto.Foto, r Recorrido, body string) *foto.Pod {
	var elegido *foto.Pod
	for _, paso := range r.Pasos {
		if paso.Tipo != PasoEndpoints {
			continue
		}
		for _, o := range paso.Objetos {
			p := podDe(f, o.ID)
			// el nombre más largo gana: un pod no se confunde con otro cuyo nombre es un prefijo del suyo
			if p != nil && strings.Contains(body, p.Nombre) && (elegido == nil || len(p.Nombre) > len(elegido.Nombre)) {
				elegido = p
			}
		}
	}
	return elegido
}

// ConRespuesta completa el recorrido con la respuesta: si se sabe qué pod respondió, el último
// paso es ese pod. Si no, el recorrido termina en los endpoints posibles.
func ConRespuesta(f foto.Foto, r Recorrido, body string) Recorrido {
	p := PodQueRespondio(f, r, body)
	if p == nil {
		return r
	}
	ip := "sin IP"
	if p.IP != nil {
		ip = *p.IP
	}
	return r.con(Paso{
		Tipo:    PasoPod,
		Titulo:  p.Nombre + " " + ip,
		Detalle: "Conocido: el body de la respuesta trae el nombre del pod",
		Objetos: []Ref{{Tipo: "pod", ID: p.ID}},
	})
}

func (r Recorrido) con(p Paso) Recorrido {
	p.N = len(r.Pasos) + 1
	if p.Objetos == nil {
		p.Objetos = []Ref{}
	}
	r.Pasos = append(r.Pasos, p)
	return r
}

func puertoDe(u *url.URL) int32 {
	if n, err := strconv.ParseInt(u.Port(), 10, 32); err == nil {
		return int32(n)
	}
	if u.Scheme == "https" {
		return 443
	}
	return 80
}

// serviceDe busca el Service por ClusterIP o por nombre DNS (`svc.ns`, `svc.ns.svc`,
// `svc.ns.svc.cluster.local`).
func serviceDe(f foto.Foto, host string) *foto.Service {
	for i := range f.Services {
		s := &f.Services[i]
		corto := s.Nombre + "." + s.Namespace
		if (s.ClusterIP != "" && s.ClusterIP != "None" && host == s.ClusterIP) ||
			host == corto || host == corto+".svc" || host == corto+".svc.cluster.local" {
			return s
		}
	}
	return nil
}

func puertoDelService(s *foto.Service, puerto int32) *foto.PuertoService {
	for i := range s.Puertos {
		p := &s.Puertos[i]
		if p.Puerto == puerto && (p.Protocolo == "" || p.Protocolo == "TCP") {
			return p
		}
	}
	return nil
}

func podDe(f foto.Foto, id string) *foto.Pod {
	for i := range f.Pods {
		if f.Pods[i].ID == id {
			return &f.Pods[i]
		}
	}
	return nil
}
