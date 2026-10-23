// Costura 2: marcas de workload en las tarjetas de pod y selección de un workload.
import { describe, expect, it } from 'vitest';
import type { Foto, Pod, Workload } from '../foto';
import { construirEscena, workloadsVisibles } from './escena';
import { textoDeCadena } from './workloads';

const AZUL = '#0f9d8a';
const VIOLETA = '#8a4fd1';
const OLIVA = '#5d7a3a';
const DORADO = '#b8860b';
const GRIS = '#7b8794';

const pod = (namespace: string, nombre: string, nodo: string, workload: string | null = null): Pod => ({
  nombre,
  namespace,
  nodo,
  ip: '10.244.1.1',
  fase: 'Running',
  estado: 'Running',
  containers: [],
  workload,
});

const workload = (tipo: Workload['tipo'], namespace: string, nombre: string, color: string, cadena: Workload['cadena']): Workload => ({
  id: `${tipo}/${namespace}/${nombre}`,
  tipo,
  nombre,
  namespace,
  color,
  cadena,
});

const BACKEND = 'Deployment/default/backend';
const DB = 'StatefulSet/default/db';
const LOG_AGENT = 'DaemonSet/observabilidad/log-agent';
const BACKUP = 'CronJob/default/backup';
const COREDNS = 'Deployment/kube-system/coredns';

// el cluster de la guía con un rollout de backend, un StatefulSet de 2, un DaemonSet de 1 pod por
// nodo, un CronJob con 2 Jobs y un Pod suelto
const lab = (): Foto => ({
  contexto: 'kind-lab',
  conectado: true,
  services: [],
  nodos: [
    { nombre: 'lab-worker', internalIP: '172.23.0.4' },
    { nombre: 'lab-worker2', internalIP: '172.23.0.3' },
  ],
  pods: [
    pod('default', 'backend-6c7d9f8b5-p8r2c', 'lab-worker', BACKEND),
    pod('default', 'backend-6c7d9f8b5-mn3vt', 'lab-worker2', BACKEND),
    pod('default', 'backend-5d8f7c9b4-h4j2w', 'lab-worker2', BACKEND),
    pod('default', 'db-0', 'lab-worker', DB),
    pod('default', 'db-1', 'lab-worker2', DB),
    pod('observabilidad', 'log-agent-r7tfm', 'lab-worker', LOG_AGENT),
    pod('observabilidad', 'log-agent-wd94k', 'lab-worker2', LOG_AGENT),
    pod('default', 'backup-28745130-7xk2d', 'lab-worker', BACKUP),
    pod('default', 'backup-28745160-c4mfn', 'lab-worker', BACKUP),
    pod('default', 'shell', 'lab-worker2'),
    pod('kube-system', 'coredns-7db6d8ff4d-5slkq', 'lab-worker', COREDNS),
  ],
  workloads: [
    workload('CronJob', 'default', 'backup', DORADO, [
      { tipo: 'CronJob', nombre: 'backup', pods: [] },
      { tipo: 'Job', nombre: 'backup-28745160', pods: ['backup-28745160-c4mfn'] },
      { tipo: 'Job', nombre: 'backup-28745130', pods: ['backup-28745130-7xk2d'] },
    ]),
    workload('Deployment', 'default', 'backend', AZUL, [
      { tipo: 'Deployment', nombre: 'backend', pods: [] },
      { tipo: 'ReplicaSet', nombre: 'backend-6c7d9f8b5', pods: ['backend-6c7d9f8b5-mn3vt', 'backend-6c7d9f8b5-p8r2c'] },
      { tipo: 'ReplicaSet', nombre: 'backend-5d8f7c9b4', pods: ['backend-5d8f7c9b4-h4j2w'] },
    ]),
    workload('StatefulSet', 'default', 'db', VIOLETA, [{ tipo: 'StatefulSet', nombre: 'db', pods: ['db-0', 'db-1'] }]),
    workload('Deployment', 'kube-system', 'coredns', GRIS, [
      { tipo: 'Deployment', nombre: 'coredns', pods: [] },
      { tipo: 'ReplicaSet', nombre: 'coredns-7db6d8ff4d', pods: ['coredns-7db6d8ff4d-5slkq'] },
    ]),
    workload('DaemonSet', 'observabilidad', 'log-agent', OLIVA, [
      { tipo: 'DaemonSet', nombre: 'log-agent', pods: ['log-agent-r7tfm', 'log-agent-wd94k'] },
    ]),
  ],
});

const podEnEscena = (escena: ReturnType<typeof construirEscena>, nombre: string) => escena.pods.find((p) => p.nombre === nombre)!;

describe('marca de workload en la tarjeta del pod', () => {
  it('lleva la etiqueta corta y el color de su workload', () => {
    const escena = construirEscena(lab());

    const marca = (nombre: string) => podEnEscena(escena, nombre).workload;
    expect(marca('backend-5d8f7c9b4-h4j2w')).toEqual({ etiqueta: 'deploy backend', color: AZUL });
    expect(marca('db-1')).toEqual({ etiqueta: 'sts db', color: VIOLETA });
    expect(marca('log-agent-r7tfm')).toEqual({ etiqueta: 'ds log-agent', color: OLIVA });
    expect(marca('backup-28745130-7xk2d')).toEqual({ etiqueta: 'cronjob backup', color: DORADO });
  });

  it('un pod sin dueño dice "Pod suelto" en gris', () => {
    const escena = construirEscena(lab());

    expect(podEnEscena(escena, 'shell').workload).toEqual({ etiqueta: 'Pod suelto', color: '#9aa3ad' });
  });

  it('un Job o un ReplicaSet sin dueño usan su propia etiqueta', () => {
    const f = lab();
    f.workloads.push(
      workload('Job', 'default', 'migracion', AZUL, [{ tipo: 'Job', nombre: 'migracion', pods: ['migracion-x2k9q'] }]),
      workload('ReplicaSet', 'default', 'cache', AZUL, [{ tipo: 'ReplicaSet', nombre: 'cache', pods: ['cache-7tq2p'] }]),
    );
    f.pods.push(pod('default', 'migracion-x2k9q', 'lab-worker', 'Job/default/migracion'));
    f.pods.push(pod('default', 'cache-7tq2p', 'lab-worker', 'ReplicaSet/default/cache'));

    const escena = construirEscena(f);

    expect(podEnEscena(escena, 'migracion-x2k9q').workload.etiqueta).toBe('job migracion');
    expect(podEnEscena(escena, 'cache-7tq2p').workload.etiqueta).toBe('rs cache');
  });
});

