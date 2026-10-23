package api_test

// Namespaces (módulo 1 de la spec): la foto trae los namespaces del cluster, de su informer, para
// que el filtro por namespace ofrezca también los que todavía no tienen pods.

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type namespaceEnFoto struct {
	ID     string `json:"id"`
	Nombre string `json:"nombre"`
}

func (k *kvizDePrueba) namespaces(t *testing.T) []namespaceEnFoto {
	t.Helper()
	var f struct {
		Namespaces []namespaceEnFoto `json:"namespaces"`
	}
	k.getJSON(t, "/api/foto", &f)
	return f.Namespaces
}

func namespace(nombre string) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nombre}}
}

func TestLaFotoTraeLosNamespacesDelClusterAunqueNoTenganPods(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", namespace("default"), namespace("vacio"), namespace("datos"),
		pod("datos", "postgres-0", "lab-worker", "10.244.1.3", corev1.PodRunning))

	got := kviz.namespaces(t)

	want := []namespaceEnFoto{{ID: "datos", Nombre: "datos"}, {ID: "default", Nombre: "default"}, {ID: "vacio", Nombre: "vacio"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("namespaces = %s, want %s", mostrar(got), mostrar(want))
	}
}

func TestUnNamespaceNuevoLlegaEnVivo(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", namespace("default"))
	stream := kviz.abrirStream(t)
	if m := stream.siguiente(t); m.Foto == nil {
		t.Fatalf("primer mensaje = %s, want la foto", mostrar(m))
	}

	if _, err := kviz.cliente.CoreV1().Namespaces().Create(context.Background(), namespace("tienda"), metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	c := stream.esperarCambio(t, "namespaces", "tienda")
	var ns namespaceEnFoto
	if err := json.Unmarshal(c.Valor, &ns); err != nil || ns.Nombre != "tienda" {
		t.Fatalf("cambio = %s, want el alta del namespace tienda", mostrar(c))
	}
}

func TestSinNamespacesLaFotoTraeUnaListaVacia(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab")

	if crudo := kviz.fotoCruda(t); string(crudo["namespaces"]) != "[]" {
		t.Errorf("namespaces = %s, want []", crudo["namespaces"])
	}
}
