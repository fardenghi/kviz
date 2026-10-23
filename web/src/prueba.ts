// Contrato de la request de prueba (POST /api/pruebas), espejo de internal/api/pruebas.go y del
// recorrido de internal/trafico. Si se agrega un campo allá, se agrega acá.

export interface Header {
  nombre: string;
  valor: string;
}

/** Lo que manda el formulario. */
export interface PedidoDePrueba {
  /** "http" por defecto. */
  tipo?: 'http';
  /** `host[:puerto]` o una URL: `backend.default.svc.cluster.local:80`, `10.96.11.121`, `backend.default/hello`. */
  destino: string;
  /** GET por defecto. */
  metodo?: string;
  /** Reemplaza al path del destino, si viene. */
  path?: string;
  headers?: Header[];
  body?: string;
}

/** Una request de prueba ya ejecutada desde el pod de sondeo. */
export interface Prueba {
  /** Numera las pruebas de la sesión. */
  id: number;
  tipo: 'http';
  /** La URL que ejecutó el pod de sondeo. */
  url: string;
  metodo: string;
  /** El pod de sondeo del que salió: `pod de sondeo kviz-sondeo-x (kviz-system, lab-worker)`. */
  origen: string;
  /** null si no hubo respuesta; entonces `error` dice por qué (conexión rechazada, timeout…). */
  respuesta: RespuestaHTTP | null;
  error?: string;
  recorrido: Recorrido;
}

export interface RespuestaHTTP {
  status: number;
  /** OK, Not Found… */
  texto: string;
  headers: Header[];
  body: string;
  /** Tiempo total en milisegundos, medido en el pod de sondeo. */
  ms: number;
}

/** Camino que siguió la request, calculado desde la configuración. */
export interface Recorrido {
  /** En orden, numerados desde 1; vacío si el destino no es un objeto de la foto. */
  pasos: Paso[];
}

export interface Paso {
  n: number;
  /** service, endpoints o pod (después: dns, puerta, ingress, salto…). */
  tipo: string;
  /** `backend 10.96.11.121:80 → :8081`. */
  titulo: string;
  detalle: string;
  /** Objetos de la foto que marca el paso en la vista. */
  objetos: RefObjeto[];
}

/** Un objeto de la foto: su tipo y su id en la lista de la foto. */
export interface RefObjeto {
  /** service, pod, ingress, puerta o nodo. */
  tipo: string;
  /** El `id` del objeto en su lista de la foto (`default/backend`, `lab-worker2/80/TCP`…). */
  id: string;
}

/** Headers escritos uno por línea como `Nombre: valor`; las líneas sin `:` se ignoran. */
export function leerHeaders(texto: string): Header[] {
  return texto
    .split('\n')
    .map((l) => l.trim())
    .filter((l) => l.includes(':'))
    .map((l) => {
      const i = l.indexOf(':');
      return { nombre: l.slice(0, i).trim(), valor: l.slice(i + 1).trim() };
    })
    .filter((h) => h.nombre !== '');
}
