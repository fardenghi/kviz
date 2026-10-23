// Franja de events (abajo, debajo del lienzo): los últimos events del cluster en vivo, del más
// reciente al más viejo, con los filtros de namespace (ver events.ts). Un clic en un event
// selecciona su objeto y abre su panel de detalle.
import type { EventDelCluster } from '../foto';
import { horaDeEvent, seleccionDeEvent } from '../events';

export function FranjaDeEvents({ events, abrir }: { events: EventDelCluster[]; abrir: (id: string) => void }) {
  return (
    <div className="franja-events" aria-label="Events del cluster">
      <span className="franja-events-titulo">Events</span>
      <div className="franja-events-lista">
        {events.length === 0 && <span className="tenue-texto">sin events recientes</span>}
        {events.map((e) => {
          const seleccion = seleccionDeEvent(e);
          const objeto = `${e.objeto.tipo} ${e.objeto.namespace ? `${e.objeto.namespace}/` : ''}${e.objeto.nombre}`;
          return (
            <button
              key={e.id}
              className={`event event-${e.tipo}`}
              disabled={seleccion === null}
              onClick={() => seleccion && abrir(seleccion)}
              title={`${objeto}\n${e.motivo}: ${e.mensaje}${e.veces > 1 ? ` (×${e.veces})` : ''}${seleccion ? '' : '\nel objeto ya no está en el cluster'}`}
            >
              <span className="mono-texto">{horaDeEvent(e)}</span> <b>{e.motivo}</b>{' '}
              <span className="event-objeto">{e.objeto.nombre}</span>
              {e.veces > 1 && <span className="tenue-texto"> ×{e.veces}</span>} <span className="tenue-texto">{e.mensaje}</span>
            </button>
          );
        })}
      </div>
    </div>
  );
}
