package foto

// Búsquedas de un objeto en la foto. Devuelven un puntero al objeto de la lista, o nil si no está.

// IDDe arma el id de un objeto con namespace: "<namespace>/<nombre>".
func IDDe(namespace, nombre string) string { return namespace + "/" + nombre }

// IDDeWorkload arma el id de un workload: "<tipo>/<namespace>/<nombre>".
func IDDeWorkload(tipo, namespace, nombre string) string { return ref{tipo, namespace, nombre}.id() }

// NodoPorNombre es el nodo con ese nombre (su id).
func (f Foto) NodoPorNombre(nombre string) *Nodo {
	return buscar(f.Nodos, func(n Nodo) bool { return n.ID == nombre })
}

// PodPorID es el pod con ese id ("<namespace>/<nombre>", ver IDDe).
func (f Foto) PodPorID(id string) *Pod {
	return buscar(f.Pods, func(p Pod) bool { return p.ID == id })
}

// PodPorIP es el pod con esa IP.
func (f Foto) PodPorIP(ip string) *Pod {
	return buscar(f.Pods, func(p Pod) bool { return p.IP != nil && *p.IP == ip })
}

// ServicePorID es el Service con ese id ("<namespace>/<nombre>", ver IDDe).
func (f Foto) ServicePorID(id string) *Service {
	return buscar(f.Services, func(s Service) bool { return s.ID == id })
}

// IngressPorID es el Ingress con ese id ("<namespace>/<nombre>", ver IDDe).
func (f Foto) IngressPorID(id string) *Ingress {
	return buscar(f.Ingress, func(i Ingress) bool { return i.ID == id })
}

// PuertaPorID es la puerta con ese id ("<nodo>/<puerto>/<protocolo>").
func (f Foto) PuertaPorID(id string) *Puerta {
	return buscar(f.Puertas, func(p Puerta) bool { return p.ID == id })
}

// WorkloadPorID es el workload con ese id ("<tipo>/<namespace>/<nombre>", ver IDDeWorkload).
func (f Foto) WorkloadPorID(id string) *Workload {
	return buscar(f.Workloads, func(w Workload) bool { return w.ID == id })
}

func buscar[T any](lista []T, es func(T) bool) *T {
	for i := range lista {
		if es(lista[i]) {
			return &lista[i]
		}
	}
	return nil
}
