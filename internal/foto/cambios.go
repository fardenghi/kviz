package foto

import (
	"bytes"
	"encoding/json"
	"sort"
)

// Cambio es una actualización de la foto, tal como viaja por el stream: la foto se manda completa
// al conectarse y después solo lo que cambió. Cada campo de la foto es una sección:
//
//   - en las secciones que son listas de objetos con "id" (nodos, pods, …) el cambio es de un
//     solo objeto: con Valor es un alta o una modificación (el objeto entero), con Borrado una baja;
//   - en las demás (conectado, motivo, …) el cambio reemplaza la sección entera por Valor, o la
//     quita si Borrado.
//
// Por eso cada objeto de una lista de la foto lleva un "id" único en su sección, y las listas
// van ordenadas por id: así la interfaz aplica los cambios sin saber nada de cada sección.
type Cambio struct {
	Seccion string          `json:"seccion"`
	ID      string          `json:"id,omitempty"`
	Valor   json.RawMessage `json:"valor,omitempty"`
	Borrado bool            `json:"borrado,omitempty"`
}

// Comparar devuelve los cambios que llevan de la foto antes a la foto después, en un orden
// determinista (por sección y por id). Si son iguales no devuelve ninguno.
func Comparar(antes, despues Foto) []Cambio {
	a, d := secciones(antes), secciones(despues)
	cambios := []Cambio{}
	for _, s := range claves(a, d) {
		va, ok := a[s]
		vd, sigue := d[s]
		switch {
		case !sigue:
			cambios = append(cambios, Cambio{Seccion: s, Borrado: true})
		case ok && bytes.Equal(va, vd):
		default:
			cambios = append(cambios, compararSeccion(s, va, vd)...)
		}
	}
	return cambios
}

func secciones(f Foto) map[string]json.RawMessage {
	crudo, err := json.Marshal(f)
	if err != nil {
		panic(err) // la foto son datos planos: siempre se puede pasar a JSON
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(crudo, &m); err != nil {
		panic(err)
	}
	return m
}

// compararSeccion compara una sección que cambió: objeto por objeto si las dos versiones son
// listas de objetos con id, o entera si no.
func compararSeccion(s string, antes, despues json.RawMessage) []Cambio {
	a, okA := porID(antes)
	d, okD := porID(despues)
	if !okA || !okD {
		return []Cambio{{Seccion: s, Valor: despues}}
	}
	var cambios []Cambio
	for _, id := range claves(a, d) {
		va, estaba := a[id]
		vd, esta := d[id]
		switch {
		case !esta:
			cambios = append(cambios, Cambio{Seccion: s, ID: id, Borrado: true})
		case !estaba || !bytes.Equal(va, vd):
			cambios = append(cambios, Cambio{Seccion: s, ID: id, Valor: vd})
		}
	}
	return cambios
}

// porID indexa por id una lista de objetos. ok es false si no es una lista o algún objeto no
// tiene id.
func porID(crudo json.RawMessage) (m map[string]json.RawMessage, ok bool) {
	var lista []json.RawMessage
	if err := json.Unmarshal(crudo, &lista); err != nil {
		return nil, false
	}
	m = make(map[string]json.RawMessage, len(lista))
	for _, o := range lista {
		var conID struct {
			ID *string `json:"id"`
		}
		if err := json.Unmarshal(o, &conID); err != nil || conID.ID == nil {
			return nil, false
		}
		m[*conID.ID] = o
	}
	return m, true
}

// claves son las claves que están en a o en b, ordenadas.
func claves(a, b map[string]json.RawMessage) []string {
	todos := make([]string, 0, len(a)+len(b))
	for id := range a {
		todos = append(todos, id)
	}
	for id := range b {
		if _, ok := a[id]; !ok {
			todos = append(todos, id)
		}
	}
	sort.Strings(todos)
	return todos
}
