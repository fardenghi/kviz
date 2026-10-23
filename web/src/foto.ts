// Contrato de la foto del cluster, tal como la entrega GET /api/foto.
// Es el espejo de internal/foto/foto.go: si se agrega un campo allá, se agrega acá.

export interface Foto {
  /** Contexto de kubectl de kviz, por ejemplo "kind-lab"; "" si kubectl no tiene contexto. */
  contexto: string;
  /** false cuando kviz no ve el cluster; entonces nodos y pods vienen vacíos. */
  conectado: boolean;
  /** Por qué kviz está desconectado, en español. */
  motivo?: string;
  /** Ordenados por nombre. */
  nodos: Nodo[];
  /** Ordenados por namespace y nombre. */
  pods: Pod[];
}

export interface Nodo {
  nombre: string;
  internalIP: string;
}

export interface Pod {
  nombre: string;
  namespace: string;
  /** null cuando el scheduler todavía no le asignó un nodo. */
  nodo: string | null;
  /** null mientras el pod no tiene IP. */
  ip: string | null;
  /** status.phase: Pending, Running, Succeeded, Failed o Unknown. */
  fase: string;
}
