package api_test

// Containers y estado del pod en la foto (ticket 03).

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Contrato JSON de los containers del pod y del estado al estilo kubectl get pods.
type podConContainers struct {
	Nombre     string            `json:"nombre"`
	Estado     string            `json:"estado"`
	Containers []containerEnFoto `json:"containers"`
}

type containerEnFoto struct {
	Nombre    string         `json:"nombre"`
	Tipo      string         `json:"tipo"`
	Imagen    string         `json:"imagen"`
	Puertos   []puertoEnFoto `json:"puertos"`
	Estado    string         `json:"estado"`
	Ready     bool           `json:"ready"`
	Started   bool           `json:"started"`
	Reinicios int32          `json:"reinicios"`
}

type puertoEnFoto struct {
	Numero    int32  `json:"numero"`
	Nombre    string `json:"nombre,omitempty"`
	Protocolo string `json:"protocolo"`
}

func (k *kvizDePrueba) podsConContainers(t *testing.T) map[string]podConContainers {
	t.Helper()
	var f struct {
		Pods []podConContainers `json:"pods"`
	}
	k.getJSON(t, "/api/foto", &f)
	porNombre := map[string]podConContainers{}
	for _, p := range f.Pods {
		porNombre[p.Nombre] = p
	}
	return porNombre
}

func TestUnPodInicializandoTraeSuInitContainerYEstadoInit(t *testing.T) {
	p := pod("default", "web-7c9d", "lab-worker", "10.244.1.9", corev1.PodPending)
	p.Spec.InitContainers = []corev1.Container{{Name: "cache-warmer", Image: "busybox:1.36"}}
	p.Spec.Containers = []corev1.Container{{
		Name:  "web",
		Image: "nginx:1.27",
		Ports: []corev1.ContainerPort{{Name: "http", ContainerPort: 80, Protocol: corev1.ProtocolTCP}},
	}}
	p.Status.InitContainerStatuses = []corev1.ContainerStatus{{
		Name:    "cache-warmer",
		Image:   "busybox:1.36",
		State:   corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
		Started: ptrBool(true),
	}}
	p.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name:  "web",
		Image: "nginx:1.27",
		State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "PodInitializing"}},
	}}
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), p)

	got := kviz.podsConContainers(t)["web-7c9d"]

	want := podConContainers{
		Nombre: "web-7c9d",
		Estado: "Init:0/1",
		Containers: []containerEnFoto{
			{Nombre: "cache-warmer", Tipo: "init", Imagen: "busybox:1.36", Puertos: []puertoEnFoto{}, Estado: "Running", Started: true},
			{Nombre: "web", Tipo: "normal", Imagen: "nginx:1.27", Puertos: []puertoEnFoto{{Numero: 80, Nombre: "http", Protocolo: "TCP"}}, Estado: "PodInitializing"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pod = %s\nwant  %s", mostrar(got), mostrar(want))
	}
}

func TestUnPodConKubectlDebugTraeElEphemeralContainer(t *testing.T) {
	p := pod("default", "backend", "lab-worker", "10.244.1.8", corev1.PodRunning)
	p.Spec.Containers = []corev1.Container{{
		Name:  "application",
		Image: "lab-backend:latest",
		Ports: []corev1.ContainerPort{{ContainerPort: 8080}},
	}}
	p.Spec.EphemeralContainers = []corev1.EphemeralContainer{{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{Name: "debugger-7xq9z", Image: "busybox:1.36"},
		TargetContainerName:      "application",
	}}
	p.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name: "application", Image: "lab-backend:latest", Ready: true, Started: ptrBool(true),
		State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
	}}
	p.Status.EphemeralContainerStatuses = []corev1.ContainerStatus{{
		Name: "debugger-7xq9z", Image: "busybox:1.36",
		State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}},
	}}
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), p)

	got := kviz.podsConContainers(t)["backend"]

	want := podConContainers{
		Nombre: "backend",
		Estado: "Running",
		Containers: []containerEnFoto{
			{Nombre: "application", Tipo: "normal", Imagen: "lab-backend:latest", Puertos: []puertoEnFoto{{Numero: 8080, Protocolo: "TCP"}}, Estado: "Running", Ready: true, Started: true},
			{Nombre: "debugger-7xq9z", Tipo: "ephemeral", Imagen: "busybox:1.36", Puertos: []puertoEnFoto{}, Estado: "Running"},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pod = %s\nwant  %s", mostrar(got), mostrar(want))
	}
}

