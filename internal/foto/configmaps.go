package foto

// ConfigMaps y Secrets de la foto. De un Secret la foto trae solo sus claves: los valores no
// salen del backend salvo por un pedido explícito (ValorDeSecret).

import (
	"sort"

	corev1 "k8s.io/api/core/v1"
)

type ConfigMap struct {
	// ID es "<namespace>/<nombre>".
	ID        string `json:"id"`
	Nombre    string `json:"nombre"`
	Namespace string `json:"namespace"`
	// Datos es el contenido (data) del ConfigMap.
	Datos map[string]string `json:"datos"`
}

// Secret es un Secret sin valores: solo sus claves.
type Secret struct {
	// ID es "<namespace>/<nombre>".
	ID        string `json:"id"`
	Nombre    string `json:"nombre"`
	Namespace string `json:"namespace"`
	// Tipo es el type del Secret: Opaque, kubernetes.io/tls…
	Tipo string `json:"tipo"`
	// Claves ordenadas.
	Claves []string `json:"claves"`
}

func configMaps(o Objetos) []ConfigMap {
	out := []ConfigMap{}
	for _, cm := range o.ConfigMaps {
		datos := cm.Data
		if datos == nil {
			datos = map[string]string{}
		}
		out = append(out, ConfigMap{ID: cm.Namespace + "/" + cm.Name, Nombre: cm.Name, Namespace: cm.Namespace, Datos: datos})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func secrets(o Objetos) []Secret {
	out := []Secret{}
	for _, s := range o.Secrets {
		out = append(out, Secret{ID: s.Namespace + "/" + s.Name, Nombre: s.Name, Namespace: s.Namespace, Tipo: string(s.Type), Claves: clavesDeSecret(s)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func clavesDeSecret(s *corev1.Secret) []string {
	claves := []string{}
	for k := range s.Data {
		claves = append(claves, k)
	}
	sort.Strings(claves)
	return claves
}

func clavesOrdenadas(m map[string]string) []string {
	claves := make([]string, 0, len(m))
	for k := range m {
		claves = append(claves, k)
	}
	sort.Strings(claves)
	return claves
}
