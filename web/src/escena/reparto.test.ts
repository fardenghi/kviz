// Costura 2: el reparto de una request de prueba repetida N veces se ve como una pastilla `n/N`
// sobre cada pod que respondió (no hay etiquetas de reparto sobre las líneas).
import { describe, expect, it } from 'vitest';
import type { Foto, Pod } from '../foto';
import type { Recorrido, Reparto } from '../prueba';
import { construirEscena } from './escena';

const pod = (nombre: string, nodo: string, namespace = 'default'): Pod => ({
  id: `${namespace}/${nombre}`,
  nombre,
  namespace,
  nodo,
  ip: '10.244.0.1',
  fase: 'Running',
  estado: 'Running',
  containers: [],
  workload: null,
  hostPorts: [],
});

const foto = (): Foto => ({
  contexto: 'kind-lab',
  conectado: true,
  nodos: [
    { id: 'lab-worker', nombre: 'lab-worker', internalIP: '172.23.0.4' },
    { id: 'lab-worker2', nombre: 'lab-worker2', internalIP: '172.23.0.3' },
  ],
  pods: [pod('backend-k2x8p', 'lab-worker'), pod('backend-7hw4q', 'lab-worker2'), pod('nginx', 'lab-worker'), pod('otro', 'lab-worker', 'oculto')],
  workloads: [],
  services: [
    {
      id: 'default/backend',
      nombre: 'backend',
      namespace: 'default',
      tipo: 'ClusterIP',
      clusterIP: '10.96.11.121',
      puertos: [{ nombre: '', protocolo: 'TCP', puerto: 80, targetPort: 8081 }],
      selector: { app: 'backend' },
      endpoints: [
        { ip: '10.244.1.5', pod: 'backend-k2x8p', nodo: 'lab-worker', listo: true },
        { ip: '10.244.2.4', pod: 'backend-7hw4q', nodo: 'lab-worker2', listo: true },
      ],
    },
  ],
  ingress: [],
  puertas: [],
});

const recorrido: Recorrido = {
  pasos: [
    { n: 1, tipo: 'service', titulo: '', detalle: '', objetos: [{ tipo: 'service', id: 'default/backend' }] },
    {
      n: 2,
      tipo: 'endpoints',
      titulo: '',
      detalle: '',
      objetos: [
        { tipo: 'pod', id: 'default/backend-k2x8p' },
        { tipo: 'pod', id: 'default/backend-7hw4q' },
      ],
    },
  ],
};

/** 8 requests: 5 a 7hw4q, 3 a k2x8p. */
const reparto: Reparto = {
  total: 8,
  pods: [
    { pod: 'default/backend-7hw4q', n: 5 },
    { pod: 'default/backend-k2x8p', n: 3 },
  ],
  sinIdentificar: 0,
  sinRespuesta: 0,
};

const pastillas = (f: Foto, r: Reparto | null, namespace?: string) =>
  Object.fromEntries(
    construirEscena(f, { recorrido, reparto: r, namespace })
      .pods.filter((p) => p.reparto)
      .map((p) => [p.id, p.reparto]),
  );

describe('reparto en la escena', () => {
  it('pone una pastilla n/N sobre cada pod que respondió', () => {
    expect(pastillas(foto(), reparto)).toEqual({
      'pod:default/backend-k2x8p': '3/8',
      'pod:default/backend-7hw4q': '5/8',
    });
  });

  it('las respuestas sin pod identificado no ponen pastilla en ningún pod', () => {
    const conDudas: Reparto = { total: 8, pods: [{ pod: 'default/backend-k2x8p', n: 3 }], sinIdentificar: 4, sinRespuesta: 1 };

    expect(pastillas(foto(), conDudas)).toEqual({ 'pod:default/backend-k2x8p': '3/8' });
  });

  it('sin reparto (una sola request) no hay pastillas', () => {
    expect(pastillas(foto(), null)).toEqual({});
  });

  it('un pod que la vista oculta no lleva pastilla y no rompe la escena', () => {
    const conOculto: Reparto = { total: 2, pods: [{ pod: 'oculto/otro', n: 2 }], sinIdentificar: 0, sinRespuesta: 0 };

    expect(pastillas(foto(), conOculto, 'default')).toEqual({});
  });

  it('no hay etiquetas de reparto sobre las líneas', () => {
    const e = construirEscena(foto(), { recorrido, reparto });

    for (const c of e.conexiones) expect(Object.keys(c)).not.toContain('reparto');
  });
});
