package api_test

// Advertencias de configuración (historias 70–75): los errores típicos que impiden que el tráfico
// llegue viajan en la foto, cada uno sobre su objeto, con explicación y comando sugerido.
// Cada regla tiene un caso que la dispara y uno parecido que no.

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Advertencia de la foto, tal como la lee la interfaz (web/src/foto.ts).
type advertenciaEnFoto struct {
	ID          string `json:"id"`
	Tipo        string `json:"tipo"`
	Namespace   string `json:"namespace"`
	Nombre      string `json:"nombre"`
	Regla       string `json:"regla"`
	Titulo      string `json:"titulo"`
	Explicacion string `json:"explicacion"`
	Comando     struct {
		Descripcion string `json:"descripcion"`
		Comando     string `json:"comando"`
	} `json:"comando"`
}

func (k *kvizDePrueba) advertencias(t *testing.T) []advertenciaEnFoto {
	t.Helper()
	var f struct {
		Advertencias []advertenciaEnFoto `json:"advertencias"`
	}
	k.getJSON(t, "/api/foto", &f)
	if f.Advertencias == nil {
		t.Fatal("la foto no trae la lista de advertencias")
	}
	return f.Advertencias
}

// unaAdvertencia exige que el objeto tenga exactamente una advertencia de la regla, con
// explicación y comando, y la devuelve.
func unaAdvertencia(t *testing.T, todas []advertenciaEnFoto, tipo, namespace, nombre, regla string) advertenciaEnFoto {
	t.Helper()
	var suyas []advertenciaEnFoto
	for _, a := range todas {
		if a.Tipo == tipo && a.Namespace == namespace && a.Nombre == nombre && a.Regla == regla {
			suyas = append(suyas, a)
		}
	}
	if len(suyas) != 1 {
		t.Fatalf("advertencias %s de %s %s/%s = %s, want una; todas: %s", regla, tipo, namespace, nombre, mostrar(suyas), mostrar(todas))
	}
	a := suyas[0]
	if a.ID == "" || a.Titulo == "" || a.Explicacion == "" || a.Comando.Descripcion == "" || !strings.HasPrefix(a.Comando.Comando, "kubectl ") {
		t.Fatalf("advertencia %s incompleta: falta id, título, explicación o comando", mostrar(a))
	}
	return a
}

// conID exige que la foto traiga la advertencia con ese id, con explicación y comando, y la devuelve.
func conID(t *testing.T, todas []advertenciaEnFoto, id string) advertenciaEnFoto {
	t.Helper()
	for _, a := range todas {
		if a.ID == id {
			if a.Titulo == "" || a.Explicacion == "" || a.Comando.Descripcion == "" || !strings.HasPrefix(a.Comando.Comando, "kubectl ") {
				t.Fatalf("advertencia %s incompleta: falta título, explicación o comando", mostrar(a))
			}
			return a
		}
	}
	t.Fatalf("no hay advertencia %s; todas: %s", id, mostrar(todas))
	return advertenciaEnFoto{}
}

// sinAdvertencias exige que la foto no traiga ninguna advertencia de la regla.
func sinAdvertencias(t *testing.T, todas []advertenciaEnFoto, regla string) {
	t.Helper()
	for _, a := range todas {
		if a.Regla == regla {
			t.Fatalf("advertencia %s, want ninguna de la regla %s", mostrar(a), regla)
		}
	}
}

// contiene exige que el texto diga cada una de las partes.
func contiene(t *testing.T, que, texto string, partes ...string) {
	t.Helper()
	for _, p := range partes {
		if !strings.Contains(texto, p) {
			t.Fatalf("%s = %q, want que diga %q", que, texto, p)
		}
	}
}

// podConLabels arma un pod corriendo con labels y un container que expone los puertos.
func podConLabels(namespace, nombre string, labels map[string]string, puertos ...corev1.ContainerPort) *corev1.Pod {
	p := pod(namespace, nombre, "lab-worker", "10.244.1.7", corev1.PodRunning)
	p.Labels = labels
	p.Spec.Containers = []corev1.Container{{Name: "app", Image: "lab-backend:1.0", Ports: puertos}}
	return p
}

func puertoContainer(numero int32, nombre string) corev1.ContainerPort {
	return corev1.ContainerPort{Name: nombre, ContainerPort: numero, Protocol: corev1.ProtocolTCP}
}

var appBackend = map[string]string{"app": "backend"}

// ---------- historia 70: Service sin pods o sin endpoints listos ----------

