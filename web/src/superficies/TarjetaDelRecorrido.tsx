// Pasos del recorrido (superficie flotante, tarjeta compacta abajo a la izquierda): el destino, la
// respuesta y los pasos numerados como en la vista. Cerrarla termina el recorrido y el foco vuelve.
import type { Prueba } from '../prueba';
import { formatearMs } from './FormularioDePrueba';

export function TarjetaDelRecorrido({ prueba: p, alCerrar }: { prueba: Prueba; alCerrar: () => void }) {
  const r = p.respuesta;
  return (
    <section className="flotante tarjeta-recorrido" aria-label="Recorrido">
      <div className="tarjeta-recorrido-cabecera">
        <b>Recorrido</b>
        {r ? (
          <span className={r.status < 400 ? 'pastilla pastilla-ok' : 'pastilla pastilla-mal'}>
            {r.status} {r.texto} · {formatearMs(r.ms)}
          </span>
        ) : (
          <span className="pastilla pastilla-mal">sin respuesta</span>
        )}
        <button className="boton" title="Cerrar el recorrido" onClick={alCerrar}>
          ✕
        </button>
      </div>
      <div className="mono-texto chica tenue-texto">
        {p.metodo} {p.url}
      </div>
      <ol className="pasos-recorrido">
        {p.recorrido.pasos.map((paso) => (
          <li key={paso.n}>
            <span className="paso-numero">{paso.n}</span>
            <span>
              <b>{NOMBRE_PASO[paso.tipo] ?? paso.tipo}</b> <span className="paso-titulo">{paso.titulo}</span>
              {paso.detalle && <div className="tenue-texto chica">{paso.detalle}</div>}
            </span>
          </li>
        ))}
      </ol>
      <div className="tenue-texto chica">Origen: {p.origen}</div>
    </section>
  );
}

const NOMBRE_PASO: Record<string, string> = {
  dns: 'DNS',
  puerta: 'puerta',
  ingress: 'regla de Ingress',
  service: 'Service',
  endpoints: 'endpoints',
  salto: 'salto entre nodos',
  pod: 'pod',
};
