// Lienzo (módulo 2 del frontend): dibuja la escena en SVG, con zoom por rueda, arrastre y botón de
// encuadrar. No decide nada sobre el cluster: todo lo que dibuja viene de la escena.
import { type ReactNode, useCallback, useEffect, useRef, useState } from 'react';
import type {
  ConexionEnEscena,
  Escena,
  IngressEnEscena,
  PodEnEscena,
  PuertaEnEscena,
  RegionEnEscena,
  ServiceEnEscena,
} from '../escena/escena';
import { BRILLO_POD_NUEVO } from '../tiempoReal';
import { encuadrar, type Vista, zoomEn } from './vista';

export function Lienzo({
  escena,
  alClicEnVacio,
  alApuntar,
  alClicEnObjeto,
  children,
}: {
  escena: Escena;
  /** Clic sobre el fondo, sin arrastrar: limpia la selección. */
  alClicEnVacio?: () => void;
  /** El mouse entra a un objeto (su id de escena) o sale de él (null): el foco por hover. */
  alApuntar?: (id: string | null) => void;
  /** Clic sin arrastrar sobre un objeto (su id de escena; un nodo es su región): abre el panel de detalle. */
  alClicEnObjeto?: (id: string) => void;
  /** Superficies flotantes que van arriba del lienzo. */
  children?: ReactNode;
}) {
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
      // sobre una superficie flotante (el panel de detalle, la lista) la rueda la desplaza a ella
      if ((e.target as Element).closest('.flotante')) return;
      e.preventDefault();
      const r = el.getBoundingClientRect();
      setVista((v) => zoomEn(v, e.deltaY < 0 ? 1.1 : 1 / 1.1, e.clientX - r.left, e.clientY - r.top));
    };
    el.addEventListener('wheel', alGirar, { passive: false });
    return () => el.removeEventListener('wheel', alGirar);
  }, []);

  const apuntar = (id: string): Apuntar => ({
    'data-objeto': id,
    onMouseEnter: () => alApuntar?.(id),
    onMouseLeave: () => alApuntar?.(null),
  });

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
      onMouseUp={(e) => {
        const a = arrastre.current;
        arrastre.current = null;
        const quieto = a && Math.abs(e.clientX - a.x) + Math.abs(e.clientY - a.y) < 4;
        if (!quieto) return;
        const objeto = (e.target as Element).closest<HTMLElement | SVGElement>('[data-objeto]');
        if (objeto) alClicEnObjeto?.(objeto.dataset.objeto!);
        else if (!(e.target as Element).closest('.objeto')) alClicEnVacio?.();
      }}
      onMouseLeave={() => (arrastre.current = null)}
    >
      <svg className="lienzo-svg">
        <defs>
          {/* relleno rayado de los ephemeral containers (kubectl debug) */}
          <pattern id="rayado" width={4} height={4} patternUnits="userSpaceOnUse" patternTransform="rotate(45)">
            <rect width={4} height={4} className="rayado-fondo" />
            <line x1={0} y1={0} x2={0} y2={4} className="rayado-linea" />
          </pattern>
          {/* flecha de las conexiones con foco, del color de la línea */}
          <marker id="flecha" viewBox="0 0 8 8" refX={7} refY={4} markerWidth={9} markerHeight={9} markerUnits="userSpaceOnUse" orient="auto">
            <path d="M0,0 L8,4 L0,8 z" className="flecha" />
          </marker>
        </defs>
        <g transform={`translate(${vista.x},${vista.y}) scale(${vista.escala})`}>
          {escena.regiones.map((r) => (
            <Region key={r.id} region={r} />
          ))}
          {escena.conexiones.map((c) => (
            <Conexion key={c.id} conexion={c} />
          ))}
          {escena.ingress.map((i) => (
            <TarjetaIngress key={i.id} ingress={i} apuntar={apuntar(i.id)} />
          ))}
          {escena.services.map((s) => (
            <TarjetaService key={s.id} service={s} apuntar={apuntar(s.id)} />
          ))}
          {escena.puertas.map((p) => (
            <PestanaPuerta key={p.id} puerta={p} apuntar={apuntar(p.id)} />
          ))}
          {escena.pods.map((p) => (
            <TarjetaPod key={p.id} pod={p} apuntar={apuntar(p.id)} />
          ))}
        </g>
      </svg>

      {children}

      <div className="flotante zoom">
        <button className="boton" title="Acercar" onClick={() => zoomCentro(1.2)}>+</button>
        <button className="boton" title="Alejar" onClick={() => zoomCentro(1 / 1.2)}>−</button>
        <button className="boton" title="Mostrar todo el cluster" onClick={alEncuadrar}>Encuadrar</button>
      </div>
    </div>
  );
}

