package foto

// Origen de env (módulo 5 de la spec): de dónde sale cada variable de entorno de un container
// (valor, ConfigMap, Secret o Downward API) y su fuente.
//
// Los valores de los Secrets nunca entran a la foto: una variable que sale de un Secret viaja con
// el Secret y la clave, sin valor. El único camino para leer un valor es revelarlo (ver
// ValorDeSecret).

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// Orígenes de una variable de entorno.
const (
	OrigenValor       = "valor"
	OrigenConfigMap   = "ConfigMap"
	OrigenSecret      = "Secret"
	OrigenDownwardAPI = "DownwardAPI"
)

// Variable es una variable de entorno de un container, tal como la ve el proceso: las de envFrom
// primero y después las de env, que pisan a las de envFrom con el mismo nombre.
type Variable struct {
	// Nombre es "" en un envFrom cuyo ConfigMap o Secret no existe (no se sabe qué claves trae).
	Nombre string `json:"nombre"`
	// Origen es valor, ConfigMap, Secret o DownwardAPI.
	Origen string `json:"origen"`
	// Fuente es el nombre del ConfigMap o del Secret, o el campo del Downward API
	// (metadata.name, status.podIP, limits.cpu…); "" en un valor escrito en el manifiesto.
	Fuente string `json:"fuente"`
	// Clave es la clave del ConfigMap o del Secret; "" en los demás orígenes.
	Clave string `json:"clave"`
	// Valor es el valor resuelto; nil si sale de un Secret (nunca viaja en la foto) o si no se
	// puede resolver (falta la fuente, o es un campo del Downward API que la foto no conoce).
	Valor *string `json:"valor"`
	// EnvFrom dice si la variable viene de un envFrom (todas las claves de un ConfigMap o Secret).
	EnvFrom bool `json:"envFrom"`
	// Falta dice si el ConfigMap, el Secret o la clave no existen.
	Falta bool `json:"falta"`
}

// completarEnv llena los volúmenes de cada pod de la foto, y el env y los montajes de sus containers.
func completarEnv(pods []Pod, o Objetos) {
	fuentes := nuevasFuentes(o)
	porID := map[string]*corev1.Pod{}
	for _, p := range o.Pods {
		porID[p.Namespace+"/"+p.Name] = p
	}
	for i := range pods {
		p := porID[pods[i].ID]
		pods[i].Volumenes = volumenes(p)
		specs := specsDeContainers(p)
		for j := range pods[i].Containers {
			c := &pods[i].Containers[j]
			s := specs[c.Nombre]
			c.Env = fuentes.env(p, s.envFrom, s.env)
			c.Montajes = montajes(s.montajes)
		}
	}
}

// specContainer es lo que tienen en común Container y EphemeralContainer para env y montajes.
type specContainer struct {
	envFrom  []corev1.EnvFromSource
	env      []corev1.EnvVar
	montajes []corev1.VolumeMount
}

func specsDeContainers(p *corev1.Pod) map[string]specContainer {
	m := map[string]specContainer{}
	for _, c := range append(append([]corev1.Container{}, p.Spec.InitContainers...), p.Spec.Containers...) {
		m[c.Name] = specContainer{c.EnvFrom, c.Env, c.VolumeMounts}
	}
	for _, c := range p.Spec.EphemeralContainers {
		m[c.Name] = specContainer{c.EnvFrom, c.Env, c.VolumeMounts}
	}
	return m
}

// fuentes son los ConfigMaps y las claves de los Secrets, por "<namespace>/<nombre>".
type fuentes struct {
	configMaps map[string]map[string]string
	secrets    map[string][]string
}

func nuevasFuentes(o Objetos) fuentes {
	f := fuentes{configMaps: map[string]map[string]string{}, secrets: map[string][]string{}}
	for _, cm := range o.ConfigMaps {
		f.configMaps[cm.Namespace+"/"+cm.Name] = cm.Data
	}
	for _, s := range o.Secrets {
		f.secrets[s.Namespace+"/"+s.Name] = clavesDeSecret(s)
	}
	return f
}

