package api_test

// Estado de probe (ticket 09): kviz deduce el estado de la liveness, readiness y startup de cada
// container (ok, fallando, pendiente o sin probe) de señales indirectas, porque Kubernetes no
// guarda el resultado de cada ejecución: Ready para la readiness, Started para la startup, y los
// reinicios y los events Unhealthy/Killing para la liveness. La foto trae también la
// configuración de cada probe, las señales usadas y los últimos fallos.

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type probeEnFoto struct {
	// Nombre es liveness, readiness o startup.
	Nombre  string       `json:"nombre"`
	Estado  string       `json:"estado"`
	Config  *configProbe `json:"config"`
	Senales []string     `json:"senales"`
	Fallos  []falloProbe `json:"fallos"`
}

type configProbe struct {
	Tipo                string   `json:"tipo"`
	Path                string   `json:"path,omitempty"`
	Puerto              string   `json:"puerto,omitempty"`
	Esquema             string   `json:"esquema,omitempty"`
	Comando             []string `json:"comando,omitempty"`
	InitialDelaySeconds int32    `json:"initialDelaySeconds"`
	PeriodSeconds       int32    `json:"periodSeconds"`
	TimeoutSeconds      int32    `json:"timeoutSeconds"`
	SuccessThreshold    int32    `json:"successThreshold"`
	FailureThreshold    int32    `json:"failureThreshold"`
}

type falloProbe struct {
	Motivo  string `json:"motivo"`
	Mensaje string `json:"mensaje"`
	Veces   int32  `json:"veces"`
	Cuando  string `json:"cuando"`
}

type containerConSalud struct {
	Nombre string        `json:"nombre"`
	Probes []probeEnFoto `json:"probes"`
}

type podConSalud struct {
	Nombre     string              `json:"nombre"`
	Containers []containerConSalud `json:"containers"`
}

// probes devuelve las probes del container del pod, por nombre (liveness, readiness, startup).
func (k *kvizDePrueba) probes(t *testing.T, pod, container string) map[string]probeEnFoto {
	t.Helper()
	var f struct {
		Pods []podConSalud `json:"pods"`
	}
	k.getJSON(t, "/api/foto", &f)
	for _, p := range f.Pods {
		for _, c := range p.Containers {
			if p.Nombre == pod && c.Nombre == container {
				if len(c.Probes) != 3 {
					t.Fatalf("probes de %s/%s = %s, want liveness, readiness y startup", pod, container, mostrar(c.Probes))
				}
				out := map[string]probeEnFoto{}
				for i, pr := range c.Probes {
					if want := []string{"liveness", "readiness", "startup"}[i]; pr.Nombre != want {
						t.Fatalf("probe %d de %s/%s = %q, want %q", i, pod, container, pr.Nombre, want)
					}
					out[pr.Nombre] = pr
				}
				return out
			}
		}
	}
	t.Fatalf("no está el container %s/%s en la foto", pod, container)
	return nil
}

// ---------- el pod backend de la guía (4-pod.yaml) ----------

var arranque = time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)

// probeStatus es la probe de la guía: GET /status al 8080, 15 s de espera, cada 10 s, 10 s de
// timeout y 3 fallos para darla por caída.
func probeStatus() *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/status", Port: intstr.FromInt32(8080)}},
		InitialDelaySeconds: 15, PeriodSeconds: 10, TimeoutSeconds: 10, FailureThreshold: 3,
	}
}

// podBackend es el pod de 4-pod.yaml corriendo, Ready y Started, desde arranque.
func podBackend() *corev1.Pod {
	p := pod("default", "backend", "lab-worker", "10.244.1.7", corev1.PodRunning)
	p.UID = "uid-backend"
	p.Spec.Containers = []corev1.Container{{
		Name: "application", Image: "lab-backend:latest",
		Ports: []corev1.ContainerPort{{ContainerPort: 8080, Protocol: corev1.ProtocolTCP}},
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("500m"), corev1.ResourceMemory: resource.MustParse("1Gi")},
			Limits:   corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("2Gi")},
		},
		LivenessProbe:  probeStatus(),
		ReadinessProbe: probeStatus(),
	}}
	p.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name: "application", Ready: true, Started: ptrBool(true),
		State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(arranque)}},
	}}
	return p
}

