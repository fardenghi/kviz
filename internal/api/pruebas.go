package api

// Request de prueba: POST /api/pruebas. kviz la origina desde el pod de sondeo del cluster
// conectado (lo crea la primera vez), y devuelve la respuesta cruda y el recorrido calculado desde
// la foto (ver internal/trafico).
//
// Pensado para crecer sin romper el contrato: la prueba de conexión TCP es otro `tipo` con su
// resultado en otro campo; las repeticiones y el reparto, campos nuevos del pedido y de la
// respuesta; el historial, GET sobre las pruebas guardadas por `id`.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"kviz/internal/conexion"
	"kviz/internal/foto"
	"kviz/internal/sondeo"
	"kviz/internal/trafico"
)

// pedidoDePrueba es el cuerpo de POST /api/pruebas.
type pedidoDePrueba struct {
	// Tipo es "http" (por defecto).
	Tipo string `json:"tipo"`
	// Destino es `host[:puerto]` o una URL: `backend.default.svc.cluster.local:80`, `10.96.11.121`,
	// `http://backend.default/hello`.
	Destino string `json:"destino"`
	// Metodo HTTP; GET por defecto.
	Metodo string `json:"metodo"`
	// Path reemplaza al del destino, si viene; por defecto "/".
	Path    string          `json:"path"`
	Headers []sondeo.Header `json:"headers"`
	Body    string          `json:"body"`
	// Repeticiones es cuántas veces se manda la misma request (1 por defecto, hasta
	// maxRepeticiones); con más de una, la prueba trae el reparto entre los pods.
	Repeticiones int `json:"repeticiones"`
}

// maxRepeticiones es el tope de repeticiones de una request de prueba: van una detrás de otra.
const maxRepeticiones = 20

// prueba es la respuesta de POST /api/pruebas: una request de prueba ya ejecutada.
type prueba struct {
	// ID numera las pruebas de la sesión (1, 2, …).
	ID   int    `json:"id"`
	Tipo string `json:"tipo"`
	// URL es la que ejecutó el pod de sondeo.
	URL    string `json:"url"`
	Metodo string `json:"metodo"`
	// Origen describe el pod de sondeo del que salió la request.
	Origen string `json:"origen"`
	// Respuesta es null si la request no recibió respuesta; entonces Error dice por qué.
	Respuesta *respuestaHTTP `json:"respuesta"`
	Error     string         `json:"error,omitempty"`
	// Recorrido es el camino calculado desde la configuración, completado con el pod que respondió
	// si se puede saber.
	Recorrido trafico.Recorrido `json:"recorrido"`
	// Contexto es el contexto de kubectl en el que se mandó: el historial dura toda la sesión.
	Contexto string `json:"contexto"`
	// Repeticiones es cuántas veces se mandó la request; Respuesta, Error y el pod del recorrido
	// son los de la primera.
	Repeticiones int `json:"repeticiones"`
	// Reparto cuenta las respuestas por pod; null si la request se mandó una sola vez.
	Reparto *trafico.Reparto `json:"reparto"`
}

type respuestaHTTP struct {
	Status int `json:"status"`
	// Texto es el texto del status: OK, Not Found…
	Texto   string          `json:"texto"`
	Headers []sondeo.Header `json:"headers"`
	Body    string          `json:"body"`
	// Ms es el tiempo total de la request en milisegundos, medido en el pod de sondeo.
	Ms float64 `json:"ms"`
}

// esperaPrueba es el tiempo máximo de una request de prueba, contando crear el pod de sondeo.
const esperaPrueba = sondeo.EsperaArranque + sondeo.EsperaRequest + 10*time.Second

