package foto

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// Tipos de container: los normales de spec.containers, los init de spec.initContainers y los
// ephemeral que agrega kubectl debug (spec.ephemeralContainers).
const (
	ContainerNormal    = "normal"
	ContainerInit      = "init"
	ContainerEphemeral = "ephemeral"
)

type Container struct {
	Nombre string `json:"nombre"`
	// Tipo es normal, init o ephemeral.
	Tipo    string   `json:"tipo"`
	Imagen  string   `json:"imagen"`
	Puertos []Puerto `json:"puertos"`
	// Estado es Running, el motivo de la espera (ContainerCreating, CrashLoopBackOff,
	// PodInitializing…) o el de la terminación (Completed, Error, OOMKilled…).
	Estado    string `json:"estado"`
	Ready     bool   `json:"ready"`
	Started   bool   `json:"started"`
	Reinicios int32  `json:"reinicios"`
	// Env son sus variables de entorno con su origen (ver env.go).
	Env []Variable `json:"env"`
	// Montajes son los volúmenes del pod montados en el container (ver volumenes.go).
	Montajes []Montaje `json:"montajes"`
}

type Puerto struct {
	Numero int32 `json:"numero"`
	// Nombre es el nombre del puerto ("http"); un targetPort puede apuntarlo por nombre.
	Nombre    string `json:"nombre,omitempty"`
	Protocolo string `json:"protocolo"`
}

// containers devuelve los containers del pod en orden de vida: init, normales y ephemeral.
func containers(p *corev1.Pod) []Container {
	var cs []Container
	for _, c := range p.Spec.InitContainers {
		cs = append(cs, container(ContainerInit, c.Name, c.Image, c.Ports, p.Status.InitContainerStatuses))
	}
	for _, c := range p.Spec.Containers {
		cs = append(cs, container(ContainerNormal, c.Name, c.Image, c.Ports, p.Status.ContainerStatuses))
	}
	for _, c := range p.Spec.EphemeralContainers {
		cs = append(cs, container(ContainerEphemeral, c.Name, c.Image, c.Ports, p.Status.EphemeralContainerStatuses))
	}
	if cs == nil {
		return []Container{}
	}
	return cs
}

func container(tipo, nombre, imagen string, puertos []corev1.ContainerPort, estados []corev1.ContainerStatus) Container {
	c := Container{Nombre: nombre, Tipo: tipo, Imagen: imagen, Puertos: []Puerto{}, Estado: "Waiting"}
	for _, p := range puertos {
		c.Puertos = append(c.Puertos, Puerto{Numero: p.ContainerPort, Nombre: p.Name, Protocolo: protocolo(p.Protocol)})
	}
	for _, s := range estados {
		if s.Name != nombre {
			continue
		}
		c.Estado = estadoContainer(s.State)
		c.Ready = s.Ready
		c.Started = s.Started != nil && *s.Started
		c.Reinicios = s.RestartCount
	}
	return c
}

func protocolo(p corev1.Protocol) string {
	if p == "" {
		return string(corev1.ProtocolTCP)
	}
	return string(p)
}

func estadoContainer(s corev1.ContainerState) string {
	switch {
	case s.Running != nil:
		return "Running"
	case s.Waiting != nil && s.Waiting.Reason != "":
		return s.Waiting.Reason
	case s.Terminated != nil && s.Terminated.Reason != "":
		return s.Terminated.Reason
	case s.Terminated != nil && s.Terminated.Signal != 0:
		return fmt.Sprintf("Signal:%d", s.Terminated.Signal)
	case s.Terminated != nil:
		return fmt.Sprintf("ExitCode:%d", s.Terminated.ExitCode)
	default:
		return "Waiting"
	}
}

// estadoPod es la columna STATUS de kubectl get pods (printPod en kubectl): la fase o el motivo
// del pod, pisado por el init container que falta, por el motivo del container que no corre
// (CrashLoopBackOff, Completed, ErrImagePull…) o por Terminating.
func estadoPod(p *corev1.Pod) string {
	motivo := string(p.Status.Phase)
	if p.Status.Reason != "" {
		motivo = p.Status.Reason
	}
	for _, c := range p.Status.Conditions {
		if c.Type == corev1.PodScheduled && c.Reason == corev1.PodReasonSchedulingGated {
			motivo = corev1.PodReasonSchedulingGated
		}
	}

	sidecar := map[string]bool{}
	for _, c := range p.Spec.InitContainers {
		sidecar[c.Name] = c.RestartPolicy != nil && *c.RestartPolicy == corev1.ContainerRestartPolicyAlways
	}

	inicializando := false
	for i, c := range p.Status.InitContainerStatuses {
		t := c.State.Terminated
		switch {
		case t != nil && t.ExitCode == 0:
			continue
		case sidecar[c.Name] && c.Started != nil && *c.Started:
			continue
		case t != nil && t.Reason != "":
			motivo = "Init:" + t.Reason
		case t != nil && t.Signal != 0:
			motivo = fmt.Sprintf("Init:Signal:%d", t.Signal)
		case t != nil:
			motivo = fmt.Sprintf("Init:ExitCode:%d", t.ExitCode)
		case c.State.Waiting != nil && c.State.Waiting.Reason != "" && c.State.Waiting.Reason != "PodInitializing":
			motivo = "Init:" + c.State.Waiting.Reason
		default:
			motivo = fmt.Sprintf("Init:%d/%d", i, len(p.Spec.InitContainers))
		}
		inicializando = true
		break
	}

	if !inicializando || condicion(p, corev1.PodInitialized) {
		corriendo := false
		for i := len(p.Status.ContainerStatuses) - 1; i >= 0; i-- {
			c := p.Status.ContainerStatuses[i]
			switch {
			case c.State.Waiting != nil && c.State.Waiting.Reason != "":
				motivo = c.State.Waiting.Reason
			case c.State.Terminated != nil:
				motivo = estadoContainer(c.State)
			case c.Ready && c.State.Running != nil:
				corriendo = true
			}
		}
		if motivo == "Completed" && corriendo {
			if condicion(p, corev1.PodReady) {
				motivo = "Running"
			} else {
				motivo = "NotReady"
			}
		}
	}

	terminada := p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed
	if p.DeletionTimestamp != nil && p.Status.Reason == "NodeLost" {
		motivo = "Unknown"
	} else if p.DeletionTimestamp != nil && !terminada {
		motivo = "Terminating"
	}
	return motivo
}

func condicion(p *corev1.Pod, tipo corev1.PodConditionType) bool {
	for _, c := range p.Status.Conditions {
		if c.Type == tipo {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}
