// Pod: por container, el estado de probe de su liveness, readiness y startup (con la config, las
// señales de las que se dedujo y los últimos fallos), y sus requests y limits de CPU y memoria
// contra el consumo real. Sin metrics-server, "sin métricas" y el comando para instalarlo en kind.
import { useEffect, useState } from 'react';
import { type ComandoSugerido, pedirComandos } from '../../api';
import type { ConfigDeProbe, Container, Probe, Recurso } from '../../foto';
import { Comando } from '../Comando';
import type { PropsDeSeccion } from '../detalle';
import { Nada, Seccion } from '../ui';

export const orden = 30;

export default function ProbesYRecursos({ detalle, foto }: PropsDeSeccion) {
  if (detalle.tipo !== 'Pod') return null;
  const metricas = foto.metricas ?? false;
  // los init y ephemeral sin probes ni recursos no tienen nada para mostrar acá
  const containers = detalle.pod.containers.filter((c) => c.tipo === 'normal' || tieneProbes(c) || tieneRecursos(c));
  if (containers.length === 0) return null;
  return (
    <Seccion titulo="Probes y recursos">
      {containers.map((c) => (
        <div key={`${c.tipo}/${c.nombre}`} className="salud">
          {(containers.length > 1 || c.tipo !== 'normal') && (
            <div className="salud-container">
              <span className="mono-texto">{c.nombre}</span>
              {c.tipo !== 'normal' && <span className="tenue-texto"> · {c.tipo}</span>}
            </div>
          )}
          <Probes probes={c.probes ?? []} />
          <Medidor etiqueta="CPU" recurso={c.recursos?.cpu} metricas={metricas} />
          <Medidor etiqueta="memoria" recurso={c.recursos?.memoria} metricas={metricas} />
        </div>
      ))}
      {!metricas && <SinMetricas contexto={foto.contexto} />}
    </Seccion>
  );
}

const tieneProbes = (c: Container) => (c.probes ?? []).some((p) => p.estado !== 'sin probe');
const tieneRecursos = (c: Container) =>
  [c.recursos?.cpu, c.recursos?.memoria].some((r) => r && (r.request || r.limit || r.consumo));

function Probes({ probes }: { probes: Probe[] }) {
  const configuradas = probes.filter((p) => p.estado !== 'sin probe');
  if (configuradas.length === 0) return <div className="probe"><Nada>sin probes</Nada></div>;
  return (
    <>
      {probes.map((p) =>
        p.estado === 'sin probe' ? (
          <div key={p.nombre} className="probe">
            <span className="probe-punto probe-sin-probe" />
            <b>{p.nombre}</b> <span className="tenue-texto">sin probe</span>
          </div>
        ) : (
          <div key={p.nombre} className="probe">
            <span className={`probe-punto probe-${p.estado}`} />
            <b>{p.nombre}</b> <span className="mono-texto">{p.config && accion(p.config)}</span> —{' '}
            <b className={`probe-estado-${p.estado}`}>{p.estado}</b>
            {p.config && <div className="probe-detalle tenue-texto">{tiempos(p.config)}</div>}
            {p.senales.length > 0 && <div className="probe-detalle">{p.senales.join(' · ')}</div>}
            {p.fallos.length > 0 && (
              <details className="probe-fallos">
                <summary>últimos fallos ({p.fallos.length})</summary>
                {p.fallos.map((f, i) => (
                  <div key={i} className="probe-fallo">
                    <span className="mono-texto tenue-texto">{hora(f.cuando)}</span> <b>{f.motivo}</b>
                    {f.veces > 1 && <span className="tenue-texto"> ×{f.veces}</span>}
                    <div className="event-mensaje">{f.mensaje}</div>
                  </div>
                ))}
              </details>
            )}
          </div>
        ),
      )}
    </>
  );
}

