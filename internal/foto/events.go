package foto

// Events del cluster (ticket 12): la franja de events de abajo y la sección de events del panel
// de detalle. Cada event dice a qué objeto de la foto se refiere (EnFoto), para que la interfaz lo
// seleccione con un clic sin saber nada de Kubernetes.

import (
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Event es un Event de Kubernetes (core/v1).
type Event struct {
	// ID es "<namespace>/<nombre>" del Event.
	ID string `json:"id"`
	// Tipo es Normal o Warning.
	Tipo    string `json:"tipo"`
	Motivo  string `json:"motivo"`
	Mensaje string `json:"mensaje"`
	// Objeto es el objeto de Kubernetes al que se refiere (involvedObject); su namespace es ""
	// si el objeto no tiene (un nodo).
	Objeto ObjetoDeEvent `json:"objeto"`
	// EnFoto es el objeto de la foto al que se refiere el event, o nil si no está en la foto.
	EnFoto *ObjetoEnFoto `json:"enFoto"`
	// UltimaVez es la última vez que pasó, en RFC 3339 (UTC); "" si no se sabe.
	UltimaVez string `json:"ultimaVez"`
	// Veces es cuántas veces pasó (al menos 1).
	Veces int32 `json:"veces"`
}

type ObjetoDeEvent struct {
	// Tipo es el kind: Pod, Node, ReplicaSet, Deployment…
	Tipo      string `json:"tipo"`
	Namespace string `json:"namespace"`
	Nombre    string `json:"nombre"`
}

// ObjetoEnFoto ubica un objeto en la foto: la sección (pods, nodos, services, ingress o
// workloads) y su id en ella.
type ObjetoEnFoto struct {
	Seccion string `json:"seccion"`
	ID      string `json:"id"`
}

func events(f Foto, o Objetos) []Event {
	enFoto := indiceDeLaFoto(f, o)
	out := make([]Event, 0, len(o.Events))
	for _, e := range o.Events {
		obj := ObjetoDeEvent{Tipo: e.InvolvedObject.Kind, Namespace: e.InvolvedObject.Namespace, Nombre: e.InvolvedObject.Name}
		out = append(out, Event{
			ID:        e.Namespace + "/" + e.Name,
			Tipo:      e.Type,
			Motivo:    e.Reason,
			Mensaje:   e.Message,
			Objeto:    obj,
			EnFoto:    enFoto(obj),
			UltimaVez: ultimaVez(e),
			Veces:     veces(e),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// indiceDeLaFoto devuelve la función que busca en la foto el objeto de un event. Los events de
// un objeto de una cadena de dueños (un ReplicaSet, un Job…) van al workload de arriba.
func indiceDeLaFoto(f Foto, o Objetos) func(ObjetoDeEvent) *ObjetoEnFoto {
	ids := map[ObjetoEnFoto]bool{}
	for _, n := range f.Nodos {
		ids[ObjetoEnFoto{"nodos", n.ID}] = true
	}
	for _, p := range f.Pods {
		ids[ObjetoEnFoto{"pods", p.ID}] = true
	}
	for _, s := range f.Services {
		ids[ObjetoEnFoto{"services", s.ID}] = true
	}
	for _, i := range f.Ingress {
		ids[ObjetoEnFoto{"ingress", i.ID}] = true
	}
	for _, w := range f.Workloads {
		ids[ObjetoEnFoto{"workloads", w.ID}] = true
	}
	dueno := duenosDeWorkloads(o)

	return func(obj ObjetoDeEvent) *ObjetoEnFoto {
		var r ObjetoEnFoto
		nsNombre := obj.Namespace + "/" + obj.Nombre
		switch {
		case obj.Tipo == "Pod":
			r = ObjetoEnFoto{"pods", nsNombre}
		case obj.Tipo == "Node":
			r = ObjetoEnFoto{"nodos", obj.Nombre}
		case obj.Tipo == "Service":
			r = ObjetoEnFoto{"services", nsNombre}
		case obj.Tipo == "Ingress":
			r = ObjetoEnFoto{"ingress", nsNombre}
		case tiposDeWorkload[obj.Tipo]:
			arriba := ref{obj.Tipo, obj.Namespace, obj.Nombre}
			for range 4 {
				d, ok := dueno[arriba]
				if !ok {
					break
				}
				arriba = d
			}
			r = ObjetoEnFoto{"workloads", arriba.id()}
		default:
			return nil
		}
		if !ids[r] {
			return nil
		}
		return &r
	}
}

// duenosDeWorkloads es el dueño de cada objeto de workload que tiene uno (ver duenoDe).
func duenosDeWorkloads(o Objetos) map[ref]ref {
	duenos := map[ref]ref{}
	agregar := func(tipo string, m metav1.ObjectMeta) {
		if d := duenoDe(m); d != nil {
			duenos[ref{tipo, m.Namespace, m.Name}] = *d
		}
	}
	for _, x := range o.Deployments {
		agregar("Deployment", x.ObjectMeta)
	}
	for _, x := range o.ReplicaSets {
		agregar("ReplicaSet", x.ObjectMeta)
	}
	for _, x := range o.StatefulSets {
		agregar("StatefulSet", x.ObjectMeta)
	}
	for _, x := range o.DaemonSets {
		agregar("DaemonSet", x.ObjectMeta)
	}
	for _, x := range o.Jobs {
		agregar("Job", x.ObjectMeta)
	}
	for _, x := range o.CronJobs {
		agregar("CronJob", x.ObjectMeta)
	}
	return duenos
}

// veces es cuántas veces pasó el event: count en los viejos (core/v1), series.count en los nuevos.
func veces(e corev1.Event) int32 {
	if e.Series != nil {
		return max(e.Series.Count, 1)
	}
	return max(e.Count, 1)
}

// ultimaVez es la última vez que pasó el event. Los events viejos (core/v1) la llevan en
// lastTimestamp; los nuevos (events.k8s.io) en series o en eventTime.
func ultimaVez(e corev1.Event) string {
	var t time.Time
	switch {
	case e.Series != nil && !e.Series.LastObservedTime.IsZero():
		t = e.Series.LastObservedTime.Time
	case !e.LastTimestamp.IsZero():
		t = e.LastTimestamp.Time
	case !e.EventTime.IsZero():
		t = e.EventTime.Time
	case !e.FirstTimestamp.IsZero():
		t = e.FirstTimestamp.Time
	case !e.CreationTimestamp.IsZero():
		t = e.CreationTimestamp.Time
	default:
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
