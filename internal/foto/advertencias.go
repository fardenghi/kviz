package foto

// Advertencia es una advertencia de configuración: un error típico que impide que el tráfico
// llegue, marcado sobre su objeto (un Pod, un Service o un Ingress), con una explicación y un
// comando sugerido para revisarlo. Las reglas están en internal/advertencias.
type Advertencia struct {
	// ID es "<tipo>/<namespace>/<nombre>/<regla>", con "/<detalle>" cuando un objeto puede tener
	// varias de la misma regla (un puerto, un container, un Service).
	ID string `json:"id"`
	// Tipo del objeto afectado: Pod, Service o Ingress.
	Tipo      string `json:"tipo"`
	Namespace string `json:"namespace"`
	Nombre    string `json:"nombre"`
	// Regla es el nombre corto de la regla que la disparó (ver internal/advertencias).
	Regla string `json:"regla"`
	// Titulo es una frase corta; Explicacion dice qué pasa y qué revisar, con los datos del caso.
	Titulo      string          `json:"titulo"`
	Explicacion string          `json:"explicacion"`
	Comando     ComandoSugerido `json:"comando"`
}

// ComandoSugerido es un comando de kubectl listo para copiar (mismo contrato que internal/comandos).
type ComandoSugerido struct {
	Descripcion string `json:"descripcion"`
	Comando     string `json:"comando"`
}
