package api_test

// Prueba de conexión TCP (POST /api/pruebas con tipo "tcp"): para destinos que no hablan HTTP,
// kviz abre una conexión TCP desde el pod de sondeo y dice si el puerto abre, rechaza o da timeout,
// con el mismo recorrido que una request de prueba. El pod de sondeo es el mismo doble que en
// pruebas_test.go, con resultados guionados.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/fardenghi/kviz/internal/sondeo"
)

// conexionJSON es el resultado de una prueba de conexión, tal como lo lee la interfaz.
type conexionJSON struct {
	Resultado string  `json:"resultado"`
	Ms        float64 `json:"ms"`
	Motivo    string  `json:"motivo"`
}

func TestUnaPruebaDeConexionAUnPuertoQueAbreDiceAbreConElTiempo(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", append(clusterDeLaGuia(), postgresConService()...)...)
	kviz.sondeo.conectar(sondeo.ResultadoTCP{Resultado: sondeo.Abre, Duracion: 1500 * time.Microsecond, IPRemota: "10.96.20.5"})

	status, got := kviz.mandarConexion(t, pedidoDePrueba{Tipo: "tcp", Destino: "postgres.default.svc.cluster.local:5432"})

	if status != http.StatusOK {
		t.Fatalf("POST /api/pruebas: status %d", status)
	}
	want := &conexionJSON{Resultado: "abre", Ms: 1.5}
	if got.Tipo != "tcp" || got.URL != "tcp://postgres.default.svc.cluster.local:5432" || got.Respuesta != nil ||
		!reflect.DeepEqual(got.Conexion, want) || got.Error != "" {
		t.Fatalf("prueba = %s, want la conexión abierta a tcp://postgres.default.svc.cluster.local:5432 en 1.5 ms", mostrar(got))
	}
	wantPedido := []sondeo.PedidoTCP{{Host: "postgres.default.svc.cluster.local", Puerto: 5432}}
	if e := kviz.sondeo.conexionesEjecutadas(); !reflect.DeepEqual(e, wantPedido) || len(kviz.sondeo.ejecutados()) != 0 {
		t.Fatalf("el pod de sondeo ejecutó las conexiones %s y %d requests, want solo %s", mostrar(e), len(kviz.sondeo.ejecutados()), mostrar(wantPedido))
	}
}

func TestLaPruebaDeConexionMuestraElMismoRecorridoQueUnaRequestDePrueba(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", append(clusterDeLaGuia(), postgresConService()...)...)

	_, got := kviz.mandarConexion(t, pedidoDePrueba{Tipo: "tcp", Destino: "10.96.20.5:5432"})

	want := []pasoJSON{
		{N: 1, Tipo: "service", Titulo: "postgres 10.96.20.5:5432 → :5432", Detalle: "ClusterIP, targetPort 5432",
			Objetos: []refJSON{{Tipo: "service", ID: "default/postgres"}}},
		{N: 2, Tipo: "endpoints", Titulo: "10.244.2.6:5432", Detalle: "1 endpoint listo",
			Objetos: []refJSON{{Tipo: "pod", ID: "default/postgres"}}},
	}
	if !reflect.DeepEqual(got.Recorrido.Pasos, want) {
		t.Fatalf("recorrido = %s\nwant %s", mostrar(got.Recorrido.Pasos), mostrar(want))
	}
}

func TestUnaPruebaDeConexionAUnPuertoCerradoDiceRechaza(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	kviz.sondeo.conectar(sondeo.ResultadoTCP{
		Resultado: sondeo.Rechaza, Duracion: 2 * time.Millisecond,
		Motivo: "Failed to connect to 10.244.1.5 port 5432 after 2 ms: Couldn't connect to server",
	})

	status, got := kviz.mandarConexion(t, pedidoDePrueba{Tipo: "tcp", Destino: "10.244.1.5:5432"})

	want := &conexionJSON{Resultado: "rechaza", Ms: 2, Motivo: "Failed to connect to 10.244.1.5 port 5432 after 2 ms: Couldn't connect to server"}
	if status != http.StatusOK || !reflect.DeepEqual(got.Conexion, want) || got.Respuesta != nil {
		t.Fatalf("status %d, prueba = %s, want la conexión rechazada con el motivo", status, mostrar(got))
	}
}

func TestUnaPruebaDeConexionSinRespuestaDiceTimeout(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", append(clusterDeLaGuia(), postgresConService()...)...)
	kviz.sondeo.conectar(sondeo.ResultadoTCP{
		Resultado: sondeo.Timeout, Duracion: sondeo.EsperaConexion,
		Motivo: "Connection timed out after 5000 milliseconds",
	})

	status, got := kviz.mandarConexion(t, pedidoDePrueba{Tipo: "tcp", Destino: "tcp://postgres.default:5432"})

	want := &conexionJSON{Resultado: "timeout", Ms: 5000, Motivo: "Connection timed out after 5000 milliseconds"}
	if status != http.StatusOK || !reflect.DeepEqual(got.Conexion, want) {
		t.Fatalf("status %d, prueba = %s, want timeout a los 5 s", status, mostrar(got))
	}
	if len(got.Recorrido.Pasos) != 3 {
		t.Fatalf("recorrido = %s, want DNS, Service y endpoints aunque no conecte", mostrar(got.Recorrido))
	}
}

