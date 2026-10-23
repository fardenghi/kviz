package advertencias

// Historias 72–74: las advertencias de cada pod.

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (r *reglas) pods() {
	for _, p := range r.o.Pods {
		ob := objeto{"Pod", p.Namespace, p.Name}
		r.sinRecursos(ob, p)
		for _, c := range p.Spec.InitContainers {
			r.probes(ob, c)
			r.imagen(ob, p, c, "initContainers", p.Status.InitContainerStatuses)
		}
		for _, c := range p.Spec.Containers {
			r.probes(ob, c)
			r.imagen(ob, p, c, "containers", p.Status.ContainerStatuses)
		}
	}
}

// ---------- historia 73: ImagePullBackOff de una imagen que parece local ----------

// imagen marca el container que no puede bajar su imagen (ImagePullBackOff o ErrImagePull) cuando
// la imagen parece local: no nombra un registry, como las que se construyen en la máquina y se
// cargan en el nodo con kind load. campo es donde está el container en el spec, para el comando.
func (r *reglas) imagen(ob objeto, p *corev1.Pod, c corev1.Container, campo string, estados []corev1.ContainerStatus) {
	if !sinPoderBajar(c.Name, estados) || !pareceLocal(c.Image) {
		return
	}
	cargar := "kind load docker-image " + c.Image
	if cluster, ok := strings.CutPrefix(r.f.Contexto, "kind-"); ok && cluster != "" {
		cargar += " --name " + cluster
	}
	nodo := "su nodo"
	if p.Spec.NodeName != "" {
		nodo = "el nodo " + p.Spec.NodeName
	}
	policy := c.ImagePullPolicy
	if policy == "" {
		policy = policyPorDefecto(c.Image)
	}
	var porQue string
	switch policy {
	case corev1.PullAlways:
		porQue = fmt.Sprintf("Su imagePullPolicy es Always (la que Kubernetes pone por defecto con :latest o sin tag), así que intenta bajarla de un registry aunque ya esté cargada en %s. Si la construiste vos, usá imagePullPolicy: IfNotPresent (o Never) y cargala en el cluster con %s.", nodo, cargar)
	default:
		porQue = fmt.Sprintf("Su imagePullPolicy es %s, así que la imagen no está cargada en %s y Kubernetes intenta bajarla de un registry. Si la construiste vos, cargala en el cluster con %s.", policy, nodo, cargar)
	}
	r.agregar(ob, "imagen-local-sin-bajar", c.Name, "ImagePullBackOff de una imagen que parece local",
		fmt.Sprintf("El container %s no puede bajar la imagen %s, que no nombra un registry: parece una imagen local. %s", c.Name, c.Image, porQue),
		r.kubectl("Ver la imagePullPolicy del container "+c.Name, ob.namespace, "get pod "+ob.nombre,
			"-o", fmt.Sprintf(`'jsonpath={.spec.%s[?(@.name=="%s")].imagePullPolicy}'`, campo, c.Name)))
}

func sinPoderBajar(container string, estados []corev1.ContainerStatus) bool {
	for _, s := range estados {
		if s.Name == container && s.State.Waiting != nil {
			motivo := s.State.Waiting.Reason
			return motivo == "ImagePullBackOff" || motivo == "ErrImagePull"
		}
	}
	return false
}

// pareceLocal dice si la imagen no nombra un registry: su primera parte no es un host
// (registry.k8s.io/…, ghcr.io/…, localhost:5001/…).
func pareceLocal(imagen string) bool {
	primera, _, conBarra := strings.Cut(imagen, "/")
	if !conBarra {
		return true
	}
	return !strings.ContainsAny(primera, ".:") && primera != "localhost"
}

// policyPorDefecto es la imagePullPolicy que pone Kubernetes si no se dice: Always con :latest o
// sin tag, IfNotPresent con cualquier otro tag o con digest.
func policyPorDefecto(imagen string) corev1.PullPolicy {
	if strings.Contains(imagen, "@") {
		return corev1.PullIfNotPresent
	}
	ultima := imagen[strings.LastIndex(imagen, "/")+1:]
	_, tag, conTag := strings.Cut(ultima, ":")
	if !conTag || tag == "latest" {
		return corev1.PullAlways
	}
	return corev1.PullIfNotPresent
}

// ---------- historia 74: Pending por falta de recursos ----------

