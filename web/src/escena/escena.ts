// Escena de la vista de topología (módulo 1 del frontend): función pura de foto + estado de la
// interfaz a escena. Decide qué se dibuja, dónde y con qué texto; el lienzo solo la pinta.
import type { Foto, Ingress, Pod, Puerta, PuertoService, ReglaIngress, Service, Workload } from '../foto';
import { indiceDeWorkloads, type MarcaDeWorkload, marcaDeWorkload, seleccionDeWorkload } from './workloads';
import { type MarcasPod, marcasDePod } from './marcas-pod';
import { aplicarFoco } from './foco';
import { aplicarRecorrido } from './recorrido';
import type { Recorrido } from '../prueba';

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
  /** id del workload seleccionado en la lista de workloads. No es foco: solo resalta sus pods. */
  workloadSeleccionado?: string | null;
  /**
   * id de escena del objeto apuntado con el mouse. Si es un Service, pod, Ingress o puerta, es el
   * foco (manda sobre la selección) y además aclara los objetos no relacionados.
   */
  hover?: string | null;
  /** id de escena del objeto seleccionado. Si es un Service, pod, Ingress o puerta, es el foco. */
  seleccionado?: string | null;
  /**
   * Recorrido de la request de prueba que se está mirando (ver recorrido.ts). Si tiene pasos, manda
   * sobre todo lo demás: apaga el foco y atenúa lo que no participa.
   */
  recorrido?: Recorrido | null;
}

export interface Caja {
  x: number;
  y: number;
  ancho: number;
  alto: number;
}

/** Cómo marca el foco a un objeto (ver foco.ts). */
export interface MarcasDeFoco {
  /**
   * Borde azul (puertas: relleno sólido). Con hover, el foco y sus relacionados; con selección, el
   * foco. Las puertas relacionadas se resaltan siempre.
   */
  resaltado: boolean;
  /** 1, o aclarado si hay hover y no está relacionado con el foco (los pods además se atenúan por otras razones). */
  opacidad: number;
}

/** Cómo marca el recorrido a un objeto (ver recorrido.ts); su opacidad va en MarcasDeFoco. */
export interface MarcasDeRecorrido {
  /** Números de los pasos del recorrido en los que participa (rosas, arriba a la izquierda); vacío si no participa. */
  pasos: number[];
}

export interface RegionEnEscena extends Caja {
  id: string;
  /** Nombre del nodo, o null para la región "Sin nodo asignado". */
  nodo: string | null;
  titulo: string;
  subtitulo: string;
}

export interface PodEnEscena extends Caja, MarcasPod, MarcasDeFoco, MarcasDeRecorrido {
  /** `pod:<namespace>/<nombre>`: único en todo el cluster. */
  id: string;
  nombre: string;
  namespace: string;
  ip: string | null;
  /** id de la región que lo contiene. */
  region: string;
  /** Acaba de aparecer: brilla 3 veces. */
  nuevo: boolean;
  /** Se está borrando (`Terminating`): se ve atenuado (ver opacidad) y punteado hasta que desaparece. */
  terminando: boolean;
  /** Barra y etiqueta corta de su workload, en su color. */
  workload: MarcaDeWorkload;
  /** Con un workload seleccionado, el color del borde de sus pods; si no, null. */
  bordeWorkload: string | null;
  /** 1, o atenuado cuando hay un workload seleccionado y el pod no es suyo, si es Terminating o por el foco. */
  opacidad: number;
}

export interface ServiceEnEscena extends Caja, MarcasDeFoco, MarcasDeRecorrido {
  /** `service:<namespace>/<nombre>`. */
  id: string;
  nombre: string;
  namespace: string;
  tipo: string;
  clusterIP: string;
  /** Un texto por puerto: `80 → 8081`, con `:30010` si abre un nodePort. */
  puertos: string[];
}

export interface IngressEnEscena extends Caja, MarcasDeFoco, MarcasDeRecorrido {
  /** `ingress:<namespace>/<nombre>`. */
  id: string;
  nombre: string;
  namespace: string;
  /** La IngressClass, o `sin clase`. */
  clase: string;
  /** Un texto por regla: `host/path → service:puerto`, con `⟲ rewrite` si el Ingress reescribe. */
  reglas: string[];
}

/** Puerta: pestaña en el borde de arriba de un nodo, para un NodePort o un hostPort. */
export interface PuertaEnEscena extends Caja, MarcasDeFoco, MarcasDeRecorrido {
  /** `puerta:<nodo>/<puerto>`, con `/<protocolo>` si no es TCP. */
  id: string;
  nodo: string;
  /** id de la región del nodo. */
  region: string;
  tipo: Puerta['tipo'];
  puerto: number;
  /** `:30010`. */
  texto: string;
  /** `NodePort 30010 → backend:80`. */
  titulo: string;
  /** id del objeto al que lleva: su Service (NodePort) o su pod (hostPort). */
  destino: string;
}

