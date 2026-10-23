# kviz

Herramienta local para ver el cluster de Kubernetes del contexto actual de kubectl en una vista de
topología. Spec: `.scratch/kviz/spec.md`; vocabulario: `GLOSSARY.md`; decisión de arquitectura:
`docs/adr/0001-app-local-con-pod-de-sondeo.md`.

## Compilar y correr

```sh
make -C kviz run
```

Compila la interfaz (React + TypeScript, con Vite) y el binario de Go que la incluye (`kviz/bin/kviz`),
y lo arranca: escucha solo en `127.0.0.1:7777` (si está ocupado usa otro puerto libre y lo imprime)
y abre el navegador. Opciones: `-puerto N`, `-no-abrir`. Sin contexto de kubectl, o con el cluster
caído, kviz arranca igual y muestra "desconectado".

Requisitos: Go 1.26+, Node 22+.

## Tests

```sh
make -C kviz test     # go test ./... + vitest
```

- **Costura 1** (`internal/api/*_test.go`): tests de la API HTTP contra el cliente falso de client-go
  (`fake.NewClientset`) y `httptest`. El arnés está en `internal/api/kviz_test.go`.
- **Costura 2** (`web/src/escena/*.test.ts`): tests con Vitest de la función pura de escena.

## Desarrollo de la interfaz

```sh
cd kviz && go run ./cmd/kviz -no-abrir   # backend en :7777
npm --prefix kviz/web run dev            # interfaz con recarga en caliente; /api va al backend
```

`go build ./...` funciona sin compilar la interfaz (`web/dist/.gitkeep` está commiteado); ese binario
sirve una página que explica cómo compilarla.

## Mapa

| Dónde | Qué (módulo de la spec) |
|---|---|
| `cmd/kviz/` | arranque: puerto en 127.0.0.1, navegador, señales |
| `internal/conexion/` | 1 · conexión al cluster: contexto de kubectl, informers, conectado/desconectado |
| `internal/foto/` | 2 · modelo del cluster: objetos de Kubernetes → foto (contrato JSON) |
| `internal/api/` | 12 · API HTTP (`GET /api/foto`) y costura 1 |
| `web/ui.go` | la interfaz compilada (`web/dist`) embebida en el binario |
| `web/src/foto.ts` | espejo TypeScript del contrato de la foto |
| `web/src/escena/` | escena de la vista de topología: función pura (costura 2) |
| `web/src/lienzo/` | lienzo SVG: zoom, arrastre, encuadrar |
| `web/src/App.tsx` | barra de arriba (contexto, conexión) y armado de la pantalla |

Para agregar un objeto a la foto: su informer en `internal/conexion`, el campo en `foto.Objetos`,
`foto.Foto` y `foto.Construir`, el espejo en `web/src/foto.ts`, y lo que dibuja en `web/src/escena`.
