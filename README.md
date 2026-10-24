# kviz

[![CI](https://github.com/fardenghi/kviz/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/fardenghi/kviz/actions/workflows/ci.yml)

kviz is a local tool that draws the Kubernetes cluster of your current kubectl context as a live
topology view, and lets you debug how traffic travels inside it.

- **Topology view.** Nodes as regions with their pods and containers inside; Services and Ingresses
  above them, connected to the pods they route to; NodePorts and the Ingress Controller's hostPort as
  doors on the edge of each node. Each pod is marked with the workload it belongs to.
- **Live.** The view follows the cluster as it changes: rollouts, restarts, endpoints that come and
  go, and a strip with the latest cluster events.
- **Detail panel.** Click any object to see what `kubectl describe` would tell you: probe status,
  requests and limits with real usage (if metrics-server is installed), environment variables and
  where they come from, volumes, ConfigMaps, Secrets (hidden until you reveal them), events and live
  container logs, plus ready-to-copy `kubectl` commands for that object.
- **Configuration warnings.** Typical mistakes that keep traffic from arriving (a selector that
  matches no pods, a `targetPort` no container exposes, a probe on the wrong port, an Ingress pointing
  to a missing Service, …) are flagged on the object with an explanation.
- **Test requests.** Send an HTTP request (or a plain TCP connection test) from inside the cluster to
  a Service, a pod IP, a NodePort or an Ingress route, and see the response and the path it took
  drawn on the diagram: DNS → door → Ingress rule → Service → endpoints → pod. Repeat it N times to
  see how the load is spread across pods.

kviz runs on your machine and only talks to the Kubernetes API; nothing is installed in the cluster
except a short-lived probe pod (see [Cluster permissions](#cluster-permissions)).

> **The user interface is in Spanish**, with Kubernetes names (Pod, Service, NodePort, readiness)
> kept in English as `kubectl` shows them. The command-line flags are in Spanish too.

## Install

**Homebrew** (macOS and Linux):

```sh
brew install fardenghi/tap/kviz
```

**krew** (as a kubectl plugin):

```sh
kubectl krew install kviz
kubectl kviz
```

**Prebuilt binaries:** download the archive for your OS and architecture (macOS, Linux and Windows;
amd64 and arm64) from the [Releases page](https://github.com/fardenghi/kviz/releases), check it
against `checksums.txt`, and put `kviz` somewhere in your `PATH`. On macOS, if Gatekeeper refuses to
open a downloaded binary, remove the quarantine attribute:

```sh
xattr -d com.apple.quarantine ./kviz
```

**From source** (requires Go 1.26+ and Node 22+):

```sh
git clone https://github.com/fardenghi/kviz.git
cd kviz
make build      # builds the UI and the Go binary that embeds it: bin/kviz
```

## Usage

```sh
kviz
```

kviz connects to the current kubectl context, starts a web server on `127.0.0.1:7777` and opens your
browser on it. If port 7777 is taken it picks a free port and prints it. Press `Ctrl+C` to stop it.

| Flag | Meaning |
|---|---|
| `-port N` | port to listen on, on `127.0.0.1` (default `7777`); if it is taken, a free one is used |
| `-no-browser` | don't open the browser on start |
| `-version` | print the version and exit |

The Spanish names `-puerto` and `-no-abrir` still work as aliases.

As a kubectl plugin the flags are passed through unchanged: `kubectl kviz -port 8000`.

You can switch to any other context of your kubeconfig from the top bar without restarting. kviz
never modifies your kubeconfig. If the cluster goes away (for example after `kind delete cluster`)
kviz shows "desconectado" and reconnects by itself when it comes back.

## Requirements

- A kubeconfig with a context, as `kubectl` uses it (`KUBECONFIG` or `~/.kube/config`). Without a
  context, or with the cluster down, kviz still starts and shows that it is disconnected.
- Optional: [metrics-server](https://github.com/kubernetes-sigs/metrics-server) in the cluster to see
  real CPU and memory usage. Without it kviz shows "sin métricas" and suggests how to install it.

## Cluster permissions

kviz uses the credentials of your kubectl context. It is **read-only**, with one exception: the probe
pod used to send test requests.

- **Reads** (get, list, watch), cluster-wide: namespaces, nodes, pods, pod logs, Services,
  EndpointSlices, Ingresses, IngressClasses, Deployments, ReplicaSets, StatefulSets, DaemonSets,
  Jobs, CronJobs, ConfigMaps, Secrets and events; and pod metrics from `metrics.k8s.io` if available.
  Secrets travel to the UI with their keys only; a value leaves kviz only when you click "Revelar"
  on it.
- **Writes**, only the first time you send a test request: kviz creates the `kviz-system` namespace
  if it doesn't exist, creates one pod there (`curlimages/curl`), and runs `curl` in it with
  `pods/exec`. The pod is deleted when kviz exits or switches context. If kviz was killed abruptly,
  it reuses or deletes the leftover pod the next time it starts.

Think about this before pointing kviz at a cluster that is not a test cluster.

## Security

kviz listens **only on `127.0.0.1`**, never on other interfaces, so nobody else on your network can
see your Secrets or send requests into your cluster through it. It also answers only requests
addressed to `localhost`/`127.0.0.1` (so a site that points its domain at your machine, a DNS
rebinding attack, can't read it), rejects state-changing requests coming from another origin, and
requires JSON bodies on its POSTs, so other pages open in your browser can't use it either. The history of test requests lives
only in memory and is lost when kviz exits.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for building, testing and finding your way around the code.

## License

[MIT](LICENSE) © 2026 Filipo Ardenghi
