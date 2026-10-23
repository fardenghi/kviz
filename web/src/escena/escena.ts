// Escena de la vista de topología (módulo 1 del frontend): función pura de foto + estado de la
// interfaz a escena. Decide qué se dibuja, dónde y con qué texto; el lienzo solo la pinta.
import type { Foto, Pod, PuertoService, Service } from '../foto';
import { type MarcasPod, marcasDePod } from './marcas-pod';

/** Estado de la interfaz que afecta la escena. Crece con foco, workload, recorrido y filtros. */
export interface EstadoInterfaz {
  /** La foto que se dibujó antes de esta, para marcar lo que cambió (reinicios que aumentaron). */
  fotoAnterior?: Foto;
  /** Interruptor de los namespaces del sistema; por defecto ocultos. `kviz-system` nunca se ve. */
  mostrarSistema?: boolean;
  /** Filtro por namespace: solo se ven los pods de este namespace. Sin valor, todos. */
  namespace?: string | null;
  /**
   * ids (`pod:<namespace>/<nombre>`) de pods que participan del recorrido y se ven aunque los
   * filtros los oculten, como el pod del Ingress Controller. Lo completa el recorrido (ticket 17).
   */
  podsDelRecorrido?: string[];
  /** ids de escena de los pods que acaban de aparecer y tienen que brillar (ver tiempoReal.ts). */
  podsNuevos?: ReadonlySet<string>;
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
  /** Acaba de aparecer: brilla 3 veces. */
  nuevo: boolean;
  /** Se está borrando (`Terminating`): se ve atenuado hasta que desaparece. */
  terminando: boolean;
}

export interface ServiceEnEscena extends Caja {
  /** `service:<namespace>/<nombre>`. */
  id: string;
  nombre: string;
  namespace: string;
  tipo: string;
  clusterIP: string;
  /** Un texto por puerto: `80 → 8081`, con `:30010` si abre un nodePort. */
  puertos: string[];
}

/** Tono de una conexión. En reposo: `reposo`, o `no-listo` hacia un endpoint no listo. */
export type TonoConexion = 'reposo' | 'no-listo';

export interface Punto {
  x: number;
  y: number;
}

/** Línea entre dos objetos de la escena, del borde de abajo de `desde` al de arriba de `hasta`. */
export interface ConexionEnEscena {
  id: string;
  /** ids de los objetos que une, por ejemplo `service:default/backend` → `pod:default/backend-x`. */
  desde: string;
  hasta: string;
  tono: TonoConexion;
  inicio: Punto;
  fin: Punto;
}

export interface Escena {
  /** Tamaño total de la escena; todo lo que se dibuja cae en [0, ancho] × [0, alto]. */
  ancho: number;
  alto: number;
  regiones: RegionEnEscena[];
  pods: PodEnEscena[];
  services: ServiceEnEscena[];
  conexiones: ConexionEnEscena[];
}

export const MEDIDAS = {
  anchoPod: 178,
  altoPod: 62,
  hueco: 10,
  relleno: 16,
  cabecera: 50,
  columnas: 2,
  separacionNodos: 40,
  anchoService: 170,
  altoService: 62,
  /** Separación mínima entre dos Services de la misma fila. */
  huecoServices: 12,
  /** Separación vertical entre filas de Services. */
  huecoFilas: 16,
  /** Espacio entre la última fila de Services y los nodos, para las líneas. */
  separacionServices: 70,
} as const;

export const idPod = (p: Pick<Pod, 'namespace' | 'nombre'>) => `pod:${p.namespace}/${p.nombre}`;
export const idService = (s: Pick<Service, 'namespace' | 'nombre'>) => `service:${s.namespace}/${s.nombre}`;

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

/** Un namespace se ve si pasa el interruptor del sistema y el filtro por namespace. */
const seVe = (ns: string, estado: EstadoInterfaz) =>
  pasaElInterruptor(ns, estado) && (!estado.namespace || ns === estado.namespace);

/** Namespaces que ofrece el filtro: los que tienen pods, sin los del sistema mientras están ocultos. */
export function namespacesDelFiltro(foto: Foto, estado: EstadoInterfaz = {}): string[] {
  const nombres = new Set(foto.pods.map((p) => p.namespace).filter((ns) => pasaElInterruptor(ns, estado)));
  return [...nombres].sort();
}

