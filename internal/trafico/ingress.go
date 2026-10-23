package trafico

// La regla de Ingress que elige el Ingress Controller (ingress-nginx) para una request, y el path
// reescrito por la anotación rewrite-target.

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/fardenghi/kviz/internal/foto"
)

// desdeIngressController sigue una request que llega a los pods del Ingress Controller: la regla
// que matchea el host y el path, y el Service al que manda.
func (c *calculo) desdeIngressController(ics []*foto.Pod) {
	refs := []Ref{}
	for _, p := range ics {
		refs = append(refs, refPod(p))
	}
	host := hostSinPuerto(c.host)
	m, ok := reglaQueMatchea(c.f, host, c.path)
	if !ok {
		c.con(Paso{Tipo: PasoIngress, Titulo: "ninguna regla para " + host + c.path,
			Detalle: "Ninguna regla de Ingress matchea el host y el path: el Ingress Controller responde 404", Objetos: refs})
		return
	}
	desde := m.regla.Host + m.regla.Path
	if desde == "" {
		desde = "(por defecto)"
	}
	paso := Paso{
		Tipo:    PasoIngress,
		Titulo:  fmt.Sprintf("%s → %s:%s", desde, m.regla.Service, m.regla.Puerto.String()),
		Detalle: "Ingress " + m.ingress.Nombre,
		Objetos: append([]Ref{refIngress(m.ingress)}, refs...),
	}
	if m.reescrito != c.path {
		paso.Detalle += fmt.Sprintf(" · path reescrito: %s → %s", c.path, m.reescrito)
	}
	s := c.f.ServicePorID(foto.IDDe(m.ingress.Namespace, m.regla.Service))
	if s == nil {
		paso.Detalle += fmt.Sprintf(" · el Service %s no existe: el Ingress Controller responde 503", m.regla.Service)
		c.con(paso)
		return
	}
	c.con(paso)
	c.desdeService(s, m.regla.Puerto, ics)
}

type reglaElegida struct {
	ingress   *foto.Ingress
	regla     foto.ReglaIngress
	reescrito string
}

// reglaQueMatchea elige la regla como ingress-nginx: entre los Ingress con clase, las reglas de
// ese host antes que las de cualquier host, y entre ellas el path más largo que matchea; el
// backend por defecto solo si no matchea ninguna. Con rewrite-target los paths son expresiones
// regulares (ingress-nginx lo impone) y el path se reescribe; si no, Exact compara el path entero
// y Prefix (o ImplementationSpecific) por segmentos.
func reglaQueMatchea(f foto.Foto, host, path string) (reglaElegida, bool) {
	var mejor, porDefecto *reglaElegida
	puntaje := func(r reglaElegida) int {
		p := len(r.regla.Path)
		if r.regla.Host != "" {
			p += 1 << 20
		}
		return p
	}
	for i := range f.Ingress {
		ing := &f.Ingress[i]
		if ing.Clase == nil {
			continue
		}
		for _, regla := range ing.Reglas {
			if regla.Host == "" && regla.Path == "" {
				if porDefecto == nil {
					porDefecto = &reglaElegida{ingress: ing, regla: regla, reescrito: path}
				}
				continue
			}
			if !hostMatchea(regla.Host, host) {
				continue
			}
			reescrito, ok := pathMatchea(regla, ing.Rewrite, path)
			if !ok {
				continue
			}
			r := reglaElegida{ingress: ing, regla: regla, reescrito: reescrito}
			if mejor == nil || puntaje(r) > puntaje(*mejor) {
				mejor = &r
			}
		}
	}
	if mejor != nil {
		return *mejor, true
	}
	if porDefecto != nil {
		return *porDefecto, true
	}
	return reglaElegida{}, false
}

// hostMatchea: una regla sin host vale para cualquiera; `*.lab.local` vale para un solo nivel.
func hostMatchea(regla, host string) bool {
	regla = strings.ToLower(regla)
	switch {
	case regla == "":
		return true
	case strings.HasPrefix(regla, "*."):
		punto := strings.Index(host, ".")
		return punto > 0 && host[punto:] == regla[1:]
	}
	return regla == host
}

// pathMatchea dice si el path de la regla matchea y cómo queda el path después del rewrite.
func pathMatchea(regla foto.ReglaIngress, rewrite *string, path string) (string, bool) {
	if rewrite != nil {
		re, err := regexp.Compile("(?i)^" + regla.Path)
		if err != nil {
			return "", false
		}
		grupos := re.FindStringSubmatch(path)
		if grupos == nil {
			return "", false
		}
		return reescribir(*rewrite, grupos), true
	}
	p := regla.Path
	if p == "" {
		p = "/"
	}
	if regla.TipoPath == "Exact" {
		return path, path == p
	}
	base := strings.TrimSuffix(p, "/")
	return path, base == "" || path == base || strings.HasPrefix(path, base+"/")
}

var referencia = regexp.MustCompile(`\$(\d)`)

// reescribir arma el path nuevo como nginx: la anotación con $1…$9 reemplazados por los grupos.
func reescribir(rewrite string, grupos []string) string {
	return referencia.ReplaceAllStringFunc(rewrite, func(r string) string {
		n := int(r[1] - '0')
		if n < len(grupos) {
			return grupos[n]
		}
		return ""
	})
}

func hostSinPuerto(h string) string {
	if sin, _, err := net.SplitHostPort(h); err == nil {
		return strings.ToLower(sin)
	}
	return strings.ToLower(h)
}
