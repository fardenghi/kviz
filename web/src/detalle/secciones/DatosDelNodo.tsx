// Nodo: InternalIP, capacidad y pods.
import { idPod, type PropsDeSeccion } from '../detalle';
import { Datos, Enlace, Nada, Seccion } from '../ui';

export const orden = 10;

export default function DatosDelNodo({ detalle, foto, abrir }: PropsDeSeccion) {
  if (detalle.tipo !== 'Nodo') return null;
  const n = detalle.nodo;
  const c = n.capacidad;
  // todos los pods del nodo, también los de los namespaces del sistema: ocupan lugar igual
  const pods = foto.pods.filter((p) => p.nodo === n.nombre);
  return (
    <>
      <Seccion>
        <Datos
          filas={[
            ['InternalIP', n.internalIP ? <span className="mono-texto">{n.internalIP}</span> : <Nada>sin InternalIP</Nada>],
            ['CPU', c?.cpu || <Nada>—</Nada>],
            ['memoria', c?.memoria || <Nada>—</Nada>],
            ['pods', c?.pods ? `${pods.length} de ${c.pods}` : String(pods.length)],
          ]}
        />
      </Seccion>
      <Seccion titulo={`Pods · ${pods.length}`}>
        {pods.length === 0 && <Nada>sin pods</Nada>}
        {pods.map((p) => (
          <div key={p.id}>
            <Enlace id={idPod(p)} abrir={abrir}>
              {p.nombre}
            </Enlace>{' '}
            <span className="tenue-texto">
              {p.namespace}
              {p.estado !== 'Running' && ` · ${p.estado}`}
            </span>
          </div>
        ))}
      </Seccion>
    </>
  );
}