/** Handlers del foco por hover de un objeto. */
type Apuntar = { 'data-objeto': string; onMouseEnter: () => void; onMouseLeave: () => void };

const clases = (...c: (string | false)[]) => c.filter(Boolean).join(' ');

function Region({ region: r }: { region: RegionEnEscena }) {
  // la región de un nodo se abre en el panel de detalle; la de "Sin nodo asignado", no
  return (
    <g data-objeto={r.nodo ? r.id : undefined}>
      <rect x={r.x} y={r.y} width={r.ancho} height={r.alto} rx={14} className={r.nodo ? 'region' : 'region region-sin-nodo'} />
      <text x={r.x + 16} y={r.y + 25} className="region-titulo">{r.titulo}</text>
      <text x={r.x + 16} y={r.y + 40} className="mono tenue">{r.subtitulo}</text>
    </g>
  );
}

// curva de abajo de un objeto a arriba del otro, con los controles a media altura
function Conexion({ conexion: c }: { conexion: ConexionEnEscena }) {
  const { inicio: a, fin: b } = c;
  const my = (a.y + b.y) / 2;
  return (
    <path
      d={`M${a.x},${a.y} C${a.x},${my} ${b.x},${my} ${b.x},${b.y}`}
      className={clases('conexion', `conexion-${c.tono}`, !!c.dePuerta && 'conexion-puerta')}
      markerEnd={c.tono === 'foco' || c.tono === 'recorrido' ? 'url(#flecha)' : undefined}
    />
  );
}

// marca roja de las advertencias de configuración: un punto con "!" y los títulos al pasar el mouse
function MarcaDeAdvertencia({ x, y, titulos }: { x: number; y: number; titulos: string[] }) {
  if (titulos.length === 0) return null;
  return (
    <g className="advertencia">
      <circle cx={x} cy={y} r={8} className="advertencia-punto" />
      <text x={x} y={y + 3.5} className="advertencia-texto">!</text>
      <title>{titulos.join('\n')}</title>
    </g>
  );
}

function TarjetaService({ service: s, apuntar }: { service: ServiceEnEscena; apuntar: Apuntar }) {
  const puertos = s.puertos.join(', ');
  return (
    <g className="objeto" opacity={s.opacidad} {...apuntar}>
      <rect x={s.x} y={s.y} width={s.ancho} height={s.alto} rx={s.alto / 2} className={clases('service', s.advertencias.length > 0 && 'con-advertencia', s.resaltado && 'resaltado', s.pasos.length > 0 && 'en-recorrido')} />
      <text x={s.x + 20} y={s.y + 20} className="pod-nombre">
        {recortar(s.nombre, s.ancho - 12 - 6.2 * (s.tipo.length + 1))} <tspan className="tipo">{s.tipo}</tspan>
        <title>{`Service ${s.nombre} · namespace ${s.namespace}`}</title>
      </text>
      <text x={s.x + 20} y={s.y + 35} className="mono tenue">{s.clusterIP || 'sin ClusterIP'}</text>
      <text x={s.x + 20} y={s.y + 50} className="mono">
        {recortar(puertos, s.ancho)}
        <title>{puertos}</title>
      </text>
      <Pasos x={s.x + 14} y={s.y} pasos={s.pasos} />
      <MarcaDeAdvertencia x={s.x + s.ancho - 14} y={s.y + 2} titulos={s.advertencias} />
    </g>
  );
}

