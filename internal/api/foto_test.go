package api_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestLaFotoTraeLosNodosConSuInternalIP(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab",
		nodo("lab-worker", "172.23.0.4"),
		nodo("lab-control-plane", "172.23.0.2"),
	)

	f := kviz.foto(t)

	want := []nodoEnFoto{
		{Nombre: "lab-control-plane", InternalIP: "172.23.0.2"},
		{Nombre: "lab-worker", InternalIP: "172.23.0.4"},
	}
	if !reflect.DeepEqual(f.Nodos, want) {
		t.Fatalf("nodos = %+v, want %+v", f.Nodos, want)
	}
}

func TestLaFotoTraeLosPodsConSuNodoIPYFase(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab",
		nodo("lab-worker", "172.23.0.4"),
		pod("default", "nginx", "lab-worker", "10.244.1.5", corev1.PodRunning),
		pod("default", "backend-79f9ffc666-k2x8p", "lab-worker", "10.244.1.7", corev1.PodRunning),
		pod("kube-system", "coredns-abc", "lab-worker", "10.244.1.2", corev1.PodRunning),
	)

	f := kviz.foto(t)

	want := []podEnFoto{
		{Nombre: "backend-79f9ffc666-k2x8p", Namespace: "default", Nodo: ptr("lab-worker"), IP: ptr("10.244.1.7"), Fase: "Running"},
		{Nombre: "nginx", Namespace: "default", Nodo: ptr("lab-worker"), IP: ptr("10.244.1.5"), Fase: "Running"},
		{Nombre: "coredns-abc", Namespace: "kube-system", Nodo: ptr("lab-worker"), IP: ptr("10.244.1.2"), Fase: "Running"},
	}
	if !reflect.DeepEqual(f.Pods, want) {
		t.Fatalf("pods = %s, want %s", mostrar(f.Pods), mostrar(want))
	}
}

func TestUnPodPendingSinNodoNoTieneNodoNiIP(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab",
		nodo("lab-worker", "172.23.0.4"),
		pod("default", "grande", "", "", corev1.PodPending),
	)

	f := kviz.foto(t)

	want := []podEnFoto{{Nombre: "grande", Namespace: "default", Nodo: nil, IP: nil, Fase: "Pending"}}
	if !reflect.DeepEqual(f.Pods, want) {
		t.Fatalf("pods = %s, want %s", mostrar(f.Pods), mostrar(want))
	}
}

func TestLaFotoDiceAQueContextoEstaConectado(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab")

	f := kviz.foto(t)

	if f.Contexto != "kind-lab" || !f.Conectado {
		t.Fatalf("contexto = %q, conectado = %v; want %q conectado", f.Contexto, f.Conectado, "kind-lab")
	}
}

func TestSinContextoDeKubectlKvizArrancaDesconectado(t *testing.T) {
	kubeconfig := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(kubeconfig, []byte("apiVersion: v1\nkind: Config\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", kubeconfig)

	kviz := nuevoKvizDesdeKubeconfig(t)
	f := kviz.foto(t)

	if f.Conectado {
		t.Fatalf("conectado = true, want false")
	}
	if !strings.Contains(f.Motivo, "contexto") {
		t.Errorf("motivo = %q, want que explique que falta el contexto", f.Motivo)
	}
	if len(f.Nodos) != 0 || len(f.Pods) != 0 {
		t.Errorf("nodos = %v, pods = %v; want vacíos", f.Nodos, f.Pods)
	}
}

func TestUnClusterVacioDaListasVaciasYNoNull(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab")

	crudo := kviz.fotoCruda(t)

	for _, campo := range []string{"nodos", "pods"} {
		if string(crudo[campo]) != "[]" {
			t.Errorf("%s = %s, want []", campo, crudo[campo])
		}
	}
}
