# Contributing to kviz

kviz is a single Go binary (the backend, built on client-go) that embeds a React + TypeScript UI
built with Vite. The code, comments and UI are in Spanish; the domain vocabulary is defined in
[docs/GLOSSARY.md](docs/GLOSSARY.md) and the main architecture decision in
[docs/adr/](docs/adr/). A few words you will meet everywhere:

- **foto** (snapshot): the model of the cluster that the backend builds from Kubernetes objects and
  sends to the UI as JSON.
- **escena** (scene): what the topology view draws, computed from the snapshot by a pure function.
- **pod de sondeo** (probe pod): the temporary pod in `kviz-system` that sends test requests.
- **recorrido** (path): the steps a test request went through (DNS, door, Ingress rule, Service,
  endpoints, pod).

Requirements: Go 1.26+ and Node 22+. All commands run from the repository root.

## Build and run

```sh
make run        # builds the UI and the binary (bin/kviz) and starts it
make build      # only builds bin/kviz
```

`go build ./...` works without building the UI (`web/dist/.gitkeep` is committed); that binary
serves a page explaining how to build it. Release builds inject the version with
`-ldflags "-X main.version=..."`; otherwise `kviz -version` prints `kviz dev`.

## Tests

```sh
make test       # go test ./... + vitest
```

Tests go through two seams:

- **Seam 1** (`internal/api/*_test.go`): tests of the HTTP API against client-go's fake client
  (`fake.NewClientset`) and `httptest`. The harness is in `internal/api/kviz_test.go`. In these
  tests `kubeconfigDePrueba` (`internal/api/contexto_test.go`) provides several fake clusters that
  can be created, deleted or brought down, and `sondeoDePrueba` stands in for the probe pod.
- **Seam 2** (`web/src/escena/*.test.ts`): Vitest tests of the pure scene function.

`cmd/kviz/main_test.go` also builds the binary to check the `-version` flag.

## Developing the UI

```sh
go run ./cmd/kviz -no-abrir    # backend on :7777
npm --prefix web run dev       # UI with hot reload; /api is proxied to the backend
```

## Code map

| Where | What |
|---|---|
| `cmd/kviz/` | startup: port on 127.0.0.1, browser, signals, `-version` |
| `internal/conexion/` | connection to the cluster: `gestor.go` (selected context, hot switching, health checks and reconnection), `kubeconfig.go` (contexts and clients from the kubeconfig), `conexion.go` (informers for one cluster), `metricas.go` (metrics-server behind the `Metricas` interface, polled every interval) |
| `internal/foto/` | cluster model: Kubernetes objects → snapshot (the JSON contract); `workloads.go`: owner chains and color per workload; `cambios.go`: snapshot changes for the stream; `detalle.go` and `rollout.go`: what only the detail panel uses; `env.go`: env var sources; `configmaps.go`: ConfigMaps and Secrets (keys only); `volumenes.go`: volumes and mounts; `probes.go`: probe status; `recursos.go`: requests, limits and usage; `events.go`: cluster events |
| `internal/advertencias/` | configuration warnings: six rules, pure functions over the snapshot (and the objects it comes from), each with an explanation and a suggested command; they travel in `foto.advertencias` |
| `internal/sondeo/` | probe pod: its lifecycle in `kviz-system` (`sondeo.go`: created on the first request, leftovers cleaned on connect, deleted on exit or context switch) and `Ejecutor`, the "run and return the raw result" interface (`exec.go`: curl via exec; `conexion.go`: TCP connection test with `curl telnet://`) |
| `internal/trafico/` | traffic debugging, pure: target URL, path from the snapshot (`Recorrer` in `recorrer.go`: DNS, door, Service, endpoints; `ingress.go`: Ingress rule and rewrite as ingress-nginx does it), the pod that answered and the hop between nodes (`respuesta.go`: `PodQueRespondio` from the body or the Ingress Controller log, `ConRespuesta`), and load spread |
| `internal/comandos/` | suggested commands: pure generator from a snapshot object to `kubectl` commands (with name, namespace and context); `tipo=metrics-server` gives how to install it on kind |
| `internal/api/` | HTTP API (`GET /api/foto`, `GET /api/contextos`, `POST /api/contexto`, `GET /api/comandos`, `POST /api/secreto` in `secreto.go`, `POST /api/pruebas` in `pruebas.go`, history `GET /api/pruebas[/{id}]` in `historial.go`), streaming (`GET /api/stream`, `stream.go`), container logs (`GET /api/logs`, `logs.go`) and seam 1 |
| `web/ui.go` | the built UI (`web/dist`) embedded in the binary |
| `web/src/foto.ts` | TypeScript mirror of the snapshot contract |
| `web/src/tiempoReal.ts` | applies stream messages to the snapshot and remembers new pods (glow) |
| `web/src/escena/` | scene of the topology view: pure function (seam 2) |
| `web/src/escena/recorrido.ts` | path of a test request: numbered steps per object (and region), dimming, `recorrido` tone, arc for the hop between nodes, selected step, Ingress Controller pods made visible; turns hover focus off |
| `web/src/escena/workloads.ts` | workload mark per pod, workload selection and the chain text |
| `web/src/escena/reparto.ts` | load spread badges (`n/N`) on pods |
| `web/src/prueba.ts` | TypeScript mirror of the test request and its path (`POST /api/pruebas`) |
| `web/src/lienzo/` | SVG canvas: zoom, drag, fit to screen |
| `web/src/superficies/` | floating surfaces (workload list, events strip, test request form, path card, …) |
| `web/src/events.ts` | cluster events: those in the strip (with the namespace filters), the object a click selects, and an object's events for the panel |
| `web/src/detalle/` | detail panel (drawer on the right): `detalle.ts` which object it shows; one section per file in `secciones/` (see `secciones.ts`); `Comando.tsx` suggested command with a copy button |
| `web/src/App.tsx` | top bar (context, connection) and screen layout |

