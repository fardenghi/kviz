package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/fardenghi/kviz/internal/conexion"
)

// contextos es la respuesta de GET /api/contextos.
type contextos struct {
	// Actual es el contexto de kviz ("" si no hay); puede faltar en Contextos si se borró su cluster.
	Actual    string   `json:"actual"`
	Contextos []string `json:"contextos"`
	// Error explica por qué no se pudo leer el kubeconfig.
	Error string `json:"error,omitempty"`
}

func listarContextos(cluster Cluster) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actual, todos, err := cluster.Contextos()
		res := contextos{Actual: actual, Contextos: todos}
		if res.Contextos == nil {
			res.Contextos = []string{}
		}
		if err != nil {
			res.Error = err.Error()
		}
		responderJSON(w, res)
	}
}

// pedidoDeContexto es el cuerpo de POST /api/contexto.
type pedidoDeContexto struct {
	Contexto string `json:"contexto"`
}

// cambiarContexto responde 204 apenas empieza el cambio: la foto del nuevo cluster llega por el
// stream cuando se sincroniza. Un contexto que no está en el kubeconfig da 404.
func cambiarContexto(cluster Cluster) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var p pedidoDeContexto
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil || p.Contexto == "" {
			http.Error(w, `se esperaba {"contexto": "<nombre>"}`, http.StatusBadRequest)
			return
		}
		switch err := cluster.CambiarContexto(p.Contexto); {
		case errors.Is(err, conexion.ErrContextoInexistente):
			http.Error(w, err.Error(), http.StatusNotFound)
		case err != nil:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}
}
