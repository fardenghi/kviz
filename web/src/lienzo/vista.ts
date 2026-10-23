// Cuentas de la cámara del lienzo: encuadre, zoom con la rueda y arrastre.

/** Transformación de la escena al lienzo: punto en pantalla = punto en escena × escala + (x, y). */
export interface Vista {
  x: number;
  y: number;
  escala: number;
}

export interface Tamano {
  ancho: number;
  alto: number;
}

export const ESCALA_MIN = 0.2;
export const ESCALA_MAX = 3;
const ESCALA_MAX_ENCUADRE = 1.15;

const limitar = (s: number) => Math.min(ESCALA_MAX, Math.max(ESCALA_MIN, s));

/** Vista que muestra toda la escena centrada en el lienzo, sin agrandarla de más. */
export function encuadrar(escena: Tamano, lienzo: Tamano, margen = 32): Vista {
  if (escena.ancho <= 0 || escena.alto <= 0) return { x: margen, y: margen, escala: 1 };
  const escala = limitar(
    Math.min((lienzo.ancho - 2 * margen) / escena.ancho, (lienzo.alto - 2 * margen) / escena.alto, ESCALA_MAX_ENCUADRE),
  );
  return {
    escala,
    x: (lienzo.ancho - escena.ancho * escala) / 2,
    y: Math.max(margen, (lienzo.alto - escena.alto * escala) / 2),
  };
}

/** Multiplica la escala por factor dejando quieto el punto (px, py) del lienzo. */
export function zoomEn(v: Vista, factor: number, px: number, py: number): Vista {
  const escala = limitar(v.escala * factor);
  const k = escala / v.escala;
  return { escala, x: px - (px - v.x) * k, y: py - (py - v.y) * k };
}
