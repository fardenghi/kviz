package api

import (
	"mime"
	"net"
	"net/http"
)

// soloLocalhost protege la API de las otras páginas abiertas en el navegador (historia 3 de la
// spec): escuchar en 127.0.0.1 no alcanza, porque el navegador manda pedidos a localhost en nombre
// de cualquier sitio.
//
//   - Host: solo localhost, 127.0.0.1 o [::1], con cualquier puerto. Frena el DNS rebinding (un
//     dominio ajeno que resuelve a 127.0.0.1 y lee /api/foto o revela un Secret).
//   - Origen: un pedido que cambia algo (POST) desde otro origen se rechaza (CSRF), con
//     http.CrossOriginProtection (Sec-Fetch-Site u Origin contra Host).
//   - Content-Type: un POST de la API tiene que decir application/json, que un formulario o un
//     fetch "simple" de otro sitio no pueden mandar sin preflight de CORS.
//
// El proxy de `npm run dev` (Vite) deja pasar el Host del navegador (localhost:5173), así que el
// mismo chequeo vale para la interfaz en desarrollo.
func soloLocalhost(siguiente http.Handler) http.Handler {
	origen := http.NewCrossOriginProtection()
	origen.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "pedido de otro origen", http.StatusForbidden)
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !hostLocal(r.Host) {
			http.Error(w, "kviz solo atiende pedidos a localhost", http.StatusForbidden)
			return
		}
		if r.Method == http.MethodPost && !esJSON(r.Header.Get("Content-Type")) {
			http.Error(w, "el cuerpo tiene que ser application/json", http.StatusUnsupportedMediaType)
			return
		}
		origen.Handler(siguiente).ServeHTTP(w, r)
	})
}

// hostLocal dice si el header Host nombra la máquina local: localhost, 127.0.0.1 o ::1, con o sin puerto.
func hostLocal(host string) bool {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	} else if len(host) > 1 && host[0] == '[' && host[len(host)-1] == ']' {
		host = host[1 : len(host)-1]
	}
	switch host {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return false
}

func esJSON(contentType string) bool {
	t, _, err := mime.ParseMediaType(contentType)
	return err == nil && t == "application/json"
}