describe('seleccionar un workload', () => {
  it('sus pods quedan con el borde de su color en todos los nodos y el resto se atenúa al 18 %', () => {
    const escena = construirEscena(lab(), { workloadSeleccionado: BACKEND });

    const suyos = escena.pods.filter((p) => p.nombre.startsWith('backend-'));
    // los dos ReplicaSets del rollout, repartidos en los dos nodos
    expect(suyos.map((p) => p.nombre).sort()).toEqual(['backend-5d8f7c9b4-h4j2w', 'backend-6c7d9f8b5-mn3vt', 'backend-6c7d9f8b5-p8r2c']);
    expect(new Set(suyos.map((p) => p.region))).toEqual(new Set(['nodo:lab-worker', 'nodo:lab-worker2']));
    for (const p of suyos) expect(p).toMatchObject({ bordeWorkload: AZUL, opacidad: 1 });
    for (const p of escena.pods.filter((p) => !p.nombre.startsWith('backend-'))) {
      expect(p).toMatchObject({ bordeWorkload: null, opacidad: 0.18 });
    }
  });

  it('sin workload seleccionado no hay bordes de workload ni pods atenuados', () => {
    const escena = construirEscena(lab());

    for (const p of escena.pods) expect(p).toMatchObject({ bordeWorkload: null, opacidad: 1 });
  });

  it('el Pod suelto se atenúa como cualquier otro pod ajeno', () => {
    const escena = construirEscena(lab(), { workloadSeleccionado: DB });

    expect(podEnEscena(escena, 'shell')).toMatchObject({ bordeWorkload: null, opacidad: 0.18 });
    expect(podEnEscena(escena, 'db-0')).toMatchObject({ bordeWorkload: VIOLETA, opacidad: 1 });
  });

  it('no es foco: no cambia el lugar ni la marca de ningún pod, solo bordes y atenuación', () => {
    const sinSeleccion = construirEscena(lab());
    const conSeleccion = construirEscena(lab(), { workloadSeleccionado: LOG_AGENT });

    const sinResalte = (e: typeof sinSeleccion) => ({
      ...e,
      pods: e.pods.map(({ bordeWorkload: _b, opacidad: _o, ...resto }) => resto),
    });
    expect(sinResalte(conSeleccion)).toEqual(sinResalte(sinSeleccion));
  });

  it('si el workload seleccionado desaparece de la foto, no atenúa nada', () => {
    const escena = construirEscena(lab(), { workloadSeleccionado: 'Deployment/default/borrado' });

    for (const p of escena.pods) expect(p.opacidad).toBe(1);
  });
});

describe('lista de workloads', () => {
  const ids = (ws: { id: string }[]) => ws.map((w) => w.id);

  it('sigue los filtros de la vista: oculta los workloads de los namespaces del sistema', () => {
    expect(ids(workloadsVisibles(lab()))).toEqual([BACKUP, BACKEND, DB, LOG_AGENT]);
    expect(ids(workloadsVisibles(lab(), { mostrarSistema: true }))).toEqual([BACKUP, BACKEND, DB, COREDNS, LOG_AGENT]);
  });

  it('con el filtro por namespace muestra solo los de ese namespace', () => {
    expect(ids(workloadsVisibles(lab(), { namespace: 'observabilidad' }))).toEqual([LOG_AGENT]);
  });
});

describe('cadena del workload seleccionado', () => {
  const de = (id: string) => lab().workloads.find((w) => w.id === id)!;

  it('durante un rollout muestra los dos ReplicaSets, cada uno con sus pods', () => {
    expect(textoDeCadena(de(BACKEND))).toEqual({
      resumen: 'Deployment → ReplicaSet ×2 → Pod ×3',
      eslabones: ['ReplicaSet backend-6c7d9f8b5 · Pod ×2', 'ReplicaSet backend-5d8f7c9b4 · Pod ×1'],
    });
  });

  it('un CronJob muestra sus Jobs', () => {
    expect(textoDeCadena(de(BACKUP))).toEqual({
      resumen: 'CronJob → Job ×2 → Pod ×2',
      eslabones: ['Job backup-28745160 · Pod ×1', 'Job backup-28745130 · Pod ×1'],
    });
  });

  it('un StatefulSet o un DaemonSet son dueños directos de sus pods', () => {
    expect(textoDeCadena(de(DB))).toEqual({ resumen: 'StatefulSet → Pod ×2', eslabones: [] });
    expect(textoDeCadena(de(COREDNS))).toEqual({
      resumen: 'Deployment → ReplicaSet → Pod ×1',
      eslabones: ['ReplicaSet coredns-7db6d8ff4d · Pod ×1'],
    });
  });
});
