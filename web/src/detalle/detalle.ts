// Panel de detalle: qué objeto de la foto muestra. El objeto seleccionado se identifica con su id
// de escena (`pod:<ns>/<nombre>`, `service:…`, `ingress:…`, `nodo:<nombre>`, y un workload de la
// lista con `workload:<id del workload>`; ver escena/ids.ts). Es la misma selección que da el foco.
import type { Foto, Ingress, Nodo, ObjetoEnFoto, Pod, Service, Workload } from '../foto';
import { idDeObjetoEnFoto, partirId } from '../escena/ids';

/** El objeto que muestra el panel, ya buscado en la foto. */
export type Detalle =
  | { tipo: 'Pod'; pod: Pod }
  | { tipo: 'Service'; service: Service }
  | { tipo: 'Ingress'; ingress: Ingress }
  | { tipo: 'Nodo'; nodo: Nodo }
  | { tipo: 'Workload'; workload: Workload };

/** Lo que recibe cada sección del panel (ver secciones.ts). */
export interface PropsDeSeccion {
  detalle: Detalle;
  foto: Foto;
  /** Abre en el panel otro objeto, por su id de selección (un pod de un Service, el nodo de un pod…). */
  abrir: (id: string) => void;
  /** Abre el formulario de request de prueba con el destino ya completado (`host:puerto`). */
  probar?: (destino: string) => void;
}

export { idIngress, idNodo, idPod, idService, idWorkload } from '../escena/ids';

/** Busca en la foto el objeto seleccionado; null si no es un objeto con panel o ya no está. */
export function detalleDe(foto: Foto, id: string): Detalle | null {
  const { tipo, resto } = partirId(id);
  const porNsNombre = <T extends { namespace: string; nombre: string }>(lista: T[]) =>
    lista.find((o) => `${o.namespace}/${o.nombre}` === resto);
  switch (tipo) {
    case 'pod': {
      const pod = porNsNombre(foto.pods);
      return pod ? { tipo: 'Pod', pod } : null;
    }
    case 'service': {
      const service = porNsNombre(foto.services);
      return service ? { tipo: 'Service', service } : null;
    }
    case 'ingress': {
      const ingress = porNsNombre(foto.ingress);
      return ingress ? { tipo: 'Ingress', ingress } : null;
    }
    case 'nodo': {
      const nodo = foto.nodos.find((n) => n.nombre === resto);
      return nodo ? { tipo: 'Nodo', nodo } : null;
    }
    case 'workload': {
      const workload = foto.workloads.find((w) => w.id === resto);
      return workload ? { tipo: 'Workload', workload } : null;
    }
  }
  return null;
}

/** Lo que cambia según el tipo de objeto del panel. */
export interface Ficha {
  /** El objeto como lo pide la API (GET /api/comandos): su tipo de Kubernetes, namespace y nombre. */
  api: { tipo: string; namespace: string; nombre: string };
  /** Dónde está en la foto: su lista y su id; así lo referencian los events. */
  enFoto: ObjetoEnFoto;
  /** id de escena (el del foco, la selección y las advertencias). */
  id: string;
  /** Encabezado del panel: el tipo (`Service · NodePort`, `Deployment`…) y el nombre. */
  titulo: { tipo: string; nombre: string };
}

/** La ficha del objeto del panel: el único lugar que distingue cada tipo de objeto. */
export function fichaDe(d: Detalle): Ficha {
  const ficha = (api: Ficha['api'], enFoto: ObjetoEnFoto, tituloTipo: string): Ficha => ({
    api,
    enFoto,
    id: idDeObjetoEnFoto(enFoto),
    titulo: { tipo: tituloTipo, nombre: api.nombre },
  });
  switch (d.tipo) {
    case 'Pod':
      return ficha({ tipo: 'Pod', namespace: d.pod.namespace, nombre: d.pod.nombre }, { seccion: 'pods', id: d.pod.id }, 'Pod');
    case 'Service': {
      const s = d.service;
      return ficha({ tipo: 'Service', namespace: s.namespace, nombre: s.nombre }, { seccion: 'services', id: s.id }, `Service · ${s.tipo}`);
    }
    case 'Ingress': {
      const i = d.ingress;
      return ficha({ tipo: 'Ingress', namespace: i.namespace, nombre: i.nombre }, { seccion: 'ingress', id: i.id }, 'Ingress');
    }
    case 'Nodo':
      return ficha({ tipo: 'Node', namespace: '', nombre: d.nodo.nombre }, { seccion: 'nodos', id: d.nodo.id }, 'Nodo');
    case 'Workload': {
      const w = d.workload;
      return ficha({ tipo: w.tipo, namespace: w.namespace, nombre: w.nombre }, { seccion: 'workloads', id: w.id }, w.tipo);
    }
  }
}

/** Edad al estilo de kubectl get: 45s, 12m, 5h, 3d. "" si no se sabe cuándo se creó. */
export function edad(creado: string | undefined, ahora: number): string {
  if (!creado) return '';
  const s = Math.max(0, Math.floor((ahora - Date.parse(creado)) / 1000));
  if (s < 120) return `${s}s`;
  if (s < 3600) return `${Math.floor(s / 60)}m`;
  if (s < 2 * 86400) return `${Math.floor(s / 3600)}h`;
  return `${Math.floor(s / 86400)}d`;
}

/** `clave=valor` ordenados por clave. */
export const paresClaveValor = (m: Record<string, string> | undefined) =>
  Object.entries(m ?? {})
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([k, v]) => `${k}=${v}`);