/** Qué hace la probe: "GET :8080/status", "TCP :http", "exec cat /tmp/listo", "gRPC :9090". */
function accion(c: ConfigDeProbe): string {
  switch (c.tipo) {
    case 'httpGet':
      return `${c.esquema === 'HTTPS' ? 'HTTPS ' : ''}GET ${c.host ?? ''}:${c.puerto}${c.path}`;
    case 'tcpSocket':
      return `TCP ${c.host ?? ''}:${c.puerto}`;
    case 'exec':
      return `exec ${(c.comando ?? []).join(' ')}`;
    case 'grpc':
      return `gRPC :${c.puerto}`;
  }
}

/** Delays y umbrales: "espera 15s · cada 10s · timeout 10s · cae tras 3 fallos". */
function tiempos(c: ConfigDeProbe): string {
  const partes = [];
  if (c.initialDelaySeconds > 0) partes.push(`espera ${c.initialDelaySeconds}s`);
  partes.push(`cada ${c.periodSeconds}s`, `timeout ${c.timeoutSeconds}s`, `cae tras ${c.failureThreshold} ${c.failureThreshold === 1 ? 'fallo' : 'fallos'}`);
  if (c.successThreshold > 1) partes.push(`vuelve tras ${c.successThreshold} éxitos`);
  return partes.join(' · ');
}

const hora = (cuando: string) => (cuando ? new Date(cuando).toLocaleTimeString() : '');

/**
 * Consumo contra request y limit: la barra llena es el limit (o el request, si no hay limit) y la
 * marca es el request.
 */
function Medidor({ etiqueta, recurso, metricas }: { etiqueta: string; recurso: Recurso | undefined; metricas: boolean }) {
  const { request, limit, consumo } = recurso ?? { request: null, limit: null, consumo: null };
  const tope = limit ?? request;
  const lleno = consumo && tope && tope.valor > 0 ? consumo.valor / tope.valor : null;
  const marca = request && limit && limit.valor > 0 ? request.valor / limit.valor : null;
  const tono = lleno === null ? '' : lleno >= 0.9 ? 'medidor-mal' : lleno >= 0.75 ? 'medidor-aviso' : '';
  return (
    <div className="medidor">
      <span className="medidor-etiqueta tenue-texto">{etiqueta}</span>
      <span className="medidor-barra">
        {lleno !== null && <span className={`medidor-relleno ${tono}`} style={{ width: `${Math.min(lleno, 1) * 100}%` }} />}
        {marca !== null && <span className="medidor-request" style={{ left: `${Math.min(marca, 1) * 100}%` }} title="request" />}
      </span>
      <span className="medidor-texto">
        {consumo ? <b>{consumo.texto}</b> : <Nada>{metricas ? 'midiendo…' : 'sin métricas'}</Nada>}
        <span className="tenue-texto">
          {' '}
          · req {request?.texto ?? '—'} · lim {limit?.texto ?? '—'}
        </span>
      </span>
    </div>
  );
}

/** Sin metrics-server: por qué no hay consumo y cómo instalarlo en kind. */
function SinMetricas({ contexto }: { contexto: string }) {
  const [comandos, setComandos] = useState<{ contexto: string; lista: ComandoSugerido[] } | null>(null);
  useEffect(() => {
    let vigente = true;
    pedirComandos({ tipo: 'metrics-server', namespace: '', nombre: '' })
      .then((lista) => vigente && setComandos({ contexto, lista }))
      .catch(() => vigente && setComandos({ contexto, lista: [] }));
    return () => {
      vigente = false;
    };
  }, [contexto]);
  return (
    <div className="sin-metricas">
      <div>
        <b>sin métricas</b>: el cluster no tiene metrics-server, así que no se ve el consumo real. Para instalarlo en kind:
      </div>
      {comandos?.contexto === contexto &&
        comandos.lista.map((c) => <Comando key={c.comando} comando={c.comando} descripcion={c.descripcion} />)}
    </div>
  );
}
