// Logs de un pod: elegir un container muestra sus logs y los sigue en vivo; si el container se
// reinició, se pueden ver los del reinicio anterior. Cerrar el panel, cambiar de container o de
// pod corta el stream anterior.
import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { Container, Pod } from '../../foto';
import { escucharLogs } from '../../logs';
import type { PropsDeSeccion } from '../detalle';
import { Nada, Seccion } from '../ui';

export const orden = 60;

/** Cuántas líneas se guardan como mucho; las más viejas se descartan. */
const MAX_LINEAS = 2000;

export default function Logs({ detalle, foto }: PropsDeSeccion) {
  if (detalle.tipo !== 'Pod') return null;
  // otro pod (u otro cluster) arranca con la elección de container de cero
  return <LogsDelPod key={`${foto.contexto}/${detalle.pod.id}`} pod={detalle.pod} />;
}

/** El container que se muestra al abrir el pod: el primero normal (no init ni ephemeral). */
function containerInicial(pod: Pod): string {
  return (pod.containers.find((c) => c.tipo === 'normal') ?? pod.containers[0])?.nombre ?? '';
}

type Estado = 'cargando' | 'en vivo' | 'terminado' | { error: string };

function LogsDelPod({ pod }: { pod: Pod }) {
  const [elegido, setElegido] = useState(() => containerInicial(pod));
  const [quiereAnterior, setQuiereAnterior] = useState(false);
  const container: Container | undefined = pod.containers.find((c) => c.nombre === elegido) ?? pod.containers[0];
  const anterior = quiereAnterior && (container?.reinicios ?? 0) > 0;

  const [lineas, setLineas] = useState<string[]>([]);
  const [estado, setEstado] = useState<Estado>('cargando');
  useEffect(() => {
    if (!container) return;
    setLineas([]);
    setEstado('cargando');
    const cortar = escucharLogs(
      { namespace: pod.namespace, pod: pod.nombre, container: container.nombre, anterior },
      (m) => {
        if (m.tipo === 'lineas') {
          setLineas((previas) => [...previas, ...m.lineas].slice(-MAX_LINEAS));
        } else if (m.tipo === 'fin') setEstado('terminado');
        else setEstado({ error: m.error });
      },
      () => setEstado((e) => (e === 'cargando' ? 'en vivo' : e)),
    );
    return cortar;
    // con cada reinicio se vuelven a abrir: los actuales terminaron con el container que murió y
    // los del reinicio anterior pasan a ser los de ese
  }, [pod.namespace, pod.nombre, container?.nombre, container?.reinicios, anterior]); // eslint-disable-line react-hooks/exhaustive-deps

  // sigue el final mientras el usuario no se haya ido para arriba a leer
  const caja = useRef<HTMLPreElement>(null);
  const alFinal = useRef(true);
  useLayoutEffect(() => {
    const c = caja.current;
    if (c && alFinal.current) c.scrollTop = c.scrollHeight;
  }, [lineas]);

  if (!container) {
    return (
      <Seccion titulo="Logs">
        <Nada>el pod no tiene containers</Nada>
      </Seccion>
    );
  }
  return (
    <Seccion titulo="Logs">
      <div className="logs-controles">
        {pod.containers.length > 1 && (
          <label className="campo">
            container
            <select value={container.nombre} onChange={(e) => setElegido(e.target.value)}>
              {pod.containers.map((c) => (
                <option key={c.nombre} value={c.nombre}>
                  {c.nombre}
                  {c.tipo !== 'normal' ? ` (${c.tipo})` : ''}
                </option>
              ))}
            </select>
          </label>
        )}
        {container.reinicios > 0 && (
          <div className="logs-cual" role="group" aria-label="Qué logs">
            <button className="boton" aria-pressed={!anterior} onClick={() => setQuiereAnterior(false)}>
              actuales
            </button>
            <button className="boton" aria-pressed={anterior} onClick={() => setQuiereAnterior(true)}>
              reinicio anterior
            </button>
          </div>
        )}
        <EstadoDeLogs estado={estado} anterior={anterior} reinicios={container.reinicios} />
      </div>
      {typeof estado === 'object' && <div className="logs-error">{estado.error}</div>}
      <pre
        ref={caja}
        className="logs"
        onScroll={(e) => {
          const c = e.currentTarget;
          alFinal.current = c.scrollHeight - c.scrollTop - c.clientHeight < 8;
        }}
      >
        {lineas.length > 0 ? lineas.join('\n') : estado === 'cargando' ? '' : 'sin logs'}
      </pre>
    </Seccion>
  );
}

function EstadoDeLogs({ estado, anterior, reinicios }: { estado: Estado; anterior: boolean; reinicios: number }) {
  if (typeof estado === 'object') return null;
  const texto =
    estado === 'cargando'
      ? 'cargando…'
      : anterior
        ? `del reinicio anterior (${reinicios} ${reinicios === 1 ? 'reinicio' : 'reinicios'})`
        : estado === 'en vivo'
          ? 'en vivo'
          : 'terminaron';
  return (
    <span className="tenue-texto logs-estado">
      {estado === 'en vivo' && !anterior && <span className="punto punto-conectado" />}
      {texto}
    </span>
  );
}
