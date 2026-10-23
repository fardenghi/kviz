package api_test

// Revelar Secret (módulo 9 de la spec): un pedido explícito, por Secret y clave, es el único
// camino por el que un valor de Secret sale de kviz.

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
)

type valorDeSecret struct {
	Valor string `json:"valor"`
}

// revelar pide el valor de una clave de un Secret (POST /api/secreto) y devuelve el status y el valor.
func (k *kvizDePrueba) revelar(t *testing.T, namespace, nombre, clave string) (int, string) {
	t.Helper()
	cuerpo, _ := json.Marshal(map[string]string{"namespace": namespace, "nombre": nombre, "clave": clave})
	res, err := http.Post(k.srv.URL+"/api/secreto", "application/json", bytes.NewReader(cuerpo))
	if err != nil {
		t.Fatalf("POST /api/secreto: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return res.StatusCode, ""
	}
	if cc := res.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store: el valor no se guarda en ninguna caché", cc)
	}
	var v valorDeSecret
	if err := json.NewDecoder(res.Body).Decode(&v); err != nil {
		t.Fatalf("POST /api/secreto: decodificar: %v", err)
	}
	return res.StatusCode, v.Valor
}

func TestRevelarDevuelveElValorDeUnaClaveDelSecret(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", postgresDeLaGuia()...)

	status, valor := kviz.revelar(t, "default", "postgres-secret", "POSTGRES_PASSWORD")

	if status != http.StatusOK || valor != contrasenaFalsa {
		t.Errorf("revelar = %d %q, want 200 %q", status, valor, contrasenaFalsa)
	}
}

func TestRevelarUnSecretOUnaClaveQueNoExistenDa404(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", postgresDeLaGuia()...)

	for _, c := range []struct{ namespace, nombre, clave string }{
		{"default", "postgres-secret", "NO_HAY"},
		{"default", "no-existe", "POSTGRES_PASSWORD"},
		{"otro", "postgres-secret", "POSTGRES_PASSWORD"},
	} {
		if status, _ := kviz.revelar(t, c.namespace, c.nombre, c.clave); status != http.StatusNotFound {
			t.Errorf("revelar %s/%s %s = %d, want 404", c.namespace, c.nombre, c.clave, status)
		}
	}
}

func TestRevelarSinSecretNiClaveEsUnPedidoInvalido(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", postgresDeLaGuia()...)

	if status, _ := kviz.revelar(t, "default", "postgres-secret", ""); status != http.StatusBadRequest {
		t.Errorf("revelar sin clave = %d, want 400", status)
	}
}

func TestRevelarSoloAceptaPOST(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", postgresDeLaGuia()...)

	res, err := http.Get(kviz.srv.URL + "/api/secreto?namespace=default&nombre=postgres-secret&clave=POSTGRES_PASSWORD")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("GET /api/secreto = %d, want 405", res.StatusCode)
	}
}
