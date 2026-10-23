package api

// Prueba de conexión TCP: POST /api/pruebas con `tipo: "tcp"`. Para destinos que no hablan HTTP
// (Postgres en 5432, por ejemplo): kviz abre una conexión TCP desde el pod de sondeo, sin hablar
// ningún protocolo, y dice si el puerto abre, rechaza o da timeout. El recorrido es el mismo que el
// de una request de prueba al mismo host y puerto, y queda en el historial igual. No se repite: el
// reparto es de requests HTTP (el pod que respondió sale del body).

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/fardenghi/kviz/internal/sondeo"
	"github.com/fardenghi/kviz/internal/trafico"
)

// conexionTCP es el resultado de una prueba de conexión, en el campo `conexion` de la prueba.
type conexionTCP struct {
	// Resultado es "abre", "rechaza" o "timeout" (sondeo.Abre, Rechaza o Timeout).
	Resultado sondeo.ResultadoDeConexion `json:"resultado"`
	// Ms es lo que tardó en abrirse, en ser rechazada o en vencer, medido en el pod de sondeo.
	Ms float64 `json:"ms"`
	// Motivo es el mensaje del pod de sondeo cuando no abre.
	Motivo string `json:"motivo,omitempty"`
}

// esperaConexion es el tiempo máximo de una prueba de conexión, contando crear el pod de sondeo.
const esperaConexion = sondeo.EsperaArranque + sondeo.EsperaConexion + 10*time.Second

func mandarConexion(w http.ResponseWriter, r *http.Request, cluster Cluster, h *historial, p pedidoDePrueba) {
	u, err := trafico.URLTCP(p.Destino)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if p.Repeticiones > 1 {
		http.Error(w, "la prueba de conexión no se repite: el reparto es de requests HTTP", http.StatusBadRequest)
		return
	}
	puerto, _ := strconv.ParseInt(u.Port(), 10, 32)
	s, err := cluster.Sondeo()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	ctx, cancelar := context.WithTimeout(r.Context(), esperaConexion)
	defer cancelar()
	res, pod, err := s.TCP(ctx, sondeo.PedidoTCP{Host: u.Hostname(), Puerto: int32(puerto)})
	var sinRespuesta sondeo.SinRespuesta
	if err != nil && !errors.As(err, &sinRespuesta) {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	f := cluster.Foto()
	out := prueba{
		ID:           h.nuevoID(),
		Tipo:         p.Tipo,
		URL:          u.String(),
		Origen:       origen(pod),
		Recorrido:    trafico.Recorrer(f, u, trafico.DesdeNodo(pod.Nodo)),
		Contexto:     f.Contexto,
		Repeticiones: 1,
	}
	if err != nil {
		out.Error = sinRespuesta.Motivo
	} else {
		out.Conexion = &conexionTCP{
			Resultado: res.Resultado,
			Ms:        float64(res.Duracion.Microseconds()) / 1000,
			Motivo:    res.Motivo,
		}
	}
	h.guardar(out)
	responderJSON(w, out)
}