func TestUnaPruebaDeConexionQueNoSaleDiceElMotivo(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	kviz.sondeo.guionarConexion(func(sondeo.PedidoTCP) (sondeo.ResultadoTCP, error) {
		return sondeo.ResultadoTCP{}, sondeo.SinRespuesta{Motivo: "Could not resolve host: nohay.default"}
	})

	status, got := kviz.mandarConexion(t, pedidoDePrueba{Tipo: "tcp", Destino: "nohay.default:5432"})

	if status != http.StatusOK || got.Conexion != nil || got.Error != "Could not resolve host: nohay.default" {
		t.Fatalf("status %d, prueba = %s, want sin resultado y con el motivo", status, mostrar(got))
	}
}

func TestUnaPruebaDeConexionNecesitaHostYPuerto(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)

	for _, destino := range []string{"postgres.default", "http://backend.default:80", " ", "postgres.default:puerto"} {
		if status, _ := kviz.mandarConexion(t, pedidoDePrueba{Tipo: "tcp", Destino: destino}); status != http.StatusBadRequest {
			t.Errorf("destino %q: status %d, want 400", destino, status)
		}
	}
	if len(kviz.sondeo.conexionesEjecutadas()) != 0 {
		t.Fatal("el pod de sondeo no debería haber ejecutado nada")
	}
}

func TestUnTipoDePruebaDesconocidoEsUnError(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)

	if status, _ := kviz.mandarConexion(t, pedidoDePrueba{Tipo: "udp", Destino: "backend.default:80"}); status != http.StatusBadRequest {
		t.Fatalf("status %d, want 400", status)
	}
}

func TestLaPruebaDeConexionQuedaEnElHistorialDeLaSesion(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", append(clusterDeLaGuia(), postgresConService()...)...)
	kviz.sondeo.conectar(sondeo.ResultadoTCP{Resultado: sondeo.Rechaza, Duracion: time.Millisecond, Motivo: "Connection refused"})

	_, got := kviz.mandarConexion(t, pedidoDePrueba{Tipo: "tcp", Destino: "postgres.default:5432"})

	var historial []pruebaDeConexionJSON
	kviz.getJSON(t, "/api/pruebas", &historial)
	if len(historial) != 1 || !reflect.DeepEqual(historial[0], got) || historial[0].Conexion == nil || historial[0].Conexion.Resultado != "rechaza" {
		t.Fatalf("historial = %s, want la prueba de conexión tal como se respondió (%s)", mostrar(historial), mostrar(got))
	}
	var contexto struct {
		Contexto     string `json:"contexto"`
		Repeticiones int    `json:"repeticiones"`
	}
	kviz.getJSON(t, fmt.Sprintf("/api/pruebas/%d", got.ID), &contexto)
	if contexto.Contexto != "kind-lab" || contexto.Repeticiones != 1 {
		t.Fatalf("prueba en el historial = %s, want del contexto kind-lab y mandada una vez", mostrar(contexto))
	}
}

func TestUnaPruebaDeConexionNoSeRepite(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)

	status, _ := kviz.repetirPrueba(t, pedidoRepetido{pedidoDePrueba: pedidoDePrueba{Tipo: "tcp", Destino: "backend.default:80"}, Repeticiones: 3})

	if status != http.StatusBadRequest || len(kviz.sondeo.conexionesEjecutadas()) != 0 {
		t.Fatalf("status %d, %d conexiones, want 400 sin conectar: el reparto es de requests HTTP", status, len(kviz.sondeo.conexionesEjecutadas()))
	}
}

// ---------- arnés ----------

// pruebaDeConexionJSON es la respuesta de POST /api/pruebas con el campo de la prueba de conexión.
type pruebaDeConexionJSON struct {
	pruebaJSON
	Conexion *conexionJSON `json:"conexion"`
}

// mandarConexion manda una prueba como el formulario de la interfaz y lee también su conexión.
func (k *kvizDePrueba) mandarConexion(t *testing.T, p pedidoDePrueba) (int, pruebaDeConexionJSON) {
	t.Helper()
	cuerpo, _ := json.Marshal(p)
	res, err := http.Post(k.srv.URL+"/api/pruebas", "application/json", bytes.NewReader(cuerpo))
	if err != nil {
		t.Fatalf("POST /api/pruebas: %v", err)
	}
	defer res.Body.Close()
	var got pruebaDeConexionJSON
	if res.StatusCode == http.StatusOK {
		if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
			t.Fatalf("POST /api/pruebas: decodificar: %v", err)
		}
	}
	return res.StatusCode, got
}

// postgresConService es 5b-postgres.yaml con un Service ClusterIP delante, como el que usa la app
// de la guía para llegar a la base.
func postgresConService() []runtime.Object {
	objetos := postgresDeLaGuia()
	for _, o := range objetos {
		if p, ok := o.(*corev1.Pod); ok {
			p.Labels = map[string]string{"app": "postgres"}
		}
	}
	return append(objetos,
		service("default", "postgres", corev1.ServiceTypeClusterIP, "10.96.20.5", map[string]string{"app": "postgres"},
			corev1.ServicePort{Protocol: corev1.ProtocolTCP, Port: 5432, TargetPort: intstr.FromInt32(5432)}),
		endpointSlice("default", "postgres-abc12", "postgres", endpoint("10.244.2.6", "postgres", "lab-worker2", ptrBool(true))),
	)
}
