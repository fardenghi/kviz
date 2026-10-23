package api

// Historial de las requests de prueba de la sesión: GET /api/pruebas (todas, en orden) y
// GET /api/pruebas/{id} (una, para volver a ver su respuesta y su recorrido). Guarda cada prueba de
// la sesión, sin tope. Vive solo en memoria: se pierde al cerrar kviz y nunca se escribe en disco.

import (
	"net/http"
	"strconv"
	"sync"
)

type historial struct {
	mu      sync.Mutex
	ultimo  int
	pruebas []prueba
}

// nuevoID numera las pruebas de la sesión: 1, 2, …
func (h *historial) nuevoID() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ultimo++
	return h.ultimo
}

func (h *historial) guardar(p prueba) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.pruebas = append(h.pruebas, p)
}

func (h *historial) listar(w http.ResponseWriter, _ *http.Request) {
	h.mu.Lock()
	defer h.mu.Unlock()
	responderJSON(w, append([]prueba{}, h.pruebas...))
}

func (h *historial) buscar(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, p := range h.pruebas {
		if err == nil && p.ID == id {
			responderJSON(w, p)
			return
		}
	}
	http.Error(w, "esa prueba no está en el historial de la sesión", http.StatusNotFound)
}