function TarjetaIngress({ ingress: i, apuntar }: { ingress: IngressEnEscena; apuntar: Apuntar }) {
  return (
    <g className="objeto" opacity={i.opacidad} {...apuntar}>
      <rect x={i.x} y={i.y} width={i.ancho} height={i.alto} rx={8} className={clases('ingress', i.advertencias.length > 0 && 'con-advertencia', i.resaltado && 'resaltado', i.pasos.length > 0 && 'en-recorrido')} />
      <text x={i.x + 12} y={i.y + 22} className="pod-nombre">
        {recortar(i.nombre, i.ancho - 6.2 * (i.clase.length + 10))} <tspan className="tipo">Ingress · {i.clase}</tspan>
        <title>{`Ingress ${i.nombre} · namespace ${i.namespace}`}</title>
      </text>
      {i.reglas.map((r, n) => (
        <text key={n} x={i.x + 12} y={i.y + 41 + n * 14} className="mono">
          {recortarMono(r, i.ancho - 24)}
          <title>{r}</title>
        </text>
      ))}
      <Pasos x={i.x} y={i.y} pasos={i.pasos} />
      <MarcaDeAdvertencia x={i.x + i.ancho} y={i.y} titulos={i.advertencias} />
    </g>
  );
}

function PestanaPuerta({ puerta: p, apuntar }: { puerta: PuertaEnEscena; apuntar: Apuntar }) {
  return (
    <g className="objeto" opacity={p.opacidad} {...apuntar}>
      <rect
        x={p.x}
        y={p.y}
        width={p.ancho}
        height={p.alto}
        rx={5}
        className={clases('puerta', `puerta-${p.tipo}`, p.resaltado && 'resaltado', p.pasos.length > 0 && 'en-recorrido')}
      />
      <text x={p.x + p.ancho / 2} y={p.y + 16} className={`puerta-texto puerta-texto-${p.tipo}`}>{p.texto}</text>
      <title>{p.titulo}</title>
      <Pasos x={p.x} y={p.y} pasos={p.pasos} />
    </g>
  );
}

// recorta un texto monoespaciado de 10.5 px (aprox. 6.3 px por letra) para que entre en el ancho
const recortarMono = (texto: string, ancho: number) => {
  const max = Math.floor(ancho / 6.3);
  return texto.length > max ? texto.slice(0, max - 1) + '…' : texto;
};

// recorta el nombre para que entre en la tarjeta (aprox. 6.8 px por letra a 11.5 px, negrita)
const recortar = (texto: string, ancho: number) => {
  const max = Math.floor((ancho - 22) / 6.8);
  return texto.length > max ? texto.slice(0, max - 1) + '…' : texto;
};

// un pod nuevo brilla BRILLO_POD_NUEVO.veces y para; uno Terminating queda punteado (la escena lo atenúa)
const brillo = { animation: `brillo ${BRILLO_POD_NUEVO.msPorVez}ms ease-in-out ${BRILLO_POD_NUEVO.veces}` };

// la etiqueta del workload va a la derecha, en la línea de la IP, cortada con el espacio que deja
// la IP (aprox. 6.4 px por letra de la IP en mono y 5.6 px por letra de la etiqueta)
const recortarEtiqueta = (texto: string, ip: string, ancho: number) => {
  const max = Math.floor((ancho - 13 - 10 - 10 - ip.length * 6.4) / 5.6);
  return texto.length > max ? texto.slice(0, Math.max(1, max - 1)) + '…' : texto;
};