// sinRecursos marca el pod Pending que el scheduler no puede ubicar porque a ningún nodo le
// queda lo que pide (el mensaje de PodScheduled dice "Insufficient cpu", "Insufficient memory"…).
func (r *reglas) sinRecursos(ob objeto, p *corev1.Pod) {
	if p.Status.Phase != corev1.PodPending || p.Spec.NodeName != "" {
		return
	}
	for _, c := range p.Status.Conditions {
		if c.Type != corev1.PodScheduled || c.Status != corev1.ConditionFalse || c.Reason != corev1.PodReasonUnschedulable || !strings.Contains(c.Message, "Insufficient") {
			continue
		}
		pide := ""
		if req := requests(p); req != "" {
			pide = " El pod pide " + req + " (requests), más de lo que le queda libre a cada nodo."
		}
		r.agregar(ob, "pending-sin-recursos", "", "Pending: ningún nodo tiene lugar",
			fmt.Sprintf("El scheduler no encuentra un nodo con recursos para el pod: %q.%s Bajá los requests, achicá otros pods o sumá un nodo.", c.Message, pide),
			r.kubectl("Ver cuánto CPU y memoria ya está pedido en cada nodo (Allocated resources)", "", "describe nodes"))
		return
	}
}

// requests es lo que piden en total sus containers: "cpu 4, memoria 8Gi"; "" si no piden nada.
func requests(p *corev1.Pod) string {
	var partes []string
	for _, rec := range []struct {
		nombre string
		r      corev1.ResourceName
	}{{"cpu", corev1.ResourceCPU}, {"memoria", corev1.ResourceMemory}} {
		var total resource.Quantity
		for _, c := range p.Spec.Containers {
			if q, ok := c.Resources.Requests[rec.r]; ok {
				total.Add(q)
			}
		}
		if !total.IsZero() {
			partes = append(partes, rec.nombre+" "+total.String())
		}
	}
	return strings.Join(partes, ", ")
}

// ---------- historia 72: probe a un puerto que el container no expone ----------

// probes marca cada probe del container que apunta a un puerto que él no expone. Por nombre, el
// puerto tiene que existir; por número, solo se puede juzgar si el container declara puertos.
func (r *reglas) probes(ob objeto, c corev1.Container) {
	for _, pr := range []struct {
		tipo  string
		probe *corev1.Probe
	}{{"liveness", c.LivenessProbe}, {"readiness", c.ReadinessProbe}, {"startup", c.StartupProbe}} {
		puerto, ok := puertoDeProbe(pr.probe)
		if !ok || expone(c, puerto) {
			continue
		}
		cual := "el " + puerto.String()
		if puerto.Type == intstr.String {
			cual = fmt.Sprintf("el puerto llamado %q", puerto.StrVal)
		}
		queExpone := "no declara ningún puerto"
		if len(c.Ports) > 0 {
			ps := map[int32]bool{}
			for _, p := range c.Ports {
				ps[p.ContainerPort] = true
			}
			queExpone = "expone " + listaDePuertos(ps)
		}
		r.agregar(ob, "probe-puerto-no-expuesto", c.Name+"/"+pr.tipo,
			fmt.Sprintf("La %s probe apunta a un puerto que el container no expone", pr.tipo),
			fmt.Sprintf("La %s probe del container %s apunta a %s, pero el container %s. La probe va a fallar: corregí el puerto de la probe o el containerPort.", pr.tipo, c.Name, cual, queExpone),
			r.kubectl("Ver la probe, los puertos del container y los events Unhealthy", ob.namespace, "describe pod", ob.nombre))
	}
}

// puertoDeProbe es el puerto al que apunta la probe; ok es false si no usa uno (exec) o no hay probe.
func puertoDeProbe(p *corev1.Probe) (intstr.IntOrString, bool) {
	switch {
	case p == nil:
		return intstr.IntOrString{}, false
	case p.HTTPGet != nil:
		return p.HTTPGet.Port, true
	case p.TCPSocket != nil:
		return p.TCPSocket.Port, true
	case p.GRPC != nil:
		return intstr.FromInt32(p.GRPC.Port), true
	}
	return intstr.IntOrString{}, false
}

func expone(c corev1.Container, puerto intstr.IntOrString) bool {
	if puerto.Type == intstr.Int && len(c.Ports) == 0 {
		return true // sin puertos declarados no se puede saber en cuál escucha
	}
	for _, p := range c.Ports {
		if (puerto.Type == intstr.String && p.Name == puerto.StrVal) || (puerto.Type == intstr.Int && p.ContainerPort == puerto.IntVal) {
			return true
		}
	}
	return false
}
