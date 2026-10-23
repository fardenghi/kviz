// Workload: cadena de dueños con sus pods, réplicas deseadas y listas, y estrategia de rollout.
import { idPod, type PropsDeSeccion } from '../detalle';
import { Datos, Enlace, Nada, Seccion } from '../ui';

export const orden = 10;

export default function DatosDelWorkload({ detalle, abrir }: PropsDeSeccion) {
  if (detalle.tipo !== 'Workload') return null;
  const w = detalle.workload;
  return (
    <>
      <Seccion>
        <Datos
          filas={[
            ['namespace', w.namespace],
            ['réplicas', w.replicas ? `${w.replicas.listas} listas de ${w.replicas.deseadas} deseadas` : null],
            ['rollout', w.estrategia || null],
          ]}
        />
      </Seccion>
      <Seccion titulo="Cadena">
        {w.cadena.map((e, i) => (
          <div key={`${e.tipo}/${e.nombre}`} className="eslabon" style={{ paddingLeft: i === 0 ? 0 : 14 }}>
            {i > 0 && '└ '}
            <b>{e.tipo}</b> {e.nombre}
            {e.pods.map((p) => (
              <div key={p} className="eslabon-pod">
                └ Pod{' '}
                <Enlace id={idPod({ namespace: w.namespace, nombre: p })} abrir={abrir}>
                  {p}
                </Enlace>
              </div>
            ))}
          </div>
        ))}
        {w.cadena.every((e) => e.pods.length === 0) && <Nada>sin pods</Nada>}
      </Seccion>
    </>
  );
}
