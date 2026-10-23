// Package web incluye en el binario la interfaz compilada por Vite (web/dist).
//
// web/dist/.gitkeep está commiteado para que `go build` funcione en un clon recién hecho, antes
// de compilar el frontend; en ese caso se sirve una página que explica cómo compilarlo.
package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var dist embed.FS

// Compilada dice si el binario trae la interfaz (si se corrió `npm run build` antes de go build).
func Compilada() bool {
	_, err := fs.Stat(dist, "dist/index.html")
	return err == nil
}

// Handler sirve la interfaz compilada, o una página de aviso si no está.
func Handler() http.Handler {
	if !Compilada() {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(sinCompilar))
		})
	}
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // dist existe siempre: lo garantiza el go:embed
	}
	return http.FileServerFS(sub)
}

const sinCompilar = `<!doctype html>
<html lang="es"><meta charset="utf-8"><title>kviz</title>
<body style="font-family: system-ui, sans-serif; padding: 2rem; color: #1d2430">
<h1>kviz</h1>
<p>Este binario se compiló sin la interfaz. Desde el directorio <code>kviz/</code> corré
<code>make run</code>, que compila la interfaz y después el binario.</p>
<p>La API sí está disponible, por ejemplo en <a href="/api/foto">/api/foto</a>.</p>
</body></html>
`