func TestUnServiceCuyoSelectorNoMatcheaNingunPodTieneAdvertencia(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab",
		nodo("lab-worker", "172.23.0.4"),
		podConLabels("default", "backend-1", map[string]string{"app": "beckend"}, puertoContainer(8081, "")),
		service("default", "backend", corev1.ServiceTypeClusterIP, "10.96.0.10", appBackend,
			corev1.ServicePort{Protocol: corev1.ProtocolTCP, Port: 80, TargetPort: intstr.FromInt32(8081)}),
	)

	a := unaAdvertencia(t, kviz.advertencias(t), "Service", "default", "backend", "service-sin-pods")

	contiene(t, "explicación", a.Explicacion, "app=backend")
	contiene(t, "comando", a.Comando.Comando, "kubectl get pods --show-labels", "-n default", "--context kind-lab")
}

func TestUnServiceConPodsYEndpointsListosNoTieneAdvertencias(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab",
		nodo("lab-worker", "172.23.0.4"),
		podConLabels("default", "backend-1", map[string]string{"app": "backend", "version": "v1"}, puertoContainer(8081, "")),
		service("default", "backend", corev1.ServiceTypeClusterIP, "10.96.0.10", appBackend,
			corev1.ServicePort{Protocol: corev1.ProtocolTCP, Port: 80, TargetPort: intstr.FromInt32(8081)}),
		endpointSlice("default", "backend-x", "backend", endpoint("10.244.1.7", "backend-1", "lab-worker", ptrBool(true))),
		// sin selector, los endpoints los arma otro: no es un Service roto
		service("default", "externo", corev1.ServiceTypeClusterIP, "10.96.0.11", nil,
			corev1.ServicePort{Protocol: corev1.ProtocolTCP, Port: 5432}),
	)

	if a := kviz.advertencias(t); len(a) != 0 {
		t.Fatalf("advertencias = %s, want ninguna", mostrar(a))
	}
}

func TestUnServiceConPodsPeroSinEndpointsListosTieneAdvertencia(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab",
		nodo("lab-worker", "172.23.0.4"),
		podConLabels("default", "backend-1", appBackend, puertoContainer(8081, "")),
		service("default", "backend", corev1.ServiceTypeClusterIP, "10.96.0.10", appBackend,
			corev1.ServicePort{Protocol: corev1.ProtocolTCP, Port: 80, TargetPort: intstr.FromInt32(8081)}),
		endpointSlice("default", "backend-x", "backend", endpoint("10.244.1.7", "backend-1", "lab-worker", ptrBool(false))),
	)

	todas := kviz.advertencias(t)

	a := unaAdvertencia(t, todas, "Service", "default", "backend", "service-sin-endpoints-listos")
	contiene(t, "explicación", a.Explicacion, "1 pod", "ninguno está listo")
	contiene(t, "comando", a.Comando.Comando, "kubectl get pods -o wide -l app=backend -n default")
	sinAdvertencias(t, todas, "service-sin-pods")
}

// ---------- historia 71: targetPort que ningún container expone ----------

func TestUnTargetPortQueNingunContainerExponeTieneAdvertencia(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab",
		nodo("lab-worker", "172.23.0.4"),
		podConLabels("default", "backend-1", appBackend, puertoContainer(8081, "")),
		podConLabels("default", "backend-2", appBackend, puertoContainer(8081, "")),
		service("default", "backend", corev1.ServiceTypeClusterIP, "10.96.0.10", appBackend,
			corev1.ServicePort{Name: "web", Protocol: corev1.ProtocolTCP, Port: 80, TargetPort: intstr.FromInt32(8080)},
			corev1.ServicePort{Name: "api", Protocol: corev1.ProtocolTCP, Port: 81, TargetPort: intstr.FromString("http")}),
		endpointSlice("default", "backend-x", "backend", endpoint("10.244.1.7", "backend-1", "lab-worker", ptrBool(true))),
	)

	todas := kviz.advertencias(t)

	// una por cada puerto del Service cuyo targetPort no se expone, con el puerto como detalle
	porNumero := conID(t, todas, "Service/default/backend/target-port-no-expuesto/8080")
	contiene(t, "explicación", porNumero.Explicacion, "8080", "8081")
	contiene(t, "comando", porNumero.Comando.Comando, "kubectl get pods -l app=backend", "-n default", "--context kind-lab")
	porNombre := conID(t, todas, "Service/default/backend/target-port-no-expuesto/http")
	contiene(t, "explicación", porNombre.Explicacion, `"http"`)
	if len(todas) != 2 {
		t.Fatalf("advertencias = %s, want solo las dos de targetPort", mostrar(todas))
	}
}

