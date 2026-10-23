package api_test

// Recorrido completo (ticket 17): DNS → puerta → regla de Ingress → Service → endpoints → salto
// entre nodos → pod. Los destinos son una ruta de Ingress (por la puerta de hostPort del Ingress
// Controller en un nodo elegido), un Service (por nombre DNS o ClusterIP), un NodePort por un nodo
// elegido y la IP de un pod. El pod exacto solo se marca cuando se sabe: por el log del Ingress
// Controller (que kviz busca por el X-Request-ID que le pone a la request) o por el body.

import (
	"fmt"
	"reflect"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"

	"github.com/fardenghi/kviz/internal/sondeo"
)

const podDelIC = "ingress-nginx-controller-6b9f-lx2wp" // en lab-worker2, 10.244.2.2

func TestElRecorridoDeLaGuiaVaDeLaPuertaDelIngressControllerAlPodDeOtroNodo(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	ic := kviz.ingressControllerQueManda(t, "10.244.1.5:8081")
	kviz.sondeo.responder(sondeo.RespuestaHTTP{Status: 200, Body: "Hello!"})

	_, got := kviz.mandarPrueba(t, pedidoDePrueba{Destino: "lab-worker2:80", Path: "/backend/hello"})

	if got.URL != "http://lab-worker2:80/backend/hello" {
		t.Errorf("url = %q, want http://lab-worker2:80/backend/hello", got.URL)
	}
	want := []pasoJSON{
		{N: 1, Tipo: "dns", Titulo: "lab-worker2 → 172.23.0.3", Detalle: "Nombre de un nodo: el DNS lo resuelve a su InternalIP",
			Objetos: []refJSON{{Tipo: "nodo", ID: "lab-worker2"}}},
		{N: 2, Tipo: "puerta", Titulo: "lab-worker2 :80 (hostPort)", Detalle: "Entra al Ingress Controller " + podDelIC,
			Objetos: []refJSON{{Tipo: "puerta", ID: "lab-worker2/80/TCP"}}},
		{N: 3, Tipo: "ingress", Titulo: "/backend(/|$)(.*) → backend:80", Detalle: "Ingress backend · path reescrito: /backend/hello → /hello",
			Objetos: []refJSON{{Tipo: "ingress", ID: "default/backend"}, {Tipo: "pod", ID: "ingress-nginx/" + podDelIC}}},
		{N: 4, Tipo: "service", Titulo: "backend 10.96.11.121:80 → :8081", Detalle: "NodePort, targetPort 8081; el Ingress Controller manda directo a los endpoints",
			Objetos: []refJSON{{Tipo: "service", ID: "default/backend"}}},
		{N: 5, Tipo: "endpoints", Titulo: "10.244.1.5:8081, 10.244.2.4:8081", Detalle: "2 endpoints listos; el Ingress Controller elige uno",
			Objetos: []refJSON{{Tipo: "pod", ID: "default/" + podK2x8p}, {Tipo: "pod", ID: "default/" + pod7hw4q}}},
		{N: 6, Tipo: "salto", Titulo: "lab-worker2 → lab-worker", Detalle: "El paquete sale del nodo 172.23.0.3 hacia 172.23.0.4, donde corre el pod",
			Objetos: []refJSON{{Tipo: "nodo", ID: "lab-worker2"}, {Tipo: "nodo", ID: "lab-worker"}}},
		{N: 7, Tipo: "pod", Titulo: podK2x8p + " 10.244.1.5", Detalle: "Conocido: el log del Ingress Controller dice que la mandó a 10.244.1.5:8081",
			Objetos: []refJSON{{Tipo: "pod", ID: "default/" + podK2x8p}}},
	}
	if !reflect.DeepEqual(got.Recorrido.Pasos, want) {
		t.Fatalf("recorrido = %s\nwant %s", mostrar(got.Recorrido.Pasos), mostrar(want))
	}
	if p := ic.pedidos(); len(p) == 0 || p[0].Container != "controller" || p[0].Follow {
		t.Errorf("pedidos de logs al Ingress Controller = %s, want los del container controller, sin seguir", mostrar(p))
	}
}

func TestSinLaLineaEnElLogDelIngressControllerElRecorridoTerminaEnLosEndpoints(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	kviz.ingressControllerQueManda(t, "")
	kviz.sondeo.responder(sondeo.RespuestaHTTP{Status: 200, Body: "Hello!"})

	_, got := kviz.mandarPrueba(t, pedidoDePrueba{Destino: "lab-worker2:80", Path: "/backend/hello"})

	if tipos := tiposDePasos(got); !reflect.DeepEqual(tipos, []string{"dns", "puerta", "ingress", "service", "endpoints"}) {
		t.Fatalf("pasos = %s, want hasta los endpoints posibles, sin salto ni pod", mostrar(got.Recorrido.Pasos))
	}
}

