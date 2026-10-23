// Package api es la API de kviz (módulo 12 de la spec) y la costura 1 de los tests: la interfaz
// le habla solo por acá y nunca directo a Kubernetes.
//
// Rutas:
//
//	GET /api/foto   foto completa del cluster (ver internal/foto)
//	GET /api/stream canal de streaming (Server-Sent Events): la foto y sus cambios (ver stream.go)
//	GET /api/contextos   el contexto de kviz y los contextos del kubeconfig (ver contexto.go)
//	POST /api/contexto   cambia de contexto en caliente
//	GET /           la interfaz compilada (web/dist)
package api

import (
	"encoding/json"
	"net/http"

	"kviz/internal/foto"
)

// Cluster es lo que la API necesita del cluster conectado.
type Cluster interface {
	Foto() foto.Foto
	// Suscribir avisa cada vez que la foto puede haber cambiado; cancelar deja de avisar.
	Suscribir() (avisos <-chan struct{}, cancelar func())
	// Contextos devuelve el contexto de kviz y todos los contextos de kubectl.
	Contextos() (actual string, todos []string, err error)
	// CambiarContexto se conecta en caliente al cluster de otro contexto.
	CambiarContexto(contexto string) error
}

// Nuevo arma el handler HTTP de kviz. ui sirve la interfaz (web.Handler); si es nil no se sirve.
func Nuevo(cluster Cluster, ui http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/foto", func(w http.ResponseWriter, r *http.Request) {
		responderJSON(w, cluster.Foto())
	})
	mux.HandleFunc("GET /api/stream", transmitirFoto(cluster))
	mux.HandleFunc("GET /api/contextos", listarContextos(cluster))
	mux.HandleFunc("POST /api/contexto", cambiarContexto(cluster))
	if ui != nil {
		mux.Handle("GET /", ui)
	}
	return mux
}

func responderJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
