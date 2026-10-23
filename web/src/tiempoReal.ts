// Tiempo real: arma la foto con los mensajes del canal de streaming (GET /api/stream) y recuerda
// qué pods acaban de aparecer, para que brillen unas pocas veces. Es pura: el tiempo entra como
// argumento.
import { idPod } from './escena/escena';
import type { Foto } from './foto';

/** Un mensaje del stream. Espejo de internal/api/stream.go; el canal admite más tipos. */
export type Mensaje =
  /** La foto completa: llega al conectarse (y al reconectarse). */
  | { tipo: 'foto'; foto: Foto }
  /** Lo que cambió de la foto desde el mensaje anterior. */
  | { tipo: 'cambios'; cambios: Cambio[] };

/**
 * Un cambio de la foto. Espejo de internal/foto/cambios.go: en una sección que es lista de objetos
 * con `id`, el cambio es de ese objeto (alta o modificación con `valor`, baja con `borrado`); sin
 * `id`, reemplaza (o quita) la sección entera.
 */
export interface Cambio {
  seccion: string;
  id?: string;
  valor?: unknown;
  borrado?: boolean;
}

/** Cuánto brilla un pod nuevo: 3 veces y nunca para siempre. */
export const BRILLO_POD_NUEVO = { veces: 3, msPorVez: 1400 } as const;
const DURACION_BRILLO = BRILLO_POD_NUEVO.veces * BRILLO_POD_NUEVO.msPorVez;

export interface EnVivo {
  /** null hasta que llega la primera foto. */
  foto: Foto | null;
  /** La foto de antes del último mensaje, para marcar lo que cambió (reinicios que aumentaron). */
  anterior: Foto | null;
  /** Pods que aparecieron por una actualización (id de escena → instante en ms). */
  nuevos: ReadonlyMap<string, number>;
}

export const EN_VIVO_INICIAL: EnVivo = { foto: null, anterior: null, nuevos: new Map() };

/** Aplica un mensaje del stream. `ahora` es el instante en ms (Date.now()). */
export function recibir(estado: EnVivo, m: Mensaje, ahora: number): EnVivo {
  switch (m.tipo) {
    case 'foto':
      // una foto completa no es "lo que apareció": al cargar o reconectar no brilla nada
      return { foto: m.foto, anterior: estado.foto, nuevos: new Map() };
    case 'cambios': {
      if (!estado.foto) return estado;
      const foto = aplicarCambios(estado.foto, m.cambios);
      const antes = new Set(estado.foto.pods.map(idPod));
      const nuevos = new Map([...estado.nuevos].filter(([, desde]) => ahora - desde < DURACION_BRILLO));
      for (const p of foto.pods) if (!antes.has(idPod(p))) nuevos.set(idPod(p), ahora);
      return { foto, anterior: estado.foto, nuevos };
    }
    default:
      return estado; // un tipo de mensaje que esta versión no conoce
  }
}

/** Los pods que todavía están brillando en el instante `ahora`. */
export function podsNuevos(estado: EnVivo, ahora: number): Set<string> {
  return new Set([...estado.nuevos].filter(([, desde]) => ahora - desde < DURACION_BRILLO).map(([id]) => id));
}

/** Cuándo deja de brillar el último pod nuevo (ms), o null si no brilla ninguno. */
export function finDelBrillo(estado: EnVivo, ahora: number): number | null {
  const fines = [...estado.nuevos.values()].map((desde) => desde + DURACION_BRILLO).filter((fin) => fin > ahora);
  return fines.length ? Math.max(...fines) : null;
}

type ConID = { id: string };

/** Aplica los cambios a la foto sin saber nada de cada sección (las listas quedan ordenadas por id). */
export function aplicarCambios(foto: Foto, cambios: Cambio[]): Foto {
  const f: Record<string, unknown> = { ...foto };
  for (const c of cambios) {
    if (c.id === undefined) {
      if (c.borrado) delete f[c.seccion];
      else f[c.seccion] = c.valor;
      continue;
    }
    const lista = ((f[c.seccion] as ConID[] | undefined) ?? []).filter((o) => o.id !== c.id);
    if (!c.borrado) {
      lista.push(c.valor as ConID);
      lista.sort((a, b) => (a.id < b.id ? -1 : a.id > b.id ? 1 : 0));
    }
    f[c.seccion] = lista;
  }
  return f as unknown as Foto;
}