### Adding an object to the snapshot

1. Its informer in `internal/conexion` (with the change handler, so its changes arrive live).
2. The field in `foto.Objetos`, `foto.Foto` and `foto.Construir`.
3. The mirror in `web/src/foto.ts`.
4. What it draws in `web/src/escena`.

Each object in a snapshot list carries an `id` that is unique in its list, and the list is sorted by
id: that way the stream sends only the objects that changed without new code.

## API notes

### Localhost only

`api.Nuevo` wraps every route in `soloLocalhost` (`internal/api/localhost.go`): a request whose
`Host` isn't `localhost`, `127.0.0.1` or `[::1]` (any port) gets 403, which stops DNS rebinding; a
`POST` from another origin gets 403 (`http.CrossOriginProtection`: `Sec-Fetch-Site`, or `Origin`
against `Host`); and a `POST` without `Content-Type: application/json` gets 415. The Vite dev
proxy keeps the browser's `Host` (`localhost:5173`) and the page's `Origin`, so `npm run dev`
passes these checks; don't set `changeOrigin` on it.

### Real time

`GET /api/stream` is a Server-Sent Events stream. Each message is `data: <JSON>` with a `tipo`
field: first `{"tipo":"foto","foto":{…}}` with the full snapshot, then
`{"tipo":"cambios","cambios":[…]}` with what changed (see `foto.Cambio`). The channel accepts more
message types; the SSE writer (`sse` in `stream.go`) also serves other routes.

`GET /api/logs?namespace=…&pod=…&container=…[&anterior=true]` uses the same writer: it sends the
container's last 1000 lines (`{"tipo":"lineas","lineas":[…]}`) and keeps following; the logs of the
previous restart (`anterior=true`, like `kubectl logs --previous`) end with `{"tipo":"fin"}`. If
Kubernetes doesn't return the logs, `{"tipo":"error","error":"…"}` arrives. Closing the connection
cancels the request to the cluster. A pod or container that is not in the snapshot, or `anterior`
without restarts, give 404.

### Secrets

The snapshot and the stream carry Secrets with their keys only, and an environment variable that
comes from a Secret travels without its value. The only way a value leaves kviz is
`POST /api/secreto` with `{"namespace", "nombre", "clave"}` → `{"valor"}` (404 if it doesn't
exist), which the detail panel uses when you click "Revelar".

