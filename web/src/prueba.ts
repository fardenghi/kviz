// Contrato de la request de prueba (POST /api/pruebas), espejo de internal/api/pruebas.go y del
// recorrido de internal/trafico. Si se agrega un campo allá, se agrega acá.

export interface Header {
  nombre: string;
  valor: string;
}

/** Request de prueba HTTP o prueba de conexión TCP. */
export type TipoDePrueba = 'http' | 'tcp';

/** Lo que manda el formulario. */
export interface PedidoDePrueba {
  /** "http" por defecto. "tcp" es una prueba de conexión: solo usa `destino`. */
  tipo?: TipoDePrueba;
  /**
   * `host[:puerto]` o una URL: `backend.default.svc.cluster.local:80`, `10.96.11.121`, `backend.default/hello`.
   * En una prueba de conexión, `host:puerto` (`postgres.default:5432`).
   */
  destino: string;
  /** GET por defecto. */
  metodo?: string;
  /** Reemplaza al path del destino, si viene. */
  path?: string;
  headers?: Header[];
  body?: string;
  /** Cuántas veces se manda la misma request (1 por defecto, hasta MAX_REPETICIONES). */
  repeticiones?: number;
}

/** Tope de repeticiones de una request de prueba (maxRepeticiones en internal/api/pruebas.go). */
export const MAX_REPETICIONES = 20;

/** Una request de prueba (o prueba de conexión) ya ejecutada desde el pod de sondeo. */
export interface Prueba {
  /** Numera las pruebas de la sesión. */
  id: number;
  tipo: TipoDePrueba;
  /** La URL que ejecutó el pod de sondeo; `tcp://host:puerto` en una prueba de conexión. */
  url: string;
  /** Vacío en una prueba de conexión. */
  metodo: string;
  /** El pod de sondeo del que salió: `pod de sondeo kviz-sondeo-x (kviz-system, lab-worker)`. */
  origen: string;
  /** null si no hubo respuesta; entonces `error` dice por qué (conexión rechazada, timeout…). */
  respuesta: RespuestaHTTP | null;
  /** Solo en una prueba de conexión; si falta, `error` dice por qué no se pudo intentar. */
  conexion?: ConexionTCP;
  error?: string;
  recorrido: Recorrido;
  /** Contexto de kubectl en el que se mandó (el historial dura toda la sesión). */
  contexto: string;
  /** Cuántas veces se mandó; respuesta, error y el pod del recorrido son los de la primera. */
  repeticiones: number;
  /** Respuestas por pod; null si se mandó una sola vez. */
  reparto: Reparto | null;
}

/** Cómo se repartieron entre los pods las respuestas de una request repetida N veces. */
export interface Reparto {
  /** Requests mandadas. */
  total: number;
  /** Respuestas de cada pod identificado (id de la foto, `<ns>/<nombre>`), de más a menos. */
  pods: { pod: string; n: number }[];
  /** Respuestas de las que no se sabe qué pod las dio. */
  sinIdentificar: number;
  /** Requests que no recibieron respuesta. */
  sinRespuesta: number;
}

/** Resultado de una prueba de conexión TCP. */
export interface ConexionTCP {
  resultado: 'abre' | 'rechaza' | 'timeout';
  /** Lo que tardó en abrirse, en ser rechazada o en vencer, medido en el pod de sondeo. */
  ms: number;
  /** El mensaje del pod de sondeo cuando no abre. */
  motivo?: string;
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
  /** dns, puerta, ingress, service, endpoints, salto o pod (ver internal/trafico). */
  tipo: string;
  /** `backend 10.96.11.121:80 → :8081`. */
  titulo: string;
  detalle: string;
  /** Objetos de la foto que marca el paso en la vista. */
  objetos: RefObjeto[];
}

/** Un objeto de la foto: su tipo y su id en la lista de la foto. */
export interface RefObjeto {
  /** service, pod, ingress, puerta o nodo. El paso salto marca dos nodos: de dónde sale y a dónde llega. */
  tipo: string;
  /** El `id` del objeto en su lista de la foto (`default/backend`, `lab-worker2/80/TCP`…). */
  id: string;
}

/** Lo que se probó, en una línea: `POST http://backend.default/hello` o `TCP postgres.default:5432`. */
export function lineaDelPedido(p: Prueba): string {
  return p.tipo === 'tcp' ? `TCP ${p.url.replace(/^tcp:\/\//, '')}` : `${p.metodo} ${p.url}`;
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

/**
 * Separa el path de un destino escrito como `host[:puerto]/path` (con o sin esquema), para que el
 * formulario lo muestre en su campo: `lab-worker2:80/backend/` → `lab-worker2:80` y `/backend/`.
 */
export function separarDestino(destino: string): { destino: string; path: string | null } {
  const esquema = destino.indexOf('://');
  const desde = esquema >= 0 ? esquema + 3 : 0;
  const barra = destino.indexOf('/', desde);
  if (barra < 0) return { destino, path: null };
  return { destino: destino.slice(0, barra), path: destino.slice(barra) };
}

/**
 * El destino entrando por un nodo: cambia el host por el nombre del nodo y deja el esquema y el
 * puerto (el de un NodePort o un hostPort): `backend.default:30010` → `lab-worker:30010`.
 */
export function entrarPorNodo(destino: string, nodo: string): string {
  const esquema = destino.indexOf('://');
  const prefijo = esquema >= 0 ? destino.slice(0, esquema + 3) : '';
  const resto = destino.slice(prefijo.length);
  const corte = resto.search(/[/?]/);
  const autoridad = corte >= 0 ? resto.slice(0, corte) : resto;
  const despues = corte >= 0 ? resto.slice(corte) : '';
  const puerto = /:(\d+)$/.exec(autoridad)?.[0] ?? '';
  return prefijo + nodo + puerto + despues;
}