func TestElRepartoPorElIngressControllerUsaSuLog(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	kviz.ingressControllerQueManda(t, "10.244.1.5:8081", "10.244.2.4:8081", "10.244.2.4:8081", "")
	kviz.sondeo.responder(sondeo.RespuestaHTTP{Status: 200, Body: "Hello!"})

	_, got := kviz.repetirPrueba(t, pedidoRepetido{
		pedidoDePrueba: pedidoDePrueba{Destino: "lab-worker2:80", Path: "/backend/hello"},
		Repeticiones:   4,
	})

	want := &repartoJSON{Total: 4, Pods: []porPodJSON{{Pod: "default/" + pod7hw4q, N: 2}, {Pod: "default/" + podK2x8p, N: 1}}, SinIdentificar: 1}
	if !reflect.DeepEqual(got.Reparto, want) {
		t.Fatalf("reparto = %s\nwant %s", mostrar(got.Reparto), mostrar(want))
	}
	ids := map[string]bool{}
	for _, e := range kviz.sondeo.ejecutados() {
		for _, h := range e.pedido.Headers {
			if h.Nombre == "X-Request-ID" {
				ids[h.Valor] = true
			}
		}
	}
	if len(ids) != 4 {
		t.Errorf("X-Request-ID distintos = %d, want uno por repetición", len(ids))
	}
}

func TestUnNodePortPorUnNodoElegidoSaltaAlNodoDelPodQueRespondio(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	kviz.sondeo.responder(sondeo.RespuestaHTTP{Status: 200, Body: "Hello, " + pod7hw4q + "!"})

	_, got := kviz.mandarPrueba(t, pedidoDePrueba{Destino: "lab-worker:30010", Path: "/hello"})

	want := []pasoJSON{
		{N: 1, Tipo: "dns", Titulo: "lab-worker → 172.23.0.4", Detalle: "Nombre de un nodo: el DNS lo resuelve a su InternalIP",
			Objetos: []refJSON{{Tipo: "nodo", ID: "lab-worker"}}},
		{N: 2, Tipo: "puerta", Titulo: "lab-worker :30010 (NodePort)", Detalle: "kube-proxy del nodo la manda al Service backend",
			Objetos: []refJSON{{Tipo: "puerta", ID: "lab-worker/30010/TCP"}}},
		{N: 3, Tipo: "service", Titulo: "backend 10.96.11.121:80 → :8081", Detalle: "NodePort, targetPort 8081",
			Objetos: []refJSON{{Tipo: "service", ID: "default/backend"}}},
		{N: 4, Tipo: "endpoints", Titulo: "10.244.1.5:8081, 10.244.2.4:8081", Detalle: "2 endpoints listos; kube-proxy elige uno",
			Objetos: []refJSON{{Tipo: "pod", ID: "default/" + podK2x8p}, {Tipo: "pod", ID: "default/" + pod7hw4q}}},
		{N: 5, Tipo: "salto", Titulo: "lab-worker → lab-worker2", Detalle: "El paquete sale del nodo 172.23.0.4 hacia 172.23.0.3, donde corre el pod",
			Objetos: []refJSON{{Tipo: "nodo", ID: "lab-worker"}, {Tipo: "nodo", ID: "lab-worker2"}}},
		{N: 6, Tipo: "pod", Titulo: pod7hw4q + " 10.244.2.4", Detalle: "Conocido: el body de la respuesta trae el nombre del pod",
			Objetos: []refJSON{{Tipo: "pod", ID: "default/" + pod7hw4q}}},
	}
	if !reflect.DeepEqual(got.Recorrido.Pasos, want) {
		t.Fatalf("recorrido = %s\nwant %s", mostrar(got.Recorrido.Pasos), mostrar(want))
	}
	if len(kviz.sondeo.ejecutados()[0].pedido.Headers) != 0 {
		t.Errorf("headers = %s, want ninguno: no pasa por el Ingress Controller", mostrar(kviz.sondeo.ejecutados()[0].pedido.Headers))
	}
}