/**
 * Tono de una conexión. En reposo: `reposo`, o `no-listo` hacia un endpoint no listo. Con foco:
 * `foco` (azul, con flecha) entre relacionados y `fantasma` (casi invisible) el resto; con un
 * workload seleccionado, `fantasma` las que no van a sus pods. Ver foco.ts. Con un recorrido:
 * `recorrido` (rosa, discontinua y animada) entre objetos del recorrido y `fantasma` el resto
 * (ver recorrido.ts).
 */
export type TonoConexion = 'reposo' | 'no-listo' | 'foco' | 'fantasma' | 'recorrido';

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
  /** Línea de una puerta a su Service o pod: solo aparece con foco (ver foco.ts). */
  dePuerta?: boolean;
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
  ingress: IngressEnEscena[];
  puertas: PuertaEnEscena[];
  conexiones: ConexionEnEscena[];
  /** id del objeto con foco (hover o selección), o null. */
  foco: string | null;
  /** ids de los objetos relacionados con el foco, incluido el foco, ordenados; vacío sin foco. */
  relacionados: string[];
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
  anchoIngress: 270,
  /** Alto de un Ingress con una regla; cada regla más suma `altoReglaIngress`. */
  altoIngress: 60,
  altoReglaIngress: 14,
  huecoIngress: 20,
  /** Espacio entre la última fila de Ingress y la primera de Services, para las líneas. */
  separacionIngress: 50,
  anchoPuerta: 62,
  altoPuerta: 24,
  /** Distancia entre el comienzo de una puerta y el de la siguiente, de derecha a izquierda. */
  pasoPuerta: 70,
  /** Margen derecho de la primera puerta dentro del ancho del nodo. */
  margenPuerta: 20,
  /** Distancia entre filas de puertas, cuando no entran en una. */
  pasoFilaPuerta: 30,
} as const;

export const idPod = (p: Pick<Pod, 'namespace' | 'nombre'>) => `pod:${p.namespace}/${p.nombre}`;
export const idService = (s: Pick<Service, 'namespace' | 'nombre'>) => `service:${s.namespace}/${s.nombre}`;
export const idIngress = (i: Pick<Ingress, 'namespace' | 'nombre'>) => `ingress:${i.namespace}/${i.nombre}`;

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

/** Opacidad de un pod Terminating: se ve atenuado hasta que desaparece de la foto. */
export const OPACIDAD_TERMINANDO = 0.4;

/** Un pod Terminating queda atenuado, y más si además lo atenúa la selección de un workload. */
function terminando<T extends { opacidad: number }>(p: Pod, marcas: T): T & { terminando: boolean } {
  const terminando = p.estado === 'Terminating';
  return { ...marcas, terminando, opacidad: terminando ? Math.min(marcas.opacidad, OPACIDAD_TERMINANDO) : marcas.opacidad };
}

/** Namespaces que ofrece el filtro: los que tienen pods, sin los del sistema mientras están ocultos. */
export function namespacesDelFiltro(foto: Foto, estado: EstadoInterfaz = {}): string[] {
  const nombres = new Set(foto.pods.map((p) => p.namespace).filter((ns) => pasaElInterruptor(ns, estado)));
  return [...nombres].sort();
}

/** Workloads de la lista: los de los namespaces que la vista muestra, con los mismos filtros que los pods. */
export function workloadsVisibles(foto: Foto, estado: EstadoInterfaz = {}): Workload[] {
  return (foto.workloads ?? []).filter((w) => seVe(w.namespace, estado));
}