function TarjetaPod({ pod: p, apuntar }: { pod: PodEnEscena; apuntar: Apuntar }) {
  // el lienzo no recorta el SVG: la barra se dibuja adentro de la tarjeta con su propio clip
  const clip = `clip-${p.id.replace(/[^a-zA-Z0-9_-]/g, '_')}`;
  return (
    <g opacity={p.opacidad} className={p.terminando ? 'objeto pod-terminando' : 'objeto'} {...apuntar}>
      <clipPath id={clip}>
        <rect x={p.x} y={p.y} width={p.ancho} height={p.alto} rx={8} />
      </clipPath>
      <rect
        x={p.x}
        y={p.y}
        width={p.ancho}
        height={p.alto}
        rx={8}
        className={clases(
          'pod',
          p.tonoEstado === 'mal' && 'pod-mal',
          p.nuevo && 'pod-nuevo',
          p.resaltado && 'resaltado',
          p.pasos.length > 0 && 'en-recorrido',
        )}
        style={{
          ...(p.bordeWorkload ? { stroke: p.bordeWorkload, strokeWidth: 2.5 } : {}),
          ...(p.nuevo ? brillo : {}),
        }}
      />
      <rect x={p.x} y={p.y} width={4} height={p.alto} fill={p.workload.color} clipPath={`url(#${clip})`} />
      <text x={p.x + 13} y={p.y + 18} className="pod-nombre">
        {p.nombreVisible}
        <title>{`${p.nombre} · namespace ${p.namespace}`}</title>
      </text>
      {p.estado && (
        <text x={p.x + p.ancho - 8} y={p.y + 18} className={`pod-estado tono-${p.tonoEstado}`}>
          {p.estado}
        </text>
      )}
      <text x={p.x + 13} y={p.y + 33} className="mono tenue">{p.ip ?? 'sin IP'}</text>
      <text x={p.x + p.ancho - 8} y={p.y + 33} textAnchor="end" className="pod-workload" fill={p.workload.color}>
        {recortarEtiqueta(p.workload.etiqueta, p.ip ?? 'sin IP', p.ancho)}
        <title>{p.workload.etiqueta}</title>
      </text>
      {p.containers.map((c, i) => (
        <g key={c.nombre || `+${i}`}>
          <rect
            x={c.x}
            y={c.y}
            width={c.ancho}
            height={c.alto}
            rx={3}
            className={`chip chip-${c.tipo ?? 'resto'}${c.noListo ? ' chip-no-listo' : ''}`}
          />
          <text x={c.x + c.ancho / 2} y={c.y + 10.5} className="chip-texto">{c.texto}</text>
          <title>{c.detalle}</title>
        </g>
      ))}
      {p.reinicios !== null && (
        <Reinicios key={p.reinicios} x={p.x + p.ancho - 8} y={p.y + 52} n={p.reinicios} titila={p.reiniciosTitilan} />
      )}
      <Pasos x={p.x} y={p.y} pasos={p.pasos} />
      <MarcaDeAdvertencia x={p.x + p.ancho} y={p.y} titulos={p.advertencias} />
    </g>
  );
}

// Se monta de nuevo con cada número (key), así la animación arranca cada vez que los reinicios
// aumentan; guarda el "titila" del montaje para que la foto siguiente no la corte a la mitad.
function Reinicios({ x, y, n, titila }: { x: number; y: number; n: number; titila: boolean }) {
  const [titilando] = useState(titila);
  return (
    <text x={x} y={y} className={titilando ? 'reinicios titila' : 'reinicios'}>
      ↻ {n}
      <title>{`${n} reinicios`}</title>
    </text>
  );
}

// números rosas de los pasos del recorrido, sobre la esquina de arriba a la izquierda del objeto
function Pasos({ x, y, pasos }: { x: number; y: number; pasos: number[] }) {
  return (
    <>
      {pasos.map((n, i) => (
        <g key={n} className="paso">
          <circle cx={x + i * 22} cy={y} r={10} className="paso-circulo" />
          <text x={x + i * 22} y={y + 4} className="paso-texto">{n}</text>
        </g>
      ))}
    </>
  );
}
