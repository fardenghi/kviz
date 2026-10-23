// Events: la franja de abajo (los últimos, con los filtros de namespace), el objeto que selecciona
// el clic en un event y los events del objeto del panel de detalle.
import { describe, expect, it } from 'vitest';
import type { EventDelCluster, Foto, ObjetoEnFoto } from './foto';
import { eventsDeLaFranja, eventsDelObjeto, seleccionDeEvent } from './events';

let n = 0;
const ev = (
  namespace: string,
  objeto: string,
  motivo: string,
  ultimaVez: string,
  enFoto: ObjetoEnFoto | null = null,
  tipoObjeto = 'Pod',
): EventDelCluster => ({
  id: `${namespace || 'default'}/${objeto}.${++n}`,
  tipo: 'Normal',
  motivo,
  mensaje: '',
  objeto: { tipo: tipoObjeto, namespace, nombre: objeto },
  enFoto,
  ultimaVez,
  veces: 1,
});

const foto = (events: EventDelCluster[]): Foto => ({
  contexto: 'kind-lab',
  conectado: true,
  nodos: [],
  pods: [],
  workloads: [],
  services: [],
  ingress: [],
  puertas: [],
  events: [...events].sort((a, b) => (a.id < b.id ? -1 : 1)),
});

const motivos = (es: EventDelCluster[]) => es.map((e) => e.motivo);

describe('franja de events', () => {
  it('muestra los últimos primero', () => {
    const f = foto([
      ev('default', 'ml-train', 'FailedScheduling', '2026-10-23T15:08:40Z'),
      ev('default', 'api', 'BackOff', '2026-10-23T15:10:31Z'),
      ev('default', 'report', 'Completed', '2026-10-23T15:02:11Z'),
    ]);

    expect(motivos(eventsDeLaFranja(f))).toEqual(['BackOff', 'FailedScheduling', 'Completed']);
  });

  it('muestra solo los más recientes cuando hay muchos', () => {
    const muchos = Array.from({ length: 80 }, (_, i) =>
      ev('default', 'web', `m${i}`, `2026-10-23T15:${String(Math.floor(i / 2)).padStart(2, '0')}:${i % 2 ? '30' : '00'}Z`),
    );

    const franja = eventsDeLaFranja(foto(muchos));

    expect(franja).toHaveLength(50);
    expect(franja[0].motivo).toBe('m79');
    expect(franja[49].motivo).toBe('m30');
  });

  it('oculta los events de los namespaces del sistema salvo con el interruptor, y los de kviz-system siempre', () => {
    const f = foto([
      ev('default', 'web', 'Pulled', '2026-10-23T15:00:01Z'),
      ev('kube-system', 'coredns', 'Started', '2026-10-23T15:00:02Z'),
      ev('kviz-system', 'kviz-sondeo', 'Created', '2026-10-23T15:00:03Z'),
    ]);

    expect(motivos(eventsDeLaFranja(f))).toEqual(['Pulled']);
    expect(motivos(eventsDeLaFranja(f, { mostrarSistema: true }))).toEqual(['Started', 'Pulled']);
  });

  it('con un namespace elegido muestra solo sus events', () => {
    const f = foto([
      ev('default', 'web', 'Pulled', '2026-10-23T15:00:01Z'),
      ev('tienda', 'api', 'BackOff', '2026-10-23T15:00:02Z'),
    ]);

    expect(motivos(eventsDeLaFranja(f, { namespace: 'default' }))).toEqual(['Pulled']);
  });

  it('los events de un nodo se ven siempre: el nodo no es de ningún namespace', () => {
    const f = foto([ev('', 'lab-worker', 'NodeReady', '2026-10-23T15:00:01Z', null, 'Node')]);

    expect(motivos(eventsDeLaFranja(f, { namespace: 'default' }))).toEqual(['NodeReady']);
  });

  it('sin events en la foto no muestra ninguno', () => {
    expect(eventsDeLaFranja({ ...foto([]), events: undefined })).toEqual([]);
  });
});

describe('clic en un event', () => {
  it('selecciona el objeto de la foto al que se refiere, con el mismo id que el foco y el panel', () => {
    const t = '2026-10-23T15:00:00Z';
    expect(seleccionDeEvent(ev('default', 'api', 'BackOff', t, { seccion: 'pods', id: 'default/api' }))).toBe('pod:default/api');
    expect(seleccionDeEvent(ev('', 'lab-worker', 'NodeReady', t, { seccion: 'nodos', id: 'lab-worker' }, 'Node'))).toBe('nodo:lab-worker');
    expect(seleccionDeEvent(ev('default', 'web', 'X', t, { seccion: 'services', id: 'default/web' }, 'Service'))).toBe('service:default/web');
    expect(seleccionDeEvent(ev('default', 'web', 'Sync', t, { seccion: 'ingress', id: 'default/web' }, 'Ingress'))).toBe('ingress:default/web');
    expect(
      seleccionDeEvent(ev('default', 'web-6c7d9f8b5', 'SuccessfulCreate', t, { seccion: 'workloads', id: 'Deployment/default/web' }, 'ReplicaSet')),
    ).toBe('workload:Deployment/default/web');
  });

  it('no selecciona nada si el objeto ya no está en la foto', () => {
    expect(seleccionDeEvent(ev('default', 'web-5d8f7c9b4-qz7lm', 'Killing', '2026-10-23T15:00:00Z'))).toBeNull();
  });
});

describe('events del objeto en el panel de detalle', () => {
  const api = { seccion: 'pods', id: 'default/api' } as const;
  const web = { seccion: 'workloads', id: 'Deployment/default/web' } as const;
  const f = foto([
    ev('default', 'api', 'Unhealthy', '2026-10-23T15:10:02Z', api),
    ev('default', 'api', 'BackOff', '2026-10-23T15:10:31Z', api),
    ev('default', 'web', 'ScalingReplicaSet', '2026-10-23T15:12:20Z', web, 'Deployment'),
    ev('default', 'web-6c7d9f8b5', 'SuccessfulCreate', '2026-10-23T15:12:21Z', web, 'ReplicaSet'),
    // otro pod con el mismo nombre en otro namespace
    ev('tienda', 'api', 'Pulled', '2026-10-23T15:13:00Z', { seccion: 'pods', id: 'tienda/api' }),
  ]);

  it('muestra los events del pod, del más reciente al más viejo', () => {
    const pod = { id: 'default/api', nombre: 'api', namespace: 'default', nodo: null, ip: null, fase: 'Running', estado: 'Running', containers: [], workload: null, hostPorts: [] };

    expect(motivos(eventsDelObjeto(f, { tipo: 'Pod', pod }))).toEqual(['BackOff', 'Unhealthy']);
  });

  it('en un workload junta los suyos y los de sus ReplicaSets o Jobs', () => {
    const workload = { id: 'Deployment/default/web', tipo: 'Deployment' as const, nombre: 'web', namespace: 'default', color: '#0f9d8a', cadena: [] };

    expect(motivos(eventsDelObjeto(f, { tipo: 'Workload', workload }))).toEqual(['SuccessfulCreate', 'ScalingReplicaSet']);
  });

  it('un objeto sin events no muestra ninguno', () => {
    const nodo = { id: 'lab-worker', nombre: 'lab-worker', internalIP: '172.23.0.4' };

    expect(eventsDelObjeto(f, { tipo: 'Nodo', nodo })).toEqual([]);
  });
});
