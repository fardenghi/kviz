// Costura 2: el recorrido de una request de prueba en la vista de topología. Lo que no participa se
// atenúa al 18 %, los pasos se numeran sobre cada objeto, las conexiones del recorrido toman su
// tono y el foco se apaga mientras hay un recorrido (prioridad: recorrido > foco > workload > reposo).
import { describe, expect, it } from 'vitest';
import type { Endpoint, Foto, Pod, Puerta, Service } from '../foto';
import type { Recorrido } from '../prueba';
import { construirEscena, type Escena, type EstadoInterfaz } from './escena';
import { OPACIDAD_ATENUADO } from './workloads';

const BACKEND = 'Deployment/default/backend';

const pod = (nombre: string, nodo: string, namespace = 'default', workload: string | null = null): Pod => ({
  id: `${namespace}/${nombre}`,
  nombre,
  namespace,
  nodo,
  ip: '10.244.0.1',
  fase: 'Running',
  estado: 'Running',
  containers: [],
  workload,
  hostPorts: [],
});
const endpoint = (pod: string, listo = true): Endpoint => ({ ip: '10.244.0.1', pod, nodo: null, listo });
const service = (nombre: string, endpoints: Endpoint[], namespace = 'default'): Service => ({
  id: `${namespace}/${nombre}`,
  nombre,
  namespace,
  tipo: 'NodePort',
  clusterIP: '10.96.11.121',
  puertos: [{ nombre: '', protocolo: 'TCP', puerto: 80, targetPort: 8081, nodePort: 30010 }],
  selector: { app: nombre },
  endpoints,
});
const nodePort = (nodo: string): Puerta => ({
  id: `${nodo}/30010/TCP`,
  nodo,
  puerto: 30010,
  protocolo: 'TCP',
  tipo: 'NodePort',
  namespace: 'default',
  service: 'backend',
  pod: null,
  haciaPuerto: 80,
});

/** backend con dos endpoints listos y uno no listo, su Ingress y su NodePort; db aparte. */
const guia = (): Foto => ({
  contexto: 'kind-lab',
  conectado: true,
  nodos: [
    { id: 'lab-worker', nombre: 'lab-worker', internalIP: '172.23.0.4' },
    { id: 'lab-worker2', nombre: 'lab-worker2', internalIP: '172.23.0.3' },
  ],
  pods: [
    pod('backend-a', 'lab-worker', 'default', BACKEND),
    pod('backend-b', 'lab-worker2', 'default', BACKEND),
    pod('backend-c', 'lab-worker2', 'default', BACKEND),
    pod('db-0', 'lab-worker'),
    pod('coredns-1', 'lab-worker', 'kube-system'),
  ],
  workloads: [{ id: BACKEND, tipo: 'Deployment', nombre: 'backend', namespace: 'default', color: '#0f9d8a', cadena: [] }],
  services: [
    service('backend', [endpoint('backend-a'), endpoint('backend-b'), endpoint('backend-c', false)]),
    service('db', [endpoint('db-0')]),
    service('kube-dns', [endpoint('coredns-1')], 'kube-system'),
  ],
  ingress: [
    {
      id: 'default/backend',
      nombre: 'backend',
      namespace: 'default',
      clase: 'nginx',
      clasePorDefecto: false,
      rewrite: null,
      reglas: [{ host: '', path: '/', tipoPath: 'Prefix', service: 'backend', puerto: 80 }],
    },
  ],
  puertas: [nodePort('lab-worker'), nodePort('lab-worker2')],
});

/** El recorrido básico de una request a backend que respondió backend-a. */
const recorrido: Recorrido = {
  pasos: [
    { n: 1, tipo: 'service', titulo: 'backend 10.96.11.121:80 → :8081', detalle: '', objetos: [{ tipo: 'service', id: 'default/backend' }] },
    {
      n: 2,
      tipo: 'endpoints',
      titulo: '…',
      detalle: '',
      objetos: [
        { tipo: 'pod', id: 'default/backend-a' },
        { tipo: 'pod', id: 'default/backend-b' },
      ],
    },
    { n: 3, tipo: 'pod', titulo: 'backend-a', detalle: '', objetos: [{ tipo: 'pod', id: 'default/backend-a' }] },
  ],
};

const SVC_BACKEND = 'service:default/backend';
const SVC_DB = 'service:default/db';
const ING_BACKEND = 'ingress:default/backend';
const POD_A = 'pod:default/backend-a';
const POD_B = 'pod:default/backend-b';
const POD_C = 'pod:default/backend-c';
const POD_DB = 'pod:default/db-0';

const conRecorrido = (estado: EstadoInterfaz = {}) => construirEscena(guia(), { recorrido, ...estado });
const objetos = (e: Escena) => [...e.pods, ...e.services, ...e.ingress, ...e.puertas];
const objeto = (e: Escena, id: string) => objetos(e).find((o) => o.id === id)!;
const tono = (e: Escena, desde: string, hasta: string) =>
  e.conexiones.find((c) => c.desde === desde && c.hasta === hasta)?.tono;

