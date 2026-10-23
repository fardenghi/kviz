package api_test

// Reparto e historial de las requests de prueba: con `repeticiones` la misma request se manda N
// veces y kviz cuenta qué pod respondió cada una (con la misma identificación que el recorrido);
// cada prueba queda en el historial de la sesión, en memoria.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/fardenghi/kviz/internal/sondeo"
)

// pedidoRepetido es el pedido del formulario con la cantidad de repeticiones.
type pedidoRepetido struct {
	pedidoDePrueba
	Repeticiones int `json:"repeticiones,omitempty"`
}

// pruebaConReparto es la prueba con los campos del reparto.
type pruebaConReparto struct {
	pruebaJSON
	Contexto     string       `json:"contexto"`
	Repeticiones int          `json:"repeticiones"`
	Reparto      *repartoJSON `json:"reparto"`
}

type repartoJSON struct {
	Total          int          `json:"total"`
	Pods           []porPodJSON `json:"pods"`
	SinIdentificar int          `json:"sinIdentificar"`
	SinRespuesta   int          `json:"sinRespuesta"`
}

type porPodJSON struct {
	Pod string `json:"pod"`
	N   int    `json:"n"`
}

// respondenPorTurno hace que el doble del pod de sondeo conteste cada request con el body que
// toca, en orden; como kube-proxy, que elige un endpoint por request.
func (s *sondeoDePrueba) respondenPorTurno(cuerpos ...string) {
	i := 0
	s.guionar(func(sondeo.PedidoHTTP) (sondeo.RespuestaHTTP, error) {
		b := cuerpos[i%len(cuerpos)]
		i++
		return sondeo.RespuestaHTTP{Status: 200, Body: b}, nil
	})
}

func TestRepetirUnaRequestOchoVecesMuestraElRepartoEntreLosPods(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	a, b := "Hello, "+podK2x8p+"!\n", "Hello, "+pod7hw4q+"!\n"
	kviz.sondeo.respondenPorTurno(a, b, b, a, b, b, a, b)

	status, got := kviz.repetirPrueba(t, pedidoRepetido{
		pedidoDePrueba: pedidoDePrueba{Destino: "backend.default.svc.cluster.local:80", Path: "/hello"},
		Repeticiones:   8,
	})

	if status != http.StatusOK {
		t.Fatalf("POST /api/pruebas: status %d", status)
	}
	if n := len(kviz.sondeo.ejecutados()); n != 8 {
		t.Fatalf("el pod de sondeo ejecutó %d requests, want 8", n)
	}
	want := &repartoJSON{Total: 8, Pods: []porPodJSON{{Pod: "default/" + pod7hw4q, N: 5}, {Pod: "default/" + podK2x8p, N: 3}}}
	if got.Repeticiones != 8 || !reflect.DeepEqual(got.Reparto, want) {
		t.Fatalf("repeticiones %d, reparto = %s\nwant 8 y %s", got.Repeticiones, mostrar(got.Reparto), mostrar(want))
	}
}

func TestLasRespuestasSinPodIdentificadoSeCuentanAparte(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	turno := 0
	kviz.sondeo.guionar(func(sondeo.PedidoHTTP) (sondeo.RespuestaHTTP, error) {
		turno++
		switch turno {
		case 1, 4:
			return sondeo.RespuestaHTTP{Status: 200, Body: "Hello, " + podK2x8p + "!"}, nil
		case 2:
			return sondeo.RespuestaHTTP{}, sondeo.SinRespuesta{Motivo: "Operation timed out"}
		default:
			return sondeo.RespuestaHTTP{Status: 200, Body: "OK"}, nil
		}
	})

	_, got := kviz.repetirPrueba(t, pedidoRepetido{pedidoDePrueba: pedidoDePrueba{Destino: "10.96.11.121"}, Repeticiones: 5})

	want := &repartoJSON{Total: 5, Pods: []porPodJSON{{Pod: "default/" + podK2x8p, N: 2}}, SinIdentificar: 2, SinRespuesta: 1}
	if !reflect.DeepEqual(got.Reparto, want) {
		t.Fatalf("reparto = %s\nwant %s", mostrar(got.Reparto), mostrar(want))
	}
	// la respuesta y el recorrido que se muestran son los de la primera request
	if got.Respuesta == nil || got.Respuesta.Body != "Hello, "+podK2x8p+"!" || got.Recorrido.Pasos[len(got.Recorrido.Pasos)-1].Tipo != "pod" {
		t.Fatalf("prueba = %s, want la respuesta de la primera request y su pod en el recorrido", mostrar(got))
	}
}

func TestUnaSolaRequestNoTraeReparto(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)

	_, got := kviz.repetirPrueba(t, pedidoRepetido{pedidoDePrueba: pedidoDePrueba{Destino: "backend.default"}})

	if got.Repeticiones != 1 || got.Reparto != nil || len(kviz.sondeo.ejecutados()) != 1 {
		t.Fatalf("repeticiones %d, reparto %s, ejecutadas %d; want 1, null y 1", got.Repeticiones, mostrar(got.Reparto), len(kviz.sondeo.ejecutados()))
	}
}

