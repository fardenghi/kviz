package api_test

import (
	"encoding/json"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Services de la foto, tal como los lee la interfaz (web/src/foto.ts).
type serviceEnFoto struct {
	Nombre    string              `json:"nombre"`
	Namespace string              `json:"namespace"`
	Tipo      string              `json:"tipo"`
	ClusterIP string              `json:"clusterIP"`
	Puertos   []puertoDeService   `json:"puertos"`
	Selector  map[string]string   `json:"selector"`
	Endpoints []endpointDeService `json:"endpoints"`
}

type puertoDeService struct {
	Nombre     string          `json:"nombre"`
	Protocolo  string          `json:"protocolo"`
	Puerto     int32           `json:"puerto"`
	TargetPort json.RawMessage `json:"targetPort"`
	NodePort   int32           `json:"nodePort"`
}

type endpointDeService struct {
	IP    string  `json:"ip"`
	Pod   *string `json:"pod"`
	Nodo  *string `json:"nodo"`
	Listo bool    `json:"listo"`
}

func (k *kvizDePrueba) services(t *testing.T) []serviceEnFoto {
	t.Helper()
	var f struct {
		Services []serviceEnFoto `json:"services"`
	}
	k.getJSON(t, "/api/foto", &f)
	return f.Services
}

func TestLaFotoTraeLosServicesConTipoClusterIPPuertosYSelector(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab",
		service("default", "backend", corev1.ServiceTypeNodePort, "10.96.11.121", map[string]string{"app": "backend"},
			corev1.ServicePort{Protocol: corev1.ProtocolTCP, Port: 80, TargetPort: intstr.FromInt32(8081), NodePort: 30010}),
		service("default", "db", corev1.ServiceTypeClusterIP, "None", map[string]string{"app": "db"},
			corev1.ServicePort{Name: "pg", Protocol: corev1.ProtocolTCP, Port: 5432, TargetPort: intstr.FromString("postgres")}),
	)

	got := kviz.services(t)

	want := []serviceEnFoto{
		{
			Nombre: "backend", Namespace: "default", Tipo: "NodePort", ClusterIP: "10.96.11.121",
			Puertos:   []puertoDeService{{Protocolo: "TCP", Puerto: 80, TargetPort: json.RawMessage(`8081`), NodePort: 30010}},
			Selector:  map[string]string{"app": "backend"},
			Endpoints: []endpointDeService{},
		},
		{
			Nombre: "db", Namespace: "default", Tipo: "ClusterIP", ClusterIP: "None",
			Puertos:   []puertoDeService{{Nombre: "pg", Protocolo: "TCP", Puerto: 5432, TargetPort: json.RawMessage(`"postgres"`)}},
			Selector:  map[string]string{"app": "db"},
			Endpoints: []endpointDeService{},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("services = %s, want %s", mostrar(got), mostrar(want))
	}
}

func TestUnServiceSinTargetPortApuntaAlMismoPuerto(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab",
		service("default", "web", corev1.ServiceTypeClusterIP, "10.96.0.20", map[string]string{"app": "web"},
			corev1.ServicePort{Protocol: corev1.ProtocolTCP, Port: 8080}),
	)

	got := kviz.services(t)

	if len(got) != 1 || string(got[0].Puertos[0].TargetPort) != "8080" {
		t.Fatalf("services = %s, want targetPort 8080", mostrar(got))
	}
}

func TestLosEndpointsListosYNoListosSalenDeLasEndpointSlices(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab",
		service("default", "web", corev1.ServiceTypeClusterIP, "10.96.0.20", map[string]string{"app": "web"},
			corev1.ServicePort{Protocol: corev1.ProtocolTCP, Port: 80, TargetPort: intstr.FromInt32(8080)}),
		endpointSlice("default", "web-abc12", "web",
			endpoint("10.244.1.8", "web-5d8f-a1", "lab-worker", ptrBool(true)),
			endpoint("10.244.2.9", "web-5d8f-b2", "lab-worker2", ptrBool(false)),
		),
		endpointSlice("default", "web-xyz89", "web",
			endpoint("10.244.2.3", "web-5d8f-c3", "lab-worker2", ptrBool(false)),
		),
	)

	got := kviz.services(t)

	want := []endpointDeService{
		{IP: "10.244.1.8", Pod: ptr("web-5d8f-a1"), Nodo: ptr("lab-worker"), Listo: true},
		{IP: "10.244.2.3", Pod: ptr("web-5d8f-c3"), Nodo: ptr("lab-worker2"), Listo: false},
		{IP: "10.244.2.9", Pod: ptr("web-5d8f-b2"), Nodo: ptr("lab-worker2"), Listo: false},
	}
	if len(got) != 1 || !reflect.DeepEqual(got[0].Endpoints, want) {
		t.Fatalf("services = %s, want endpoints %s", mostrar(got), mostrar(want))
	}
}

