// Package comandos es el generador de comandos sugeridos (módulo 7 de la spec): de un objeto de
// la foto a la lista de comandos de kubectl que aplican a él, completos con nombre, namespace y
// contexto, listos para copiar. kviz no ejecuta ninguno: solo los muestra.
package comandos

import (
	"fmt"
	"regexp"
	"strings"

	"kviz/internal/foto"
)

// Comando es un comando sugerido.
type Comando struct {
	// Descripcion dice, en español, para qué sirve.
	Descripcion string `json:"descripcion"`
	// Comando es la línea completa, lista para pegar en la terminal.
	Comando string `json:"comando"`
}

// Objeto identifica un objeto de la foto por su tipo de Kubernetes (Pod, Service, Ingress, Node,
// Deployment, ReplicaSet, StatefulSet, DaemonSet, Job o CronJob), su namespace ("" en un Node) y
// su nombre.
type Objeto struct {
	Tipo      string
	Namespace string
	Nombre    string
}

// MetricsServer es el "tipo" de objeto que pide los comandos para instalar metrics-server en
// kind, cuando el cluster no lo tiene (Foto.Metricas es false). No lleva namespace ni nombre.
const MetricsServer = "metrics-server"

// instalarMetricsServer instala metrics-server y le agrega --kubelet-insecure-tls: en kind los
// certificados del kubelet no están firmados por la CA del cluster y, sin esa opción,
// metrics-server no puede medir.
func instalarMetricsServer(contexto string) []Comando {
	k := kubectl{contexto: contexto}
	sistema := kubectl{contexto: contexto, namespace: "kube-system"}
	return []Comando{
		{"Instalar metrics-server", k.linea("apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml")},
		{"Agregarle --kubelet-insecure-tls (en kind hace falta para que pueda medir)", sistema.linea("patch deployment metrics-server", "--type=json", "-p",
			`'[{"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"}]'`)},
		{"Esperar a que esté listo (el consumo aparece en kviz un minuto después)", sistema.linea("rollout status deployment/metrics-server")},
	}
}

// Para devuelve los comandos sugeridos del objeto. ok es false si el objeto no está en la foto.
func Para(f foto.Foto, o Objeto) (cs []Comando, ok bool) {
	k := kubectl{contexto: f.Contexto, namespace: o.Namespace}
	switch o.Tipo {
	case MetricsServer:
		return instalarMetricsServer(f.Contexto), true
	case "Pod":
		for _, p := range f.Pods {
			if p.Namespace == o.Namespace && p.Nombre == o.Nombre {
				return dePod(k, p), true
			}
		}
	case "Service":
		for _, sv := range f.Services {
			if sv.Namespace == o.Namespace && sv.Nombre == o.Nombre {
				return deService(k, sv), true
			}
		}
	case "Ingress":
		for _, i := range f.Ingress {
			if i.Namespace == o.Namespace && i.Nombre == o.Nombre {
				return []Comando{describe(k, "ingress", i.Nombre)}, true
			}
		}
	case "Node":
		for _, n := range f.Nodos {
			if n.Nombre == o.Nombre {
				k.namespace = ""
				return []Comando{
					describe(k, "node", n.Nombre),
					{"Ver los pods del nodo, de todos los namespaces", k.linea("get pods -A -o wide", "--field-selector", "spec.nodeName="+n.Nombre)},
				}, true
			}
		}
	default:
		for _, w := range f.Workloads {
			if w.Tipo == o.Tipo && w.Namespace == o.Namespace && w.Nombre == o.Nombre {
				return deWorkload(k, w), true
			}
		}
	}
	return nil, false
}

func dePod(k kubectl, p foto.Pod) []Comando {
	normales := []string{}
	puertos := []int32{}
	for _, c := range p.Containers {
		if c.Tipo != foto.ContainerNormal {
			continue
		}
		normales = append(normales, c.Nombre)
		for _, pt := range c.Puertos {
			if pt.Protocolo == "TCP" {
				puertos = append(puertos, pt.Numero)
			}
		}
	}
	// con un solo container kubectl lo elige solo; con varios hay que decir cuál (-c)
	porContainer := [][]string{{p.Nombre}}
	if len(normales) > 1 {
		porContainer = nil
		for _, c := range normales {
			porContainer = append(porContainer, []string{p.Nombre, "-c", c})
		}
	}

	var cs []Comando
	for _, args := range porContainer {
		cs = append(cs, Comando{"Ver los logs" + de(args), k.linea("logs", args...)})
	}
	for _, args := range porContainer {
		cs = append(cs, Comando{"Abrir una shell" + de(args), k.linea("exec -it", args...) + " -- sh"})
	}
	cs = append(cs,
		describe(k, "pod", p.Nombre),
		Comando{"Borrar el pod (si tiene workload, el workload crea otro)", k.linea("delete pod", p.Nombre)},
	)
	if len(puertos) > 0 {
		cs = append(cs, Comando{"Abrir sus puertos en tu máquina (127.0.0.1)", k.linea("port-forward pod/"+p.Nombre, mapeo(puertos)...)})
	}
	debug := k.linea("debug -it", p.Nombre) + " --image=" + imagenDeDebug
	if len(normales) > 0 {
		debug += " --target=" + normales[0]
	}
	return append(cs, Comando{"Sumar un container de depuración que ve sus procesos", debug})
}

