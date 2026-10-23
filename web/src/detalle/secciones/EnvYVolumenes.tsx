// Pod: env de cada container con su origen (valor, ConfigMap, Secret o Downward API) y su fuente,
// y los volúmenes con dónde se monta cada uno en cada container. Un ConfigMap se abre para ver
// su contenido. Los valores de Secret no vienen en la foto: se muestran ocultos (••••) y se
// revelan de a uno con un pedido explícito al backend.
import { useState } from 'react';
import { revelarSecret } from '../../api';
import type { Container, Foto, Pod, Variable, Volumen } from '../../foto';
import type { PropsDeSeccion } from '../detalle';
import { Nada, Seccion } from '../ui';

export const orden = 40;

export default function EnvYVolumenes({ detalle, foto }: PropsDeSeccion) {
  if (detalle.tipo !== 'Pod') return null;
  // con otro pod se cierra todo lo abierto y se vuelven a ocultar los Secrets revelados
  return <EnvDelPod key={detalle.pod.id} pod={detalle.pod} foto={foto} />;
}

const ETIQUETA_ORIGEN: Record<Variable['origen'], string> = {
  valor: 'valor',
  ConfigMap: 'ConfigMap',
  Secret: 'Secret',
  DownwardAPI: 'Downward API',
};

function EnvDelPod({ pod, foto }: { pod: Pod; foto: Foto }) {
  const conEnv = pod.containers.filter((c) => (c.env ?? []).length > 0);
  const volumenes = pod.volumenes ?? [];
  return (
    <>
      {conEnv.length === 0 ? (
        <Seccion titulo="Env">
          <Nada>sin variables de entorno</Nada>
        </Seccion>
      ) : (
        conEnv.map((c) => (
          <Seccion key={c.nombre} titulo={pod.containers.length > 1 ? `Env · ${c.nombre}` : 'Env'}>
            {(c.env ?? []).map((v, i) => (
              <FilaDeVariable key={`${v.nombre}/${i}`} v={v} pod={pod} foto={foto} />
            ))}
          </Seccion>
        ))
      )}
      <Seccion titulo="Volúmenes">
        {volumenes.length === 0 && <Nada>sin volúmenes</Nada>}
        {volumenes.map((v) => (
          <FilaDeVolumen key={v.nombre} v={v} pod={pod} foto={foto} />
        ))}
      </Seccion>
    </>
  );
}

function FilaDeVariable({ v, pod, foto }: { v: Variable; pod: Pod; foto: Foto }) {
  const [abierto, setAbierto] = useState(false);
  const fuente =
    v.origen === 'ConfigMap' ? (
      <button className="enlace" title="Ver el contenido del ConfigMap" onClick={() => setAbierto(!abierto)}>
        {v.fuente}
      </button>
    ) : (
      v.fuente
    );
  return (
    <div className="variable">
      <div className="variable-linea">
        <span className="mono-texto variable-nombre">{v.nombre || <i>envFrom</i>}</span>
        <span className="mono-texto variable-valor">
          {v.nombre === '' ? (
            <Nada>todas sus claves</Nada>
          ) : v.origen === 'Secret' && !v.falta ? (
            <ValorOculto namespace={pod.namespace} secret={v.fuente} clave={v.clave} />
          ) : v.valor !== null ? (
            v.valor
          ) : v.origen === 'DownwardAPI' ? (
            <Nada>lo resuelve el kubelet</Nada>
          ) : (
            <Nada>—</Nada>
          )}
        </span>
      </div>
      <div className="variable-origen tenue-texto">
        <span className={`origen origen-${v.origen}`}>{ETIQUETA_ORIGEN[v.origen]}</span>
        {v.fuente && <> {fuente}</>}
        {v.clave && v.clave !== v.nombre && <> · clave {v.clave}</>}
        {v.envFrom && v.nombre !== '' && ' · envFrom'}
        {v.falta && <span className="falta"> · no existe</span>}
      </div>
      {abierto && <ContenidoDeConfigMap foto={foto} namespace={pod.namespace} nombre={v.fuente} />}
    </div>
  );
}