### Context and reconnection

kviz starts on kubectl's current context and never modifies the kubeconfig. `GET /api/contextos`
returns `{"actual", "contextos"}` (the kubeconfig is reread on every request) and
`POST /api/contexto` with `{"contexto": "<name>"}` switches live (204; 404 if the context doesn't
exist). The `Gestor` checks every 2 s that the cluster answers: if it stops answering (or its context
disappears from the kubeconfig, as with `kind delete cluster`) it stops the informers and the
snapshot becomes disconnected with the reason; it retries with a new client from the kubeconfig until
the cluster is back. Without a context it waits for kubectl to have one. The stream sends the full
snapshot (`tipo: "foto"`) every time the context or the connection state changes.

### Test requests and the probe pod

`POST /api/pruebas` with `{"destino", "metodo", "path", "headers": [{"nombre","valor"}], "body"}`
sends the request from the probe pod and answers `{"id", "url", "origen", "respuesta": {"status",
"texto", "headers", "body", "ms"} | null, "error", "recorrido": {"pasos": [{"n", "tipo", "titulo",
"detalle", "objetos": [{"tipo", "id"}]}]}}`. Each step's `objetos` are references into the snapshot
(type and `id` in its list); the scene turns them into scene ids (`escena/recorrido.ts`,
`idEnEscena`), so a new kind of step (DNS, door, Ingress, hop) is drawn without touching the scene.
The probe pod (`curlimages/curl`, in `kviz-system`, with the labels `sondeo.Etiquetas`) is kviz's
only write to the cluster, besides creating `kviz-system` if it doesn't exist.

The target can be a Service (DNS name or ClusterIP), a pod IP, or a node (name or InternalIP) with a
door's port: a NodePort, or the Ingress Controller's hostPort (an Ingress route: `lab-worker2:80`
with path `/backend/hello`). The path is DNS (if the target is a name) → door → Ingress rule with the
rewritten path → Service → endpoints, and it adds the hop between nodes and the pod only when kviz
knows which pod answered: because the body carries its name, or because the Ingress Controller log
says so. The hop starts at the entry node: the door's node, the Ingress Controller pod's node, or,
for a request that goes through neither (a Service by name or ClusterIP, a pod IP), the probe pod's
node (`trafico.DesdeNodo`); a pod IP target gets the hop straight away. For that, kviz adds `X-Request-ID: kviz-…` to a request that goes through the Ingress
Controller (if the request doesn't carry one); ingress-nginx writes it at the end of each log line
(`$req_id`), and kviz then reads the last lines of the controller's log (`GET /api/logs` uses the
same `Cluster.Logs`, here without following) to get `$upstream_addr`. A TCP connection test doesn't
go through the Ingress rules: it ends at the Ingress Controller pod.

The TCP connection test is the same `POST /api/pruebas` with `{"tipo": "tcp", "destino":
"host:port"}` (`internal/api/conexion.go`): the probe pod opens a connection with `curl telnet://`
without speaking any protocol (`internal/sondeo/conexion.go`, which cuts the exec as soon as curl
reports it connected) and the answer carries `"conexion": {"resultado": "abre" | "rechaza" |
"timeout", "ms", "motivo"}` instead of `respuesta` (null, with `error`, if it couldn't even be
attempted), the same path (`trafico.URLTCP` → `Recorrer`), and it goes into the history. It can't be
repeated (400 with `repeticiones` > 1).

With `"repeticiones": N` (1–20) the same request is sent N times, one after the other; `respuesta`,
`error` and the pod in the path are those of the first one, and `reparto` (null with a single one)
counts the answers per pod with the same identification as the path (`trafico.Repartir` over
`PodQueRespondio`): `{"total", "pods": [{"pod", "n"}], "sinIdentificar", "sinRespuesta"}`. The
scene draws it as an `n/N` badge on each pod (`escena/reparto.ts`) and the path card as bars. Every
test (with its `contexto`) goes into the session history: `GET /api/pruebas` returns them all, oldest
first, and `GET /api/pruebas/{id}` one (404 if it isn't there). It lives only in memory (the last
100) and is lost when kviz exits.