func TestUnTargetPortQueExponeAlgunContainerNoTieneAdvertencia(t *testing.T) {
	// el pod intruso de la guía (6a) matchea el selector sin exponer puertos, pero los de backend sí
	// exponen el 8081; y un container sin puertos declarados no dice nada de lo que escucha
	intruso := podConLabels("default", "intruso", appBackend)
	kviz := nuevoKviz(t, "kind-lab",
		nodo("lab-worker", "172.23.0.4"),
		podConLabels("default", "backend-1", appBackend, puertoContainer(8081, "http")),
		intruso,
		service("default", "backend", corev1.ServiceTypeClusterIP, "10.96.0.10", appBackend,
			corev1.ServicePort{Name: "web", Protocol: corev1.ProtocolTCP, Port: 80, TargetPort: intstr.FromInt32(8081)},
			corev1.ServicePort{Name: "api", Protocol: corev1.ProtocolTCP, Port: 81, TargetPort: intstr.FromString("http")}),
		podConLabels("otro", "sin-puertos", map[string]string{"app": "web"}),
		service("otro", "web", corev1.ServiceTypeClusterIP, "10.96.0.12", map[string]string{"app": "web"},
			corev1.ServicePort{Protocol: corev1.ProtocolTCP, Port: 80, TargetPort: intstr.FromInt32(8080)}),
		endpointSlice("default", "backend-x", "backend", endpoint("10.244.1.7", "backend-1", "lab-worker", ptrBool(true))),
		endpointSlice("otro", "web-x", "web", endpoint("10.244.1.8", "sin-puertos", "lab-worker", ptrBool(true))),
	)

	sinAdvertencias(t, kviz.advertencias(t), "target-port-no-expuesto")
}

// ---------- historia 72: probe a un puerto que el container no expone ----------

func probeHTTP(puerto intstr.IntOrString) *corev1.Probe {
	return &corev1.Probe{ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/status", Port: puerto}}}
}

func TestUnaProbeAUnPuertoQueElContainerNoExponeTieneAdvertencia(t *testing.T) {
	// el caso del prototipo: liveness a :8000 con el container en :8080
	p := podConLabels("default", "api-1", nil, puertoContainer(8080, "http"))
	p.Spec.Containers[0].LivenessProbe = probeHTTP(intstr.FromInt32(8000))
	p.Spec.Containers[0].ReadinessProbe = &corev1.Probe{ProbeHandler: corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromString("metricas")}}}
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), p)

	todas := kviz.advertencias(t)

	liveness := conID(t, todas, "Pod/default/api-1/probe-puerto-no-expuesto/app/liveness")
	contiene(t, "explicación", liveness.Explicacion, "liveness", "8000", "app", "8080")
	contiene(t, "comando", liveness.Comando.Comando, "kubectl describe pod api-1 -n default --context kind-lab")
	readiness := conID(t, todas, "Pod/default/api-1/probe-puerto-no-expuesto/app/readiness")
	contiene(t, "explicación", readiness.Explicacion, `"metricas"`)
	if len(todas) != 2 {
		t.Fatalf("advertencias = %s, want solo las dos de probes", mostrar(todas))
	}
}

func TestUnaProbeAUnPuertoQueElContainerExponeNoTieneAdvertencia(t *testing.T) {
	// 4-pod.yaml de la guía: liveness y readiness a /status:8080 con el container en 8080
	guia := podConLabels("default", "backend", nil, puertoContainer(8080, ""))
	guia.Spec.Containers[0].LivenessProbe = probeHTTP(intstr.FromInt32(8080))
	guia.Spec.Containers[0].ReadinessProbe = probeHTTP(intstr.FromInt32(8080))
	porNombre := podConLabels("default", "api-1", nil, puertoContainer(8080, "http"))
	porNombre.Spec.Containers[0].StartupProbe = probeHTTP(intstr.FromString("http"))
	// sin puertos declarados no se sabe en cuál escucha; un exec no usa puerto
	sinPuertos := podConLabels("default", "db-0", nil)
	sinPuertos.Spec.Containers[0].LivenessProbe = &corev1.Probe{ProbeHandler: corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt32(5432)}}}
	sinPuertos.Spec.Containers[0].ReadinessProbe = &corev1.Probe{ProbeHandler: corev1.ProbeHandler{Exec: &corev1.ExecAction{Command: []string{"pg_isready"}}}}
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), guia, porNombre, sinPuertos)

	sinAdvertencias(t, kviz.advertencias(t), "probe-puerto-no-expuesto")
}

// ---------- historia 73: ImagePullBackOff de una imagen que parece local ----------

