// Contrato de la foto del cluster, tal como la entrega GET /api/foto y el stream (ver tiempoReal.ts).
// Es el espejo de internal/foto/foto.go: si se agrega un campo allá, se agrega acá.
// Cada objeto de una lista lleva un `id` único en su lista, y las listas van ordenadas por id.

export interface Foto {
  /** Contexto de kubectl de kviz, por ejemplo "kind-lab"; "" si kubectl no tiene contexto. */
  contexto: string;
  /** false cuando kviz no ve el cluster; entonces nodos y pods vienen vacíos. */
  conectado: boolean;
  /** Por qué kviz está desconectado, en español. */
  motivo?: string;
  /** Ordenados por id (el nombre). */
  nodos: Nodo[];
  /** Ordenados por id (namespace y nombre). */
  pods: Pod[];
}

export interface Nodo {
  /** El nombre del nodo. */
  id: string;
  nombre: string;
  internalIP: string;
}

export interface Pod {
  /** `<namespace>/<nombre>`. */
  id: string;
  nombre: string;
  namespace: string;
  /** null cuando el scheduler todavía no le asignó un nodo. */
  nodo: string | null;
  /** null mientras el pod no tiene IP. */
  ip: string | null;
  /** status.phase: Pending, Running, Succeeded, Failed o Unknown. */
  fase: string;
  /** true mientras el pod se está borrando (`Terminating`): tiene deletionTimestamp. */
  terminando: boolean;
}