function FilaDeVolumen({ v, pod, foto }: { v: Volumen; pod: Pod; foto: Foto }) {
  const [abierto, setAbierto] = useState(false);
  const montajes = pod.containers.flatMap((c: Container) => (c.montajes ?? []).filter((m) => m.volumen === v.nombre).map((m) => ({ c, m })));
  const abrible = v.tipo === 'ConfigMap' || v.tipo === 'Secret';
  return (
    <div className="volumen">
      <div>
        <span className="mono-texto">{v.nombre}</span> <span className="tenue-texto">· {v.tipo}</span>{' '}
        {v.fuente &&
          (abrible ? (
            <button className="enlace" title={`Ver el contenido del ${v.tipo}`} onClick={() => setAbierto(!abierto)}>
              {v.fuente}
            </button>
          ) : (
            <span className="mono-texto">{v.fuente}</span>
          ))}
      </div>
      {montajes.length === 0 && (
        <div className="montaje">
          <Nada>no lo monta ningún container</Nada>
        </div>
      )}
      {montajes.map(({ c, m }) => (
        <div key={`${c.nombre}${m.ruta}`} className="montaje mono-texto">
          → {c.nombre}: {m.ruta}
          {m.subPath && <span className="tenue-texto"> (subPath {m.subPath})</span>}
          {m.soloLectura && <span className="tenue-texto"> · solo lectura</span>}
        </div>
      ))}
      {abierto && v.tipo === 'ConfigMap' && <ContenidoDeConfigMap foto={foto} namespace={pod.namespace} nombre={v.fuente} />}
      {abierto && v.tipo === 'Secret' && <ClavesDeSecret foto={foto} namespace={pod.namespace} nombre={v.fuente} />}
    </div>
  );
}

/** El contenido de un ConfigMap: cada clave con su valor (un archivo, si se monta como volumen). */
function ContenidoDeConfigMap({ foto, namespace, nombre }: { foto: Foto; namespace: string; nombre: string }) {
  const cm = (foto.configMaps ?? []).find((x) => x.namespace === namespace && x.nombre === nombre);
  if (!cm) return <div className="contenido"><Nada>el ConfigMap {nombre} no existe</Nada></div>;
  const claves = Object.keys(cm.datos).sort();
  return (
    <div className="contenido">
      {claves.length === 0 && <Nada>vacío</Nada>}
      {claves.map((k) => (
        <div key={k}>
          <div className="mono-texto contenido-clave">{k}</div>
          <pre className="contenido-valor">{cm.datos[k]}</pre>
        </div>
      ))}
    </div>
  );
}

/** Las claves de un Secret montado como volumen, con sus valores ocultos. */
function ClavesDeSecret({ foto, namespace, nombre }: { foto: Foto; namespace: string; nombre: string }) {
  const s = (foto.secrets ?? []).find((x) => x.namespace === namespace && x.nombre === nombre);
  if (!s) return <div className="contenido"><Nada>el Secret {nombre} no existe</Nada></div>;
  return (
    <div className="contenido">
      {s.claves.length === 0 && <Nada>vacío</Nada>}
      {s.claves.map((k) => (
        <div key={k} className="variable-linea">
          <span className="mono-texto variable-nombre">{k}</span>
          <span className="mono-texto variable-valor">
            <ValorOculto namespace={namespace} secret={nombre} clave={k} />
          </span>
        </div>
      ))}
    </div>
  );
}

type EstadoDelValor = { tipo: 'oculto' } | { tipo: 'cargando' } | { tipo: 'visible'; valor: string } | { tipo: 'error'; mensaje: string };

/** El valor de una clave de un Secret: oculto hasta que se pide revelarlo. */
function ValorOculto({ namespace, secret, clave }: { namespace: string; secret: string; clave: string }) {
  const [estado, setEstado] = useState<EstadoDelValor>({ tipo: 'oculto' });
  const revelar = () => {
    setEstado({ tipo: 'cargando' });
    revelarSecret({ namespace, nombre: secret, clave })
      .then((valor) => setEstado({ tipo: 'visible', valor }))
      .catch((e: Error) => setEstado({ tipo: 'error', mensaje: e.message }));
  };
  switch (estado.tipo) {
    case 'visible':
      return (
        <>
          <span className="valor-revelado">{estado.valor}</span>{' '}
          <button className="boton" title="Volver a ocultar" onClick={() => setEstado({ tipo: 'oculto' })}>
            Ocultar
          </button>
        </>
      );
    case 'error':
      return <span className="falta" title={estado.mensaje}>no se pudo revelar</span>;
    default:
      return (
        <>
          ••••{' '}
          <button className="boton" title="Pedir el valor al backend" disabled={estado.tipo === 'cargando'} onClick={revelar}>
            Revelar
          </button>
        </>
      );
  }
}
