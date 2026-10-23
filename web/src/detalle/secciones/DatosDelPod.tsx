// Pod: namespace, labels, nodo, IP, edad, estado y workload.
import { useEffect, useState } from 'react';
import { etiquetaDeWorkload } from '../../escena/workloads';
import { edad, idNodo, idWorkload, paresClaveValor, type PropsDeSeccion } from '../detalle';
import { Datos, Enlace, Nada, Pares, Seccion } from '../ui';

export const orden = 10;

export default function DatosDelPod({ detalle, foto, abrir }: PropsDeSeccion) {
  const ahora = useAhora();
  if (detalle.tipo !== 'Pod') return null;
  const p = detalle.pod;
  const w = p.workload ? foto.workloads.find((x) => x.id === p.workload) : undefined;
  const anios = edad(p.creado, ahora);
  return (
    <Seccion>
      <Datos
        filas={[
          ['namespace', p.namespace],
          ['nodo', p.nodo ? <Enlace id={idNodo(p.nodo)} abrir={abrir}>{p.nodo}</Enlace> : <Nada>sin nodo asignado</Nada>],
          ['IP', p.ip ? <span className="mono-texto">{p.ip}</span> : <Nada>sin IP</Nada>],
          ['estado', p.estado === p.fase ? p.estado : `${p.estado} (fase ${p.fase})`],
          ['edad', anios || null],
          [
            'workload',
            w ? (
              <Enlace id={idWorkload(w)} abrir={abrir}>
                {etiquetaDeWorkload(w)}
              </Enlace>
            ) : (
              <Nada>Pod suelto</Nada>
            ),
          ],
          ['labels', <Pares pares={paresClaveValor(p.labels)} />],
        ]}
      />
    </Seccion>
  );
}

/** La hora actual, que avanza cada 10 s para que la edad se mantenga al día. */
function useAhora() {
  const [ahora, setAhora] = useState(Date.now);
  useEffect(() => {
    const t = setInterval(() => setAhora(Date.now()), 10_000);
    return () => clearInterval(t);
  }, []);
  return ahora;
}
