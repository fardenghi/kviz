// Package foto es el modelo del cluster (módulo 2 de la spec): traduce los objetos de Kubernetes
// a la foto, con el vocabulario del glosario. La foto es el contrato con la interfaz
// (web/src/foto.ts): para agregar algo, sumar un campo a Objetos (la entrada), un campo a Foto
// (la salida) y su traducción en Construir.
package foto

import (
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Foto es el estado del cluster tal como lo ve la interfaz. Viaja completa al conectarse y
// después como cambios (ver Cambio): cada objeto de una lista lleva un "id" único en su lista,
// y las listas van ordenadas por id.
type Foto struct {
	// Contexto es el contexto de kubectl de kviz (por ejemplo "kind-lab"); "" si no hay ninguno.
	Contexto string `json:"contexto"`
	// Conectado dice si la foto refleja el cluster. Desconectada, las listas vienen vacías.
	Conectado bool `json:"conectado"`
	// Motivo explica, en español, por qué kviz está desconectado.
	Motivo string `json:"motivo,omitempty"`
	Nodos  []Nodo `json:"nodos"`
	Pods   []Pod  `json:"pods"`
	// Workloads son los workloads de arriba de cada cadena de dueños (ver workloads.go), ordenados por id.
	Workloads []Workload `json:"workloads"`
	// Services ordenados por id (namespace y nombre).
	Services []Service `json:"services"`
	// Ingress ordenados por id (namespace y nombre).
	Ingress []Ingress `json:"ingress"`
	// Puertas ordenadas por id (nodo, puerto y protocolo).
	Puertas []Puerta `json:"puertas"`
}

type Nodo struct {
	// ID es el nombre del nodo.
	ID         string `json:"id"`
	Nombre     string `json:"nombre"`
	InternalIP string `json:"internalIP"`
	// Capacidad es status.capacity del nodo (ver detalle.go).
	Capacidad Capacidad `json:"capacidad"`
}

type Pod struct {
	// ID es "<namespace>/<nombre>".
	ID        string `json:"id"`
	Nombre    string `json:"nombre"`
	Namespace string `json:"namespace"`
	// Labels del pod; vacío si no tiene.
	Labels map[string]string `json:"labels"`
	// Creado es la hora de creación en RFC 3339 (UTC), para calcular la edad; "" si no se sabe.
	Creado string `json:"creado"`
	// Nodo es nil cuando el scheduler todavía no le asignó un nodo.
	Nodo *string `json:"nodo"`
	// IP es nil mientras el pod no tiene IP.
	IP *string `json:"ip"`
	// Fase es status.phase: Pending, Running, Succeeded, Failed o Unknown.
	Fase string `json:"fase"`
	// Estado es la columna STATUS de kubectl get pods: Running, Init:0/1, CrashLoopBackOff,
	// Completed, Terminating…
	Estado string `json:"estado"`
	// Containers en orden de vida: init, normales y ephemeral.
	Containers []Container `json:"containers"`
	// Workload es el id del workload dueño del pod, o nil si es un Pod suelto.
	Workload *string `json:"workload"`
	// HostPorts son los puertos de sus containers que se abren en su nodo.
	HostPorts []HostPort `json:"hostPorts"`
}

// Service es un Service con sus endpoints, tomados de sus EndpointSlices.
type Service struct {
	// ID es "<namespace>/<nombre>".
	ID        string `json:"id"`
	Nombre    string `json:"nombre"`
	Namespace string `json:"namespace"`
	// Tipo es spec.type: ClusterIP, NodePort, LoadBalancer o ExternalName.
	Tipo string `json:"tipo"`
	// ClusterIP es "None" en un Service headless y "" si no tiene (ExternalName).
	ClusterIP string            `json:"clusterIP"`
	Puertos   []PuertoService   `json:"puertos"`
	Selector  map[string]string `json:"selector"`
	// Endpoints listos y no listos, ordenados por IP.
	Endpoints []Endpoint `json:"endpoints"`
}

type PuertoService struct {
	Nombre    string `json:"nombre"`
	Protocolo string `json:"protocolo"`
	Puerto    int32  `json:"puerto"`
	// TargetPort es un número, o el nombre de un puerto de los containers.
	TargetPort intstr.IntOrString `json:"targetPort"`
	// NodePort es 0 si el Service no abre un puerto en los nodos.
	NodePort int32 `json:"nodePort,omitempty"`
}

// Endpoint es una dirección de una EndpointSlice del Service.
type Endpoint struct {
	IP string `json:"ip"`
	// Pod es el nombre del pod (del namespace del Service) detrás del endpoint, si se sabe.
	Pod  *string `json:"pod"`
	Nodo *string `json:"nodo"`
	// Listo es conditions.ready; sin valor cuenta como listo, como dice la API.
	Listo bool `json:"listo"`
}

// Objetos son los objetos de Kubernetes de los que sale la foto.
type Objetos struct {
	Nodos          []*corev1.Node
	Pods           []*corev1.Pod
	Deployments    []*appsv1.Deployment
	ReplicaSets    []*appsv1.ReplicaSet
	StatefulSets   []*appsv1.StatefulSet
	DaemonSets     []*appsv1.DaemonSet
	Jobs           []*batchv1.Job
	CronJobs       []*batchv1.CronJob
	Services       []*corev1.Service
	EndpointSlices []*discoveryv1.EndpointSlice
	Ingress        []*networkingv1.Ingress
}

// Construir traduce los objetos a la foto. Es pura y determinista: ordena cada lista por id
// (nodos por nombre, pods por namespace y nombre), así la misma entrada da siempre la misma foto.
func Construir(contexto string, o Objetos) Foto {
	f := Foto{Contexto: contexto, Conectado: true, Nodos: []Nodo{}, Pods: []Pod{}, Services: services(o), Workloads: []Workload{}, Ingress: ingresses(o)}
	for _, n := range o.Nodos {
		f.Nodos = append(f.Nodos, Nodo{ID: n.Name, Nombre: n.Name, InternalIP: internalIP(n), Capacidad: capacidadDe(n)})
	}
	sort.Slice(f.Nodos, func(i, j int) bool { return f.Nodos[i].ID < f.Nodos[j].ID })

	workloads, workloadDePod := cadenas(o)
	f.Workloads = workloads
	completarRollout(f.Workloads, o)
	for _, p := range o.Pods {
		f.Pods = append(f.Pods, Pod{
			ID:         p.Namespace + "/" + p.Name,
			Nombre:     p.Name,
			Namespace:  p.Namespace,
			Labels:     etiquetas(p.Labels),
			Creado:     creado(p.ObjectMeta),
			Nodo:       opcional(p.Spec.NodeName),
			IP:         opcional(p.Status.PodIP),
			Fase:       string(p.Status.Phase),
			Estado:     estadoPod(p),
			Containers: containers(p),
			Workload:   opcional(workloadDePod[p.Namespace+"/"+p.Name]),
			HostPorts:  hostPorts(p),
		})
	}
	sort.Slice(f.Pods, func(i, j int) bool { return f.Pods[i].ID < f.Pods[j].ID })
	f.Puertas = puertas(f.Nodos, f.Services, o)
	return f
}

// Desconectada es la foto de cuando kviz no puede ver el cluster: sin objetos y con el motivo.
func Desconectada(contexto, motivo string) Foto {
	return Foto{Contexto: contexto, Conectado: false, Motivo: motivo, Nodos: []Nodo{}, Pods: []Pod{}, Services: []Service{}, Workloads: []Workload{}, Ingress: []Ingress{}, Puertas: []Puerta{}}
}

func services(o Objetos) []Service {
	// las EndpointSlices se ligan a su Service por la etiqueta kubernetes.io/service-name
	slices := map[string][]*discoveryv1.EndpointSlice{}
	for _, es := range o.EndpointSlices {
		if svc := es.Labels[discoveryv1.LabelServiceName]; svc != "" {
			slices[es.Namespace+"/"+svc] = append(slices[es.Namespace+"/"+svc], es)
		}
	}

	out := []Service{}
	for _, s := range o.Services {
		svc := Service{
			ID:        s.Namespace + "/" + s.Name,
			Nombre:    s.Name,
			Namespace: s.Namespace,
			Tipo:      string(s.Spec.Type),
			ClusterIP: s.Spec.ClusterIP,
			Puertos:   []PuertoService{},
			Selector:  s.Spec.Selector,
			Endpoints: endpoints(slices[s.Namespace+"/"+s.Name]),
		}
		if svc.Tipo == "" {
			svc.Tipo = string(corev1.ServiceTypeClusterIP)
		}
		if svc.Selector == nil {
			svc.Selector = map[string]string{}
		}
		for _, p := range s.Spec.Ports {
			target := p.TargetPort
			if target.Type == intstr.Int && target.IntVal == 0 {
				// sin targetPort, Kubernetes manda el tráfico al mismo número de puerto
				target = intstr.FromInt32(p.Port)
			}
			svc.Puertos = append(svc.Puertos, PuertoService{
				Nombre:     p.Name,
				Protocolo:  string(p.Protocol),
				Puerto:     p.Port,
				TargetPort: target,
				NodePort:   p.NodePort,
			})
		}
		out = append(out, svc)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// endpoints junta las direcciones de las EndpointSlices de un Service, sin repetir: una misma
// dirección puede aparecer en varias slices (por ejemplo, una por cada conjunto de puertos).
func endpoints(slices []*discoveryv1.EndpointSlice) []Endpoint {
	out := []Endpoint{}
	vistos := map[string]bool{}
	for _, es := range slices {
		for _, e := range es.Endpoints {
			var pod *string
			if e.TargetRef != nil && e.TargetRef.Kind == "Pod" {
				pod = opcional(e.TargetRef.Name)
			}
			var nodo *string
			if e.NodeName != nil {
				nodo = opcional(*e.NodeName)
			}
			listo := e.Conditions.Ready == nil || *e.Conditions.Ready
			for _, ip := range e.Addresses {
				if vistos[ip] {
					continue
				}
				vistos[ip] = true
				out = append(out, Endpoint{IP: ip, Pod: pod, Nodo: nodo, Listo: listo})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].IP < out[j].IP })
	return out
}

func internalIP(n *corev1.Node) string {
	for _, a := range n.Status.Addresses {
		if a.Type == corev1.NodeInternalIP {
			return a.Address
		}
	}
	return ""
}

func opcional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
