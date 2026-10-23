// Package advertencias son las advertencias de configuración (módulo 6 de la spec): reglas puras
// sobre la foto que marcan los errores típicos que impiden que el tráfico llegue, cada una sobre
// su objeto y con una explicación y un comando sugerido. Son exactamente las seis de las
// historias 70–75:
//
//	70 service-sin-pods, service-sin-endpoints-listos   el selector no matchea pods, o ninguno está listo
//	71 target-port-no-expuesto                          ningún container de sus pods expone el targetPort
//	72 probe-puerto-no-expuesto                         una probe apunta a un puerto que el container no expone
//	73 imagen-local-sin-bajar                           ImagePullBackOff de una imagen que parece local
//	74 pending-sin-recursos                             Pending porque ningún nodo tiene CPU o memoria libre
//	75 ingress-sin-clase, ingress-service-inexistente   Ingress sin IngressClass o hacia un Service que no existe
//
// La foto no trae todo lo que miran las reglas (probes, imagePullPolicy, el motivo del scheduler,
// las IngressClasses): eso lo leen de los objetos de Kubernetes de los que salió la foto.
package advertencias

import (
	"fmt"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/fardenghi/kviz/internal/comandos"
	"github.com/fardenghi/kviz/internal/foto"
)

// De calcula las advertencias de la foto f, construida a partir de los objetos o. Es pura y
// devuelve la lista ordenada por id.
func De(f foto.Foto, o foto.Objetos) []foto.Advertencia {
	r := reglas{f: f, o: o, out: []foto.Advertencia{}}
	r.services()
	r.pods()
	r.ingress()
	sort.Slice(r.out, func(i, j int) bool { return r.out[i].ID < r.out[j].ID })
	return r.out
}

type reglas struct {
	f   foto.Foto
	o   foto.Objetos
	out []foto.Advertencia
}

// objeto es el objeto de la foto sobre el que va una advertencia.
type objeto struct{ tipo, namespace, nombre string }

// agregar suma una advertencia; detalle distingue varias de la misma regla en un objeto.
func (r *reglas) agregar(ob objeto, regla, detalle, titulo, explicacion string, c foto.ComandoSugerido) {
	id := ob.tipo + "/" + ob.namespace + "/" + ob.nombre + "/" + regla
	if detalle != "" {
		id += "/" + detalle
	}
	r.out = append(r.out, foto.Advertencia{
		ID: id, Tipo: ob.tipo, Namespace: ob.namespace, Nombre: ob.nombre, Regla: regla,
		Titulo: titulo, Explicacion: explicacion, Comando: c,
	})
}

// kubectl arma un comando sugerido en el namespace dado, con el contexto de la foto.
func (r *reglas) kubectl(descripcion, namespace, verbo string, args ...string) foto.ComandoSugerido {
	return foto.ComandoSugerido{Descripcion: descripcion, Comando: comandos.Linea(r.f.Contexto, namespace, verbo, args...)}
}

// ---------- historias 70 y 71: Services ----------

func (r *reglas) services() {
	for _, s := range r.f.Services {
		// un ExternalName no manda a pods, y sin selector los endpoints los arma otro (a mano)
		if s.Tipo == "ExternalName" || len(s.Selector) == 0 {
			continue
		}
		ob := objeto{"Service", s.Namespace, s.Nombre}
		selector := textoSelector(s.Selector)
		suyos := r.podsDe(s)
		if len(suyos) == 0 {
			r.agregar(ob, "service-sin-pods", "", "El selector no matchea ningún pod",
				fmt.Sprintf("El selector %s no matchea ningún pod del namespace %s, así que el Service no tiene a quién mandar el tráfico. Revisá que las labels de los pods digan exactamente lo mismo que el selector.", selector, s.Namespace),
				r.kubectl("Comparar las labels de los pods con el selector "+selector, s.Namespace, "get pods", "--show-labels"))
			continue
		}
		r.targetPorts(ob, s, suyos, selector)
		if !algunoListo(s.Endpoints) {
			r.agregar(ob, "service-sin-endpoints-listos", "", "Ningún endpoint listo",
				fmt.Sprintf("El selector %s matchea %s, pero ninguno está listo (Ready): el Service solo manda tráfico a los pods listos. Revisá su estado y su readiness probe.", selector, cantidadDePods(len(suyos))),
				r.kubectl("Ver los pods del selector y si están listos (columna READY)", s.Namespace, "get pods", "-o wide", "-l", selector))
		}
	}
}

