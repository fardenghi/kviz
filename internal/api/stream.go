package api

// Canal de streaming de kviz: Server-Sent Events sobre HTTP. Cada mensaje es una línea
// `data: <JSON>` con un campo "tipo" que dice qué trae, así el mismo canal sirve para otros tipos
// de mensaje (y otras rutas, como los logs, pueden usar el mismo escritor).

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"kviz/internal/foto"
)

// Tipos de mensaje del stream de la foto (GET /api/stream).
const (
	// MensajeFoto trae la foto completa: es el primer mensaje al conectarse, y llega de nuevo
	// cuando la foto pasa a ser de otro cluster (cambio de contexto) o kviz se desconecta o se
	// reconecta.
	MensajeFoto = "foto"
	// MensajeCambios trae lo que cambió de la foto desde el mensaje anterior (ver foto.Cambio).
	MensajeCambios = "cambios"
)

// juntarAvisos es cuánto se espera después de un aviso para juntar en una sola actualización
// las ráfagas de cambios (un rollout cambia muchos objetos casi a la vez).
const juntarAvisos = 50 * time.Millisecond

// mensaje es lo que viaja por el stream. Según Tipo, viene uno solo de los otros campos.
type mensaje struct {
	Tipo    string        `json:"tipo"`
	Foto    *foto.Foto    `json:"foto,omitempty"`
	Cambios []foto.Cambio `json:"cambios,omitempty"`
}

// sse escribe mensajes de Server-Sent Events y los manda enseguida al navegador.
type sse struct {
	w  http.ResponseWriter
	rc *http.ResponseController
}

// abrirSSE responde los encabezados de un stream de Server-Sent Events.
func abrirSSE(w http.ResponseWriter) (*sse, error) {
	h := w.Header()
	h.Set("Content-Type", "text/event-stream; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	s := &sse{w: w, rc: http.NewResponseController(w)}
	return s, s.rc.Flush()
}

// enviar manda v como un mensaje `data:` en JSON (el JSON no tiene saltos de línea).
func (s *sse) enviar(v any) error {
	datos, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(s.w, "data: %s\n\n", datos); err != nil {
		return err
	}
	return s.rc.Flush()
}

// transmitirFoto manda la foto completa y después sus actualizaciones hasta que se corta el pedido.
func transmitirFoto(cluster Cluster) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		avisos, cancelar := cluster.Suscribir()
		defer cancelar()
		s, err := abrirSSE(w)
		if err != nil {
			return
		}
		ultima := cluster.Foto()
		if s.enviar(mensaje{Tipo: MensajeFoto, Foto: &ultima}) != nil {
			return
		}
		for {
			select {
			case <-r.Context().Done():
				return
			case <-avisos:
			}
			select {
			case <-r.Context().Done():
				return
			case <-time.After(juntarAvisos):
			}
			nueva := cluster.Foto()
			if otroCluster(ultima, nueva) {
				ultima = nueva
				if s.enviar(mensaje{Tipo: MensajeFoto, Foto: &nueva}) != nil {
					return
				}
				continue
			}
			cambios := foto.Comparar(ultima, nueva)
			ultima = nueva
			if len(cambios) == 0 {
				continue
			}
			if s.enviar(mensaje{Tipo: MensajeCambios, Cambios: cambios}) != nil {
				return
			}
		}
	}
}

// otroCluster dice si la foto nueva ya no es la continuación de la anterior: cambió el contexto,
// o kviz se desconectó o se reconectó (un cluster recreado es otro cluster con el mismo nombre).
func otroCluster(antes, despues foto.Foto) bool {
	return antes.Contexto != despues.Contexto || antes.Conectado != despues.Conectado
}
