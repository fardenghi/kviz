// Service: tipo, ClusterIP, puertos con nodePort, selector y endpoints listos y no listos.
import { idPod, paresClaveValor, type PropsDeSeccion } from '../detalle';
import { Datos, Enlace, Nada, Pares, Seccion } from '../ui';

export const orden = 10;

export default function DatosDelService({ detalle, abrir }: PropsDeSeccion) {
  if (detalle.tipo !== 'Service') return null;
  const s = detalle.service;
  const listos = s.endpoints.filter((e) => e.listo);
  const noListos = s.endpoints.filter((e) => !e.listo);
  return (
    <>
      <Seccion>
        <Datos
          filas={[
            ['namespace', s.namespace],
            ['tipo', s.tipo],
            ['ClusterIP', s.clusterIP ? <span className="mono-texto">{s.clusterIP}</span> : <Nada>sin ClusterIP</Nada>],
            [
              'puertos',
              s.puertos.length ? (
                <span className="mono-texto">
                  {s.puertos.map((p) => (
                    <div key={`${p.puerto}/${p.protocolo}`}>
                      {p.puerto} → {p.targetPort}
                      {p.protocolo !== 'TCP' && `/${p.protocolo}`}
                      {p.nodePort ? ` · nodePort ${p.nodePort}` : ''}
                      {p.nombre && <span className="tenue-texto"> ({p.nombre})</span>}
                    </div>
                  ))}
                </span>
              ) : (
                <Nada>sin puertos</Nada>
              ),
            ],
            ['selector', <Pares pares={paresClaveValor(s.selector)} />],
            ['DNS', <span className="mono-texto">{`${s.nombre}.${s.namespace}.svc.cluster.local`}</span>],
          ]}
        />
      </Seccion>
      <Seccion titulo={`Endpoints · ${listos.length} listos, ${noListos.length} no listos`}>
        {s.endpoints.length === 0 && <Nada>sin endpoints: ningún pod listo coincide con el selector</Nada>}
        {[...listos, ...noListos].map((e) => (
          <div key={e.ip} className={e.listo ? 'endpoint' : 'endpoint endpoint-no-listo'}>
            <span className="mono-texto">
              {e.listo ? '✓' : '✗'} {e.ip}
            </span>{' '}
            {e.pod ? (
              <Enlace id={idPod({ namespace: s.namespace, nombre: e.pod })} abrir={abrir}>
                {e.pod}
              </Enlace>
            ) : null}
            {e.nodo && <span className="tenue-texto"> · {e.nodo}</span>}
            {!e.listo && <span className="tenue-texto"> · no listo</span>}
          </div>
        ))}
      </Seccion>
    </>
  );
}
