// Panel de detalle: qué objeto de la foto muestra. El objeto seleccionado se identifica con su id
// de escena (`pod:<ns>/<nombre>`, `service:…`, `ingress:…`, `nodo:<nombre>`, ver escena.ts), y un
// workload de la lista con `workload:<id del workload>`. Es la misma selección que da el foco.
import type { Foto, Ingress, Nodo, Pod, Service, Workload } from '../foto';
import { idIngress, idPod, idService } from '../escena/escena';

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
}

export const idDeWorkload = (w: Pick<Workload, 'id'>) => `workload:${w.id}`;
export const idDeNodo = (nombre: string) => `nodo:${nombre}`;
export { idIngress, idPod, idService };

/** Busca en la foto el objeto seleccionado; null si no es un objeto con panel o ya no está. */
export function detalleDe(foto: Foto, id: string): Detalle | null {
  const corte = id.indexOf(':');
  const tipo = id.slice(0, corte);
  const resto = id.slice(corte + 1);
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

/** El objeto como lo pide la API (GET /api/comandos): su tipo de Kubernetes, namespace y nombre. */
export function objetoDeLaApi(d: Detalle): { tipo: string; namespace: string; nombre: string } {
  switch (d.tipo) {
    case 'Pod':
      return { tipo: 'Pod', namespace: d.pod.namespace, nombre: d.pod.nombre };
    case 'Service':
      return { tipo: 'Service', namespace: d.service.namespace, nombre: d.service.nombre };
    case 'Ingress':
      return { tipo: 'Ingress', namespace: d.ingress.namespace, nombre: d.ingress.nombre };
    case 'Nodo':
      return { tipo: 'Node', namespace: '', nombre: d.nodo.nombre };
    case 'Workload':
      return { tipo: d.workload.tipo, namespace: d.workload.namespace, nombre: d.workload.nombre };
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
