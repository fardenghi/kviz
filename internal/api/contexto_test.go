package api_test

// Contexto y reconexión: la API lista los contextos de kubectl y cambia de contexto en caliente;
// si el cluster deja de responder la foto queda desconectada, y kviz se reconecta solo cuando
// vuelve.

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

type contextosJSON struct {
	Actual    string   `json:"actual"`
	Contextos []string `json:"contextos"`
}

func TestLaAPIListaLosContextosDelKubeconfig(t *testing.T) {
	kc := nuevoKubeconfig("kind-lab")
	kc.crearCluster("kind-lab", nodo("lab-worker", "172.23.0.4"))
	kc.crearCluster("kind-otro", nodo("otro-worker", "172.24.0.4"))
	kviz := nuevoKvizConKubeconfig(t, kc)

	var got contextosJSON
	kviz.getJSON(t, "/api/contextos", &got)

	want := contextosJSON{Actual: "kind-lab", Contextos: []string{"kind-lab", "kind-otro"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("contextos = %s, want %s", mostrar(got), mostrar(want))
	}
}

func TestCambiarDeContextoReemplazaLaFotoPorLaDelOtroCluster(t *testing.T) {
	kc := nuevoKubeconfig("kind-lab")
	kc.crearCluster("kind-lab", nodo("lab-worker", "172.23.0.4"),
		pod("default", "nginx", "lab-worker", "10.244.1.5", corev1.PodRunning))
	kc.crearCluster("kind-otro", nodo("otro-worker", "172.24.0.4"))
	kviz := nuevoKvizConKubeconfig(t, kc)
	kviz.esperarConectado(t)
	stream := kviz.abrirStream(t)
	stream.siguiente(t)

	if status := kviz.cambiarContexto(t, "kind-otro"); status != http.StatusNoContent {
		t.Fatalf("POST /api/contexto: status %d, want 204", status)
	}

	f := stream.esperarFoto(t, func(f fotoJSON) bool { return f.Conectado })
	want := []nodoEnFoto{{Nombre: "otro-worker", InternalIP: "172.24.0.4"}}
	if f.Contexto != "kind-otro" || !reflect.DeepEqual(f.Nodos, want) || len(f.Pods) != 0 {
		t.Fatalf("foto = %s, want la del cluster de kind-otro", mostrar(f))
	}
	var got contextosJSON
	kviz.getJSON(t, "/api/contextos", &got)
	if got.Actual != "kind-otro" {
		t.Errorf("contexto actual = %q, want kind-otro", got.Actual)
	}
}

func TestCambiarAUnContextoQueNoExisteDaErrorYSigueEnElActual(t *testing.T) {
	kc := nuevoKubeconfig("kind-lab")
	kc.crearCluster("kind-lab", nodo("lab-worker", "172.23.0.4"))
	kviz := nuevoKvizConKubeconfig(t, kc)
	kviz.esperarConectado(t)

	if status := kviz.cambiarContexto(t, "kind-fantasma"); status != http.StatusNotFound {
		t.Fatalf("POST /api/contexto: status %d, want 404", status)
	}

	if f := kviz.foto(t); f.Contexto != "kind-lab" || !f.Conectado {
		t.Fatalf("foto = %s, want sigue conectado a kind-lab", mostrar(f))
	}
}

func TestSiElClusterDejaDeResponderLaFotoQuedaDesconectada(t *testing.T) {
	kc := nuevoKubeconfig("kind-lab")
	lab := kc.crearCluster("kind-lab", nodo("lab-worker", "172.23.0.4"))
	kviz := nuevoKvizConKubeconfig(t, kc)
	kviz.esperarConectado(t)
	stream := kviz.abrirStream(t)
	stream.siguiente(t)

	lab.caido.Store(true)

	f := stream.esperarFoto(t, func(f fotoJSON) bool { return !f.Conectado })
	if f.Contexto != "kind-lab" || f.Motivo == "" || len(f.Nodos) != 0 {
		t.Fatalf("foto = %s, want kind-lab desconectado, con el motivo y sin objetos", mostrar(f))
	}
	if f := kviz.foto(t); f.Conectado {
		t.Fatalf("GET /api/foto: conectado = true, want false")
	}
}

func TestCuandoElClusterVuelveKvizSeReconectaSoloYMandaLaFoto(t *testing.T) {
	kc := nuevoKubeconfig("kind-lab")
	lab := kc.crearCluster("kind-lab", nodo("lab-worker", "172.23.0.4"))
	kviz := nuevoKvizConKubeconfig(t, kc)
	kviz.esperarConectado(t)
	stream := kviz.abrirStream(t)
	stream.siguiente(t)
	lab.caido.Store(true)
	stream.esperarFoto(t, func(f fotoJSON) bool { return !f.Conectado })

	lab.caido.Store(false)

	f := stream.esperarFoto(t, func(f fotoJSON) bool { return f.Conectado })
	want := []nodoEnFoto{{Nombre: "lab-worker", InternalIP: "172.23.0.4"}}
	if f.Contexto != "kind-lab" || !reflect.DeepEqual(f.Nodos, want) {
		t.Fatalf("foto = %s, want kind-lab otra vez con su nodo", mostrar(f))
	}
}

func TestUnClusterBorradoYRecreadoVuelveConSusObjetosNuevos(t *testing.T) {
	kc := nuevoKubeconfig("kind-lab")
	kc.crearCluster("kind-lab", nodo("lab-worker", "172.23.0.4"),
		pod("default", "nginx", "lab-worker", "10.244.1.5", corev1.PodRunning))
	kviz := nuevoKvizConKubeconfig(t, kc)
	kviz.esperarConectado(t)
	stream := kviz.abrirStream(t)
	stream.siguiente(t)

	kc.borrarCluster("kind-lab") // kind delete cluster
	stream.esperarFoto(t, func(f fotoJSON) bool { return !f.Conectado })
	kc.crearCluster("kind-lab", nodo("lab-worker", "172.23.0.9")) // kind create cluster

	f := stream.esperarFoto(t, func(f fotoJSON) bool { return f.Conectado })
	want := []nodoEnFoto{{Nombre: "lab-worker", InternalIP: "172.23.0.9"}}
	if f.Contexto != "kind-lab" || !reflect.DeepEqual(f.Nodos, want) || len(f.Pods) != 0 {
		t.Fatalf("foto = %s, want el cluster nuevo: el nodo con su IP nueva y sin el pod de antes", mostrar(f))
	}
}

// Con el kubeconfig de kubectl de verdad (KUBECONFIG), apuntando a clusters que no existen.
func TestConElKubeconfigDeKubectlListaSusContextosYQuedaDesconectado(t *testing.T) {
	t.Setenv("KUBECONFIG", escribirKubeconfig(t, "kind-lab", "kind-lab", "kind-otro"))

	kviz := nuevoKvizDesdeKubeconfig(t)

	var got contextosJSON
	kviz.getJSON(t, "/api/contextos", &got)
	want := contextosJSON{Actual: "kind-lab", Contextos: []string{"kind-lab", "kind-otro"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("contextos = %s, want %s", mostrar(got), mostrar(want))
	}
	if f := kviz.foto(t); f.Contexto != "kind-lab" || f.Conectado || f.Motivo == "" {
		t.Fatalf("foto = %s, want kind-lab desconectado con el motivo", mostrar(f))
	}
}

func TestSinContextoKvizEsperaAQueKubectlTengaUno(t *testing.T) {
	kubeconfig := escribirKubeconfig(t, "")
	t.Setenv("KUBECONFIG", kubeconfig)
	kviz := nuevoKvizDesdeKubeconfig(t)
	stream := kviz.abrirStream(t)
	if f := stream.siguiente(t).Foto; f == nil || f.Contexto != "" || f.Conectado {
		t.Fatalf("foto = %s, want desconectado y sin contexto", mostrar(f))
	}

	// kind create cluster agrega el contexto y lo deja como actual
	if err := os.Rename(escribirKubeconfig(t, "kind-lab", "kind-lab"), kubeconfig); err != nil {
		t.Fatal(err)
	}

	f := stream.esperarFoto(t, func(f fotoJSON) bool { return f.Contexto == "kind-lab" })
	if f.Conectado {
		t.Fatalf("foto = %s, want kind-lab todavía desconectado (su cluster no existe)", mostrar(f))
	}
}

// ---------- arnés: un kubeconfig con varios clusters falsos ----------

// cambiarContexto elige otro contexto como lo hace el selector de la interfaz y devuelve el status.
func (k *kvizDePrueba) cambiarContexto(t *testing.T, contexto string) int {
	t.Helper()
	cuerpo := strings.NewReader(fmt.Sprintf(`{"contexto":%q}`, contexto))
	res, err := http.Post(k.srv.URL+"/api/contexto", "application/json", cuerpo)
	if err != nil {
		t.Fatalf("POST /api/contexto: %v", err)
	}
	res.Body.Close()
	return res.StatusCode
}

// esperarFoto lee el stream hasta que llega una foto completa que cumple la condición, y la devuelve.
func (s *streamDePrueba) esperarFoto(t *testing.T, cumple func(fotoJSON) bool) fotoJSON {
	t.Helper()
	limite := time.After(esperaStream)
	var vistas []fotoJSON
	for {
		select {
		case m, ok := <-s.mensajes:
			if !ok {
				t.Fatal("el stream se cerró")
			}
			if m.Tipo == "foto" && m.Foto != nil {
				if cumple(*m.Foto) {
					return *m.Foto
				}
				vistas = append(vistas, *m.Foto)
			}
		case <-limite:
			t.Fatalf("no llegó la foto esperada; llegaron %s", mostrar(vistas))
		}
	}
}

// escribirKubeconfig escribe un kubeconfig con los contextos dados, cada uno con un cluster en
// una dirección donde no responde nadie, y devuelve su ruta.
func escribirKubeconfig(t *testing.T, actual string, contextos ...string) string {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "apiVersion: v1\nkind: Config\ncurrent-context: %q\nclusters:\n", actual)
	for _, c := range contextos {
		fmt.Fprintf(&b, "- name: %s\n  cluster:\n    server: https://127.0.0.1:1\n", c)
	}
	b.WriteString("users:\n- name: alumno\n  user:\n    token: falso\ncontexts:\n")
	for _, c := range contextos {
		fmt.Fprintf(&b, "- name: %s\n  context:\n    cluster: %s\n    user: alumno\n", c, c)
	}
	ruta := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(ruta, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return ruta
}

// kubeconfigDePrueba hace de kubeconfig de kubectl: cada contexto tiene su cluster falso. Los
// tests pueden crear y borrar clusters (como kind) o hacer que uno deje de responder.
type kubeconfigDePrueba struct {
	mu       sync.Mutex
	actual   string
	clusters map[string]*clusterDePrueba
}

type clusterDePrueba struct {
	cliente *fake.Clientset
	caido   atomic.Bool
}

func nuevoKubeconfig(actual string) *kubeconfigDePrueba {
	return &kubeconfigDePrueba{actual: actual, clusters: map[string]*clusterDePrueba{}}
}

// crearCluster es `kind create cluster`: el contexto aparece en el kubeconfig con un cluster nuevo.
func (kc *kubeconfigDePrueba) crearCluster(contexto string, objetos ...runtime.Object) *clusterDePrueba {
	c := &clusterDePrueba{cliente: fake.NewClientset(objetos...)}
	sinRespuesta := errors.New("dial tcp 127.0.0.1:6443: connect: connection refused")
	c.cliente.PrependReactor("*", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
		return c.caido.Load(), nil, sinRespuesta
	})
	c.cliente.PrependWatchReactor("*", func(k8stesting.Action) (bool, watch.Interface, error) {
		return c.caido.Load(), nil, sinRespuesta
	})
	// el scheduler y el kubelet: un pod creado (el pod de sondeo) queda corriendo en un nodo
	c.cliente.PrependReactor("create", "pods", func(a k8stesting.Action) (bool, runtime.Object, error) {
		if p, ok := a.(k8stesting.CreateAction).GetObject().(*corev1.Pod); ok && p.Status.Phase == "" {
			p.Spec.NodeName, p.Status.Phase, p.Status.PodIP = "lab-control-plane", corev1.PodRunning, "10.244.0.9"
		}
		return false, nil, nil
	})
	kc.mu.Lock()
	defer kc.mu.Unlock()
	kc.clusters[contexto] = c
	return c
}

// borrarCluster es `kind delete cluster`: el cluster deja de responder y su contexto se va del kubeconfig.
func (kc *kubeconfigDePrueba) borrarCluster(contexto string) {
	kc.mu.Lock()
	defer kc.mu.Unlock()
	kc.clusters[contexto].caido.Store(true)
	delete(kc.clusters, contexto)
}

func (kc *kubeconfigDePrueba) cluster(contexto string) *clusterDePrueba {
	kc.mu.Lock()
	defer kc.mu.Unlock()
	return kc.clusters[contexto]
}

func (kc *kubeconfigDePrueba) Contextos() (string, []string, error) {
	kc.mu.Lock()
	defer kc.mu.Unlock()
	todos := []string{}
	for nombre := range kc.clusters {
		todos = append(todos, nombre)
	}
	sort.Strings(todos)
	return kc.actual, todos, nil
}

func (kc *kubeconfigDePrueba) Cliente(contexto string) (kubernetes.Interface, error) {
	kc.mu.Lock()
	defer kc.mu.Unlock()
	c, ok := kc.clusters[contexto]
	if !ok {
		return nil, fmt.Errorf("context %q does not exist", contexto)
	}
	return c.cliente, nil
}