describe('recorrido en la escena', () => {
  it('numera los pasos sobre cada objeto que participa; un pod puede tener varios', () => {
    const e = conRecorrido();

    const pasos = Object.fromEntries(objetos(e).filter((o) => o.pasos.length > 0).map((o) => [o.id, o.pasos]));
    expect(pasos).toEqual({
      [SVC_BACKEND]: [1],
      [POD_A]: [2, 3],
      [POD_B]: [2],
    });
  });

  it('atenúa al 18 % lo que no participa y deja entero lo que sí', () => {
    const e = conRecorrido();

    for (const id of [SVC_BACKEND, POD_A, POD_B]) expect(objeto(e, id).opacidad, id).toBe(1);
    for (const id of [SVC_DB, ING_BACKEND, POD_C, POD_DB, 'puerta:lab-worker/30010']) {
      expect(objeto(e, id).opacidad, id).toBe(OPACIDAD_ATENUADO);
    }
    expect(OPACIDAD_ATENUADO).toBe(0.18);
  });

  it('las conexiones del recorrido toman su tono y las demás casi desaparecen', () => {
    const e = conRecorrido();

    expect(tono(e, SVC_BACKEND, POD_A)).toBe('recorrido');
    expect(tono(e, SVC_BACKEND, POD_B)).toBe('recorrido');
    // el endpoint no listo no recibe tráfico: no es parte del recorrido
    expect(tono(e, SVC_BACKEND, POD_C)).toBe('fantasma');
    expect(tono(e, ING_BACKEND, SVC_BACKEND)).toBe('fantasma');
    expect(tono(e, SVC_DB, POD_DB)).toBe('fantasma');
  });

  it('apaga el foco: ni apuntar ni seleccionar marcan relacionados mientras hay un recorrido', () => {
    const e = conRecorrido({ apuntado: SVC_DB, seleccionado: ING_BACKEND });

    expect(e.foco).toBeNull();
    expect(e.relacionados).toEqual([]);
    expect(e.conexiones.some((c) => c.tono === 'foco')).toBe(false);
    expect(objetos(e).some((o) => o.marcado)).toBe(false);
    // apuntar tampoco aclara: lo que participa sigue entero
    expect(objeto(e, POD_A).opacidad).toBe(1);
  });

  it('al cerrar el recorrido el foco vuelve a funcionar', () => {
    const e = construirEscena(guia(), { recorrido: null, apuntado: SVC_DB });

    expect(e.foco).toBe(SVC_DB);
    expect(tono(e, SVC_DB, POD_DB)).toBe('foco');
    expect(objetos(e).every((o) => o.pasos.length === 0)).toBe(true);
  });

  it('el recorrido manda sobre el workload seleccionado', () => {
    const e = conRecorrido({ workloadSeleccionado: BACKEND });

    expect(tono(e, SVC_BACKEND, POD_A)).toBe('recorrido');
    expect(tono(e, SVC_DB, POD_DB)).toBe('fantasma');
    expect(objeto(e, POD_C).opacidad).toBe(OPACIDAD_ATENUADO);
  });

  it('un recorrido sin pasos (destino fuera de la foto) no cambia la vista', () => {
    const e = construirEscena(guia(), { recorrido: { pasos: [] }, seleccionado: SVC_DB });

    expect(e.foco).toBe(SVC_DB);
    expect(objeto(e, POD_C).opacidad).toBe(1);
  });

  it('las puertas del recorrido muestran su línea al destino con el tono del recorrido', () => {
    const conPuerta: Recorrido = {
      pasos: [
        { n: 1, tipo: 'puerta', titulo: '', detalle: '', objetos: [{ tipo: 'puerta', id: 'lab-worker2/30010/TCP' }] },
        ...recorrido.pasos.map((p) => ({ ...p, n: p.n + 1 })),
      ],
    };
    const e = construirEscena(guia(), { recorrido: conPuerta });

    expect(objeto(e, 'puerta:lab-worker2/30010').pasos).toEqual([1]);
    const lineas = e.conexiones.filter((c) => c.dePuerta);
    expect(lineas.map((c) => [c.desde, c.hasta, c.tono])).toEqual([['puerta:lab-worker2/30010', SVC_BACKEND, 'recorrido']]);
  });

  it('los objetos del recorrido que la vista oculta no se dibujan', () => {
    const alDns: Recorrido = {
      pasos: [
        { n: 1, tipo: 'service', titulo: '', detalle: '', objetos: [{ tipo: 'service', id: 'kube-system/kube-dns' }] },
        { n: 2, tipo: 'endpoints', titulo: '', detalle: '', objetos: [{ tipo: 'pod', id: 'kube-system/coredns-1' }] },
      ],
    };
    const e = construirEscena(guia(), { recorrido: alDns });

    expect(objetos(e).filter((o) => o.pasos.length > 0)).toEqual([]);
    expect(objetos(e).every((o) => o.opacidad === OPACIDAD_ATENUADO)).toBe(true);
  });
});
