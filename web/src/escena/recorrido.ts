// Recorrido de una request de prueba en la vista de topología. Va arriba de todo en la prioridad
// (recorrido > foco > workload > reposo): mientras hay un recorrido el foco está apagado (ver
// focoDe), lo que no participa se atenúa al 18 %, cada objeto lleva los números de sus pasos y las
// conexiones entre objetos del recorrido toman el tono `recorrido` (rosa, discontinuas y animadas).
import type { Foto } from '../foto';
import type { Recorrido, RefObjeto } from '../prueba';
import type { Escena, EstadoInterfaz, MarcasDeFoco, MarcasDeRecorrido } from './escena';
import { idIngress, idPod, idPuerta, idService, OPACIDAD_TERMINANDO } from './escena';
import { lineasDePuerta } from './foco';
import { OPACIDAD_ATENUADO } from './workloads';

/** Hay un recorrido que dibujar: uno con pasos (el de un destino fuera de la foto no tiene). */
export const hayRecorrido = (estado: EstadoInterfaz): boolean => (estado.recorrido?.pasos.length ?? 0) > 0;

/** id de escena del objeto de la foto que marca un paso; null si no es algo que la escena dibuja. */
export function idEnEscena(foto: Foto, ref: RefObjeto): string | null {
  const nsNombre = (id: string) => {
    const corte = id.indexOf('/');
    return { namespace: id.slice(0, corte), nombre: id.slice(corte + 1) };
  };
  switch (ref.tipo) {
    case 'service':
      return idService(nsNombre(ref.id));
    case 'pod':
      return idPod(nsNombre(ref.id));
    case 'ingress':
      return idIngress(nsNombre(ref.id));
    case 'puerta': {
      const p = foto.puertas.find((p) => p.id === ref.id);
      return p ? idPuerta(p) : null;
    }
    case 'nodo':
      return `nodo:${ref.id}`;
  }
  return null;
}

/** Números de paso de cada objeto (id de escena), en orden y sin repetir. */
export function pasosPorObjeto(foto: Foto, recorrido: Recorrido): Map<string, number[]> {
  const pasos = new Map<string, number[]>();
  for (const paso of recorrido.pasos) {
    for (const ref of paso.objetos) {
      const id = idEnEscena(foto, ref);
      if (!id) continue;
      const suyos = pasos.get(id) ?? [];
      if (!suyos.includes(paso.n)) pasos.set(id, [...suyos, paso.n].sort((a, b) => a - b));
    }
  }
  return pasos;
}

/**
 * Aplica el recorrido a la escena ya armada (y con el foco apagado). Sin recorrido, solo deja los
 * pasos vacíos.
 */
export function aplicarRecorrido(escena: Escena, foto: Foto, estado: EstadoInterfaz): Escena {
  if (!hayRecorrido(estado)) return escena;
  const pasos = pasosPorObjeto(foto, estado.recorrido!);
  const participa = (id: string) => pasos.has(id);
  const marcar = <T extends { id: string } & MarcasDeFoco & MarcasDeRecorrido>(o: T, entero = 1): T => ({
    ...o,
    pasos: pasos.get(o.id) ?? [],
    opacidad: participa(o.id) ? entero : OPACIDAD_ATENUADO,
  });
  const delRecorrido = new Set(pasos.keys());
  return {
    ...escena,
    pods: escena.pods.map((p) => marcar(p, p.terminando ? OPACIDAD_TERMINANDO : 1)),
    services: escena.services.map((s) => marcar(s)),
    ingress: escena.ingress.map((i) => marcar(i)),
    puertas: escena.puertas.map((p) => marcar(p)),
    conexiones: [
      ...lineasDePuerta(escena, delRecorrido, 'recorrido'),
      ...escena.conexiones
        .filter((c) => !c.dePuerta)
        .map((c) => ({ ...c, tono: participa(c.desde) && participa(c.hasta) ? ('recorrido' as const) : ('fantasma' as const) })),
    ],
  };
}
