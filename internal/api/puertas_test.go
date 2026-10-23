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
	Nodo        string  `json:"nodo"`
	Puerto      int32   `json:"puerto"`
	Protocolo   string  `json:"protocolo"`
	Tipo        string  `json:"tipo"`
	Namespace   string  `json:"namespace"`
	Service     *string `json:"service"`
	Pod         *string `json:"pod"`
	HaciaPuerto int32   `json:"haciaPuerto"`
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
		return puertaEnFoto{Nodo: nodo, Puerto: 30010, Protocolo: "TCP", Tipo: "NodePort", Namespace: "default",
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
		return puertaEnFoto{Nodo: "lab-worker2", Puerto: puerto, Protocolo: "TCP", Tipo: "hostPort", Namespace: "ingress-nginx",
			Pod: ptr("ingress-nginx-controller-6b9f-lx2wp"), HaciaPuerto: puerto}
	}
	want := []puertaEnFoto{puerta(80), puerta(443)}
	if !reflect.DeepEqual(hostPorts, want) {
		t.Fatalf("puertas de hostPort = %s, want %s", mostrar(hostPorts), mostrar(want))
	}
}

func TestLasPuertasDeLaGuiaVanPorNodoConLasDeNodePortPrimero(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)

	got := kviz.fotoConPuertas(t).Puertas

	resumen := []string{}
	for _, p := range got {
		resumen = append(resumen, p.Nodo+" "+p.Tipo+" "+itoa(p.Puerto))
	}
	want := []string{
		"lab-control-plane NodePort 30010",
		"lab-worker NodePort 30010",
		"lab-worker2 NodePort 30010",
		"lab-worker2 hostPort 80",
		"lab-worker2 hostPort 443",
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

func TestSinNodePortsNiHostPortsLaFotoTraeUnaListaVaciaDePuertas(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"))

	crudo := kviz.fotoCruda(t)

	if string(crudo["puertas"]) != "[]" {
		t.Errorf("puertas = %s, want []", crudo["puertas"])
	}
}

func itoa(n int32) string { return mostrar(n) }
