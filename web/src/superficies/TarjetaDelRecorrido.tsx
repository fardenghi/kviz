// Pasos del recorrido (superficie flotante, tarjeta compacta abajo a la izquierda): el destino, la
// respuesta y los pasos numerados como en la vista; si la request se repitió, las barras del
// reparto. Un clic en un paso resalta solo ese paso en la vista; otro clic (o "todos los pasos")
// vuelve a mostrarlos todos. Cerrarla termina el recorrido y el foco vuelve.
import { nsNombre } from '../escena/ids';
import { lineaDelPedido, type Prueba, type Reparto } from '../prueba';
import { PastillaDelResultado } from './FormularioDePrueba';

export function TarjetaDelRecorrido({
  prueba: p,
  elegido = null,
  alElegir,
  alCerrar,
}: {
  prueba: Prueba;
  /** Número del paso elegido, o null si se ven todos. */
  elegido?: number | null;
  alElegir?: (n: number | null) => void;
  alCerrar: () => void;
}) {
  return (
    <section className="flotante tarjeta-recorrido" aria-label="Recorrido">
      <div className="tarjeta-recorrido-cabecera">
        <b>Recorrido</b>
        <PastillaDelResultado prueba={p} />
        <button className="boton" title="Cerrar el recorrido" onClick={alCerrar}>
          ✕
        </button>
      </div>
      <div className="mono-texto chica tenue-texto">{lineaDelPedido(p)}</div>
      {elegido !== null && (
        <button className="boton todos-los-pasos" onClick={() => alElegir?.(null)}>
          todos los pasos
        </button>
      )}
      <ol className="pasos-recorrido">
        {p.recorrido.pasos.map((paso) => (
          <li
            key={paso.n}
            className={clasesDelPaso(paso.n, elegido)}
            role="button"
            tabIndex={0}
            title={paso.n === elegido ? 'Ver todos los pasos' : 'Resaltar solo este paso'}
            onClick={() => alElegir?.(paso.n === elegido ? null : paso.n)}
            onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), alElegir?.(paso.n === elegido ? null : paso.n))}
          >
            <span className="paso-numero">{paso.n}</span>
            <span>
              <b>{NOMBRE_PASO[paso.tipo] ?? paso.tipo}</b> <span className="paso-titulo">{paso.titulo}</span>
              {paso.detalle && <div className="tenue-texto chica">{paso.detalle}</div>}
            </span>
          </li>
        ))}
      </ol>
      {p.reparto && <BarrasDeReparto reparto={p.reparto} />}
      <div className="tenue-texto chica">Origen: {p.origen}</div>
    </section>
  );
}

// reparto de la request repetida: una barra por pod que respondió, y aparte las respuestas sin pod
// identificado y las requests sin respuesta
function BarrasDeReparto({ reparto: r }: { reparto: Reparto }) {
  const fila = (clave: string, nombre: string, n: number, sinPod = false) => (
    <li key={clave}>
      <span className={sinPod ? 'reparto-nombre tenue-texto' : 'reparto-nombre'} title={nombre}>
        {nombre}
      </span>
      <span className={sinPod ? 'barra-reparto sin-pod' : 'barra-reparto'}>
        <span style={{ width: `${(100 * n) / r.total}%` }} />
      </span>
      <span className="reparto-n">
        {n}/{r.total}
      </span>
    </li>
  );
  return (
    <div>
      <b>Reparto</b> <span className="tenue-texto chica">· {r.total} requests</span>
      <ul className="barras-reparto">
        {r.pods.map((p) => fila(p.pod, nsNombre(p.pod).nombre, p.n))}
        {r.sinIdentificar > 0 && fila('?', 'pod sin identificar', r.sinIdentificar, true)}
        {r.sinRespuesta > 0 && fila('✕', 'sin respuesta', r.sinRespuesta, true)}
      </ul>
    </div>
  );
}

const clasesDelPaso = (n: number, elegido: number | null) =>
  ['paso-de-la-tarjeta', n === elegido && 'paso-elegido', elegido !== null && n !== elegido && 'paso-apagado'].filter(Boolean).join(' ');

const NOMBRE_PASO: Record<string, string> = {
  dns: 'DNS',
  puerta: 'puerta',
  ingress: 'regla de Ingress',
  service: 'Service',
  endpoints: 'endpoints',
  salto: 'salto entre nodos',
  pod: 'pod',
};