func TestLasRepeticionesTienenUnTope(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)

	for _, n := range []int{-1, 21} {
		if status, _ := kviz.repetirPrueba(t, pedidoRepetido{pedidoDePrueba: pedidoDePrueba{Destino: "backend.default"}, Repeticiones: n}); status != http.StatusBadRequest {
			t.Errorf("%d repeticiones: status %d, want 400", n, status)
		}
	}
	if len(kviz.sondeo.ejecutados()) != 0 {
		t.Fatal("el pod de sondeo no debería haber ejecutado nada")
	}
}

// ---------- historial ----------

func TestElHistorialGuardaCadaRequestDeLaSesionConSuRespuestaYSuRecorrido(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	kviz.sondeo.respondenPorTurno("Hello, "+podK2x8p+"!", "Hello, "+pod7hw4q+"!")
	_, primera := kviz.repetirPrueba(t, pedidoRepetido{pedidoDePrueba: pedidoDePrueba{Destino: "backend.default", Path: "/hello"}})
	_, segunda := kviz.repetirPrueba(t, pedidoRepetido{pedidoDePrueba: pedidoDePrueba{Destino: "10.96.11.121"}, Repeticiones: 4})

	var historial []pruebaConReparto
	kviz.getJSON(t, "/api/pruebas", &historial)

	if !reflect.DeepEqual(historial, []pruebaConReparto{primera, segunda}) {
		t.Fatalf("historial = %s\nwant las dos pruebas tal como se respondieron", mostrar(historial))
	}
	if primera.Contexto != "kind-lab" || primera.ID == segunda.ID {
		t.Fatalf("pruebas = %s, want con su contexto y un id cada una", mostrar(historial))
	}

	// al hacer clic en una se vuelve a ver su respuesta y su recorrido
	var vuelta pruebaConReparto
	kviz.getJSON(t, "/api/pruebas/"+strconv.Itoa(primera.ID), &vuelta)
	if !reflect.DeepEqual(vuelta, primera) || vuelta.Recorrido.Pasos[2].Objetos[0].ID != "default/"+podK2x8p {
		t.Fatalf("GET /api/pruebas/%d = %s\nwant %s", primera.ID, mostrar(vuelta), mostrar(primera))
	}
}

func TestElHistorialNoOlvidaNingunaPruebaDeLaSesion(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	const muchas = 150
	for range muchas {
		kviz.repetirPrueba(t, pedidoRepetido{pedidoDePrueba: pedidoDePrueba{Destino: "backend.default"}})
	}

	var historial []pruebaConReparto
	kviz.getJSON(t, "/api/pruebas", &historial)

	if len(historial) != muchas || historial[0].ID != 1 {
		t.Fatalf("historial con %d pruebas, la primera %d; want las %d desde la 1", len(historial), historial[0].ID, muchas)
	}
	var primera pruebaConReparto
	kviz.getJSON(t, "/api/pruebas/1", &primera)
}

func TestUnaPruebaQueNoEstaEnElHistorialEs404(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	kviz.repetirPrueba(t, pedidoRepetido{pedidoDePrueba: pedidoDePrueba{Destino: "backend.default"}})

	for _, id := range []string{"2", "x"} {
		res, err := http.Get(kviz.srv.URL + "/api/pruebas/" + id)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusNotFound {
			t.Errorf("GET /api/pruebas/%s: status %d, want 404", id, res.StatusCode)
		}
	}
}

func TestElHistorialSePierdeAlCerrarKviz(t *testing.T) {
	directorio := t.TempDir()
	t.Chdir(directorio)
	antes := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	antes.repetirPrueba(t, pedidoRepetido{pedidoDePrueba: pedidoDePrueba{Destino: "backend.default"}, Repeticiones: 3})
	antes.cerrar(t)
	antes.srv.Close()

	despues := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)
	var historial []pruebaConReparto
	despues.getJSON(t, "/api/pruebas", &historial)

	if historial == nil || len(historial) != 0 {
		t.Fatalf("historial al volver a abrir kviz = %s, want vacío ([])", mostrar(historial))
	}
	if archivos, _ := os.ReadDir(directorio); len(archivos) != 0 {
		t.Fatalf("kviz escribió %d archivos en el directorio de trabajo, want ninguno", len(archivos))
	}
}

// ---------- arnés ----------

// repetirPrueba manda una request de prueba con repeticiones, como el formulario de la interfaz.
func (k *kvizDePrueba) repetirPrueba(t *testing.T, p pedidoRepetido) (int, pruebaConReparto) {
	t.Helper()
	cuerpo, _ := json.Marshal(p)
	res, err := http.Post(k.srv.URL+"/api/pruebas", "application/json", bytes.NewReader(cuerpo))
	if err != nil {
		t.Fatalf("POST /api/pruebas: %v", err)
	}
	defer res.Body.Close()
	var got pruebaConReparto
	if res.StatusCode == http.StatusOK {
		if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
			t.Fatalf("POST /api/pruebas: decodificar: %v", err)
		}
	}
	return res.StatusCode, got
}
