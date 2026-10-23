package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/fardenghi/kviz/internal/conexion"
)

// pedidoDeSecret es el cuerpo de POST /api/secreto.
type pedidoDeSecret struct {
	Namespace string `json:"namespace"`
	Nombre    string `json:"nombre"`
	Clave     string `json:"clave"`
}

// valorDeSecret es la respuesta de POST /api/secreto.
type valorDeSecret struct {
	Valor string `json:"valor"`
}

// revelarSecret devuelve el valor de una clave de un Secret: es el único camino por el que un
// valor de Secret sale de kviz (la foto y el stream traen solo las claves). Es POST para que
// sea siempre un pedido explícito, y la respuesta no se guarda en ninguna caché.
func revelarSecret(cluster Cluster) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var p pedidoDeSecret
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil || p.Namespace == "" || p.Nombre == "" || p.Clave == "" {
			http.Error(w, `se esperaba {"namespace": "…", "nombre": "…", "clave": "…"}`, http.StatusBadRequest)
			return
		}
		valor, err := cluster.ValorDeSecret(p.Namespace, p.Nombre, p.Clave)
		switch {
		case errors.Is(err, conexion.ErrSecretInexistente):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, conexion.ErrDesconectado):
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
		case err != nil:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		default:
			responderJSON(w, valorDeSecret{Valor: valor})
		}
	}
}