// targetPorts marca cada puerto del Service cuyo targetPort no lo expone ningún container de sus
// pods. Un targetPort por nombre tiene que ser el nombre de un puerto de algún container (si no,
// Kubernetes no arma endpoints). Uno por número solo se puede juzgar si los containers declaran
// puertos: un container sin puertos declarados igual puede escuchar en cualquiera.
func (r *reglas) targetPorts(ob objeto, s foto.Service, suyos []foto.Pod, selector string) {
	numeros, nombres := map[int32]bool{}, map[string]bool{}
	for _, p := range suyos {
		for _, c := range p.Containers {
			if c.Tipo != foto.ContainerNormal {
				continue
			}
			for _, pt := range c.Puertos {
				numeros[pt.Numero] = true
				if pt.Nombre != "" {
					nombres[pt.Nombre] = true
				}
			}
		}
	}
	queExponen := "no declaran ningún puerto"
	if len(numeros) > 0 {
		queExponen = "exponen " + listaDePuertos(numeros)
	}

	vistos := map[string]bool{}
	for _, pt := range s.Puertos {
		target := pt.TargetPort.String()
		var expuesto bool
		var cual string
		if pt.TargetPort.Type == intstr.String {
			expuesto, cual = nombres[target], fmt.Sprintf("el puerto llamado %q", target)
		} else {
			expuesto, cual = len(numeros) == 0 || numeros[pt.TargetPort.IntVal], "el "+target
		}
		if expuesto || vistos[target] {
			continue
		}
		vistos[target] = true
		r.agregar(ob, "target-port-no-expuesto", target, fmt.Sprintf("targetPort %s: ningún container lo expone", target),
			fmt.Sprintf("El puerto %d del Service manda el tráfico a %s (targetPort), pero los containers de sus pods (selector %s) %s. Corregí el targetPort o el containerPort para que coincidan.", pt.Puerto, cual, selector, queExponen),
			r.kubectl("Ver qué puertos exponen los containers de sus pods", s.Namespace, "get pods", "-l", selector,
				"-o", "'custom-columns=POD:.metadata.name,PUERTOS:.spec.containers[*].ports[*].containerPort,NOMBRES:.spec.containers[*].ports[*].name'"))
	}
}

// listaDePuertos es "80, 443, 8080": los puertos en orden numérico.
func listaDePuertos(ps map[int32]bool) string {
	ns := make([]int, 0, len(ps))
	for p := range ps {
		ns = append(ns, int(p))
	}
	sort.Ints(ns)
	textos := make([]string, len(ns))
	for i, n := range ns {
		textos[i] = fmt.Sprint(n)
	}
	return strings.Join(textos, ", ")
}

func algunoListo(es []foto.Endpoint) bool {
	for _, e := range es {
		if e.Listo {
			return true
		}
	}
	return false
}

func cantidadDePods(n int) string {
	if n == 1 {
		return "1 pod"
	}
	return fmt.Sprintf("%d pods", n)
}

// podsDe son los pods del namespace del Service cuyas labels incluyen todo su selector.
func (r *reglas) podsDe(s foto.Service) []foto.Pod {
	var out []foto.Pod
	for _, p := range r.f.Pods {
		if p.Namespace == s.Namespace && matchea(s.Selector, p.Labels) {
			out = append(out, p)
		}
	}
	return out
}

func matchea(selector, labels map[string]string) bool {
	for k, v := range selector {
		if l, ok := labels[k]; !ok || l != v {
			return false
		}
	}
	return true
}

// textoSelector es "clave=valor,…" ordenado por clave, como lo acepta kubectl -l.
func textoSelector(sel map[string]string) string {
	pares := make([]string, 0, len(sel))
	for k, v := range sel {
		pares = append(pares, k+"="+v)
	}
	sort.Strings(pares)
	return strings.Join(pares, ",")
}
