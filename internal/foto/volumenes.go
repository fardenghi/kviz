package foto

// Volúmenes del pod y dónde se monta cada uno en cada container (panel de detalle, ticket 10).

import (
	corev1 "k8s.io/api/core/v1"
)

// Volumen es un volumen del pod (spec.volumes).
type Volumen struct {
	Nombre string `json:"nombre"`
	// Tipo es ConfigMap, Secret, emptyDir, hostPath, PersistentVolumeClaim, projected,
	// downwardAPI… o "otro".
	Tipo string `json:"tipo"`
	// Fuente es el ConfigMap, el Secret o el PVC, o la ruta del nodo de un hostPath; "" si no tiene.
	Fuente string `json:"fuente"`
}

// Montaje es un volumen montado en un container (volumeMounts).
type Montaje struct {
	Volumen     string `json:"volumen"`
	Ruta        string `json:"ruta"`
	SoloLectura bool   `json:"soloLectura"`
	// SubPath es el archivo o carpeta del volumen que se monta; "" si se monta entero.
	SubPath string `json:"subPath"`
}

func volumenes(p *corev1.Pod) []Volumen {
	out := []Volumen{}
	for _, v := range p.Spec.Volumes {
		out = append(out, volumen(v))
	}
	return out
}

func volumen(v corev1.Volume) Volumen {
	s := v.VolumeSource
	vol := Volumen{Nombre: v.Name, Tipo: "otro"}
	switch {
	case s.ConfigMap != nil:
		vol.Tipo, vol.Fuente = "ConfigMap", s.ConfigMap.Name
	case s.Secret != nil:
		vol.Tipo, vol.Fuente = "Secret", s.Secret.SecretName
	case s.PersistentVolumeClaim != nil:
		vol.Tipo, vol.Fuente = "PersistentVolumeClaim", s.PersistentVolumeClaim.ClaimName
	case s.HostPath != nil:
		vol.Tipo, vol.Fuente = "hostPath", s.HostPath.Path
	case s.EmptyDir != nil:
		vol.Tipo = "emptyDir"
	case s.Projected != nil:
		vol.Tipo = "projected"
	case s.DownwardAPI != nil:
		vol.Tipo = "downwardAPI"
	}
	return vol
}

func montajes(ms []corev1.VolumeMount) []Montaje {
	out := []Montaje{}
	for _, m := range ms {
		out = append(out, Montaje{Volumen: m.Name, Ruta: m.MountPath, SoloLectura: m.ReadOnly, SubPath: m.SubPath})
	}
	return out
}
