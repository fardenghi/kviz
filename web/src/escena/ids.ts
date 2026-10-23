// ids de escena: cada objeto que la vista dibuja o el panel abre se identifica con
// `<tipo>:<id en la foto>`: `pod:<ns>/<nombre>`, `service:<ns>/<nombre>`, `ingress:<ns>/<nombre>`,
// `puerta:<nodo>/<puerto>[/<protocolo>]`, `nodo:<nombre>` y `workload:<id del workload>`. Es la
// misma identificación para el foco, la selección y el panel de detalle; este módulo es el único
// que los arma y los parte.
import type { Puerta, Workload } from '../foto';

/** Un objeto con namespace, como lo identifica la foto: `<namespace>/<nombre>`. */
export interface NsNombre {
  namespace: string;
  nombre: string;
}

export const idPod = (p: NsNombre) => `pod:${p.namespace}/${p.nombre}`;
export const idService = (s: NsNombre) => `service:${s.namespace}/${s.nombre}`;
export const idIngress = (i: NsNombre) => `ingress:${i.namespace}/${i.nombre}`;
export const idNodo = (nombre: string) => `nodo:${nombre}`;
export const idWorkload = (w: Pick<Workload, 'id'>) => `workload:${w.id}`;
export const idPuerta = (p: Pick<Puerta, 'nodo' | 'puerto' | 'protocolo'>) =>
  `puerta:${p.nodo}/${p.puerto}` + (p.protocolo !== 'TCP' ? `/${p.protocolo}` : '');

/** Parte un id de la foto `<namespace>/<nombre>` (el de un pod, Service o Ingress). */
export function nsNombre(idEnFoto: string): NsNombre {
  const corte = idEnFoto.indexOf('/');
  return { namespace: idEnFoto.slice(0, corte), nombre: idEnFoto.slice(corte + 1) };
}

/** id de escena de un pod a partir de su id en la foto (`<namespace>/<nombre>`). */
export const idPodDeLaFoto = (idEnFoto: string) => idPod(nsNombre(idEnFoto));

/** Parte un id de escena en su tipo (`pod`, `nodo`, `workload`…) y el resto (el id en la foto). */
export function partirId(id: string): { tipo: string; resto: string } {
  const corte = id.indexOf(':');
  return { tipo: id.slice(0, corte), resto: id.slice(corte + 1) };
}
