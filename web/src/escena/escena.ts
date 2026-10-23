// Escena de la vista de topología (módulo 1 del frontend): función pura de foto + estado de la
// interfaz a escena. Decide qué se dibuja, dónde y con qué texto; el lienzo solo la pinta.
import type { Foto, Pod, Workload } from '../foto';
import { indiceDeWorkloads, type MarcaDeWorkload, marcaDeWorkload, seleccionDeWorkload } from './workloads';

/** Estado de la interfaz que afecta la escena. Crece con foco, workload, recorrido y filtros. */
export interface EstadoInterfaz {
  /** Interruptor de los namespaces del sistema; por defecto ocultos. `kviz-system` nunca se ve. */
  mostrarSistema?: boolean;
  /** Filtro por namespace: solo se ven los pods de este namespace. Sin valor, todos. */
  namespace?: string | null;
  /**
   * ids (`pod:<namespace>/<nombre>`) de pods que participan del recorrido y se ven aunque los
   * filtros los oculten, como el pod del Ingress Controller. Lo completa el recorrido (ticket 17).
   */
  podsDelRecorrido?: string[];
  /** id del workload seleccionado en la lista de workloads. No es foco: solo resalta sus pods. */
  workloadSeleccionado?: string | null;
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

export interface PodEnEscena extends Caja {
  /** `pod:<namespace>/<nombre>`: único en todo el cluster. */
  id: string;
  nombre: string;
  namespace: string;
  ip: string | null;
  /** id de la región que lo contiene. */
  region: string;
  /** Barra y etiqueta corta de su workload, en su color. */
  workload: MarcaDeWorkload;
  /** Con un workload seleccionado, el color del borde de sus pods; si no, null. */
  bordeWorkload: string | null;
  /** 1, o atenuado cuando hay un workload seleccionado y el pod no es suyo. */
  opacidad: number;
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

/** Namespaces del sistema: ocultos salvo con el interruptor. */
export const NAMESPACES_DEL_SISTEMA = [
  'kube-system',
  'kube-public',
  'kube-node-lease',
  'ingress-nginx',
  'local-path-storage',
  'kviz-system',
];

export const esNamespaceDelSistema = (ns: string) => NAMESPACES_DEL_SISTEMA.includes(ns);

/** Donde vive el pod de sondeo de kviz: no es parte del cluster del estudiante. */
const NAMESPACE_DE_KVIZ = 'kviz-system';

const pasaElInterruptor = (ns: string, estado: EstadoInterfaz) =>
  ns !== NAMESPACE_DE_KVIZ && (!!estado.mostrarSistema || !esNamespaceDelSistema(ns));

/** Namespaces que ofrece el filtro: los que tienen pods, sin los del sistema mientras están ocultos. */
export function namespacesDelFiltro(foto: Foto, estado: EstadoInterfaz = {}): string[] {
  const nombres = new Set(foto.pods.map((p) => p.namespace).filter((ns) => pasaElInterruptor(ns, estado)));
  return [...nombres].sort();
}

/** Workloads de la lista: los de los namespaces que la vista muestra, con los mismos filtros que los pods. */
export function workloadsVisibles(foto: Foto, estado: EstadoInterfaz = {}): Workload[] {
  return (foto.workloads ?? []).filter(
    (w) => pasaElInterruptor(w.namespace, estado) && (!estado.namespace || w.namespace === estado.namespace),
  );
}

export function construirEscena(foto: Foto, estado: EstadoInterfaz = {}): Escena {
  const M = MEDIDAS;
  const regiones: RegionEnEscena[] = [];
  const pods: PodEnEscena[] = [];

  const delRecorrido = new Set(estado.podsDelRecorrido);
  const workloads = indiceDeWorkloads(foto);
  const seleccionado = estado.workloadSeleccionado ? workloads.get(estado.workloadSeleccionado) : undefined;
  const visibles = foto.pods.filter((p) =>
    delRecorrido.has(idPod(p))
      ? p.namespace !== NAMESPACE_DE_KVIZ
      : pasaElInterruptor(p.namespace, estado) && (!estado.namespace || p.namespace === estado.namespace),
  );

  // una región por nodo, lado a lado, y al final "Sin nodo asignado" solo si hace falta
  const grupos = foto.nodos.map((n) => ({
    id: `nodo:${n.nombre}`,
    nodo: n.nombre as string | null,
    titulo: n.nombre,
    subtitulo: n.internalIP,
    pods: visibles.filter((p) => p.nodo === n.nombre),
  }));
  const sinNodo = visibles.filter((p) => p.nodo === null);
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
      pods.push({
        id: idPod(p),
        nombre: p.nombre,
        namespace: p.namespace,
        ip: p.ip,
        region: g.id,
        workload: marcaDeWorkload(p, workloads),
        ...seleccionDeWorkload(p, seleccionado),
        x: x + M.relleno + (i % M.columnas) * (M.anchoPod + M.hueco),
        y: y + M.cabecera + Math.floor(i / M.columnas) * (M.altoPod + M.hueco),
        ancho: M.anchoPod,
        alto: M.altoPod,
      });
    });
    x += anchoRegion + M.separacionNodos;
  }

  const ancho = Math.max(0, ...regiones.map((r) => r.x + r.ancho));
  const alto = Math.max(0, ...regiones.map((r) => r.y + r.alto));
  return { ancho, alto, regiones, pods };
}
