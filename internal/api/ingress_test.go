package api_test

import (
	"encoding/json"
	"reflect"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Ingress de la foto, tal como los lee la interfaz (web/src/foto.ts).
type ingressEnFoto struct {
	ID        string  `json:"id"`
	Nombre    string  `json:"nombre"`
	Namespace string  `json:"namespace"`
	Clase     *string `json:"clase"`
	// ClasePorDefecto: el Ingress no dice clase y toma la IngressClass por defecto del cluster.
	ClasePorDefecto bool             `json:"clasePorDefecto"`
	Rewrite         *string          `json:"rewrite"`
	Reglas          []reglaDeIngress `json:"reglas"`
}

type reglaDeIngress struct {
	Host     string          `json:"host"`
	Path     string          `json:"path"`
	TipoPath string          `json:"tipoPath"`
	Service  string          `json:"service"`
	Puerto   json.RawMessage `json:"puerto"`
}

func (k *kvizDePrueba) ingress(t *testing.T) []ingressEnFoto {
	t.Helper()
	var f struct {
		Ingress []ingressEnFoto `json:"ingress"`
	}
	k.getJSON(t, "/api/foto", &f)
	return f.Ingress
}

func TestLaFotoTraeElIngressDeLaGuiaConClaseReglaYRewrite(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)

	got := kviz.ingress(t)

	want := []ingressEnFoto{{
		ID: "default/backend", Nombre: "backend", Namespace: "default", Clase: ptr("nginx"), Rewrite: ptr("/$2"),
		Reglas: []reglaDeIngress{{
			Path: "/backend(/|$)(.*)", TipoPath: "ImplementationSpecific", Service: "backend", Puerto: json.RawMessage(`80`),
		}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ingress = %s, want %s", mostrar(got), mostrar(want))
	}
}

func TestUnIngressSinClaseNiRewriteConVariosHostsYPuertoPorNombre(t *testing.T) {
	prefijo := networkingv1.PathTypePrefix
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Namespace: "otro", Name: "legacy"},
		Spec: networkingv1.IngressSpec{
			DefaultBackend: backendIngress("pagina-404", 8080, ""),
			Rules: []networkingv1.IngressRule{
				reglaIngress("api.lab.local", "/v1", &prefijo, backendIngress("old-api", 0, "http")),
				reglaIngress("web.lab.local", "/", &prefijo, backendIngress("web", 80, "")),
			},
		},
	}
	kviz := nuevoKviz(t, "kind-lab", ing)

	got := kviz.ingress(t)

	want := []ingressEnFoto{{
		ID: "otro/legacy", Nombre: "legacy", Namespace: "otro", Clase: nil, Rewrite: nil,
		Reglas: []reglaDeIngress{
			{Host: "api.lab.local", Path: "/v1", TipoPath: "Prefix", Service: "old-api", Puerto: json.RawMessage(`"http"`)},
			{Host: "web.lab.local", Path: "/", TipoPath: "Prefix", Service: "web", Puerto: json.RawMessage(`80`)},
			// el backend por defecto atiende todo lo que no matchea ninguna regla: va sin host ni path
			{Service: "pagina-404", Puerto: json.RawMessage(`8080`)},
		},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ingress = %s, want %s", mostrar(got), mostrar(want))
	}
}

func TestLaClaseDeUnIngressPuedeVenirDeLaAnotacionVieja(t *testing.T) {
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   "default",
			Name:        "viejo",
			Annotations: map[string]string{"kubernetes.io/ingress.class": "nginx"},
		},
		Spec: networkingv1.IngressSpec{DefaultBackend: backendIngress("backend", 80, "")},
	}
	kviz := nuevoKviz(t, "kind-lab", ing)

	got := kviz.ingress(t)

	if len(got) != 1 || got[0].Clase == nil || *got[0].Clase != "nginx" {
		t.Fatalf("ingress = %s, want clase nginx", mostrar(got))
	}
}

func TestLosIngressVanOrdenadosPorID(t *testing.T) {
	ingress := func(ns, nombre string) *networkingv1.Ingress {
		return &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: nombre},
			Spec:       networkingv1.IngressSpec{DefaultBackend: backendIngress("web", 80, "")},
		}
	}
	kviz := nuevoKviz(t, "kind-lab", ingress("default", "web"), ingress("a", "web"), ingress("a-b", "web"))

	got := []string{}
	for _, i := range kviz.ingress(t) {
		got = append(got, i.ID)
	}

	// el orden del id, como lo inserta la interfaz al recibir un cambio ("-" va antes que "/")
	want := []string{"a-b/web", "a/web", "default/web"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
}

func TestSinIngressLaFotoTraeUnaListaVacia(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab")

	crudo := kviz.fotoCruda(t)

	if string(crudo["ingress"]) != "[]" {
		t.Errorf("ingress = %s, want []", crudo["ingress"])
	}
}

// ---------- objetos de Kubernetes para armar los casos ----------

func backendIngress(service string, numero int32, nombrePuerto string) *networkingv1.IngressBackend {
	return &networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
		Name: service,
		Port: networkingv1.ServiceBackendPort{Number: numero, Name: nombrePuerto},
	}}
}

func reglaIngress(host, path string, tipo *networkingv1.PathType, backend *networkingv1.IngressBackend) networkingv1.IngressRule {
	return networkingv1.IngressRule{
		Host: host,
		IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
			Paths: []networkingv1.HTTPIngressPath{{Path: path, PathType: tipo, Backend: *backend}},
		}},
	}
}
