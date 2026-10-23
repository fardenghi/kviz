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

	"kviz/internal/sondeo"
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

// ---------- arnés: el log del Ingress Controller ----------

// logDelIC es el Ingress Controller del cluster falso: su log tiene una línea por cada request de
// prueba que pasó por él, con el X-Request-ID que le puso kviz y el upstream que eligió.
type logDelIC struct {
	mu       sync.Mutex
	upstream string
	lineas   []string
	todos    []corev1.PodLogOptions
}

func (l *logDelIC) pedidos() []corev1.PodLogOptions {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]corev1.PodLogOptions{}, l.todos...)
}

// ingressControllerQueManda hace que las requests de prueba que lleguen con X-Request-ID dejen en
// el log del Ingress Controller una línea con el formato de ingress-nginx que dice que la mandó
// al upstream dado. Con upstream "" la request no deja línea (como si el log todavía no estuviera).
func (k *kvizDePrueba) ingressControllerQueManda(t *testing.T, upstream string) *logDelIC {
	t.Helper()
	l := &logDelIC{upstream: upstream, lineas: []string{
		`10.244.0.9 - - [23/Oct/2026:15:02:10 +0000] "GET /backend/hello HTTP/1.1" 200 37 "-" "curl/8.10.1" 85 0.002 [default-backend-80] [] 10.244.2.4:8081 37 0.002 200 otra-request`,
	}}
	k.sondeo.alEjecutar(func(p sondeo.PedidoHTTP) {
		id := ""
		for _, h := range p.Headers {
			if h.Nombre == "X-Request-ID" {
				id = h.Valor
			}
		}
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