func mandarPrueba(cluster Cluster, h *historial) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var p pedidoDePrueba
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, "se esperaba un pedido de prueba en JSON: "+err.Error(), http.StatusBadRequest)
			return
		}
		if p.Tipo == "" {
			p.Tipo = "http"
		}
		if p.Tipo != "http" {
			http.Error(w, fmt.Sprintf("tipo de prueba desconocido: %q", p.Tipo), http.StatusBadRequest)
			return
		}
		u, err := trafico.URL(p.Destino, p.Path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if p.Repeticiones == 0 {
			p.Repeticiones = 1
		}
		if p.Repeticiones < 1 || p.Repeticiones > maxRepeticiones {
			http.Error(w, fmt.Sprintf("las repeticiones van de 1 a %d", maxRepeticiones), http.StatusBadRequest)
			return
		}
		metodo := strings.ToUpper(strings.TrimSpace(p.Metodo))
		if metodo == "" {
			metodo = http.MethodGet
		}
		s, err := cluster.Sondeo()
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		// si la request pasa por el Ingress Controller, cada repetición lleva un X-Request-ID para
		// encontrarla después en su log y saber a qué pod la mandó
		host := trafico.ConHost(valorDe(p.Headers, "Host"))
		porIC := len(trafico.PodsDelIngressController(trafico.Recorrer(cluster.Foto(), u, host))) > 0
		armar := func() (sondeo.PedidoHTTP, string) {
			pedido := sondeo.PedidoHTTP{Metodo: metodo, URL: u.String(), Headers: p.Headers, Body: p.Body}
			if !porIC {
				return pedido, ""
			}
			if id := valorDe(p.Headers, trafico.HeaderDeRequest); id != "" {
				return pedido, id
			}
			id := nuevoIDDeRequest()
			pedido.Headers = append(slices.Clone(p.Headers), sondeo.Header{Nombre: trafico.HeaderDeRequest, Valor: id})
			return pedido, id
		}

		ctx, cancelar := context.WithTimeout(r.Context(), esperaPrueba+time.Duration(p.Repeticiones-1)*sondeo.EsperaRequest)
		defer cancelar()
		pedido, id := armar()
		res, pod, err := s.HTTP(ctx, pedido)
		var sinRespuesta sondeo.SinRespuesta
		if err != nil && !errors.As(err, &sinRespuesta) {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		// las repeticiones van una detrás de otra; la primera es la que se muestra
		var respuestas []trafico.Respuesta
		var ids []string
		sinRespuestas := 0
		if err == nil {
			respuestas, ids = append(respuestas, trafico.Respuesta{Body: res.Body}), append(ids, id)
		} else {
			sinRespuestas++
		}
		for range p.Repeticiones - 1 {
			pedido, id := armar()
			otra, _, errOtra := s.HTTP(ctx, pedido)
			switch {
			case errOtra == nil:
				respuestas, ids = append(respuestas, trafico.Respuesta{Body: otra.Body}), append(ids, id)
			case errors.As(errOtra, new(sondeo.SinRespuesta)):
				sinRespuestas++
			default:
				http.Error(w, errOtra.Error(), http.StatusServiceUnavailable)
				return
			}
		}

		// el recorrido se calcula con la foto de después de la request: la más cercana a la respuesta
		f := cluster.Foto()
		recorrido := trafico.Recorrer(f, u, host)
		if porIC && len(respuestas) > 0 {
			upstreams := upstreamsDelIngressController(ctx, cluster, f, recorrido, ids)
			for i := range respuestas {
				respuestas[i].Upstream = upstreams[ids[i]]
			}
		}
		out := prueba{
			ID:        h.nuevoID(),
			Tipo:      p.Tipo,
			URL:       u.String(),
			Metodo:    metodo,
			Origen:    origen(pod),
			Recorrido: recorrido,
		}
		out.Contexto, out.Repeticiones = f.Contexto, p.Repeticiones
		if p.Repeticiones > 1 {
			reparto := trafico.Repartir(f, recorrido, respuestas, sinRespuestas)
			out.Reparto = &reparto
		}
		if err != nil {
			out.Error = sinRespuesta.Motivo
		} else {
			out.Respuesta = &respuestaHTTP{
				Status:  res.Status,
				Texto:   http.StatusText(res.Status),
				Headers: res.Headers,
				Body:    res.Body,
				Ms:      float64(res.Duracion.Microseconds()) / 1000,
			}
			if out.Respuesta.Headers == nil {
				out.Respuesta.Headers = []sondeo.Header{}
			}
			out.Recorrido = trafico.ConRespuesta(f, recorrido, respuestas[0])
		}
		h.guardar(out)
		responderJSON(w, out)
	}
}

// esperaLogDelIC es cuánto se espera a que las líneas de las requests aparezcan en el log del
// Ingress Controller (nginx las escribe al terminar cada request, y el log llega un poco después).
const (
	esperaLogDelIC    = 2 * time.Second
	intervaloLogDelIC = 100 * time.Millisecond
)

// upstreamsDelIngressController busca en el final del log de los pods del Ingress Controller del
// recorrido la línea de cada request (por su X-Request-ID) y devuelve el endpoint al que la mandó;
// las que no aparecen a tiempo quedan sin upstream.
func upstreamsDelIngressController(ctx context.Context, cluster Cluster, f foto.Foto, r trafico.Recorrido, ids []string) map[string]string {
	upstreams := map[string]string{}
	limite := time.Now().Add(esperaLogDelIC)
	for {
		var lineas []string
		for _, ref := range trafico.PodsDelIngressController(r) {
			pedido, ok := pedidoDeLogDelIC(f, ref.ID)
			if !ok {
				continue
			}
			logs, err := cluster.Logs(ctx, pedido)
			if err != nil {
				continue
			}
			texto, _ := io.ReadAll(io.LimitReader(logs, 1<<20))
			logs.Close()
			lineas = append(lineas, strings.Split(string(texto), "\n")...)
		}
		faltan := 0
		for _, id := range ids {
			if upstreams[id] == "" {
				upstreams[id] = trafico.UpstreamDelLog(lineas, id)
			}
			if upstreams[id] == "" {
				faltan++
			}
		}
		if faltan == 0 || time.Now().After(limite) {
			return upstreams
		}
		select {
		case <-ctx.Done():
			return upstreams
		case <-time.After(intervaloLogDelIC):
		}
	}
}

// pedidoDeLogDelIC pide las últimas líneas del log del primer container normal del pod.
func pedidoDeLogDelIC(f foto.Foto, id string) (conexion.PedidoDeLogs, bool) {
	for _, p := range f.Pods {
		if p.ID != id {
			continue
		}
		for _, c := range p.Containers {
			if c.Tipo == "normal" {
				return conexion.PedidoDeLogs{Namespace: p.Namespace, Pod: p.Nombre, Container: c.Nombre, SinSeguir: true, Ultimas: 200}, true
			}
		}
	}
	return conexion.PedidoDeLogs{}, false
}

// valorDe es el valor del header (sin importar mayúsculas), o "".
func valorDe(headers []sondeo.Header, nombre string) string {
	for _, h := range headers {
		if strings.EqualFold(h.Nombre, nombre) {
			return h.Valor
		}
	}
	return ""
}

func nuevoIDDeRequest() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "kviz-" + hex.EncodeToString(b)
}

func origen(p sondeo.Pod) string {
	o := fmt.Sprintf("pod de sondeo %s (%s", p.Nombre, p.Namespace)
	if p.Nodo != "" {
		o += ", " + p.Nodo
	}
	return o + ")"
}
