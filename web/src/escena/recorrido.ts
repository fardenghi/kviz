// Recorrido de una request de prueba en la vista de topología. Va arriba de todo en la prioridad
// (recorrido > foco > workload > reposo): mientras hay un recorrido el foco está apagado (ver
// focoDe), lo que no participa se atenúa al 18 %, cada objeto lleva los números de sus pasos y las
// conexiones entre objetos del recorrido toman el tono `recorrido` (rosa, discontinuas y animadas).
// El salto entre nodos es un arco por debajo de las regiones. Con un paso elegido (clic en la
// tarjeta), todo eso vale solo para ese paso.
import type { Foto } from '../foto';
import type { Paso, Recorrido, RefObjeto } from '../prueba';
import type { Caja, Escena, EstadoInterfaz, MarcasDeFoco, MarcasDeRecorrido, Punto, SaltoEnEscena } from './escena';
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

/**
 * ids de escena de los pods del Ingress Controller por los que pasa el recorrido (los que marca el
 * paso de la regla de Ingress): se ven aunque los namespaces del sistema estén ocultos.
 */
export function podsDelIngressController(recorrido: Recorrido | null | undefined): string[] {
  return (recorrido?.pasos ?? [])
    .filter((p) => p.tipo === 'ingress')
    .flatMap((p) => p.objetos.filter((o) => o.tipo === 'pod'))
    .map((o) => {
      const corte = o.id.indexOf('/');
      return idPod({ namespace: o.id.slice(0, corte), nombre: o.id.slice(corte + 1) });
    });
}

/** Los pasos que se resaltan: el elegido, si es uno del recorrido, o todos. */
const pasosVisibles = (recorrido: Recorrido, elegido: number | null | undefined): Paso[] => {
  const solo = recorrido.pasos.filter((p) => p.n === elegido);
  return solo.length > 0 ? solo : recorrido.pasos;
};

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
  const recorrido = estado.recorrido!;
  const visibles = pasosVisibles(recorrido, estado.pasoElegido);
  const pasos = pasosPorObjeto(foto, { pasos: visibles });
  const participa = (id: string) => pasos.has(id);
  const marcar = <T extends { id: string } & MarcasDeFoco & MarcasDeRecorrido>(o: T, entero = 1): T => ({
    ...o,
    pasos: pasos.get(o.id) ?? [],
    opacidad: participa(o.id) ? entero : OPACIDAD_ATENUADO,
  });
  const delRecorrido = new Set(pasos.keys());
  const saltos = saltosDe(escena, foto, recorrido).map((s) => ({
    ...s,
    opacidad: visibles.some((p) => p.n === s.paso) ? 1 : OPACIDAD_ATENUADO,
  }));
  return {
    ...escena,
    alto: Math.max(escena.alto, ...saltos.map((s) => s.etiquetaEn.y + MARGEN_DEBAJO_DEL_SALTO)),
    regiones: escena.regiones.map((r) => ({ ...r, pasos: pasos.get(r.id) ?? [] })),
    saltos,
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

/** Cuánto baja el punto más bajo del arco de salto por debajo de la región más baja. */
const BAJADA_DEL_SALTO = 40;
/** Lugar para la etiqueta debajo del punto más bajo del arco. */
const MARGEN_DEBAJO_DEL_SALTO = 24;

/**
 * Un arco por cada paso de salto (`objetos`: el nodo de donde sale y el nodo al que llega). Sale
 * de lo que manda la request en el nodo de salida (el pod del paso de Ingress, o la puerta del
 * paso de puerta) y llega al pod del paso pod; si no están en la escena, de la región misma.
 */
function saltosDe(escena: Escena, foto: Foto, recorrido: Recorrido): SaltoEnEscena[] {
  const fondo = Math.max(0, ...escena.regiones.map((r) => r.y + r.alto)) + BAJADA_DEL_SALTO;
  const regiones = new Map(escena.regiones.map((r) => [r.id, r]));
  const cajas = new Map<string, Caja & { region: string }>([...escena.pods, ...escena.puertas].map((c) => [c.id, c]));
  const abajo = (c: Caja): Punto => ({ x: c.x + c.ancho / 2, y: c.y + c.alto });
  /** La caja que marca un paso del tipo dado dentro de la región. */
  const delPaso = (tipo: string, region: string) =>
    recorrido.pasos
      .filter((p) => p.tipo === tipo)
      .flatMap((p) => p.objetos.map((o) => idEnEscena(foto, o)))
      .map((id) => (id ? cajas.get(id) : undefined))
      .find((c) => c?.region === region);

  return recorrido.pasos.flatMap((paso) => {
    if (paso.tipo !== 'salto' || paso.objetos.length < 2) return [];
    const [de, a] = paso.objetos.map((o) => regiones.get(`nodo:${o.id}`));
    if (!de || !a) return [];
    const desde = abajo(delPaso('ingress', de.id) ?? delPaso('puerta', de.id) ?? de);
    const hasta = abajo(delPaso('pod', a.id) ?? a);
    // cúbica con los dos controles a yControl: en t = 0.5 pasa por (x1 + x2)/2, (y1 + y2)/8 + 3/4 · yControl
    const yControl = (fondo - (desde.y + hasta.y) / 8) / (3 / 4);
    const [nodoDe, nodoA] = paso.objetos.map((o) => o.id);
    return [
      {
        id: `salto:${paso.n}`,
        paso: paso.n,
        desde,
        hasta,
        yControl,
        etiqueta: `salto ${nodoDe} → ${nodoA}`,
        etiquetaEn: { x: (desde.x + hasta.x) / 2, y: fondo },
        opacidad: 1,
      },
    ];
  });
}
