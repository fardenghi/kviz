package foto

import (
	"sort"

	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Ingress es un Ingress con su clase, sus reglas y la anotación de rewrite de ingress-nginx.
type Ingress struct {
	// ID es "<namespace>/<nombre>".
	ID        string `json:"id"`
	Nombre    string `json:"nombre"`
	Namespace string `json:"namespace"`
	// Clase es spec.ingressClassName, o la anotación vieja kubernetes.io/ingress.class, o si no dice
	// ninguna la IngressClass por defecto del cluster (con ClasePorDefecto); nil si no tiene.
	Clase *string `json:"clase"`
	// ClasePorDefecto: el Ingress no dice clase y la toma de la IngressClass por defecto.
	ClasePorDefecto bool `json:"clasePorDefecto"`
	// Rewrite es la anotación nginx.ingress.kubernetes.io/rewrite-target; vale para todas las reglas.
	Rewrite *string `json:"rewrite"`
	// Reglas en el orden del Ingress; el backend por defecto, si hay, va al final sin host ni path.
	Reglas []ReglaIngress `json:"reglas"`
}

// ReglaIngress es un path de una regla del Ingress y el Service (de su namespace) al que manda.
type ReglaIngress struct {
	// Host es "" cuando la regla vale para cualquier host.
	Host string `json:"host"`
	Path string `json:"path"`
	// TipoPath es Exact, Prefix o ImplementationSpecific.
	TipoPath string `json:"tipoPath"`
	Service  string `json:"service"`
	// Puerto del Service: número, o el nombre de un puerto del Service.
	Puerto intstr.IntOrString `json:"puerto"`
}

const (
	anotacionClase   = "kubernetes.io/ingress.class"
	anotacionRewrite = "nginx.ingress.kubernetes.io/rewrite-target"
	// anotacionClasePorDefecto marca la IngressClass que toman los Ingress que no dicen ninguna.
	anotacionClasePorDefecto = "ingressclass.kubernetes.io/is-default-class"
)

// ClasePorDefecto es la IngressClass marcada por defecto ("" si no hay): la que atiende a los
// Ingress que no dicen clase. Con varias, la primera por nombre.
func ClasePorDefecto(clases []*networkingv1.IngressClass) string {
	porDefecto := ""
	for _, c := range clases {
		if c.Annotations[anotacionClasePorDefecto] == "true" && (porDefecto == "" || c.Name < porDefecto) {
			porDefecto = c.Name
		}
	}
	return porDefecto
}

func ingresses(o Objetos) []Ingress {
	out := []Ingress{}
	porDefecto := ClasePorDefecto(o.IngressClasses)
	for _, ing := range o.Ingress {
		i := Ingress{ID: ing.Namespace + "/" + ing.Name, Nombre: ing.Name, Namespace: ing.Namespace, Reglas: []ReglaIngress{}}
		switch {
		case ing.Spec.IngressClassName != nil:
			i.Clase = opcional(*ing.Spec.IngressClassName)
		case ing.Annotations[anotacionClase] != "":
			i.Clase = opcional(ing.Annotations[anotacionClase])
		default:
			i.Clase = opcional(porDefecto)
			i.ClasePorDefecto = porDefecto != ""
		}
		i.Rewrite = opcional(ing.Annotations[anotacionRewrite])
		for _, r := range ing.Spec.Rules {
			if r.HTTP == nil {
				continue
			}
			for _, p := range r.HTTP.Paths {
				regla, ok := reglaHacia(p.Backend)
				if !ok {
					continue
				}
				regla.Host, regla.Path = r.Host, p.Path
				if p.PathType != nil {
					regla.TipoPath = string(*p.PathType)
				}
				i.Reglas = append(i.Reglas, regla)
			}
		}
		if ing.Spec.DefaultBackend != nil {
			if regla, ok := reglaHacia(*ing.Spec.DefaultBackend); ok {
				i.Reglas = append(i.Reglas, regla)
			}
		}
		out = append(out, i)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// reglaHacia arma la regla de un backend que es un Service; los backends de otro tipo
// (spec.resource) no llevan tráfico a un Service y no se muestran.
func reglaHacia(b networkingv1.IngressBackend) (ReglaIngress, bool) {
	if b.Service == nil {
		return ReglaIngress{}, false
	}
	puerto := intstr.FromInt32(b.Service.Port.Number)
	if b.Service.Port.Name != "" {
		puerto = intstr.FromString(b.Service.Port.Name)
	}
	return ReglaIngress{Service: b.Service.Name, Puerto: puerto}, true
}