func TestUnEndpointSinCondicionReadyCuentaComoListo(t *testing.T) {
	// La API de EndpointSlice dice que ready sin valor se interpreta como listo.
	kviz := nuevoKviz(t, "kind-lab",
		service("default", "web", corev1.ServiceTypeClusterIP, "10.96.0.20", map[string]string{"app": "web"},
			corev1.ServicePort{Protocol: corev1.ProtocolTCP, Port: 80}),
		endpointSlice("default", "web-abc12", "web", endpoint("10.244.1.8", "web-5d8f-a1", "lab-worker", nil)),
	)

	got := kviz.services(t)

	if len(got) != 1 || len(got[0].Endpoints) != 1 || !got[0].Endpoints[0].Listo {
		t.Fatalf("services = %s, want un endpoint listo", mostrar(got))
	}
}

func TestLasEndpointSlicesSeAsignanSoloASuService(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab",
		service("default", "web", corev1.ServiceTypeClusterIP, "10.96.0.20", map[string]string{"app": "web"},
			corev1.ServicePort{Protocol: corev1.ProtocolTCP, Port: 80}),
		service("otro", "web", corev1.ServiceTypeClusterIP, "10.96.0.30", map[string]string{"app": "web"},
			corev1.ServicePort{Protocol: corev1.ProtocolTCP, Port: 80}),
		service("default", "frontend", corev1.ServiceTypeClusterIP, "10.96.0.40", map[string]string{"app": "nadie"},
			corev1.ServicePort{Protocol: corev1.ProtocolTCP, Port: 80}),
		endpointSlice("default", "web-abc12", "web", endpoint("10.244.1.8", "web-5d8f-a1", "lab-worker", ptrBool(true))),
		endpointSlice("otro", "web-def34", "web", endpoint("10.244.2.5", "web-77aa-z9", "lab-worker2", ptrBool(true))),
	)

	got := kviz.services(t)

	endpoints := map[string][]string{}
	for _, s := range got {
		ips := []string{}
		for _, e := range s.Endpoints {
			ips = append(ips, e.IP)
		}
		endpoints[s.Namespace+"/"+s.Nombre] = ips
	}
	want := map[string][]string{
		"default/frontend": {},
		"default/web":      {"10.244.1.8"},
		"otro/web":         {"10.244.2.5"},
	}
	if !reflect.DeepEqual(endpoints, want) {
		t.Fatalf("endpoints por service = %v, want %v", endpoints, want)
	}
	orden := []string{}
	for _, s := range got {
		orden = append(orden, s.Namespace+"/"+s.Nombre)
	}
	if !reflect.DeepEqual(orden, []string{"default/frontend", "default/web", "otro/web"}) {
		t.Errorf("orden = %v, want por namespace y nombre", orden)
	}
}

func TestSinServicesLaFotoTraeUnaListaVacia(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab")

	crudo := kviz.fotoCruda(t)

	if string(crudo["services"]) != "[]" {
		t.Errorf("services = %s, want []", crudo["services"])
	}
}

// ---------- objetos de Kubernetes para armar los casos ----------

func service(namespace, nombre string, tipo corev1.ServiceType, clusterIP string, selector map[string]string, puertos ...corev1.ServicePort) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: nombre},
		Spec: corev1.ServiceSpec{
			Type:      tipo,
			ClusterIP: clusterIP,
			Selector:  selector,
			Ports:     puertos,
		},
	}
}

// endpointSlice arma una EndpointSlice del Service, ligada como lo hace Kubernetes: con la
// etiqueta kubernetes.io/service-name.
func endpointSlice(namespace, nombre, service string, endpoints ...discoveryv1.Endpoint) *discoveryv1.EndpointSlice {
	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      nombre,
			Labels:    map[string]string{discoveryv1.LabelServiceName: service},
		},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints:   endpoints,
	}
}

// endpoint es un endpoint que apunta a un pod; ready nil es "sin condición".
func endpoint(ip, pod, nodo string, ready *bool) discoveryv1.Endpoint {
	return discoveryv1.Endpoint{
		Addresses:  []string{ip},
		Conditions: discoveryv1.EndpointConditions{Ready: ready},
		NodeName:   &nodo,
		TargetRef:  &corev1.ObjectReference{Kind: "Pod", Name: pod},
	}
}

func ptrBool(b bool) *bool { return &b }
