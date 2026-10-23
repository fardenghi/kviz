// Escena de la vista de topología (módulo 1 del frontend): función pura de foto + estado de la
// interfaz a escena. Decide qué se dibuja, dónde y con qué texto; el lienzo solo la pinta.
import type { Foto, Pod } from '../foto';
import { type MarcasPod, marcasDePod } from './marcas-pod';

/** Estado de la interfaz que afecta la escena. Crece con foco, workload, recorrido y filtros. */
export interface EstadoInterfaz {
  /** La foto que se dibujó antes de esta, para marcar lo que cambió (reinicios que aumentaron). */
  fotoAnterior?: Foto;
}

export interface Caja {
  x: number;
  y: number;
  ancho: number;
  alto: number;
}

export interface RegionEnEscena extends Caja {
  id: string;
  /** Nombre del nodo, o null para la región "Sin nodo asignado". */
  nodo: string | null;
  titulo: string;
  subtitulo: string;
}

export interface PodEnEscena extends Caja, MarcasPod {
  /** `pod:<namespace>/<nombre>`: único en todo el cluster. */
  id: string;
  nombre: string;
  namespace: string;
  ip: string | null;
  /** id de la región que lo contiene. */
  region: string;
}

export interface Escena {
  /** Tamaño total de la escena; todo lo que se dibuja cae en [0, ancho] × [0, alto]. */
  ancho: number;
  alto: number;
  regiones: RegionEnEscena[];
  pods: PodEnEscena[];
}

export const MEDIDAS = {
  anchoPod: 178,
  altoPod: 62,
  hueco: 10,
  relleno: 16,
  cabecera: 50,
  columnas: 2,
  separacionNodos: 40,
} as const;

export const idPod = (p: Pick<Pod, 'namespace' | 'nombre'>) => `pod:${p.namespace}/${p.nombre}`;

export const SIN_NODO = 'sin-nodo';

export function construirEscena(foto: Foto, estado: EstadoInterfaz = {}): Escena {
  const M = MEDIDAS;
  const anteriores = new Map((estado.fotoAnterior?.pods ?? []).map((p) => [idPod(p), p]));
  const regiones: RegionEnEscena[] = [];
  const pods: PodEnEscena[] = [];

  // una región por nodo, lado a lado, y al final "Sin nodo asignado" solo si hace falta
  const grupos = foto.nodos.map((n) => ({
    id: `nodo:${n.nombre}`,
    nodo: n.nombre as string | null,
    titulo: n.nombre,
    subtitulo: n.internalIP,
    pods: foto.pods.filter((p) => p.nodo === n.nombre),
  }));
  const sinNodo = foto.pods.filter((p) => p.nodo === null);
  if (sinNodo.length > 0) {
    grupos.push({ id: SIN_NODO, nodo: null, titulo: 'Sin nodo asignado', subtitulo: 'Pending', pods: sinNodo });
  }

  // los pods van en una grilla de 2 columnas debajo de la cabecera (nombre e IP) de la región
  const anchoRegion = M.columnas * M.anchoPod + (M.columnas - 1) * M.hueco + 2 * M.relleno;
  const y = 0;
  let x = 0;
  for (const { pods: suyos, ...g } of grupos) {
    const filas = Math.max(1, Math.ceil(suyos.length / M.columnas));
    const alto = M.cabecera + filas * (M.altoPod + M.hueco) - M.hueco + M.relleno;
    regiones.push({ ...g, x, y, ancho: anchoRegion, alto });
    suyos.forEach((p, i) => {
      const tarjeta = {
        x: x + M.relleno + (i % M.columnas) * (M.anchoPod + M.hueco),
        y: y + M.cabecera + Math.floor(i / M.columnas) * (M.altoPod + M.hueco),
        ancho: M.anchoPod,
        alto: M.altoPod,
      };
      pods.push({
        id: idPod(p),
        nombre: p.nombre,
        namespace: p.namespace,
        ip: p.ip,
        region: g.id,
        ...marcasDePod(p, tarjeta, anteriores.get(idPod(p))),
        ...tarjeta,
      });
    });
    x += anchoRegion + M.separacionNodos;
  }

  const ancho = Math.max(0, ...regiones.map((r) => r.x + r.ancho));
  const alto = Math.max(0, ...regiones.map((r) => r.y + r.alto));
  return { ancho, alto, regiones, pods };
}