export function construirEscena(foto: Foto, estado: EstadoInterfaz = {}): Escena {
  const M = MEDIDAS;
  const anteriores = new Map((estado.fotoAnterior?.pods ?? []).map((p) => [idPod(p), p]));
  const regiones: RegionEnEscena[] = [];
  const pods: PodEnEscena[] = [];

  const delRecorrido = new Set(estado.podsDelRecorrido);
  const visibles = foto.pods.filter((p) =>
    delRecorrido.has(idPod(p))
      ? p.namespace !== NAMESPACE_DE_KVIZ
      : seVe(p.namespace, estado),
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
        nuevo: estado.podsNuevos?.has(idPod(p)) ?? false,
        terminando: p.estado === 'Terminating',
        ...marcasDePod(p, tarjeta, anteriores.get(idPod(p))),
        ...tarjeta,
      });
    });
    x += anchoRegion + M.separacionNodos;
  }

  const services = ubicarServices(
    foto.services.filter((s) => seVe(s.namespace, estado)),
    pods,
    Math.max(0, ...regiones.map((r) => r.x + r.ancho)),
  );

  // los nodos bajan para dejar arriba las filas de Services
  if (services.length > 0) {
    const bajar = Math.max(...services.map((s) => s.y + s.alto)) + M.separacionServices;
    for (const c of [...regiones, ...pods, ...pods.flatMap((p) => p.containers)]) c.y += bajar;
  }

  const conexiones = conexionesServicePod(foto.services, services, pods);

  const cajas: Caja[] = [...regiones, ...services];
  const ancho = Math.max(0, ...cajas.map((c) => c.x + c.ancho));
  const alto = Math.max(0, ...cajas.map((c) => c.y + c.alto));
  return { ancho, alto, regiones, pods, services, conexiones };
}

/** Pods de la escena detrás de los endpoints de un Service (listos y no listos). */
const podsDe = (s: Service, porId: Map<string, PodEnEscena>) =>
  s.endpoints.flatMap((e) => {
    const p = e.pod === null ? undefined : porId.get(idPod({ namespace: s.namespace, nombre: e.pod }));
    return p ? [{ pod: p, listo: e.listo }] : [];
  });

/**
 * Cada Service va arriba del centro de sus pods. Si se pisa con uno ya ubicado, baja a la fila
 * siguiente (las que hagan falta). Los Services sin pods van a la derecha de todo, en la primera fila.
 * Las filas empiezan en y = 0; la escena después baja los nodos debajo de la última.
 */
function ubicarServices(visibles: Service[], pods: PodEnEscena[], finRegiones: number): ServiceEnEscena[] {
  const M = MEDIDAS;
  const porId = new Map(pods.map((p) => [p.id, p]));
  const filas: { desde: number; hasta: number }[][] = [];
  const libre = (fila: number, x: number) =>
    (filas[fila] ?? []).every((o) => x + M.anchoService + M.huecoServices <= o.desde || x >= o.hasta + M.huecoServices);

  const conCentro = visibles.map((s) => {
    const suyos = podsDe(s, porId);
    const centro = suyos.length ? suyos.reduce((t, { pod: p }) => t + p.x + p.ancho / 2, 0) / suyos.length : null;
    return { s, centro };
  });
  const conPods = conCentro.filter((c) => c.centro !== null).sort((a, b) => a.centro! - b.centro!);
  const sinPods = conCentro.filter((c) => c.centro === null);

  const ubicados: ServiceEnEscena[] = [];
  const poner = (s: Service, x: number, fila: number) => {
    (filas[fila] ??= []).push({ desde: x, hasta: x + M.anchoService });
    ubicados.push({
      id: idService(s),
      nombre: s.nombre,
      namespace: s.namespace,
      tipo: s.tipo,
      clusterIP: s.clusterIP,
      puertos: s.puertos.map(textoPuerto),
      x,
      y: fila * (M.altoService + M.huecoFilas),
      ancho: M.anchoService,
      alto: M.altoService,
    });
  };

  for (const { s, centro } of conPods) {
    const x = Math.max(0, centro! - M.anchoService / 2);
    let fila = 0;
    while (!libre(fila, x)) fila++;
    poner(s, x, fila);
  }
  let x = Math.max(
    finRegiones > 0 ? finRegiones + M.separacionNodos : 0,
    ...ubicados.map((u) => u.x + u.ancho + M.huecoServices),
  );
  for (const { s } of sinPods) {
    poner(s, x, 0);
    x += M.anchoService + M.huecoServices;
  }
  return ubicados;
}

const textoPuerto = (p: PuertoService) =>
  `${p.puerto} → ${p.targetPort}` + (p.protocolo && p.protocolo !== 'TCP' ? `/${p.protocolo}` : '') + (p.nodePort ? ` · :${p.nodePort}` : '');

/** Una línea de cada Service a cada pod de sus endpoints, que distingue los no listos. */
function conexionesServicePod(todos: Service[], services: ServiceEnEscena[], pods: PodEnEscena[]): ConexionEnEscena[] {
  const porId = new Map(pods.map((p) => [p.id, p]));
  const enEscena = new Map(services.map((s) => [s.id, s]));
  const conexiones: ConexionEnEscena[] = [];
  for (const s of todos) {
    const caja = enEscena.get(idService(s));
    if (!caja) continue;
    const vistos = new Set<string>();
    for (const { pod: p, listo } of podsDe(s, porId)) {
      if (vistos.has(p.id)) continue;
      vistos.add(p.id);
      conexiones.push({
        id: `${caja.id}->${p.id}`,
        desde: caja.id,
        hasta: p.id,
        tono: listo ? 'reposo' : 'no-listo',
        inicio: { x: caja.x + caja.ancho / 2, y: caja.y + caja.alto },
        fin: { x: p.x + p.ancho / 2, y: p.y },
      });
    }
  }
  return conexiones;
}