// podConImagen arma un pod con un container de esa imagen y policy, esperando por el motivo dado
// ("" si corre).
func podConImagen(nombre, imagen string, policy corev1.PullPolicy, espera string) *corev1.Pod {
	p := pod("default", nombre, "lab-worker", "", corev1.PodPending)
	p.Spec.Containers = []corev1.Container{{Name: "application", Image: imagen, ImagePullPolicy: policy}}
	estado := corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}
	if espera != "" {
		estado = corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: espera}}
	} else {
		p.Status.Phase = corev1.PodRunning
	}
	p.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "application", Image: imagen, State: estado}}
	return p
}

func TestUnImagePullBackOffDeUnaImagenLocalSugiereRevisarLaImagePullPolicy(t *testing.T) {
	// 2-pod.yaml de la guía sin imagePullPolicy: con :latest Kubernetes la baja siempre (Always)
	sinPolicy := podConImagen("backend", "lab-backend:latest", "", "ImagePullBackOff")
	// con IfNotPresent, la imagen no está cargada en el nodo
	noCargada := podConImagen("backend-v2", "lab-backend:2.0", corev1.PullIfNotPresent, "ErrImagePull")
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), sinPolicy, noCargada)

	todas := kviz.advertencias(t)

	a := conID(t, todas, "Pod/default/backend/imagen-local-sin-bajar/application")
	contiene(t, "explicación", a.Explicacion, "lab-backend:latest", "imagePullPolicy", "Always", "IfNotPresent", "kind load docker-image lab-backend:latest --name lab")
	contiene(t, "comando", a.Comando.Comando, "kubectl get pod backend", "imagePullPolicy", "-n default --context kind-lab")
	b := conID(t, todas, "Pod/default/backend-v2/imagen-local-sin-bajar/application")
	contiene(t, "explicación", b.Explicacion, "IfNotPresent", "lab-worker", "kind load docker-image lab-backend:2.0 --name lab")
}

func TestUnImagePullBackOffDeUnRegistryOUnaImagenLocalQueCorreNoTienenAdvertencia(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"),
		podConImagen("remota", "ghcr.io/lab/backend:1.0", corev1.PullIfNotPresent, "ImagePullBackOff"),
		podConImagen("local", "localhost:5001/backend:1.0", "", "ImagePullBackOff"),
		// 2-pod.yaml tal cual: la imagen local con IfNotPresent corre
		podConImagen("backend", "lab-backend:latest", corev1.PullIfNotPresent, ""),
		podConImagen("creando", "lab-backend:latest", "", "ContainerCreating"),
	)

	sinAdvertencias(t, kviz.advertencias(t), "imagen-local-sin-bajar")
}

// ---------- historia 74: Pending por falta de recursos ----------

// podSinLugar arma un pod Pending que el scheduler no pudo ubicar, con su mensaje.
func podSinLugar(nombre, mensaje string) *corev1.Pod {
	p := pod("default", nombre, "", "", corev1.PodPending)
	p.Spec.Containers = []corev1.Container{{Name: "app", Image: "lab-backend:1.0", Resources: corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4"), corev1.ResourceMemory: resource.MustParse("8Gi")},
	}}}
	p.Status.Conditions = []corev1.PodCondition{{
		Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: corev1.PodReasonUnschedulable, Message: mensaje,
	}}
	return p
}

func TestUnPodPendingPorFaltaDeRecursosTieneAdvertencia(t *testing.T) {
	const mensaje = "0/3 nodes are available: 3 Insufficient cpu, 3 Insufficient memory. preemption: 0/3 nodes are available: 3 No preemption victims found for incoming pod."
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), podSinLugar("pesado", mensaje))

	a := unaAdvertencia(t, kviz.advertencias(t), "Pod", "default", "pesado", "pending-sin-recursos")

	contiene(t, "explicación", a.Explicacion, "3 Insufficient cpu, 3 Insufficient memory", "cpu 4", "memoria 8Gi")
	contiene(t, "comando", a.Comando.Comando, "kubectl describe nodes --context kind-lab")
}

func TestUnPodPendingPorOtroMotivoNoTieneAdvertenciaDeRecursos(t *testing.T) {
	creando := pod("default", "creando", "lab-worker", "", corev1.PodPending)
	creando.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "app", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}}}}
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"),
		podSinLugar("elegido", "0/3 nodes are available: 3 node(s) didn't match Pod's node affinity/selector."),
		creando,
	)

	sinAdvertencias(t, kviz.advertencias(t), "pending-sin-recursos")
}

// ---------- historia 75: Ingress sin IngressClass o hacia un Service que no existe ----------

