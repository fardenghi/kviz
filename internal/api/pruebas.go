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
	"sync/atomic"
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
}

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

func mandarPrueba(cluster Cluster, ids *atomic.Int64) http.HandlerFunc {
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
		metodo := strings.ToUpper(strings.TrimSpace(p.Metodo))
		if metodo == "" {
			metodo = http.MethodGet
		}
		s, err := cluster.Sondeo()
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		// si la request pasa por el Ingress Controller, lleva un X-Request-ID para encontrarla
		// después en su log y saber a qué pod la mandó
		host := trafico.ConHost(valorDe(p.Headers, "Host"))
		headers := p.Headers
		idDeRequest := ""
		if len(trafico.PodsDelIngressController(trafico.Recorrer(cluster.Foto(), u, host))) > 0 {
			if idDeRequest = valorDe(headers, trafico.HeaderDeRequest); idDeRequest == "" {
				idDeRequest = nuevoIDDeRequest()
				headers = append(slices.Clone(headers), sondeo.Header{Nombre: trafico.HeaderDeRequest, Valor: idDeRequest})
			}
		}

		ctx, cancelar := context.WithTimeout(r.Context(), esperaPrueba)
		defer cancelar()
		res, pod, err := s.HTTP(ctx, sondeo.PedidoHTTP{Metodo: metodo, URL: u.String(), Headers: headers, Body: p.Body})
		var sinRespuesta sondeo.SinRespuesta
		if err != nil && !errors.As(err, &sinRespuesta) {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		// el recorrido se calcula con la foto de después de la request: la más cercana a la respuesta
		f := cluster.Foto()
		out := prueba{
			ID:        int(ids.Add(1)),
			Tipo:      p.Tipo,
			URL:       u.String(),
			Metodo:    metodo,
			Origen:    origen(pod),
			Recorrido: trafico.Recorrer(f, u, host),
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
			out.Recorrido = conPodQueRespondio(ctx, cluster, f, out.Recorrido, res.Body, idDeRequest)
		}
		responderJSON(w, out)
	}
}

// conPodQueRespondio completa el recorrido con el pod que respondió, si se sabe: por el body o,
// si pasó por el Ingress Controller, por la línea de su log con el X-Request-ID de la request.
func conPodQueRespondio(ctx context.Context, cluster Cluster, f foto.Foto, r trafico.Recorrido, body, idDeRequest string) trafico.Recorrido {
	if trafico.PodQueRespondio(f, r, body) != nil || idDeRequest == "" {
		return trafico.ConRespuesta(f, r, body)
	}
	if upstream := upstreamDelIngressController(ctx, cluster, f, r, idDeRequest); upstream != "" {
		return trafico.ConUpstream(f, r, upstream)
	}
	return r
}

// esperaLogDelIC es cuánto se espera a que la línea de la request aparezca en el log del Ingress
// Controller (nginx la escribe al terminar la request, y el log llega un poco después).
const (
	esperaLogDelIC    = 2 * time.Second
	intervaloLogDelIC = 100 * time.Millisecond
)

// upstreamDelIngressController busca en el final del log de los pods del Ingress Controller del
// recorrido la línea de la request y devuelve el endpoint al que la mandó; "" si no aparece.
func upstreamDelIngressController(ctx context.Context, cluster Cluster, f foto.Foto, r trafico.Recorrido, idDeRequest string) string {
	limite := time.Now().Add(esperaLogDelIC)
	for {
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
			if upstream := trafico.UpstreamDelLog(strings.Split(string(texto), "\n"), idDeRequest); upstream != "" {
				return upstream
			}
		}
		if time.Now().After(limite) {
			return ""
		}
		select {
		case <-ctx.Done():
			return ""
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
