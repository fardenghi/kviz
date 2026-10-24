package api_test

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// hostPorts y puertas de la foto, tal como los lee la interfaz (web/src/foto.ts).
type hostPortEnFoto struct {
	Puerto        int32  `json:"puerto"`
	Protocolo     string `json:"protocolo"`
	Container     string `json:"container"`
	ContainerPort int32  `json:"containerPort"`
}

type puertaEnFoto struct {
	ID          string  `json:"id"`
	Nodo        string  `json:"nodo"`
	Puerto      int32   `json:"puerto"`
	Protocolo   string  `json:"protocolo"`
	Tipo        string  `json:"tipo"`
	Namespace   string  `json:"namespace"`
	Service     *string `json:"service"`
	Pod         *string `json:"pod"`
	HaciaPuerto int32   `json:"haciaPuerto"`
	// IngressController marca las puertas de hostPort del Ingress Controller.
	IngressController bool `json:"ingressController"`
}

type fotoConPuertas struct {
	Pods []struct {
		Nombre    string           `json:"nombre"`
		HostPorts []hostPortEnFoto `json:"hostPorts"`
	} `json:"pods"`
	Puertas []puertaEnFoto `json:"puertas"`
}

func (k *kvizDePrueba) fotoConPuertas(t *testing.T) fotoConPuertas {
	t.Helper()
	var f fotoConPuertas
	k.getJSON(t, "/api/foto", &f)
	return f
}

func TestLaFotoTraeLosHostPortsDeCadaPod(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)

	f := kviz.fotoConPuertas(t)

	hostPorts := map[string][]hostPortEnFoto{}
	for _, p := range f.Pods {
		hostPorts[p.Nombre] = p.HostPorts
	}
	want := map[string][]hostPortEnFoto{
		"backend-79f9ffc666-7hw4q": {},
		"backend-79f9ffc666-k2x8p": {},
		"ingress-nginx-controller-6b9f-lx2wp": {
			{Puerto: 80, Protocolo: "TCP", Container: "controller", ContainerPort: 80},
			{Puerto: 443, Protocolo: "TCP", Container: "controller", ContainerPort: 443},
		},
	}
	if !reflect.DeepEqual(hostPorts, want) {
		t.Fatalf("hostPorts por pod = %s, want %s", mostrar(hostPorts), mostrar(want))
	}
}

func TestUnServiceNodePortEnUnClusterDeTresNodosAbreTresPuertas(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab",
		nodo("lab-control-plane", "172.23.0.2"),
		nodo("lab-worker", "172.23.0.4"),
		nodo("lab-worker2", "172.23.0.3"),
		service("default", "backend", corev1.ServiceTypeNodePort, "10.96.11.121", map[string]string{"app": "backend"},
			corev1.ServicePort{Protocol: corev1.ProtocolTCP, Port: 80, TargetPort: intstr.FromInt32(8081), NodePort: 30010}),
		service("default", "interno", corev1.ServiceTypeClusterIP, "10.96.0.20", map[string]string{"app": "interno"},
			corev1.ServicePort{Protocol: corev1.ProtocolTCP, Port: 80}),
	)

	got := kviz.fotoConPuertas(t).Puertas

	puerta := func(nodo string) puertaEnFoto {
		return puertaEnFoto{ID: nodo + "/30010/TCP", Nodo: nodo, Puerto: 30010, Protocolo: "TCP", Tipo: "NodePort", Namespace: "default",
			Service: ptr("backend"), HaciaPuerto: 80}
	}
	want := []puertaEnFoto{puerta("lab-control-plane"), puerta("lab-worker"), puerta("lab-worker2")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("puertas = %s, want %s", mostrar(got), mostrar(want))
	}
}

func TestUnHostPortAbreUnaSolaPuertaEnElNodoDeSuPod(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)

	got := kviz.fotoConPuertas(t).Puertas

	hostPorts := []puertaEnFoto{}
	for _, p := range got {
		if p.Tipo == "hostPort" {
			hostPorts = append(hostPorts, p)
		}
	}
	puerta := func(puerto int32) puertaEnFoto {
		return puertaEnFoto{ID: "lab-worker2/" + itoa(puerto) + "/TCP", Nodo: "lab-worker2", Puerto: puerto, Protocolo: "TCP", Tipo: "hostPort", Namespace: "ingress-nginx",
			Pod: ptr("ingress-nginx-controller-6b9f-lx2wp"), HaciaPuerto: puerto, IngressController: true}
	}
	// por id: el orden es el del texto, no el numérico (la escena las ordena para dibujarlas)
	want := []puertaEnFoto{puerta(443), puerta(80)}
	if !reflect.DeepEqual(hostPorts, want) {
		t.Fatalf("puertas de hostPort = %s, want %s", mostrar(hostPorts), mostrar(want))
	}
}

