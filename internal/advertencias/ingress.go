package advertencias

// Historia 75: Ingress sin IngressClass o hacia un Service que no existe.

import (
	"fmt"
	"sort"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
)

// anotacionClasePorDefecto marca la IngressClass que toman los Ingress que no dicen ninguna.
const anotacionClasePorDefecto = "ingressclass.kubernetes.io/is-default-class"

func (r *reglas) ingress() {
	clases := map[string]bool{}
	porDefecto := false
	var nombres []string
	for _, c := range r.o.IngressClasses {
		clases[c.Name] = true
		nombres = append(nombres, c.Name)
		porDefecto = porDefecto || c.Annotations[anotacionClasePorDefecto] == "true"
	}
	sort.Strings(nombres)
	hay := "El cluster no tiene ninguna IngressClass: falta instalar un Ingress Controller."
	if len(nombres) > 0 {
		hay = "Las IngressClasses del cluster son: " + strings.Join(nombres, ", ") + "."
	}

	services := map[string]bool{}
	for _, s := range r.f.Services {
		services[s.ID] = true
	}

	for _, ing := range r.o.Ingress {
		ob := objeto{"Ingress", ing.Namespace, ing.Name}
		if explicacion := sinClase(ing, clases, porDefecto); explicacion != "" {
			r.agregar(ob, "ingress-sin-clase", "", "Ingress sin IngressClass", explicacion+" "+hay,
				r.kubectl("Ver las IngressClasses del cluster", "", "get ingressclass"))
		}
	}
	for _, i := range r.f.Ingress {
		ob := objeto{"Ingress", i.Namespace, i.Nombre}
		vistos := map[string]bool{}
		for _, regla := range i.Reglas {
			if services[i.Namespace+"/"+regla.Service] || vistos[regla.Service] {
				continue
			}
			vistos[regla.Service] = true
			r.agregar(ob, "ingress-service-inexistente", regla.Service, fmt.Sprintf("El Service %s no existe", regla.Service),
				fmt.Sprintf("Una regla del Ingress manda el tráfico al Service %q, pero no hay ningún Service con ese nombre en el namespace %s (un Ingress solo apunta a Services de su namespace). El Ingress Controller va a responder 503: revisá el nombre.", regla.Service, i.Namespace),
				r.kubectl("Ver los Services del namespace "+i.Namespace, i.Namespace, "get services"))
		}
	}
}

// sinClase explica por qué ningún Ingress Controller atiende el Ingress, o "" si tiene clase:
// spec.ingressClassName de una IngressClass que existe, la anotación vieja kubernetes.io/ingress.class
// (que cada controller interpreta a su manera), o ninguna pero con una IngressClass por defecto.
func sinClase(ing *networkingv1.Ingress, clases map[string]bool, porDefecto bool) string {
	switch {
	case ing.Spec.IngressClassName != nil && *ing.Spec.IngressClassName != "":
		if clases[*ing.Spec.IngressClassName] {
			return ""
		}
		return fmt.Sprintf("El Ingress pide la IngressClass %q, que no existe: ningún Ingress Controller lo va a atender.", *ing.Spec.IngressClassName)
	case ing.Annotations["kubernetes.io/ingress.class"] != "" || porDefecto:
		return ""
	}
	return "El Ingress no dice qué Ingress Controller lo atiende (spec.ingressClassName) y el cluster no tiene una IngressClass por defecto: nadie lo va a atender."
}