export function construirEscena(foto: Foto, estado: EstadoInterfaz = {}): Escena {
  const M = MEDIDAS;
  const anteriores = new Map((estado.fotoAnterior?.pods ?? []).map((p) => [idPod(p), p]));
  const regiones: RegionEnEscena[] = [];
  const pods: PodEnEscena[] = [];

  const delRecorrido = new Set(estado.podsDelRecorrido);
  const workloads = indiceDeWorkloads(foto);
  const seleccionado = estado.workloadSeleccionado ? workloads.get(estado.workloadSeleccionado) : undefined;
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
        resaltado: false,
        pasos: [],
        ...terminando(p, seleccionDeWorkload(p, seleccionado)),
        workload: marcaDeWorkload(p, workloads),
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

  const ingressVisibles = foto.ingress.filter((i) => seVe(i.namespace, estado));
  const ingress = ubicarIngress(
    ingressVisibles,
    services,
    Math.max(0, ...[...regiones, ...services].map((c) => c.x + c.ancho)),
  );

  // los Services bajan para dejar arriba las filas de Ingress
  if (ingress.length > 0) {
    const bajar = Math.max(...ingress.map((i) => i.y + i.alto)) + M.separacionIngress;
    for (const s of services) s.y += bajar;
  }

  // las puertas de cada nodo, de derecha a izquierda (NodePort primero, después por número); si no
  // entran en una fila, suben a otra. La foto las trae por id, que no es el orden numérico.
  const puertasPorNodo = new Map<string, Puerta[]>();
  for (const p of foto.puertas.filter((p) => seVePuerta(p, estado, delRecorrido)).sort(ordenPuertas)) {
    puertasPorNodo.set(p.nodo, [...(puertasPorNodo.get(p.nodo) ?? []), p]);
  }
  const porFila = Math.max(1, Math.floor((anchoRegion - M.margenPuerta - M.anchoPuerta) / M.pasoPuerta) + 1);
  const filasDePuertas = Math.max(0, ...[...puertasPorNodo.values()].map((ps) => Math.ceil(ps.length / porFila)));
  const filasExtra = Math.max(0, filasDePuertas - 1) * M.pasoFilaPuerta;

  // los nodos bajan para dejar arriba las filas de Ingress y Services, y lugar para las puertas
  const arriba = [...ingress, ...services];
  const bajar = Math.max(
    arriba.length > 0 ? Math.max(...arriba.map((c) => c.y + c.alto)) + M.separacionServices + filasExtra : 0,
    filasDePuertas > 0 ? M.altoPuerta / 2 + filasExtra : 0,
  );
  for (const c of [...regiones, ...pods, ...pods.flatMap((p) => p.containers)]) c.y += bajar;

  const puertas = regiones.flatMap((r) =>
    (r.nodo === null ? [] : (puertasPorNodo.get(r.nodo) ?? [])).map((p, i) => ({
      id: idPuerta(p),
      nodo: p.nodo,
      region: r.id,
      tipo: p.tipo,
      puerto: p.puerto,
      texto: `:${p.puerto}` + (p.protocolo !== 'TCP' ? `/${p.protocolo}` : ''),
      titulo: `${p.tipo} ${p.puerto} → ${p.service ?? p.pod}:${p.haciaPuerto}`,
      destino: p.service !== null ? idService({ namespace: p.namespace, nombre: p.service }) : idPod({ namespace: p.namespace, nombre: p.pod! }),
      x: r.x + r.ancho - M.margenPuerta - ((i % porFila) + 1) * M.pasoPuerta + (M.pasoPuerta - M.anchoPuerta),
      y: r.y - M.altoPuerta / 2 - Math.floor(i / porFila) * M.pasoFilaPuerta,
      ancho: M.anchoPuerta,
      alto: M.altoPuerta,
      resaltado: false,
      opacidad: 1,
      pasos: [],
    })),
  );

  const conexiones = [
    ...conexionesIngressService(ingressVisibles, ingress, services),
    ...conexionesServicePod(foto.services, services, pods),
  ];

  const delWorkload = seleccionado
    ? new Set(visibles.filter((p) => p.workload === seleccionado.id).map(idPod))
    : null;

  const cajas: Caja[] = [...regiones, ...ingress, ...services];
  const ancho = Math.max(0, ...cajas.map((c) => c.x + c.ancho));
  const alto = Math.max(0, ...cajas.map((c) => c.y + c.alto));
  const escena = aplicarFoco({ ancho, alto, regiones, pods, services, ingress, puertas, conexiones, foco: null, relacionados: [] }, estado, delWorkload);
  return aplicarRecorrido(escena, foto, estado);
}

const ordenPuertas = (a: Puerta, b: Puerta) =>
  Number(b.tipo === 'NodePort') - Number(a.tipo === 'NodePort') ||
  a.puerto - b.puerto ||
  (a.protocolo < b.protocolo ? -1 : a.protocolo > b.protocolo ? 1 : 0);

export const idPuerta = (p: Pick<Puerta, 'nodo' | 'puerto' | 'protocolo'>) =>
  `puerta:${p.nodo}/${p.puerto}` + (p.protocolo !== 'TCP' ? `/${p.protocolo}` : '');

/**
 * Una puerta NodePort se ve si se ve su Service. Una de hostPort es del nodo: se ve aunque los
 * namespaces del sistema estén ocultos (el hostPort del Ingress Controller es la entrada al
 * cluster), pero respeta el filtro por namespace. Las de kviz-system no se ven nunca.
 */
