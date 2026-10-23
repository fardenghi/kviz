package api_test

// Solo localhost (historia 3 de la spec): kviz escucha en 127.0.0.1, pero eso no alcanza para que
// otra página abierta en el navegador no pueda ver los Secrets ni lanzar requests en el cluster.
// La API rechaza los pedidos cuyo Host no es localhost (DNS rebinding), los que cambian algo y
// vienen de otro origen (CSRF), y los POST de JSON que no dicen serlo (formularios cross-site).

import (
	"net/http"
	"strings"
	"testing"
)

// pedir manda un pedido a kviz con el Host y los headers dados, y devuelve el status.
func (k *kvizDePrueba) pedir(t *testing.T, metodo, ruta, host string, headers map[string]string, cuerpo string) int {
	t.Helper()
	req, err := http.NewRequest(metodo, k.srv.URL+ruta, strings.NewReader(cuerpo))
	if err != nil {
		t.Fatal(err)
	}
	if host != "" {
		req.Host = host
	}
	for n, v := range headers {
		req.Header.Set(n, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", metodo, ruta, err)
	}
	res.Body.Close()
	return res.StatusCode
}

const pedidoDeSecret = `{"namespace":"default","nombre":"postgres-secret","clave":"POSTGRES_PASSWORD"}`

func TestUnHostQueNoEsLocalhostSeRechaza(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", postgresDeLaGuia()...)

	for _, host := range []string{"evil.example", "evil.example:7777", "10.0.0.5:7777", "localhost.evil.example"} {
		if s := kviz.pedir(t, http.MethodGet, "/api/foto", host, nil, ""); s != http.StatusForbidden {
			t.Errorf("GET /api/foto con Host %s = %d, want 403", host, s)
		}
		json := map[string]string{"Content-Type": "application/json"}
		if s := kviz.pedir(t, http.MethodPost, "/api/secreto", host, json, pedidoDeSecret); s != http.StatusForbidden {
			t.Errorf("POST /api/secreto con Host %s = %d, want 403", host, s)
		}
	}
}

func TestLocalhostYLaIPDeLoopbackSeAceptanConOSinPuerto(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", postgresDeLaGuia()...)

	for _, host := range []string{"127.0.0.1", "127.0.0.1:7777", "localhost", "localhost:5173", "[::1]:7777"} {
		if s := kviz.pedir(t, http.MethodGet, "/api/foto", host, nil, ""); s != http.StatusOK {
			t.Errorf("GET /api/foto con Host %s = %d, want 200", host, s)
		}
	}
}

func TestUnPedidoQueCambiaAlgoDesdeOtroOrigenSeRechaza(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", postgresDeLaGuia()...)

	casos := []map[string]string{
		{"Content-Type": "application/json", "Origin": "https://evil.example"},
		{"Content-Type": "application/json", "Origin": "https://evil.example", "Sec-Fetch-Site": "cross-site"},
		// otro puerto de localhost también es otro origen
		{"Content-Type": "application/json", "Origin": "http://localhost:3000", "Sec-Fetch-Site": "same-site"},
	}
	for _, h := range casos {
		if s := kviz.pedir(t, http.MethodPost, "/api/secreto", "localhost:7777", h, pedidoDeSecret); s != http.StatusForbidden {
			t.Errorf("POST /api/secreto con %v = %d, want 403", h, s)
		}
	}
}

func TestUnPedidoDelMismoOrigenSeAcepta(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", postgresDeLaGuia()...)

	// la interfaz servida por kviz, y la de `npm run dev`, cuyo proxy deja el Host del navegador
	casos := []struct {
		host    string
		headers map[string]string
	}{
		{"127.0.0.1:7777", map[string]string{"Origin": "http://127.0.0.1:7777", "Sec-Fetch-Site": "same-origin"}},
		{"localhost:5173", map[string]string{"Origin": "http://localhost:5173", "Sec-Fetch-Site": "same-origin"}},
		{"localhost:5173", map[string]string{"Origin": "http://localhost:5173"}},
	}
	for _, c := range casos {
		c.headers["Content-Type"] = "application/json"
		if s := kviz.pedir(t, http.MethodPost, "/api/secreto", c.host, c.headers, pedidoDeSecret); s != http.StatusOK {
			t.Errorf("POST /api/secreto desde %s con %v = %d, want 200", c.host, c.headers, s)
		}
	}
}

func TestUnPostSinContentTypeJSONSeRechaza(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", postgresDeLaGuia()...)

	for _, ct := range []string{"", "text/plain", "application/x-www-form-urlencoded", "multipart/form-data; boundary=x"} {
		h := map[string]string{"Content-Type": ct}
		for _, ruta := range []string{"/api/secreto", "/api/pruebas", "/api/contexto"} {
			if s := kviz.pedir(t, http.MethodPost, ruta, "", h, pedidoDeSecret); s != http.StatusUnsupportedMediaType {
				t.Errorf("POST %s con Content-Type %q = %d, want 415", ruta, ct, s)
			}
		}
	}
	h := map[string]string{"Content-Type": "application/json; charset=utf-8"}
	if s := kviz.pedir(t, http.MethodPost, "/api/secreto", "", h, pedidoDeSecret); s != http.StatusOK {
		t.Errorf("POST /api/secreto con charset = %d, want 200", s)
	}
}
