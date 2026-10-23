// Lienzo (módulo 2 del frontend): dibuja la escena en SVG, con zoom por rueda, arrastre y botón de
// encuadrar. No decide nada sobre el cluster: todo lo que dibuja viene de la escena.
import { useCallback, useEffect, useRef, useState } from 'react';
import type { Escena, PodEnEscena, RegionEnEscena } from '../escena/escena';
import { encuadrar, type Vista, zoomEn } from './vista';

export function Lienzo({ escena }: { escena: Escena }) {
  const marco = useRef<HTMLDivElement>(null);
  const [vista, setVista] = useState<Vista>({ x: 0, y: 0, escala: 1 });
  const arrastre = useRef<{ x: number; y: number; desde: Vista } | null>(null);

  const alEncuadrar = useCallback(() => {
    const el = marco.current;
    if (!el) return;
    setVista(encuadrar(escena, { ancho: el.clientWidth, alto: el.clientHeight }));
  }, [escena.ancho, escena.alto]); // eslint-disable-line react-hooks/exhaustive-deps

  // encuadra solo al aparecer la escena o cuando cambia de tamaño
  useEffect(alEncuadrar, [alEncuadrar]);

  // la rueda se escucha a mano para poder cancelar el scroll de la página (passive: false)
  useEffect(() => {
    const el = marco.current;
    if (!el) return;
    const alGirar = (e: WheelEvent) => {
      e.preventDefault();
      const r = el.getBoundingClientRect();
      setVista((v) => zoomEn(v, e.deltaY < 0 ? 1.1 : 1 / 1.1, e.clientX - r.left, e.clientY - r.top));
    };
    el.addEventListener('wheel', alGirar, { passive: false });
    return () => el.removeEventListener('wheel', alGirar);
  }, []);

  const zoomCentro = (factor: number) => {
    const el = marco.current;
    if (el) setVista((v) => zoomEn(v, factor, el.clientWidth / 2, el.clientHeight / 2));
  };

  return (
    <div
      className="lienzo"
      ref={marco}
      onMouseDown={(e) => {
        if ((e.target as Element).closest('.flotante')) return;
        arrastre.current = { x: e.clientX, y: e.clientY, desde: vista };
      }}
      onMouseMove={(e) => {
        const a = arrastre.current;
        if (a) setVista({ ...a.desde, x: a.desde.x + e.clientX - a.x, y: a.desde.y + e.clientY - a.y });
      }}
      onMouseUp={() => (arrastre.current = null)}
      onMouseLeave={() => (arrastre.current = null)}
    >
      <svg className="lienzo-svg">
        <g transform={`translate(${vista.x},${vista.y}) scale(${vista.escala})`}>
          {escena.regiones.map((r) => (
            <Region key={r.id} region={r} />
          ))}
          {escena.pods.map((p) => (
            <TarjetaPod key={p.id} pod={p} />
          ))}
        </g>
      </svg>

      <div className="flotante zoom">
        <button className="boton" title="Acercar" onClick={() => zoomCentro(1.2)}>+</button>
        <button className="boton" title="Alejar" onClick={() => zoomCentro(1 / 1.2)}>−</button>
        <button className="boton" title="Mostrar todo el cluster" onClick={alEncuadrar}>Encuadrar</button>
      </div>
    </div>
  );
}

function Region({ region: r }: { region: RegionEnEscena }) {
  return (
    <g>
      <rect x={r.x} y={r.y} width={r.ancho} height={r.alto} rx={14} className={r.nodo ? 'region' : 'region region-sin-nodo'} />
      <text x={r.x + 16} y={r.y + 25} className="region-titulo">{r.titulo}</text>
      <text x={r.x + 16} y={r.y + 40} className="mono tenue">{r.subtitulo}</text>
    </g>
  );
}

// recorta el nombre para que entre en la tarjeta (aprox. 6.8 px por letra a 11.5 px, negrita)
const recortar = (texto: string, ancho: number) => {
  const max = Math.floor((ancho - 22) / 6.8);
  return texto.length > max ? texto.slice(0, max - 1) + '…' : texto;
};

function TarjetaPod({ pod: p }: { pod: PodEnEscena }) {
  return (
    <g>
      <rect x={p.x} y={p.y} width={p.ancho} height={p.alto} rx={8} className="pod" />
      <text x={p.x + 13} y={p.y + 20} className="pod-nombre">
        {recortar(p.nombre, p.ancho)}
        <title>{`${p.nombre} · namespace ${p.namespace}`}</title>
      </text>
      <text x={p.x + 13} y={p.y + 36} className="mono tenue">{p.ip ?? 'sin IP'}</text>
    </g>
  );
}