function seVePuerta(p: Puerta, estado: EstadoInterfaz, delRecorrido: Set<string>): boolean {
  if (p.tipo === 'NodePort') return seVe(p.namespace, estado);
  if (p.namespace === NAMESPACE_DE_KVIZ) return false;
  return !estado.namespace || p.namespace === estado.namespace || delRecorrido.has(idPod({ namespace: p.namespace, nombre: p.pod! }));
}

/** Services de la escena a los que manda un Ingress (los de sus reglas, en su namespace), sin repetir. */
const servicesDe = (i: Ingress, porId: Map<string, ServiceEnEscena>) => [
  ...new Set(
    i.reglas.flatMap((r) => {
      const s = porId.get(idService({ namespace: i.namespace, nombre: r.service }));
      return s ? [s] : [];
    }),
  ),
];

/**
 * Cada Ingress va arriba del centro de sus Services. Si se pisa con uno ya ubicado, baja a la fila
 * siguiente. Los Ingress sin Services en la escena van a la derecha de todo, en la primera fila.
 * Las filas empiezan en y = 0 y cada una es tan alta como su Ingress más alto.
 */
function ubicarIngress(visibles: Ingress[], services: ServiceEnEscena[], finEscena: number): IngressEnEscena[] {
  const M = MEDIDAS;
  const porId = new Map(services.map((s) => [s.id, s]));
  const filas: IngressEnEscena[][] = [];
  const libre = (fila: number, x: number) =>
    (filas[fila] ?? []).every((o) => x + M.anchoIngress + M.huecoIngress <= o.x || x >= o.x + o.ancho + M.huecoIngress);

  const conCentro = visibles.map((i) => {
    const suyos = servicesDe(i, porId);
    const centro = suyos.length ? suyos.reduce((t, s) => t + s.x + s.ancho / 2, 0) / suyos.length : null;
    return { i, centro };
  });
  const conServices = conCentro.filter((c) => c.centro !== null).sort((a, b) => a.centro! - b.centro!);
  const sinServices = conCentro.filter((c) => c.centro === null);

  const poner = (i: Ingress, x: number, fila: number) => {
    const reglas = i.reglas.length ? i.reglas.map((r) => textoRegla(r, i.rewrite)) : ['sin reglas'];
    (filas[fila] ??= []).push({
      id: idIngress(i),
      nombre: i.nombre,
      namespace: i.namespace,
      clase: i.clase ?? 'sin clase',
      reglas,
      x,
      y: 0,
      ancho: M.anchoIngress,
      alto: M.altoIngress + (reglas.length - 1) * M.altoReglaIngress,
      resaltado: false,
      opacidad: 1,
      pasos: [],
    });
  };

  for (const { i, centro } of conServices) {
    const x = Math.max(0, centro! - M.anchoIngress / 2);
    let fila = 0;
    while (!libre(fila, x)) fila++;
    poner(i, x, fila);
  }
  let x = Math.max(
    finEscena > 0 ? finEscena + M.huecoIngress : 0,
    ...filas.flat().map((u) => u.x + u.ancho + M.huecoIngress),
  );
  for (const { i } of sinServices) {
    poner(i, x, 0);
    x += M.anchoIngress + M.huecoIngress;
  }

  let y = 0;
  for (const fila of filas) {
    for (const i of fila) i.y = y;
    y += Math.max(...fila.map((i) => i.alto)) + M.huecoFilas;
  }
  return filas.flat();
}

/** `host/path → service:puerto ⟲ rewrite`; el backend por defecto (sin host ni path) dice `(por defecto)`. */
function textoRegla(r: ReglaIngress, rewrite: string | null): string {
  const desde = r.host + r.path || '(por defecto)';
  return `${desde} → ${r.service}:${r.puerto}` + (rewrite && r.path ? ` ⟲ ${rewrite}` : '');
}

/** Una línea de cada Ingress a cada Service distinto de sus reglas. */
function conexionesIngressService(
  visibles: Ingress[],
  ingress: IngressEnEscena[],
  services: ServiceEnEscena[],
): ConexionEnEscena[] {
  const enEscena = new Map(ingress.map((i) => [i.id, i]));
  const porId = new Map(services.map((s) => [s.id, s]));
  return visibles.flatMap((i) => {
    const caja = enEscena.get(idIngress(i))!;
    return servicesDe(i, porId).map((s) => ({
      id: `${caja.id}->${s.id}`,
      desde: caja.id,
      hasta: s.id,
      tono: 'reposo' as const,
      inicio: { x: caja.x + caja.ancho / 2, y: caja.y + caja.alto },
      fin: { x: s.x + s.ancho / 2, y: s.y },
    }));
  });
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
      resaltado: false,
      opacidad: 1,
      pasos: [],
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
