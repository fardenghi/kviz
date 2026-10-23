package foto

import (
	"hash/fnv"
	"sort"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Workload es un workload de arriba de una cadena de dueños: Deployment, StatefulSet, DaemonSet,
// CronJob, o un ReplicaSet o Job que no tienen dueño.
type Workload struct {
	// ID es "<Tipo>/<namespace>/<nombre>", por ejemplo "Deployment/default/backend".
	ID        string `json:"id"`
	Tipo      string `json:"tipo"`
	Nombre    string `json:"nombre"`
	Namespace string `json:"namespace"`
	// Color es un color CSS propio del workload, estable entre actualizaciones.
	Color string `json:"color"`
	// Cadena son los eslabones de la cadena de dueños, de arriba hacia abajo: primero el workload
	// y después sus ReplicaSets o Jobs, del más nuevo al más viejo. Un ReplicaSet viejo sin
	// réplicas ni pods no se muestra; durante un rollout aparecen los dos.
	Cadena []Eslabon `json:"cadena"`
}

// Eslabon es un objeto de la cadena de dueños con los pods que crea directamente.
type Eslabon struct {
	Tipo   string `json:"tipo"`
	Nombre string `json:"nombre"`
	// Pods son los nombres de los pods de los que este eslabón es dueño (mismo namespace), en orden.
	Pods []string `json:"pods"`
}

// tiposDeWorkload son los dueños que kviz entiende; un pod con otro dueño es un Pod suelto.
var tiposDeWorkload = map[string]bool{
	"Deployment": true, "ReplicaSet": true, "StatefulSet": true, "DaemonSet": true, "Job": true, "CronJob": true,
}

// ref identifica un objeto con dueño o dueño de algo, dentro de su namespace.
type ref struct{ tipo, namespace, nombre string }

func (r ref) id() string { return r.tipo + "/" + r.namespace + "/" + r.nombre }

// eslabonable es lo que la cadena necesita de cada objeto de workload.
type eslabonable struct {
	dueno  *ref
	creado time.Time
	// visible dice si el objeto aparece en la cadena aunque no tenga pods: un ReplicaSet con
	// réplicas deseadas (está en uso o es parte de un rollout) o un Job.
	visible bool
}

// cadenas arma los workloads desde las ownerReferences y devuelve, además, el workload de cada
// pod (por namespace/nombre; ausente si es un Pod suelto).
func cadenas(o Objetos) ([]Workload, map[string]string) {
	objetos := map[ref]eslabonable{}
	agregar := func(tipo string, m metav1.ObjectMeta, visible bool) {
		objetos[ref{tipo, m.Namespace, m.Name}] = eslabonable{dueno: duenoDe(m), creado: m.CreationTimestamp.Time, visible: visible}
	}
	for _, d := range o.Deployments {
		agregar("Deployment", d.ObjectMeta, true)
	}
	for _, rs := range o.ReplicaSets {
		agregar("ReplicaSet", rs.ObjectMeta, rs.Spec.Replicas == nil || *rs.Spec.Replicas > 0)
	}
	for _, s := range o.StatefulSets {
		agregar("StatefulSet", s.ObjectMeta, true)
	}
	for _, d := range o.DaemonSets {
		agregar("DaemonSet", d.ObjectMeta, true)
	}
	for _, j := range o.Jobs {
		agregar("Job", j.ObjectMeta, true)
	}
	for _, c := range o.CronJobs {
		agregar("CronJob", c.ObjectMeta, true)
	}

	// raiz sube por los dueños hasta el de arriba. Si un dueño no está en las cachés (todavía no
	// llegó, o ya se borró), la cadena termina en él.
	raiz := func(r ref) ref {
		for range 4 {
			o, ok := objetos[r]
			if !ok || o.dueno == nil {
				break
			}
			r = *o.dueno
		}
		return r
	}

	workloads := map[ref]*Workload{}
	eslabones := map[ref]*Eslabon{}
	asegurar := func(r ref) *Eslabon {
		if e, ok := eslabones[r]; ok {
			return e
		}
		e := &Eslabon{Tipo: r.tipo, Nombre: r.nombre, Pods: []string{}}
		eslabones[r] = e
		arriba := raiz(r)
		if _, ok := workloads[arriba]; !ok {
			workloads[arriba] = &Workload{ID: arriba.id(), Tipo: arriba.tipo, Nombre: arriba.nombre, Namespace: arriba.namespace, Color: colorDe(arriba.id())}
		}
		return e
	}
	for r, o := range objetos {
		if o.dueno == nil || o.visible {
			asegurar(r)
		}
	}
	workloadDePod := map[string]string{}
	for _, p := range o.Pods {
		d := duenoDe(p.ObjectMeta)
		if d == nil {
			continue
		}
		e := asegurar(*d)
		e.Pods = append(e.Pods, p.Name)
		workloadDePod[p.Namespace+"/"+p.Name] = raiz(*d).id()
	}

	// cada eslabón va en la cadena de su workload: primero el de arriba, después los del medio
	// del más nuevo al más viejo
	medio := map[ref][]ref{}
	for r := range eslabones {
		if arriba := raiz(r); arriba != r {
			medio[arriba] = append(medio[arriba], r)
		}
	}
	lista := make([]Workload, 0, len(workloads))
	for arriba, w := range workloads {
		hijos := medio[arriba]
		sort.Slice(hijos, func(i, j int) bool {
			a, b := objetos[hijos[i]].creado, objetos[hijos[j]].creado
			if !a.Equal(b) {
				return a.After(b)
			}
			return hijos[i].nombre < hijos[j].nombre
		})
		for _, r := range append([]ref{arriba}, hijos...) {
			e := eslabones[r]
			if e == nil {
				// el de arriba no tiene pods propios (un Deployment o un CronJob)
				e = &Eslabon{Tipo: r.tipo, Nombre: r.nombre, Pods: []string{}}
			}
			sort.Strings(e.Pods)
			w.Cadena = append(w.Cadena, *e)
		}
		lista = append(lista, *w)
	}
	sort.Slice(lista, func(i, j int) bool {
		a, b := lista[i], lista[j]
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		if a.Nombre != b.Nombre {
			return a.Nombre < b.Nombre
		}
		return a.Tipo < b.Tipo
	})
	return lista, workloadDePod
}

// duenoDe es el controller del objeto según sus ownerReferences, si es un workload que kviz
// entiende. Sin controller marcado, usa la primera ownerReference.
func duenoDe(m metav1.ObjectMeta) *ref {
	var elegido *metav1.OwnerReference
	for i, o := range m.OwnerReferences {
		if o.Controller != nil && *o.Controller {
			elegido = &m.OwnerReferences[i]
			break
		}
		if elegido == nil {
			elegido = &m.OwnerReferences[i]
		}
	}
	if elegido == nil || !tiposDeWorkload[elegido.Kind] {
		return nil
	}
	return &ref{elegido.Kind, m.Namespace, elegido.Name}
}

// paletaDeWorkloads son colores distinguibles entre sí que evitan los que ya significan algo en
// la vista: azul del foco, rosa del recorrido, verde azulado y naranja de las puertas, rojo de error.
var paletaDeWorkloads = []string{
	"#0f9d8a", "#e0731b", "#8a4fd1", "#5d7a3a", "#b8860b", "#6d4c41", "#5c6bc0", "#7cb342", "#c06c84", "#455a64",
}

// colorDe da el color de un workload a partir de su id: el mismo id da siempre el mismo color,
// sin importar qué otros workloads haya en el cluster.
func colorDe(id string) string {
	h := fnv.New32a()
	h.Write([]byte(id))
	return paletaDeWorkloads[h.Sum32()%uint32(len(paletaDeWorkloads))]
}