func ingressHacia(nombre string, clase *string, services ...string) *networkingv1.Ingress {
	prefijo := networkingv1.PathTypePrefix
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: nombre},
		Spec:       networkingv1.IngressSpec{IngressClassName: clase},
	}
	for _, s := range services {
		ing.Spec.Rules = append(ing.Spec.Rules, reglaIngress("", "/"+s, &prefijo, backendIngress(s, 80, "")))
	}
	return ing
}

func ingressClass(nombre string, porDefecto bool) *networkingv1.IngressClass {
	c := &networkingv1.IngressClass{ObjectMeta: metav1.ObjectMeta{Name: nombre}, Spec: networkingv1.IngressClassSpec{Controller: "k8s.io/ingress-nginx"}}
	if porDefecto {
		c.Annotations = map[string]string{"ingressclass.kubernetes.io/is-default-class": "true"}
	}
	return c
}

func serviceBackend() *corev1.Service {
	return service("default", "backend", corev1.ServiceTypeClusterIP, "10.96.0.10", nil, corev1.ServicePort{Protocol: corev1.ProtocolTCP, Port: 80})
}

func TestUnIngressSinIngressClassTieneAdvertencia(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", serviceBackend(), ingressClass("nginx", false),
		ingressHacia("sin-clase", nil, "backend"),
		ingressHacia("otra-clase", ptr("traefik"), "backend"),
	)

	todas := kviz.advertencias(t)

	a := unaAdvertencia(t, todas, "Ingress", "default", "sin-clase", "ingress-sin-clase")
	contiene(t, "explicación", a.Explicacion, "ingressClassName", "nginx")
	contiene(t, "comando", a.Comando.Comando, "kubectl get ingressclass --context kind-lab")
	b := unaAdvertencia(t, todas, "Ingress", "default", "otra-clase", "ingress-sin-clase")
	contiene(t, "explicación", b.Explicacion, `"traefik"`, "no existe")
}

func TestUnIngressHaciaUnServiceQueNoExisteTieneAdvertencia(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", serviceBackend(), ingressClass("nginx", false),
		ingressHacia("roto", ptr("nginx"), "backend", "backnd", "backnd"),
	)

	todas := kviz.advertencias(t)

	a := conID(t, todas, "Ingress/default/roto/ingress-service-inexistente/backnd")
	contiene(t, "explicación", a.Explicacion, `"backnd"`, "default")
	contiene(t, "comando", a.Comando.Comando, "kubectl get services -n default --context kind-lab")
	if len(todas) != 1 {
		t.Fatalf("advertencias = %s, want solo la del Service backnd, una vez", mostrar(todas))
	}
}

func TestUnIngressConClaseOConClasePorDefectoYSusServicesNoTieneAdvertencias(t *testing.T) {
	vieja := ingressHacia("con-anotacion", nil, "backend")
	vieja.Annotations = map[string]string{"kubernetes.io/ingress.class": "nginx"}
	kviz := nuevoKviz(t, "kind-lab", serviceBackend(), ingressClass("nginx", true),
		ingressHacia("con-clase", ptr("nginx"), "backend"),
		ingressHacia("por-defecto", nil, "backend"),
		vieja,
	)

	if a := kviz.advertencias(t); len(a) != 0 {
		t.Fatalf("advertencias = %s, want ninguna", mostrar(a))
	}
}

func TestElClusterDeLaGuiaNoTieneAdvertencias(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)

	if a := kviz.advertencias(t); len(a) != 0 {
		t.Fatalf("advertencias = %s, want ninguna", mostrar(a))
	}
}

func TestLaAdvertenciaSeVaEnVivoCuandoAparecenLaIngressClassYElService(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", ingressHacia("backend", ptr("nginx"), "backend"))
	stream := kviz.abrirStream(t)
	m := stream.siguiente(t)
	if m.Foto == nil {
		t.Fatalf("primer mensaje = %s, want la foto", mostrar(m))
	}

	ctx := context.Background()
	if _, err := kviz.cliente.NetworkingV1().IngressClasses().Create(ctx, ingressClass("nginx", false), metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if c := stream.esperarCambio(t, "advertencias", "Ingress/default/backend/ingress-sin-clase"); !c.Borrado {
		t.Fatalf("cambio = %s, want que la advertencia se vaya", mostrar(c))
	}
	if _, err := kviz.cliente.CoreV1().Services("default").Create(ctx, serviceBackend(), metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if c := stream.esperarCambio(t, "advertencias", "Ingress/default/backend/ingress-service-inexistente/backend"); !c.Borrado {
		t.Fatalf("cambio = %s, want que la advertencia se vaya", mostrar(c))
	}
}