func TestLasPuertasDeLaGuiaVanOrdenadasPorID(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)

	got := kviz.fotoConPuertas(t).Puertas

	resumen := []string{}
	for _, p := range got {
		resumen = append(resumen, p.ID+" "+p.Tipo)
	}
	want := []string{
		"lab-control-plane/30010/TCP NodePort",
		"lab-worker/30010/TCP NodePort",
		"lab-worker2/30010/TCP NodePort",
		"lab-worker2/443/TCP hostPort",
		"lab-worker2/80/TCP hostPort",
	}
	if !reflect.DeepEqual(resumen, want) {
		t.Fatalf("puertas = %v, want %v", resumen, want)
	}
}

func TestUnPodSinNodoOTerminadoNoAbrePuertas(t *testing.T) {
	conHostPort := func(p *corev1.Pod) *corev1.Pod {
		p.Spec.Containers = []corev1.Container{{Name: "web", Ports: []corev1.ContainerPort{{ContainerPort: 8080, HostPort: 8080}}}}
		return p
	}
	kviz := nuevoKviz(t, "kind-lab",
		nodo("lab-worker", "172.23.0.4"),
		conHostPort(pod("default", "pendiente", "", "", corev1.PodPending)),
		conHostPort(pod("default", "terminado", "lab-worker", "", corev1.PodSucceeded)),
		conHostPort(pod("default", "fallado", "lab-worker", "", corev1.PodFailed)),
	)

	f := kviz.fotoConPuertas(t)

	if len(f.Puertas) != 0 {
		t.Fatalf("puertas = %s, want ninguna", mostrar(f.Puertas))
	}
	for _, p := range f.Pods {
		if len(p.HostPorts) != 1 {
			t.Errorf("pod %s: hostPorts = %s, want el de su spec", p.Nombre, mostrar(p.HostPorts))
		}
	}
}

func TestDosPuertasEnElMismoNodoPuertoYProtocoloQuedanEnUna(t *testing.T) {
	conHostPort := func(p *corev1.Pod) *corev1.Pod {
		p.Spec.Containers = []corev1.Container{{Name: "web", Ports: []corev1.ContainerPort{{ContainerPort: 8080, HostPort: 30010}}}}
		return p
	}
	kviz := nuevoKviz(t, "kind-lab",
		nodo("lab-worker", "172.23.0.4"),
		service("default", "backend", corev1.ServiceTypeNodePort, "10.96.11.121", map[string]string{"app": "backend"},
			corev1.ServicePort{Protocol: corev1.ProtocolTCP, Port: 80, NodePort: 30010}),
		conHostPort(pod("default", "web", "lab-worker", "10.244.1.9", corev1.PodRunning)),
	)

	got := kviz.fotoConPuertas(t).Puertas

	// un id es único en su lista: de un cluster mal armado queda una sola puerta, la NodePort
	if len(got) != 1 || got[0].ID != "lab-worker/30010/TCP" || got[0].Tipo != "NodePort" {
		t.Fatalf("puertas = %s, want solo la NodePort lab-worker/30010/TCP", mostrar(got))
	}
}

func TestSinNodePortsNiHostPortsLaFotoTraeUnaListaVaciaDePuertas(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"))

	crudo := kviz.fotoCruda(t)

	if string(crudo["puertas"]) != "[]" {
		t.Errorf("puertas = %s, want []", crudo["puertas"])
	}
}

func itoa(n int32) string { return mostrar(n) }

func TestElHostPortDeUnPodDelSistemaNoEsDelIngressController(t *testing.T) {
	// el apiserver de kind es un pod estático con hostNetwork: Kubernetes le completa hostPort 6443
	apiserver := pod("kube-system", "kube-apiserver-lab-control-plane", "lab-control-plane", "172.23.0.2", corev1.PodRunning)
	apiserver.Spec.Containers = []corev1.Container{{Name: "kube-apiserver", Image: "registry.k8s.io/kube-apiserver:v1.33.1",
		Ports: []corev1.ContainerPort{{ContainerPort: 6443, HostPort: 6443, Protocol: corev1.ProtocolTCP}}}}
	kviz := nuevoKviz(t, "kind-lab", append(clusterDeLaGuia(), apiserver)...)

	porID := map[string]puertaEnFoto{}
	for _, p := range kviz.fotoConPuertas(t).Puertas {
		porID[p.ID] = p
	}

	if p := porID["lab-control-plane/6443/TCP"]; p.Tipo != "hostPort" || p.IngressController {
		t.Errorf("puerta del apiserver = %s, want hostPort que no es del Ingress Controller", mostrar(p))
	}
	if p := porID["lab-worker2/80/TCP"]; !p.IngressController {
		t.Errorf("puerta :80 de lab-worker2 = %s, want del Ingress Controller", mostrar(p))
	}
}
