// Package api es la API de kviz (módulo 12 de la spec) y la costura 1 de los tests: la interfaz
// le habla solo por acá y nunca directo a Kubernetes.
//
// Rutas:
//
//	GET /api/foto   foto completa del cluster (ver internal/foto)
//	GET /api/stream canal de streaming (Server-Sent Events): la foto y sus cambios (ver stream.go)
//	GET /api/contextos   el contexto de kviz y los contextos del kubeconfig (ver contexto.go)
//	POST /api/contexto   cambia de contexto en caliente
//	POST /api/pruebas   request de prueba desde el pod de sondeo, con su respuesta y recorrido (ver pruebas.go)
//	GET /api/logs?namespace=default&pod=nginx&container=web[&anterior=true]
//	                logs de un container por Server-Sent Events: en vivo o del reinicio anterior (ver logs.go)
//	GET /api/comandos?tipo=Pod&namespace=default&nombre=nginx
//	                comandos sugeridos de un objeto de la foto (ver internal/comandos); 404 si no está
//	POST /api/secreto    revela el valor de una clave de un Secret: {"namespace", "nombre", "clave"} → {"valor"} (ver secreto.go)
//	GET /           la interfaz compilada (web/dist)
package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync/atomic"

	"kviz/internal/comandos"
	"kviz/internal/conexion"
	"kviz/internal/foto"
	"kviz/internal/sondeo"
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
	// Sondeo es el pod de sondeo del cluster conectado; error si kviz no está conectado.
	Sondeo() (*sondeo.Sondeo, error)
	// ValorDeSecret revela el valor de una clave de un Secret (conexion.ErrSecretInexistente si
	// no existe). Es el único camino por el que un valor de Secret sale de kviz.
	ValorDeSecret(namespace, nombre, clave string) (string, error)
	// Logs abre los logs de un container: los actuales, que siguen en vivo hasta que se cancela
	// ctx, o los del reinicio anterior.
	Logs(ctx context.Context, p conexion.PedidoDeLogs) (io.ReadCloser, error)
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
	mux.HandleFunc("POST /api/pruebas", mandarPrueba(cluster, new(atomic.Int64)))
	mux.HandleFunc("POST /api/secreto", revelarSecret(cluster))
	mux.HandleFunc("GET /api/logs", transmitirLogs(cluster))
	mux.HandleFunc("GET /api/comandos", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		cs, ok := comandos.Para(cluster.Foto(), comandos.Objeto{Tipo: q.Get("tipo"), Namespace: q.Get("namespace"), Nombre: q.Get("nombre")})
		if !ok {
			http.Error(w, "no hay ese objeto en la foto", http.StatusNotFound)
			return
		}
		responderJSON(w, cs)
	})
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
