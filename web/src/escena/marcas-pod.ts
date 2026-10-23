// Marcas por pod en la tarjeta (parte de la escena): el estado si no es Running, los reinicios si
// hay y los containers como chips. Lo que no es normal resalta; lo normal no ensucia.
import type { Container, Pod, TipoContainer } from '../foto';

interface Rect {
  x: number;
  y: number;
  ancho: number;
  alto: number;
}

export interface MarcasPod {
  /** Nombre recortado (con "…") para no pisarse con el estado. Sin namespace. */
  nombreVisible: string;
  /** Estado al estilo kubectl get pods, o null si es Running: nunca se escribe "Running". */
  estado: string | null;
  /** Color del estado: mal (falla), aviso (espera o no está listo), terminado (ya no corre). */
  tonoEstado: TonoEstado | null;
  /** Reinicios de todos sus containers, o null si nunca se reinició. */
  reinicios: number | null;
  /** true si los reinicios aumentaron respecto de la foto anterior: el contador titila. */
  reiniciosTitilan: boolean;
  /** Chips de los containers en orden de vida (init, normales, ephemeral), en la última fila. */
  containers: ChipContainer[];
}

export type TonoEstado = 'mal' | 'aviso' | 'terminado';

export interface ChipContainer extends Rect {
  /** Nombre del container, o "" en el chip "+N" de los que no entran. */
  nombre: string;
  /** normal (lleno), init (punteado) o ephemeral de kubectl debug (rayado); null en el chip "+N". */
  tipo: TipoContainer | null;
  /** ":<primer puerto>", o los primeros 7 caracteres del nombre si no expone puertos. */
  texto: string;
  /** Un container normal que no está Ready. */
  noListo: boolean;
  /** Texto de ayuda al pasar el mouse. */
  detalle: string;
}

// Medidas de la tarjeta. La primera fila se reparte entre el nombre (izquierda) y el estado
// (derecha); la última, entre los chips (izquierda) y el contador de reinicios (derecha). El ancho
// por letra es una estimación de las fuentes del lienzo.
export const TARJETA = {
  margenIzquierdo: 13,
  margenDerecho: 8,
  separacion: 10,
  letraNombre: 6.4,
  letraEstado: 6.3,
  chip: { ancho: 45, alto: 14, hueco: 5, y: 41 },
  lugarReinicios: 40,
} as const;

/** `anterior` es el mismo pod en la foto anterior, si estaba. */
export function marcasDePod(p: Pod, tarjeta: Rect, anterior?: Pod): MarcasPod {
  const T = TARJETA;
  const estado = p.estado === 'Running' ? null : p.estado;
  const anchoEstado = estado ? estado.length * T.letraEstado + T.separacion : 0;
  const lugarNombre = tarjeta.ancho - T.margenIzquierdo - T.margenDerecho - anchoEstado;
  const reinicios = reiniciosDe(p);
  return {
    nombreVisible: recortar(p.nombre, Math.floor(lugarNombre / T.letraNombre)),
    estado,
    tonoEstado: estado ? tono(estado) : null,
    reinicios: reinicios || null,
    reiniciosTitilan: anterior !== undefined && reinicios > reiniciosDe(anterior),
    containers: chips(p.containers, tarjeta, reinicios > 0),
  };
}

function chips(containers: Container[], tarjeta: Rect, conReinicios: boolean): ChipContainer[] {
  const { chip, margenIzquierdo, margenDerecho, lugarReinicios } = TARJETA;
  const lugar = tarjeta.ancho - margenIzquierdo - margenDerecho - (conReinicios ? lugarReinicios : 0);
  const entran = Math.max(1, Math.floor((lugar + chip.hueco) / (chip.ancho + chip.hueco)));
  const visibles = containers.length > entran ? containers.slice(0, entran - 1) : containers;

  const caja = (i: number): Rect => ({
    x: tarjeta.x + margenIzquierdo + i * (chip.ancho + chip.hueco),
    y: tarjeta.y + chip.y,
    ancho: chip.ancho,
    alto: chip.alto,
  });
  const resultado: ChipContainer[] = visibles.map((c, i) => ({
    ...caja(i),
    nombre: c.nombre,
    tipo: c.tipo,
    texto: c.puertos.length > 0 ? `:${c.puertos[0].numero}` : c.nombre.slice(0, 7),
    noListo: noListo(c),
    detalle: detalle(c),
  }));
  const faltan = containers.slice(visibles.length);
  if (faltan.length > 0) {
    resultado.push({
      ...caja(visibles.length),
      nombre: '',
      tipo: null,
      texto: `+${faltan.length}`,
      noListo: faltan.some((c) => noListo(c)),
      detalle: faltan.map(detalle).join('\n'),
    });
  }
  return resultado;
}

// un container normal que no está Ready, salvo que haya terminado bien (Completed)
const noListo = (c: Container) => c.tipo === 'normal' && !c.ready && c.estado !== 'Completed';

const NOMBRE_TIPO: Record<TipoContainer, string> = {
  normal: 'container',
  init: 'init container',
  ephemeral: 'ephemeral container (kubectl debug)',
};

function detalle(c: Container): string {
  const puertos = c.puertos.map((p) => `${p.numero}/${p.protocolo}`).join(', ');
  return [`${NOMBRE_TIPO[c.tipo]} ${c.nombre}`, c.imagen, puertos, c.estado].filter(Boolean).join(' · ');
}

const TERMINADO = new Set(['Completed', 'Succeeded', 'Terminating']);
// motivos de kubelet que dicen que algo falló (BackOff, ErrImagePull, Error, OOMKilled…)
const FALLA = /BackOff|Err|Error|Failed|OOMKilled|ExitCode|Signal|Evicted|Unknown/;

function tono(estado: string): TonoEstado {
  if (TERMINADO.has(estado)) return 'terminado';
  if (FALLA.test(estado)) return 'mal';
  return 'aviso';
}

const reiniciosDe = (p: Pod) => p.containers.reduce((n, c) => n + c.reinicios, 0);

const recortar = (texto: string, letras: number) =>
  texto.length > letras ? texto.slice(0, Math.max(1, letras - 1)) + '…' : texto;