func TestUnNodePortPorLaIPDelNodoDelPodNoTieneDNSNiSalto(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	kviz.sondeo.responder(sondeo.RespuestaHTTP{Status: 200, Body: "Hello, " + pod7hw4q + "!"})

	_, got := kviz.mandarPrueba(t, pedidoDePrueba{Destino: "172.23.0.3:30010"})

	if tipos := tiposDePasos(got); !reflect.DeepEqual(tipos, []string{"puerta", "service", "endpoints", "pod"}) {
		t.Fatalf("pasos = %s, want puerta, Service, endpoints y pod", mostrar(got.Recorrido.Pasos))
	}
	if p := got.Recorrido.Pasos[0]; p.Titulo != "lab-worker2 :30010 (NodePort)" || !reflect.DeepEqual(p.Objetos, []refJSON{{Tipo: "puerta", ID: "lab-worker2/30010/TCP"}}) {
		t.Fatalf("paso puerta = %s, want la puerta :30010 de lab-worker2", mostrar(p))
	}
}

func TestUnaIPDePodVaDirectoAlPod(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	kviz.sondeo.responder(sondeo.RespuestaHTTP{Status: 200, Body: "Hello!"})

	_, got := kviz.mandarPrueba(t, pedidoDePrueba{Destino: "10.244.1.5:8081", Path: "/hello"})

	want := []pasoJSON{
		{N: 1, Tipo: "pod", Titulo: podK2x8p + " 10.244.1.5:8081", Detalle: "IP de un pod: la request va directo al pod, sin Service ni kube-proxy",
			Objetos: []refJSON{{Tipo: "pod", ID: "default/" + podK2x8p}}},
	}
	if !reflect.DeepEqual(got.Recorrido.Pasos, want) {
		t.Fatalf("recorrido = %s\nwant %s", mostrar(got.Recorrido.Pasos), mostrar(want))
	}
}

func TestUnPuertoQueElNodoNoAbreTerminaEnElNodo(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	kviz.sondeo.guionar(func(sondeo.PedidoHTTP) (sondeo.RespuestaHTTP, error) {
		return sondeo.RespuestaHTTP{}, sondeo.SinRespuesta{Motivo: "Connection refused"}
	})

	// el hostPort 80 del Ingress Controller solo existe en lab-worker2
	_, got := kviz.mandarPrueba(t, pedidoDePrueba{Destino: "lab-worker:80", Path: "/backend/hello"})

	want := []pasoJSON{
		{N: 1, Tipo: "dns", Titulo: "lab-worker → 172.23.0.4", Detalle: "Nombre de un nodo: el DNS lo resuelve a su InternalIP",
			Objetos: []refJSON{{Tipo: "nodo", ID: "lab-worker"}}},
		{N: 2, Tipo: "puerta", Titulo: "lab-worker :80", Detalle: "Ningún NodePort ni hostPort abre el puerto 80 en este nodo: la request no llega a ningún pod",
			Objetos: []refJSON{{Tipo: "nodo", ID: "lab-worker"}}},
	}
	if !reflect.DeepEqual(got.Recorrido.Pasos, want) {
		t.Fatalf("recorrido = %s\nwant %s", mostrar(got.Recorrido.Pasos), mostrar(want))
	}
}

func TestUnPathSinReglaDeIngressTerminaEnElIngressController(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	kviz.sondeo.responder(sondeo.RespuestaHTTP{Status: 404, Body: "404 Not Found"})

	_, got := kviz.mandarPrueba(t, pedidoDePrueba{Destino: "lab-worker2", Path: "/frontend"})

	if tipos := tiposDePasos(got); !reflect.DeepEqual(tipos, []string{"dns", "puerta", "ingress"}) {
		t.Fatalf("pasos = %s, want DNS, puerta e Ingress", mostrar(got.Recorrido.Pasos))
	}
	want := pasoJSON{N: 3, Tipo: "ingress", Titulo: "ninguna regla para lab-worker2/frontend",
		Detalle: "Ninguna regla de Ingress matchea el host y el path: el Ingress Controller responde 404",
		Objetos: []refJSON{{Tipo: "pod", ID: "ingress-nginx/" + podDelIC}}}
	if got := got.Recorrido.Pasos[2]; !reflect.DeepEqual(got, want) {
		t.Fatalf("paso de Ingress = %s\nwant %s", mostrar(got), mostrar(want))
	}
}

