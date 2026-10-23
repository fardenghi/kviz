// Lista de workloads (superficie flotante, arriba a la izquierda): plegable; muestra la cadena de
// dueños solo del workload seleccionado. Seleccionar uno resalta sus pods; no es foco.
import { useState } from 'react';
import type { Workload } from '../foto';
import { textoDeCadena } from '../escena/workloads';

export function ListaDeWorkloads({
  workloads,
  seleccionado,
  alSeleccionar,
}: {
  workloads: Workload[];
  seleccionado: string | null;
  /** Con null se deselecciona. */
  alSeleccionar: (id: string | null) => void;
}) {
  const [abierta, setAbierta] = useState(true);
  return (
    <div className="flotante lista-workloads">
      <button className="lista-workloads-cabecera" onClick={() => setAbierta((a) => !a)} aria-expanded={abierta}>
        Workloads <span className="tenue-texto">{workloads.length}</span>
        <span className="tenue-texto plegar">{abierta ? '▾' : '▸'}</span>
      </button>
      {abierta && (
        <ul className="lista-workloads-items">
          {workloads.length === 0 && <li className="tenue-texto vacio">No hay workloads.</li>}
          {workloads.map((w) => {
            const elegido = w.id === seleccionado;
            const cadena = elegido ? textoDeCadena(w) : null;
            return (
              <li key={w.id}>
                <button
                  className={elegido ? 'workload workload-elegido' : 'workload'}
                  onClick={() => alSeleccionar(elegido ? null : w.id)}
                  title={`${w.tipo} ${w.nombre} · namespace ${w.namespace}`}
                >
                  <span className="muestra" style={{ background: w.color }} />
                  <b className="workload-nombre">{w.nombre}</b>
                  <span className="tenue-texto workload-tipo">{w.tipo}</span>
                </button>
                {cadena && (
                  <div className="cadena">
                    <div>{cadena.resumen}</div>
                    {cadena.eslabones.map((e) => (
                      <div key={e} className="mono tenue-texto">
                        {e}
                      </div>
                    ))}
                  </div>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