func deService(k kubectl, s foto.Service) []Comando {
	var cs []Comando
	puertos := []int32{}
	for _, p := range s.Puertos {
		if p.Protocolo == "TCP" {
			puertos = append(puertos, p.Puerto)
		}
	}
	if len(puertos) > 0 && s.Tipo != "ExternalName" {
		cs = append(cs, Comando{"Abrir sus puertos en tu máquina (127.0.0.1), por uno de sus pods", k.linea("port-forward service/"+s.Nombre, mapeo(puertos)...)})
	}
	return append(cs,
		Comando{"Ver sus EndpointSlices (a qué pods manda tráfico)", k.linea("get endpointslices", "-l", "kubernetes.io/service-name="+s.Nombre)},
		describe(k, "service", s.Nombre),
	)
}

func deWorkload(k kubectl, w foto.Workload) []Comando {
	recurso := strings.ToLower(w.Tipo) + "/" + w.Nombre
	var cs []Comando
	switch w.Tipo {
	case "Deployment":
		replicas := int32(1)
		if w.Replicas != nil {
			replicas = w.Replicas.Deseadas
		}
		cs = []Comando{
			{"Cambiar la cantidad de réplicas", k.linea("scale "+recurso, fmt.Sprintf("--replicas=%d", replicas))},
			{"Reiniciar sus pods con un rollout nuevo", k.linea("rollout restart " + recurso)},
			{"Volver a la versión anterior", k.linea("rollout undo " + recurso)},
			{"Ver las versiones del rollout", k.linea("rollout history " + recurso)},
		}
	case "CronJob":
		cs = []Comando{{"Disparar un Job ahora, a mano", k.linea("create job "+nombreDeJobManual(w.Nombre), "--from=cronjob/"+w.Nombre)}}
	}
	return append(cs, describe(k, strings.ToLower(w.Tipo), w.Nombre))
}

// nombreDeJobManual es "<cronjob>-manual", recortado al largo máximo de un nombre (63).
func nombreDeJobManual(cronJob string) string {
	nombre := cronJob + "-manual"
	if len(nombre) > 63 {
		nombre = strings.TrimRight(nombre[:63], "-")
	}
	return nombre
}

func describe(k kubectl, recurso, nombre string) Comando {
	return Comando{"Ver el detalle y los events", k.linea("describe "+recurso, nombre)}
}

// imagenDeDebug es la imagen chica que se sugiere para kubectl debug.
const imagenDeDebug = "busybox:1.36"

// de es " de <container>" si los args eligen un container con -c.
func de(args []string) string {
	if len(args) == 3 {
		return " de " + args[2]
	}
	return ""
}

// mapeo arma los pares local:remoto de port-forward. Los puertos privilegiados (menos de 1024)
// se abren en tu máquina 8000 más arriba (80 → 8080), y si dos chocan se usa el siguiente libre.
func mapeo(puertos []int32) []string {
	usados := map[int32]bool{}
	var pares []string
	for _, p := range puertos {
		local := p
		if local < 1024 {
			local += 8000
		}
		for usados[local] {
			local++
		}
		usados[local] = true
		pares = append(pares, fmt.Sprintf("%d:%d", local, p))
	}
	return pares
}

// Linea es "kubectl <verbo> <args> -n <namespace> --context <contexto>", con el contexto entre
// comillas si la shell lo interpretaría; sin namespace o sin contexto, sin esas opciones. Los args
// van tal cual: el que tenga caracteres de la shell tiene que venir ya entre comillas.
func Linea(contexto, namespace, verbo string, args ...string) string {
	return kubectl{contexto: contexto, namespace: namespace}.linea(verbo, args...)
}

// kubectl arma líneas de kubectl con el namespace y el contexto del objeto.
type kubectl struct{ contexto, namespace string }

// linea es "kubectl <verbo> <args> -n <namespace> --context <contexto>"; sin namespace o sin
// contexto, sin esas opciones.
func (k kubectl) linea(verbo string, args ...string) string {
	partes := append([]string{"kubectl", verbo}, args...)
	if k.namespace != "" {
		partes = append(partes, "-n", k.namespace)
	}
	if k.contexto != "" {
		partes = append(partes, "--context", comillas(k.contexto))
	}
	return strings.Join(partes, " ")
}

// seguro son los caracteres que la shell no interpreta.
var seguro = regexp.MustCompile(`^[A-Za-z0-9._:/@=+,-]+$`)

// comillas pone el texto entre comillas simples si la shell lo interpretaría (espacios,
// comillas, $…). Los nombres y namespaces de Kubernetes no las necesitan; un contexto, sí puede.
func comillas(s string) string {
	if seguro.MatchString(s) {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
