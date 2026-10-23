package foto

import "sort"

// Namespace es un namespace del cluster. El filtro por namespace de la interfaz los ofrece
// todos, también los que todavía no tienen pods.
type Namespace struct {
	// ID es el nombre del namespace.
	ID     string `json:"id"`
	Nombre string `json:"nombre"`
}

func namespaces(o Objetos) []Namespace {
	out := []Namespace{}
	for _, ns := range o.Namespaces {
		out = append(out, Namespace{ID: ns.Name, Nombre: ns.Name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