func TestUnaPruebaDeConexionPorLaPuertaDelIngressControllerNoPasaPorLasReglasDeIngress(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	kviz.sondeo.conectar(sondeo.ResultadoTCP{Resultado: sondeo.Abre})

	// una conexión TCP no tiene path ni Host: llega al pod del hostPort y ahí termina
	_, got := kviz.mandarConexion(t, pedidoDePrueba{Tipo: "tcp", Destino: "lab-worker2:80"})

	want := []pasoJSON{
		{N: 1, Tipo: "dns", Titulo: "lab-worker2 → 172.23.0.3", Detalle: "Nombre de un nodo: el DNS lo resuelve a su InternalIP",
			Objetos: []refJSON{{Tipo: "nodo", ID: "lab-worker2"}}},
		{N: 2, Tipo: "puerta", Titulo: "lab-worker2 :80 (hostPort)", Detalle: "Entra directo al pod " + podDelIC + " (hostPort 80 → :80)",
			Objetos: []refJSON{{Tipo: "puerta", ID: "lab-worker2/80/TCP"}}},
		{N: 3, Tipo: "pod", Titulo: podDelIC + " 10.244.2.2:80", Detalle: "Conocido: un hostPort lleva a un solo pod",
			Objetos: []refJSON{{Tipo: "pod", ID: "ingress-nginx/" + podDelIC}}},
	}
	if !reflect.DeepEqual(got.Recorrido.Pasos, want) {
		t.Fatalf("recorrido = %s\nwant %s", mostrar(got.Recorrido.Pasos), mostrar(want))
	}
}

func TestUnaPruebaDeConexionAUnNodePortPorUnNodo(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	kviz.sondeo.conectar(sondeo.ResultadoTCP{Resultado: sondeo.Abre})

	_, got := kviz.mandarConexion(t, pedidoDePrueba{Tipo: "tcp", Destino: "lab-worker:30010"})

	if tipos := tiposDePasos(got.pruebaJSON); !reflect.DeepEqual(tipos, []string{"dns", "puerta", "service", "endpoints"}) {
		t.Fatalf("pasos = %s, want DNS, puerta, Service y endpoints", mostrar(got.Recorrido.Pasos))
	}
}

func tiposDePasos(p pruebaJSON) []string {
	tipos := []string{}
	for _, paso := range p.Recorrido.Pasos {
		tipos = append(tipos, paso.Tipo)
	}
	return tipos
}

// ---------- arnés: el log del Ingress Controller ----------

// logDelIC es el Ingress Controller del cluster falso: su log tiene una línea por cada request de
// prueba que pasó por él, con el X-Request-ID que le puso kviz y el upstream que eligió.
type logDelIC struct {
	mu     sync.Mutex
	lineas []string
	todos  []corev1.PodLogOptions
}

func (l *logDelIC) pedidos() []corev1.PodLogOptions {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]corev1.PodLogOptions{}, l.todos...)
}

// ingressControllerQueManda hace que las requests de prueba que lleguen con X-Request-ID dejen en
// el log del Ingress Controller una línea con el formato de ingress-nginx que dice que la mandó a
// un upstream: el que toca, por turno. Con upstream "" la request no deja línea (como si el log
// todavía no la tuviera).
func (k *kvizDePrueba) ingressControllerQueManda(t *testing.T, upstreams ...string) *logDelIC {
	t.Helper()
	turno := 0
	l := &logDelIC{lineas: []string{
		`10.244.0.9 - - [23/Oct/2026:15:02:10 +0000] "GET /backend/hello HTTP/1.1" 200 37 "-" "curl/8.10.1" 85 0.002 [default-backend-80] [] 10.244.2.4:8081 37 0.002 200 otra-request`,
	}}
	k.sondeo.alEjecutar(func(p sondeo.PedidoHTTP) {
		id := ""
		for _, h := range p.Headers {
			if h.Nombre == "X-Request-ID" {
				id = h.Valor
			}
		}
		upstream := upstreams[turno%len(upstreams)]
		turno++
		if id == "" || upstream == "" {
			return
		}
		l.mu.Lock()
		defer l.mu.Unlock()
		l.lineas = append(l.lineas, fmt.Sprintf(
			`10.244.0.9 - - [23/Oct/2026:15:02:11 +0000] "%s /backend/hello HTTP/1.1" 200 37 "-" "curl/8.10.1" 85 0.002 [default-backend-80] [] %s 37 0.002 200 %s`,
			p.Metodo, upstream, id))
	})
	k.cliente.PrependReactor("get", "pods", func(a k8stesting.Action) (bool, runtime.Object, error) {
		if a.GetSubresource() != "log" || a.GetNamespace() != "ingress-nginx" {
			return false, nil, nil
		}
		opts := a.(k8stesting.GenericAction).GetValue().(*corev1.PodLogOptions)
		l.mu.Lock()
		defer l.mu.Unlock()
		l.todos = append(l.todos, *opts)
		texto := ""
		for _, linea := range l.lineas {
			texto += linea + "\n"
		}
		return true, &runtime.Unknown{Raw: []byte(texto)}, nil
	})
	return l
}