func TestUnaProbeQuePasaEstaOkYTraeSuConfiguracion(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), podBackend())

	probes := kviz.probes(t, "backend", "application")

	wantConfig := &configProbe{Tipo: "httpGet", Path: "/status", Puerto: "8080", Esquema: "HTTP",
		InitialDelaySeconds: 15, PeriodSeconds: 10, TimeoutSeconds: 10, SuccessThreshold: 1, FailureThreshold: 3}
	for _, nombre := range []string{"liveness", "readiness"} {
		p := probes[nombre]
		if p.Estado != "ok" || !reflect.DeepEqual(p.Config, wantConfig) || len(p.Fallos) != 0 {
			t.Errorf("%s = %s, want ok con la config %s y sin fallos", nombre, mostrar(p), mostrar(wantConfig))
		}
	}
	if got := probes["readiness"].Senales; !reflect.DeepEqual(got, []string{"Ready: true"}) {
		t.Errorf("señales de la readiness = %q, want [Ready: true]", got)
	}
}

// eventDeProbe es un event del kubelet sobre un container del pod: Unhealthy cuando una probe
// falla, Killing cuando lo reinicia. veces es el contador del event y cuando, la última vez.
func eventDeProbe(p *corev1.Pod, container, motivo, mensaje string, veces int32, cuando time.Time) *corev1.Event {
	tipo := corev1.EventTypeWarning
	if motivo == "Killing" {
		tipo = corev1.EventTypeNormal
	}
	return &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{Namespace: p.Namespace, Name: p.Name + "." + motivo + "." + cuando.Format("150405")},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Pod", Namespace: p.Namespace, Name: p.Name, UID: p.UID,
			FieldPath: "spec.containers{" + container + "}",
		},
		Reason: motivo, Message: mensaje, Type: tipo, Count: veces,
		FirstTimestamp: metav1.NewTime(cuando.Add(-time.Duration(veces) * 10 * time.Second)),
		LastTimestamp:  metav1.NewTime(cuando),
		Source:         corev1.EventSource{Component: "kubelet", Host: "lab-worker"},
	}
}

func TestUnaReadinessQueFallaDesdeQueArrancoEstaFallandoConSusUltimosFallos(t *testing.T) {
	p := podBackend()
	p.Status.ContainerStatuses[0].Ready = false
	viejo := eventDeProbe(p, "application", "Unhealthy", "Readiness probe failed: HTTP probe failed with statuscode: 500", 4, arranque.Add(time.Minute))
	nuevo := eventDeProbe(p, "application", "Unhealthy", "Readiness probe failed: Get \"http://10.244.1.7:8080/status\": context deadline exceeded", 2, arranque.Add(2*time.Minute))
	// los events de otra probe o de otro pod no cuentan
	deLiveness := eventDeProbe(p, "application", "Unhealthy", "Liveness probe failed: HTTP probe failed with statuscode: 500", 1, arranque.Add(90*time.Second))
	otro := podBackend()
	otro.Name, otro.UID = "backend-2", "uid-backend-2"
	deOtroPod := eventDeProbe(otro, "application", "Unhealthy", "Readiness probe failed: connection refused", 1, arranque.Add(time.Minute))
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), p, viejo, nuevo, deLiveness, deOtroPod)

	readiness := kviz.probes(t, "backend", "application")["readiness"]

	wantFallos := []falloProbe{
		{Motivo: "Unhealthy", Mensaje: "Readiness probe failed: Get \"http://10.244.1.7:8080/status\": context deadline exceeded", Veces: 2, Cuando: "2026-10-23T12:02:00Z"},
		{Motivo: "Unhealthy", Mensaje: "Readiness probe failed: HTTP probe failed with statuscode: 500", Veces: 4, Cuando: "2026-10-23T12:01:00Z"},
	}
	wantSenales := []string{"Ready: false", "6 fallos (events Unhealthy) desde que arrancó el container"}
	if readiness.Estado != "fallando" || !reflect.DeepEqual(readiness.Fallos, wantFallos) || !reflect.DeepEqual(readiness.Senales, wantSenales) {
		t.Errorf("readiness = %s\nwant fallando, con las señales %q y los fallos %s", mostrar(readiness), wantSenales, mostrar(wantFallos))
	}
}

func TestUnaReadinessQueTodaviaNoPasoNiFalloEstaPendiente(t *testing.T) {
	p := podBackend()
	p.Status.ContainerStatuses[0].Ready = false
	// un fallo de antes del último arranque es del container anterior
	deAntes := eventDeProbe(p, "application", "Unhealthy", "Readiness probe failed: connection refused", 3, arranque.Add(-time.Minute))
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), p, deAntes)

	readiness := kviz.probes(t, "backend", "application")["readiness"]

	wantSenales := []string{"Ready: false", "sin fallos desde que arrancó el container: todavía no pasó ninguna vez (initialDelaySeconds 15)"}
	if readiness.Estado != "pendiente" || !reflect.DeepEqual(readiness.Senales, wantSenales) || len(readiness.Fallos) != 1 {
		t.Errorf("readiness = %s\nwant pendiente, con las señales %q y el fallo de antes", mostrar(readiness), wantSenales)
	}
}

