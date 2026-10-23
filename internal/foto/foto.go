// Package foto es el modelo del cluster (módulo 2 de la spec): traduce los objetos de Kubernetes
// a la foto, con el vocabulario del glosario. La foto es el contrato con la interfaz
// (web/src/foto.ts): para agregar algo, sumar un campo a Objetos (la entrada), un campo a Foto
// (la salida) y su traducción en Construir.
package foto

import (
	"sort"

	corev1 "k8s.io/api/core/v1"
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
}

type Nodo struct {
	// ID es el nombre del nodo.
	ID         string `json:"id"`
	Nombre     string `json:"nombre"`
	InternalIP string `json:"internalIP"`
}

type Pod struct {
	// ID es "<namespace>/<nombre>".
	ID        string `json:"id"`
	Nombre    string `json:"nombre"`
	Namespace string `json:"namespace"`
	// Nodo es nil cuando el scheduler todavía no le asignó un nodo.
	Nodo *string `json:"nodo"`
	// IP es nil mientras el pod no tiene IP.
	IP *string `json:"ip"`
	// Fase es status.phase: Pending, Running, Succeeded, Failed o Unknown.
	Fase string `json:"fase"`
	// Terminando es true mientras el pod se está borrando (`Terminating` en kubectl): tiene
	// deletionTimestamp y todavía no desapareció.
	Terminando bool `json:"terminando"`
}

// Objetos son los objetos de Kubernetes de los que sale la foto.
type Objetos struct {
	Nodos []*corev1.Node
	Pods  []*corev1.Pod
}

// Construir traduce los objetos a la foto. Es pura y determinista: ordena cada lista por id
// (nodos por nombre, pods por namespace y nombre), así la misma entrada da siempre la misma foto.
func Construir(contexto string, o Objetos) Foto {
	f := Foto{Contexto: contexto, Conectado: true, Nodos: []Nodo{}, Pods: []Pod{}}
	for _, n := range o.Nodos {
		f.Nodos = append(f.Nodos, Nodo{ID: n.Name, Nombre: n.Name, InternalIP: internalIP(n)})
	}
	sort.Slice(f.Nodos, func(i, j int) bool { return f.Nodos[i].ID < f.Nodos[j].ID })

	for _, p := range o.Pods {
		f.Pods = append(f.Pods, Pod{
			ID:         p.Namespace + "/" + p.Name,
			Nombre:     p.Name,
			Namespace:  p.Namespace,
			Nodo:       opcional(p.Spec.NodeName),
			IP:         opcional(p.Status.PodIP),
			Fase:       string(p.Status.Phase),
			Terminando: p.DeletionTimestamp != nil,
		})
	}
	sort.Slice(f.Pods, func(i, j int) bool { return f.Pods[i].ID < f.Pods[j].ID })
	return f
}

// Desconectada es la foto de cuando kviz no puede ver el cluster: sin objetos y con el motivo.
func Desconectada(contexto, motivo string) Foto {
	return Foto{Contexto: contexto, Conectado: false, Motivo: motivo, Nodos: []Nodo{}, Pods: []Pod{}}
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
