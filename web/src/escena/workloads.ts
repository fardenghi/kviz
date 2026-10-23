// Workloads en la escena: la marca de workload de cada pod (barra y etiqueta corta en su color)
// y lo que cambia al seleccionar un workload. Seleccionar un workload no es foco: solo resalta
// sus pods con el borde de su color y atenúa el resto, sin halo.
import type { Foto, Pod, TipoDeWorkload, Workload } from '../foto';

export interface MarcaDeWorkload {
  /** Etiqueta corta: `deploy backend`, `sts db`, `ds log-agent`, `cronjob backup` o `Pod suelto`. */
  etiqueta: string;
  /** Color de la barra y la etiqueta. */
  color: string;
}

/** Opacidad de los pods que no son del workload seleccionado. */
export const OPACIDAD_ATENUADO = 0.18;

export const COLOR_POD_SUELTO = '#9aa3ad';

const CORTO: Record<TipoDeWorkload, string> = {
  Deployment: 'deploy',
  ReplicaSet: 'rs',
  StatefulSet: 'sts',
  DaemonSet: 'ds',
  Job: 'job',
  CronJob: 'cronjob',
};

export const etiquetaDeWorkload = (w: Pick<Workload, 'tipo' | 'nombre'>) => `${CORTO[w.tipo] ?? w.tipo} ${w.nombre}`;

export function indiceDeWorkloads(foto: Foto): Map<string, Workload> {
  return new Map((foto.workloads ?? []).map((w) => [w.id, w]));
}

export function marcaDeWorkload(p: Pod, workloads: Map<string, Workload>): MarcaDeWorkload {
  const w = p.workload ? workloads.get(p.workload) : undefined;
  return w ? { etiqueta: etiquetaDeWorkload(w), color: w.color } : { etiqueta: 'Pod suelto', color: COLOR_POD_SUELTO };
}

/**
 * Cómo se ve un pod según el workload seleccionado: los suyos con el borde del color del
 * workload; los demás atenuados. Sin workload seleccionado (o si ya no existe), todo normal.
 */
export function seleccionDeWorkload(
  p: Pod,
  seleccionado: Workload | undefined,
): { bordeWorkload: string | null; opacidad: number } {
  if (!seleccionado) return { bordeWorkload: null, opacidad: 1 };
  return p.workload === seleccionado.id
    ? { bordeWorkload: seleccionado.color, opacidad: 1 }
    : { bordeWorkload: null, opacidad: OPACIDAD_ATENUADO };
}

/**
 * Texto de la cadena de dueños para la lista de workloads: un resumen
 * (`Deployment → ReplicaSet → Pod ×N`, con `ReplicaSet ×2` durante un rollout) y una línea por
 * cada ReplicaSet o Job del medio con sus pods.
 */
export function textoDeCadena(w: Workload): { resumen: string; eslabones: string[] } {
  const [arriba, ...medio] = w.cadena;
  const pods = w.cadena.reduce((n, e) => n + e.pods.length, 0);
  const partes: string[] = [arriba?.tipo ?? w.tipo];
  if (medio.length > 0) partes.push(medio.length > 1 ? `${medio[0].tipo} ×${medio.length}` : medio[0].tipo);
  partes.push(`Pod ×${pods}`);
  return {
    resumen: partes.join(' → '),
    eslabones: medio.map((e) => `${e.tipo} ${e.nombre} · Pod ×${e.pods.length}`),
  };
}