func TestUnaLivenessQueFallaYYaReinicioElContainerEstaFallando(t *testing.T) {
	p := podBackend()
	p.Status.ContainerStatuses[0].RestartCount = 4
	unhealthy := eventDeProbe(p, "application", "Unhealthy", "Liveness probe failed: HTTP probe failed with statuscode: 503", 14, arranque.Add(20*time.Second))
	killing := eventDeProbe(p, "application", "Killing", "Container application failed liveness probe, will be restarted", 4, arranque.Add(-time.Second))
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), p, unhealthy, killing)

	liveness := kviz.probes(t, "backend", "application")["liveness"]

	wantSenales := []string{
		"14 fallos (events Unhealthy) desde que arrancó el container",
		"4 reinicios, 4 por la liveness (events Killing)",
	}
	wantFallos := []falloProbe{
		{Motivo: "Unhealthy", Mensaje: "Liveness probe failed: HTTP probe failed with statuscode: 503", Veces: 14, Cuando: "2026-10-23T12:00:20Z"},
		{Motivo: "Killing", Mensaje: "Container application failed liveness probe, will be restarted", Veces: 4, Cuando: "2026-10-23T11:59:59Z"},
	}
	if liveness.Estado != "fallando" || !reflect.DeepEqual(liveness.Senales, wantSenales) || !reflect.DeepEqual(liveness.Fallos, wantFallos) {
		t.Errorf("liveness = %s\nwant fallando, con las señales %q y los fallos %s", mostrar(liveness), wantSenales, mostrar(wantFallos))
	}
}

func TestUnContainerEnCrashLoopPorLaLivenessTieneLaLivenessFallando(t *testing.T) {
	p := podBackend()
	p.Status.ContainerStatuses[0] = corev1.ContainerStatus{
		Name: "application", RestartCount: 7, Started: ptrBool(false),
		State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
		LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
			Reason: "Error", ExitCode: 137, StartedAt: metav1.NewTime(arranque), FinishedAt: metav1.NewTime(arranque.Add(45 * time.Second)),
		}},
	}
	killing := eventDeProbe(p, "application", "Killing", "Container application failed liveness probe, will be restarted", 7, arranque.Add(45*time.Second))
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), p, killing)

	probes := kviz.probes(t, "backend", "application")

	if l := probes["liveness"]; l.Estado != "fallando" || !reflect.DeepEqual(l.Senales, []string{
		"el container no está corriendo (CrashLoopBackOff)", "7 reinicios, 7 por la liveness (events Killing)",
	}) {
		t.Errorf("liveness = %s, want fallando: la liveness lo reinicia una y otra vez", mostrar(l))
	}
	if r := probes["readiness"]; r.Estado != "pendiente" {
		t.Errorf("readiness = %s, want pendiente: no corre mientras el container no corre", mostrar(r))
	}
}

func TestUnaLivenessSinFallosDesdeQueArrancoEstaOkAunqueAntesHayaReiniciado(t *testing.T) {
	p := podBackend()
	p.Status.ContainerStatuses[0].RestartCount = 1
	deAntes := eventDeProbe(p, "application", "Unhealthy", "Liveness probe failed: connection refused", 3, arranque.Add(-30*time.Second))
	killing := eventDeProbe(p, "application", "Killing", "Container application failed liveness probe, will be restarted", 1, arranque.Add(-time.Second))
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), p, deAntes, killing)

	liveness := kviz.probes(t, "backend", "application")["liveness"]

	wantSenales := []string{"sin fallos (events Unhealthy) desde que arrancó el container", "1 reinicio, 1 por la liveness (events Killing)"}
	if liveness.Estado != "ok" || !reflect.DeepEqual(liveness.Senales, wantSenales) || len(liveness.Fallos) != 2 {
		t.Errorf("liveness = %s\nwant ok, con las señales %q y los dos fallos de antes", mostrar(liveness), wantSenales)
	}
}