func TestUnPodEnCrashLoopBackOffTraeSusReinicios(t *testing.T) {
	p := pod("default", "api-5d8f", "lab-worker2", "10.244.2.7", corev1.PodRunning)
	p.Spec.Containers = []corev1.Container{{
		Name:  "api",
		Image: "api:1.0",
		Ports: []corev1.ContainerPort{{ContainerPort: 8080, Protocol: corev1.ProtocolTCP}},
	}}
	p.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name: "api", Image: "api:1.0", RestartCount: 7, Started: ptrBool(false),
		State:                corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
		LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Error", ExitCode: 137}},
	}}
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker2", "172.23.0.3"), p)

	got := kviz.podsConContainers(t)["api-5d8f"]

	want := podConContainers{
		Nombre: "api-5d8f",
		Estado: "CrashLoopBackOff",
		Containers: []containerEnFoto{
			{Nombre: "api", Tipo: "normal", Imagen: "api:1.0", Puertos: []puertoEnFoto{{Numero: 8080, Protocolo: "TCP"}}, Estado: "CrashLoopBackOff", Reinicios: 7},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pod = %s\nwant  %s", mostrar(got), mostrar(want))
	}
}

// Columna STATUS de kubectl get pods para los casos que la tarjeta tiene que mostrar.
func TestElEstadoDelPodEsElDeKubectlGetPods(t *testing.T) {
	terminado := func(motivo string, codigo int32) corev1.ContainerState {
		return corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: motivo, ExitCode: codigo}}
	}
	casos := []struct {
		nombre string
		armar  func(p *corev1.Pod)
		want   string
	}{
		{"job-terminado", func(p *corev1.Pod) {
			p.Status.Phase = corev1.PodSucceeded
			p.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "app", State: terminado("Completed", 0)}}
		}, "Completed"},
		{"sin-nodo", func(p *corev1.Pod) {
			p.Spec.NodeName = ""
			p.Status.Phase = corev1.PodPending
		}, "Pending"},
		{"imagen-que-no-baja", func(p *corev1.Pod) {
			p.Status.Phase = corev1.PodPending
			p.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "app", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}}}}
		}, "ImagePullBackOff"},
		{"init-que-falla", func(p *corev1.Pod) {
			p.Status.Phase = corev1.PodPending
			p.Spec.InitContainers = []corev1.Container{{Name: "migrar"}}
			p.Status.InitContainerStatuses = []corev1.ContainerStatus{{Name: "migrar", State: terminado("Error", 1)}}
		}, "Init:Error"},
		{"borrandose", func(p *corev1.Pod) {
			ahora := metav1.Now()
			p.DeletionTimestamp = &ahora
			p.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "app", Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}}
		}, "Terminating"},
	}
	var objetos []runtime.Object
	for _, c := range casos {
		p := pod("default", c.nombre, "lab-worker", "10.244.1.20", corev1.PodRunning)
		p.Spec.Containers = []corev1.Container{{Name: "app", Image: "app:1"}}
		c.armar(p)
		objetos = append(objetos, p)
	}
	kviz := nuevoKviz(t, "kind-lab", append(objetos, nodo("lab-worker", "172.23.0.4"))...)

	pods := kviz.podsConContainers(t)

	for _, c := range casos {
		if got := pods[c.nombre].Estado; got != c.want {
			t.Errorf("%s: estado = %q, want %q", c.nombre, got, c.want)
		}
	}
}

func ptrBool(b bool) *bool { return &b }
