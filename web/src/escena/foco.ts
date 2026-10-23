// Foco en la vista de topología: el Service, pod, Ingress o puerta que el usuario apunta con el mouse o
// selecciona. Sus relacionados se calculan sobre la escena ya ubicada (conexiones y destinos de
// puertas), así solo cuentan los objetos que se ven.
import type { ConexionEnEscena, Escena, EstadoInterfaz, MarcasDeFoco, TonoConexion } from './escena';
import { partirId } from './ids';
import { hayRecorrido } from './recorrido';

/** Opacidad de los objetos sin relación con el foco mientras hay un objeto apuntado. */
export const OPACIDAD_ACLARADO = 0.45;

/** Solo estos objetos toman foco: seleccionar un nodo o un workload no es foco. */
const TIPOS_CON_FOCO = ['service', 'pod', 'ingress', 'puerta'];

const tipoDe = (id: string) => partirId(id).tipo;

/**
 * El foco: el objeto apuntado manda sobre la selección. Un id que no está en la escena no es foco. Mientras
 * hay un recorrido no hay foco (un clic solo abre el panel de detalle).
 */
export function focoDe(escena: Escena, estado: EstadoInterfaz): { foco: string | null; porApuntado: boolean } {
  if (hayRecorrido(estado)) return { foco: null, porApuntado: false };
  const enEscena = (id: string | null | undefined): id is string =>
    !!id &&
    TIPOS_CON_FOCO.includes(tipoDe(id)) &&
    [escena.pods, escena.services, escena.ingress, escena.puertas].some((l) => l.some((o) => o.id === id));
  if (enEscena(estado.apuntado)) return { foco: estado.apuntado, porApuntado: true };
  if (enEscena(estado.seleccionado)) return { foco: estado.seleccionado, porApuntado: false };
  return { foco: null, porApuntado: false };
}

/**
 * Relacionados con el foco (incluye al foco):
 * - Service: su Ingress, sus endpoints listos y no listos y sus puertas NodePort;
 * - pod: sus Services y los Ingress y puertas de esos Services, y sus puertas hostPort;
 * - Ingress: sus Services y lo que cada uno arrastra;
 * - puerta: su Service (con lo que arrastra), o su pod si es hostPort.
 */
export function relacionadosCon(escena: Escena, foco: string | null): Set<string> {
  const rel = new Set<string>();
  if (!foco) return rel;
  rel.add(foco);
  const { conexiones, puertas } = escena;
  const conService = (id: string, conPods = true) => {
    rel.add(id);
    for (const c of conexiones) {
      if (conPods && c.desde === id && tipoDe(c.hasta) === 'pod') rel.add(c.hasta);
      if (c.hasta === id && tipoDe(c.desde) === 'ingress') rel.add(c.desde);
    }
    for (const p of puertas) if (p.destino === id) rel.add(p.id);
  };
  const tipo = tipoDe(foco);
  if (tipo === 'service') conService(foco);
  if (tipo === 'ingress') for (const c of conexiones) if (c.desde === foco) conService(c.hasta);
  if (tipo === 'pod') {
    for (const c of conexiones) if (c.hasta === foco && tipoDe(c.desde) === 'service') conService(c.desde, false);
    for (const p of puertas) if (p.destino === foco) rel.add(p.id);
  }
  if (tipo === 'puerta') {
    const destino = puertas.find((p) => p.id === foco)!.destino;
    if (tipoDe(destino) === 'service') conService(destino);
    else rel.add(destino);
  }
  return rel;
}

/**
 * Las líneas de puerta solo aparecen con foco o durante un recorrido, cuando la puerta y su destino
 * están en `rel` (tres puertas al mismo Service llenarían el diagrama de líneas). NodePort: de arriba de la
 * puerta al borde de abajo del Service, corrida a la derecha del centro para no pisar las otras
 * líneas. hostPort: de abajo de la puerta a arriba de su pod.
 */
export function lineasDePuerta(escena: Escena, rel: Set<string>, tono: TonoConexion = 'foco'): ConexionEnEscena[] {
  const cajas = new Map([...escena.services, ...escena.pods].map((c) => [c.id, c]));
  return escena.puertas.flatMap((p) => {
    const destino = cajas.get(p.destino);
    if (!destino || !rel.has(p.id) || !rel.has(p.destino)) return [];
    const haciaService = tipoDe(p.destino) === 'service';
    return [
      {
        id: `${p.id}->${p.destino}`,
        desde: p.id,
        hasta: p.destino,
        tono,
        dePuerta: true,
        inicio: { x: p.x + p.ancho / 2, y: haciaService ? p.y : p.y + p.alto },
        fin: haciaService
          ? { x: destino.x + destino.ancho / 2 + DESVIO_LINEA_DE_PUERTA, y: destino.y + destino.alto }
          : { x: destino.x + destino.ancho / 2, y: destino.y },
      },
    ];
  });
}

/** Cuánto se corre a la derecha del centro del Service la punta de una línea de puerta. */
const DESVIO_LINEA_DE_PUERTA = 30;

/**
 * Aplica el foco a la escena: el conjunto de relacionados y el tono de cada conexión, con esta
 * prioridad (el recorrido se suma arriba de todo):
 *   foco → `foco` si une dos relacionados, si no `fantasma`;
 *   workload seleccionado → la de sus pods queda en reposo, las demás `fantasma`;
 *   si no, el tono de reposo (`reposo` o `no-listo`).
 *
 * `podsDelWorkload`: ids de escena de los pods del workload seleccionado, o null sin selección.
 */
export function aplicarFoco(escena: Escena, estado: EstadoInterfaz, podsDelWorkload: ReadonlySet<string> | null): Escena {
  const { foco, porApuntado } = focoDe(escena, estado);
  const rel = relacionadosCon(escena, foco);
  const tono = (c: ConexionEnEscena): TonoConexion => {
    if (foco) return rel.has(c.desde) && rel.has(c.hasta) ? 'foco' : 'fantasma';
    if (podsDelWorkload) return podsDelWorkload.has(c.hasta) ? c.tono : 'fantasma';
    return c.tono;
  };
  // al apuntar se aclara lo no relacionado; la opacidad que ya traía (workload, Terminating) se respeta
  const marcar = <T extends { id: string } & MarcasDeFoco>(o: T, marcado: boolean): T => ({
    ...o,
    marcado,
    opacidad: porApuntado && !rel.has(o.id) ? Math.min(o.opacidad, OPACIDAD_ACLARADO) : o.opacidad,
  });
  const caja = <T extends { id: string } & MarcasDeFoco>(o: T) => marcar(o, o.id === foco || (porApuntado && rel.has(o.id)));
  return {
    ...escena,
    pods: escena.pods.map(caja),
    services: escena.services.map(caja),
    ingress: escena.ingress.map(caja),
    puertas: escena.puertas.map((p) => marcar(p, rel.has(p.id))),
    conexiones: [...lineasDePuerta(escena, rel), ...escena.conexiones.map((c) => ({ ...c, tono: tono(c) }))],
    foco,
    relacionados: [...rel].sort(),
  };
}