func TestUnaStartupQueTodaviaNoPasoEstaPendienteYFrenaALasOtras(t *testing.T) {
	p := podBackend()
	c := &p.Spec.Containers[0]
	c.StartupProbe = &corev1.Probe{
		ProbeHandler:     corev1.ProbeHandler{TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromString("http")}},
		PeriodSeconds:    5,
		FailureThreshold: 30,
	}
	p.Status.ContainerStatuses[0].Started = ptrBool(false)
	p.Status.ContainerStatuses[0].Ready = false
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), p)

	probes := kviz.probes(t, "backend", "application")

	startup := probes["startup"]
	wantConfig := &configProbe{Tipo: "tcpSocket", Puerto: "http", PeriodSeconds: 5, TimeoutSeconds: 1, SuccessThreshold: 1, FailureThreshold: 30}
	wantSenales := []string{"Started: false", "sin fallos desde que arrancó el container: todavía no pasó ninguna vez (initialDelaySeconds 0)"}
	if startup.Estado != "pendiente" || !reflect.DeepEqual(startup.Config, wantConfig) || !reflect.DeepEqual(startup.Senales, wantSenales) {
		t.Errorf("startup = %s\nwant pendiente, con la config %s y las señales %q", mostrar(startup), mostrar(wantConfig), wantSenales)
	}
	for _, nombre := range []string{"liveness", "readiness"} {
		if p := probes[nombre]; p.Estado != "pendiente" {
			t.Errorf("%s = %s, want pendiente: no corre hasta que pase la startup", nombre, mostrar(p))
		}
	}
}

func TestUnaStartupQueFallaDesdeQueArrancoEstaFallando(t *testing.T) {
	p := podBackend()
	p.Spec.Containers[0].StartupProbe = &corev1.Probe{ProbeHandler: corev1.ProbeHandler{
		Exec: &corev1.ExecAction{Command: []string{"cat", "/tmp/listo"}},
	}}
	p.Status.ContainerStatuses[0].Started = ptrBool(false)
	unhealthy := eventDeProbe(p, "application", "Unhealthy", "Startup probe failed: cat: can't open '/tmp/listo': No such file or directory", 3, arranque.Add(30*time.Second))
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), p, unhealthy)

	startup := kviz.probes(t, "backend", "application")["startup"]

	if startup.Estado != "fallando" || startup.Config.Tipo != "exec" || !reflect.DeepEqual(startup.Config.Comando, []string{"cat", "/tmp/listo"}) ||
		!reflect.DeepEqual(startup.Senales, []string{"Started: false", "3 fallos (events Unhealthy) desde que arrancó el container"}) {
		t.Errorf("startup = %s, want fallando, exec cat /tmp/listo, con 3 fallos", mostrar(startup))
	}
}

func TestUnFalloDeProbeNuevoLlegaEnVivo(t *testing.T) {
	p := podBackend()
	p.Status.ContainerStatuses[0].Ready = false
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), p)
	stream := kviz.abrirStream(t)
	stream.siguiente(t)

	e := eventDeProbe(p, "application", "Unhealthy", "Readiness probe failed: connection refused", 1, arranque.Add(time.Minute))
	if _, err := kviz.cliente.CoreV1().Events(e.Namespace).Create(context.Background(), e, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	var got podConSalud
	c := stream.esperarCambio(t, "pods", "default/backend")
	if err := json.Unmarshal(c.Valor, &got); err != nil || len(got.Containers) != 1 || len(got.Containers[0].Probes) != 3 {
		t.Fatalf("cambio = %s (%v), want el pod backend con sus probes", mostrar(c), err)
	}
	if r := got.Containers[0].Probes[1]; r.Estado != "fallando" || len(r.Fallos) != 1 {
		t.Errorf("readiness = %s, want fallando con el fallo nuevo", mostrar(r))
	}
}

// El pod shell de la guía (3-pod-shell.yaml): busybox sin probes.
func TestUnContainerSinProbesTieneLasTresSinProbe(t *testing.T) {
	shell := pod("default", "shell", "lab-worker", "10.244.1.9", corev1.PodRunning)
	shell.Spec.Containers = []corev1.Container{{Name: "shell", Image: "busybox:latest", Command: []string{"/bin/sh", "-ec", "sleep 1000"}}}
	shell.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name: "shell", Ready: true, Started: ptrBool(true), RestartCount: 3,
		State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(arranque)}},
	}}
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), shell)

	for nombre, p := range kviz.probes(t, "shell", "shell") {
		want := probeEnFoto{Nombre: nombre, Estado: "sin probe", Senales: []string{}, Fallos: []falloProbe{}}
		if !reflect.DeepEqual(p, want) {
			t.Errorf("%s = %s, want %s", nombre, mostrar(p), mostrar(want))
		}
	}
}
