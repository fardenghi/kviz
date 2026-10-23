// Events del objeto (de un workload, también los de sus ReplicaSets o Jobs), del más reciente al
// más viejo.
import { eventsDelObjeto, horaDeEvent } from '../../events';
import type { PropsDeSeccion } from '../detalle';
import { Nada, Seccion } from '../ui';

export const orden = 50;

export default function Events({ detalle, foto }: PropsDeSeccion) {
  const events = eventsDelObjeto(foto, detalle);
  return (
    <Seccion titulo="Events">
      {events.length === 0 && <Nada>sin events recientes</Nada>}
      {events.map((e) => (
        <div key={e.id} className={`event-detalle event-detalle-${e.tipo}`}>
          <span className="mono-texto tenue-texto">{horaDeEvent(e)}</span> <b>{e.motivo}</b>
          {e.veces > 1 && <span className="tenue-texto"> ×{e.veces}</span>}
          {detalle.tipo === 'Workload' && e.objeto.tipo !== detalle.workload.tipo && (
            <span className="tenue-texto">
              {' '}
              · {e.objeto.tipo} {e.objeto.nombre}
            </span>
          )}
          {e.mensaje && <div className="event-mensaje">{e.mensaje}</div>}
        </div>
      ))}
    </Seccion>
  );
}