func (f fuentes) env(p *corev1.Pod, envFrom []corev1.EnvFromSource, env []corev1.EnvVar) []Variable {
	vars := []Variable{}
	posicion := map[string]int{}
	agregar := func(v Variable) {
		if i, ok := posicion[v.Nombre]; ok && v.Nombre != "" {
			vars[i] = v
			return
		}
		posicion[v.Nombre] = len(vars)
		vars = append(vars, v)
	}

	for _, ef := range envFrom {
		switch {
		case ef.ConfigMapRef != nil:
			datos, ok := f.configMaps[p.Namespace+"/"+ef.ConfigMapRef.Name]
			if !ok {
				agregar(Variable{Origen: OrigenConfigMap, Fuente: ef.ConfigMapRef.Name, EnvFrom: true, Falta: true})
				continue
			}
			for _, clave := range clavesOrdenadas(datos) {
				agregar(Variable{Nombre: ef.Prefix + clave, Origen: OrigenConfigMap, Fuente: ef.ConfigMapRef.Name, Clave: clave, Valor: ptr(datos[clave]), EnvFrom: true})
			}
		case ef.SecretRef != nil:
			claves, ok := f.secrets[p.Namespace+"/"+ef.SecretRef.Name]
			if !ok {
				agregar(Variable{Origen: OrigenSecret, Fuente: ef.SecretRef.Name, EnvFrom: true, Falta: true})
				continue
			}
			for _, clave := range claves {
				agregar(Variable{Nombre: ef.Prefix + clave, Origen: OrigenSecret, Fuente: ef.SecretRef.Name, Clave: clave, EnvFrom: true})
			}
		}
	}

	for _, e := range env {
		v := Variable{Nombre: e.Name, Origen: OrigenValor}
		switch r := e.ValueFrom; {
		case r == nil:
			v.Valor = ptr(e.Value)
		case r.ConfigMapKeyRef != nil:
			v.Origen, v.Fuente, v.Clave = OrigenConfigMap, r.ConfigMapKeyRef.Name, r.ConfigMapKeyRef.Key
			datos, ok := f.configMaps[p.Namespace+"/"+r.ConfigMapKeyRef.Name]
			if valor, hay := datos[r.ConfigMapKeyRef.Key]; ok && hay {
				v.Valor = ptr(valor)
			} else {
				v.Falta = true
			}
		case r.SecretKeyRef != nil:
			v.Origen, v.Fuente, v.Clave = OrigenSecret, r.SecretKeyRef.Name, r.SecretKeyRef.Key
			v.Falta = !contiene(f.secrets[p.Namespace+"/"+r.SecretKeyRef.Name], r.SecretKeyRef.Key)
		case r.FieldRef != nil:
			v.Origen, v.Fuente = OrigenDownwardAPI, r.FieldRef.FieldPath
			v.Valor = campoDelPod(p, r.FieldRef.FieldPath)
		case r.ResourceFieldRef != nil:
			v.Origen, v.Fuente = OrigenDownwardAPI, r.ResourceFieldRef.Resource
		}
		agregar(v)
	}
	return vars
}

func ptr(s string) *string { return &s }

// campoDelPod resuelve un campo del Downward API (fieldRef); nil si el campo está vacío o no es
// uno de los que la foto conoce.
func campoDelPod(p *corev1.Pod, campo string) *string {
	var v string
	switch campo {
	case "metadata.name":
		v = p.Name
	case "metadata.namespace":
		v = p.Namespace
	case "metadata.uid":
		v = string(p.UID)
	case "spec.nodeName":
		v = p.Spec.NodeName
	case "spec.serviceAccountName":
		v = p.Spec.ServiceAccountName
	case "status.hostIP":
		v = p.Status.HostIP
	case "status.podIP":
		v = p.Status.PodIP
	default:
		if clave, ok := entreComillas(campo, "metadata.labels['"); ok {
			v = p.Labels[clave]
		} else if clave, ok := entreComillas(campo, "metadata.annotations['"); ok {
			v = p.Annotations[clave]
		}
	}
	return opcional(v)
}

// entreComillas saca la clave de "metadata.labels['<clave>']".
func entreComillas(campo, prefijo string) (string, bool) {
	resto, ok := strings.CutPrefix(campo, prefijo)
	if !ok {
		return "", false
	}
	return strings.CutSuffix(resto, "']")
}

func contiene(lista []string, s string) bool {
	for _, x := range lista {
		if x == s {
			return true
		}
	}
	return false
}
