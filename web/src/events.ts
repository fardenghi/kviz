// Events del cluster en la interfaz: la franja de abajo con los últimos events (con los mismos
// filtros de namespace que la vista), el objeto que se selecciona al hacer clic en uno, y los
// events del objeto que muestra el panel de detalle. Funciones puras.
import { type Detalle, fichaDe } from './detalle/detalle';
import { idDeObjetoEnFoto } from './escena/ids';
import { type EstadoInterfaz, seVe } from './escena/escena';
import type { EventDelCluster, Foto } from './foto';

/** Cuántos events muestra la franja: los más recientes. */
export const EVENTS_EN_LA_FRANJA = 50;

/** Del más reciente al más viejo; a igual hora, el último en llegar primero. */
const recientesPrimero = (a: EventDelCluster, b: EventDelCluster) =>
  a.ultimaVez !== b.ultimaVez ? (a.ultimaVez < b.ultimaVez ? 1 : -1) : a.id < b.id ? 1 : -1;

/**
 * Los últimos events del cluster para la franja, del más reciente al más viejo, con los filtros
 * de namespace de la vista. Los de un objeto sin namespace (un nodo) se ven siempre, como los nodos.
 */
export function eventsDeLaFranja(
  foto: Foto,
  estado: Pick<EstadoInterfaz, 'mostrarSistema' | 'namespace'> = {},
): EventDelCluster[] {
  return (foto.events ?? [])
    .filter((e) => e.objeto.namespace === '' || seVe(e.objeto.namespace, estado))
    .sort(recientesPrimero)
    .slice(0, EVENTS_EN_LA_FRANJA);
}

/**
 * El objeto que selecciona un clic en el event (id de selección, el mismo del foco y el panel de
 * detalle), o null si el objeto no está en la foto.
 */
export function seleccionDeEvent(e: EventDelCluster): string | null {
  return e.enFoto ? idDeObjetoEnFoto(e.enFoto) : null;
}

/** Los events del objeto del panel de detalle, del más reciente al más viejo. */
export function eventsDelObjeto(foto: Foto, d: Detalle): EventDelCluster[] {
  const o = fichaDe(d).enFoto;
  return (foto.events ?? []).filter((e) => e.enFoto?.seccion === o.seccion && e.enFoto.id === o.id).sort(recientesPrimero);
}

/** Hora local de un event (`15:10:31`); "" si no se sabe. */
export function horaDeEvent(e: EventDelCluster): string {
  if (!e.ultimaVez) return '';
  return new Date(e.ultimaVez).toLocaleTimeString('es-AR', { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false });
}
