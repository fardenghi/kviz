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
| `internal/conexion/` | 1 · conexión al cluster: `gestor.go` (contexto elegido, cambio en caliente, vigilancia y reconexión), `kubeconfig.go` (contextos y clientes del kubeconfig), `conexion.go` (informers de un cluster) |
| `internal/foto/` | 2 · modelo del cluster: objetos de Kubernetes → foto (contrato JSON); `workloads.go`: cadenas de dueños y color por workload; `cambios.go`: los cambios de la foto para el stream; `detalle.go` y `rollout.go`: lo que solo usa el panel de detalle |
| `internal/comandos/` | 7 · comandos sugeridos: generador puro de objeto de la foto a comandos de kubectl (con nombre, namespace y contexto) |
| `internal/api/` | 12 · API HTTP (`GET /api/foto`, `GET /api/contextos`, `POST /api/contexto`, `GET /api/comandos`), canal de streaming (`GET /api/stream`, `stream.go`), logs de un container (`GET /api/logs`, `logs.go`) y costura 1 |
| `web/ui.go` | la interfaz compilada (`web/dist`) embebida en el binario |
| `web/src/foto.ts` | espejo TypeScript del contrato de la foto |
| `web/src/tiempoReal.ts` | aplica los mensajes del stream a la foto y recuerda los pods nuevos (brillo) |
| `web/src/escena/` | escena de la vista de topología: función pura (costura 2) |
| `web/src/escena/workloads.ts` | marca de workload por pod, selección de workload y texto de la cadena |
| `web/src/lienzo/` | lienzo SVG: zoom, arrastre, encuadrar |
| `web/src/superficies/` | superficies flotantes (lista de workloads, franja de events, …) |
| `web/src/events.ts` | events del cluster (`internal/foto/events.go`): los de la franja (con los filtros de namespace), el objeto que selecciona un clic y los events de un objeto para el panel |
| `web/src/detalle/` | panel de detalle (cajón a la derecha): `detalle.ts` qué objeto muestra; una sección por archivo en `secciones/` (ver `secciones.ts`); `Comando.tsx` comando sugerido con botón de copiar |
| `web/src/App.tsx` | barra de arriba (contexto, conexión) y armado de la pantalla |

Para agregar un objeto a la foto: su informer en `internal/conexion` (con el handler de avisos, para
que sus cambios lleguen en vivo), el campo en `foto.Objetos`, `foto.Foto` y `foto.Construir`, el espejo
en `web/src/foto.ts`, y lo que dibuja en `web/src/escena`. Cada objeto de una lista de la foto lleva
un `id` único en su lista y la lista va ordenada por id: así el stream manda solo los objetos que
cambiaron sin código nuevo.

## Tiempo real

`GET /api/stream` es un stream de Server-Sent Events. Cada mensaje es `data: <JSON>` con un campo
`tipo`: primero `{"tipo":"foto","foto":{…}}` con la foto completa y después
`{"tipo":"cambios","cambios":[…]}` con lo que cambió (ver `foto.Cambio`). El canal admite más tipos
de mensaje; el escritor de SSE (`sse` en `stream.go`) sirve también para otras rutas.

`GET /api/logs?namespace=…&pod=…&container=…[&anterior=true]` usa el mismo escritor: manda las
últimas 1000 líneas del container (`{"tipo":"lineas","lineas":[…]}`) y sigue en vivo; los del
reinicio anterior (`anterior=true`, `kubectl logs --previous`) terminan con `{"tipo":"fin"}`. Si
Kubernetes no da los logs llega `{"tipo":"error","error":"…"}`. Cerrar la conexión corta el pedido
al cluster. Un pod o container que no está en la foto, o `anterior` sin reinicios, dan 404.

## Contexto y reconexión

kviz arranca en el contexto actual de kubectl y nunca modifica el kubeconfig. `GET /api/contextos`
devuelve `{"actual", "contextos"}` (se relee el kubeconfig en cada pedido) y `POST /api/contexto`
con `{"contexto": "<nombre>"}` cambia en caliente (204; 404 si el contexto no existe). El `Gestor`
revisa cada 2 s que el cluster responda: si deja de responder (o su contexto se va del kubeconfig,
como con `kind delete cluster`) apaga los informers y la foto queda desconectada con el motivo;
reintenta con un cliente nuevo del kubeconfig hasta que el cluster vuelve. Sin contexto, espera a
que kubectl tenga uno. El stream manda la foto completa (`tipo: "foto"`) cada vez que cambia el
contexto o el estado de conexión. En los tests, `kubeconfigDePrueba` (`internal/api/contexto_test.go`)
tiene varios clusters falsos que se pueden crear, borrar o hacer caer.
