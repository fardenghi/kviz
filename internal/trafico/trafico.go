// Package trafico es la depuración de tráfico (módulo 11 de la spec): a partir del destino de una
// request de prueba y la foto, calcula el recorrido desde la configuración (no rastrea paquetes).
// Es puro: no habla con el cluster ni con el pod de sondeo.
//
// El recorrido es DNS → puerta → regla de Ingress → Service → endpoints → salto entre nodos →
// pod, con los pasos que correspondan al destino (recorrer.go, ingress.go); el pod y el salto se
// suman solo cuando se sabe qué pod respondió (respuesta.go). La interfaz dibuja cada paso por los
// objetos que marca, sin conocer el tipo.
package trafico

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/fardenghi/kviz/internal/foto"
)

// Recorrido es el camino que siguió una request de prueba, en pasos numerados.
type Recorrido struct {
	// Pasos en orden, numerados desde 1. Vacío si el destino no es un objeto de la foto.
	Pasos []Paso `json:"pasos"`
	// sondeo es el nodo del pod de sondeo, de donde sale la request (ver DesdeNodo); "" si no se sabe.
	sondeo string
}

// Tipos de paso.
const (
	PasoDNS       = "dns"
	PasoPuerta    = "puerta"
	PasoIngress   = "ingress"
	PasoService   = "service"
	PasoEndpoints = "endpoints"
	PasoSalto     = "salto"
	PasoPod       = "pod"
)

// Paso es un paso del recorrido.
type Paso struct {
	N int `json:"n"`
	// Tipo: dns, puerta, ingress, service, endpoints, salto o pod (ver las constantes Paso…).
	Tipo string `json:"tipo"`
	// Titulo es la línea principal del paso: `backend 10.96.11.121:80 → :8081`.
	Titulo string `json:"titulo"`
	// Detalle explica el paso en español.
	Detalle string `json:"detalle"`
	// Objetos de la foto que el paso marca en la vista de topología.
	Objetos []Ref `json:"objetos"`
}

// Ref es un objeto de la foto: su tipo (service, pod, ingress, puerta, nodo) y su id en la lista
// de la foto (`<namespace>/<nombre>` para un Service, pod o Ingress; `<nodo>/<puerto>/<protocolo>`
// para una puerta; el nombre para un nodo). El paso salto marca dos nodos: de dónde sale y a dónde
// llega, en ese orden.
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

// URLTCP arma el destino de una prueba de conexión como URL `tcp://host:puerto`, para calcular su
// recorrido con Recorrer igual que el de una request. destino es `host:puerto` (con o sin `tcp://`);
// el puerto es obligatorio, porque no hay uno por defecto.
func URLTCP(destino string) (*url.URL, error) {
	destino = strings.TrimSpace(destino)
	if destino == "" {
		return nil, fmt.Errorf("falta el destino")
	}
	if esquema, _, ok := strings.Cut(destino, "://"); ok && esquema != "tcp" {
		return nil, fmt.Errorf("la prueba de conexión es TCP: el destino no puede ser %s://", esquema)
	}
	u, err := url.Parse("tcp://" + strings.TrimPrefix(destino, "tcp://"))
	if err != nil {
		return nil, fmt.Errorf("el destino %q no es host:puerto: %w", destino, err)
	}
	if u.Hostname() == "" {
		return nil, fmt.Errorf("el destino %q no tiene host", destino)
	}
	if n, err := strconv.ParseUint(u.Port(), 10, 16); err != nil || n == 0 {
		return nil, fmt.Errorf("la prueba de conexión necesita un puerto: host:puerto, como postgres.default:5432")
	}
	if (u.Path != "" && u.Path != "/") || u.RawQuery != "" {
		return nil, fmt.Errorf("la prueba de conexión va a host:puerto, sin path")
	}
	u.Path = ""
	return u, nil
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

// puertoDelService es el puerto TCP del Service con ese número, o con ese nombre (una regla de
// Ingress puede nombrarlo).
func puertoDelService(s *foto.Service, puerto intstr.IntOrString) *foto.PuertoService {
	for i := range s.Puertos {
		p := &s.Puertos[i]
		if p.Protocolo != "" && p.Protocolo != "TCP" {
			continue
		}
		if (puerto.Type == intstr.Int && p.Puerto == puerto.IntVal) || (puerto.Type == intstr.String && p.Nombre == puerto.StrVal) {
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
