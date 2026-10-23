// Costura 2: foco en la vista de topología (relacionados, tonos de conexión, líneas de puerta,
// aclarado por hover) y su prioridad sobre el workload seleccionado.
import { describe, expect, it } from 'vitest';
import type { Endpoint, Foto, Pod, Puerta, Service } from '../foto';
import { construirEscena, type Escena, type EstadoInterfaz } from './escena';

const BACKEND = 'Deployment/default/backend';
const DB = 'StatefulSet/default/db';

const nodo = (nombre: string) => ({ id: nombre, nombre, internalIP: '172.23.0.2' });
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
const service = (nombre: string, endpoints: Endpoint[], parte: Partial<Service> = {}): Service => ({
  id: `default/${nombre}`,
  nombre,
  namespace: 'default',
  tipo: 'ClusterIP',
  clusterIP: '10.96.0.10',
  puertos: [{ nombre: '', protocolo: 'TCP', puerto: 80, targetPort: 8080 }],
  selector: { app: nombre },
  endpoints,
  ...parte,
});
const nodePort = (nodo: string, puerto: number, service: string): Puerta => ({
  id: `${nodo}/${puerto}/TCP`,
  nodo,
  puerto,
  protocolo: 'TCP',
  tipo: 'NodePort',
  namespace: 'default',
  service,
  pod: null,
  haciaPuerto: 80,
});
const hostPort = (nodo: string, puerto: number, pod: string): Puerta => ({
  id: `${nodo}/${puerto}/TCP`,
  nodo,
  puerto,
  protocolo: 'TCP',
  tipo: 'hostPort',
  namespace: 'ingress-nginx',
  service: null,
  pod,
  haciaPuerto: puerto,
});

/**
 * El cluster de la guía: backend NodePort 30010 con 2 pods listos y uno no listo, Ingress backend,
 * IC con hostPort 80 y 443 en lab-worker2, y aparte un Service db (sin Ingress ni puertas) con su pod.
 */
const guia = (): Foto => ({
  contexto: 'kind-lab',
  conectado: true,
  nodos: [nodo('lab-control-plane'), nodo('lab-worker'), nodo('lab-worker2')],
  pods: [
    pod('backend-a', 'lab-worker', 'default', BACKEND),
    pod('backend-b', 'lab-worker2', 'default', BACKEND),
    pod('backend-c', 'lab-worker2', 'default', BACKEND),
    pod('db-0', 'lab-worker', 'default', DB),
    pod('ingress-nginx-controller', 'lab-worker2', 'ingress-nginx'),
  ],
  workloads: [
    { id: BACKEND, tipo: 'Deployment', nombre: 'backend', namespace: 'default', color: '#0f9d8a', cadena: [] },
    { id: DB, tipo: 'StatefulSet', nombre: 'db', namespace: 'default', color: '#8a4fd1', cadena: [] },
  ],
  services: [
    service('backend', [endpoint('backend-a'), endpoint('backend-b'), endpoint('backend-c', false)], {
      tipo: 'NodePort',
      puertos: [{ nombre: '', protocolo: 'TCP', puerto: 80, targetPort: 8081, nodePort: 30010 }],
    }),
    service('db', [endpoint('db-0')]),
  ],
  ingress: [
    {
      id: 'default/backend',
      nombre: 'backend',
      namespace: 'default',
      clase: 'nginx',
      rewrite: '/$2',
      reglas: [{ host: '', path: '/backend(/|$)(.*)', tipoPath: 'ImplementationSpecific', service: 'backend', puerto: 80 }],
    },
  ],
  puertas: [
    nodePort('lab-control-plane', 30010, 'backend'),
    nodePort('lab-worker', 30010, 'backend'),
    nodePort('lab-worker2', 30010, 'backend'),
    hostPort('lab-worker2', 443, 'ingress-nginx-controller'),
    hostPort('lab-worker2', 80, 'ingress-nginx-controller'),
  ],
});

// el IC es del sistema: el interruptor lo muestra (sus puertas hostPort se ven igual)
const conFoco = (estado: EstadoInterfaz) => construirEscena(guia(), { mostrarSistema: true, ...estado });

const SVC_BACKEND = 'service:default/backend';
const SVC_DB = 'service:default/db';
const ING_BACKEND = 'ingress:default/backend';
const POD_A = 'pod:default/backend-a';
const POD_B = 'pod:default/backend-b';
const POD_C = 'pod:default/backend-c';
const POD_DB = 'pod:default/db-0';
const POD_IC = 'pod:ingress-nginx/ingress-nginx-controller';
const NODEPORTS = ['puerta:lab-control-plane/30010', 'puerta:lab-worker/30010', 'puerta:lab-worker2/30010'];
const HOSTPORTS = ['puerta:lab-worker2/443', 'puerta:lab-worker2/80'];

const ordenados = (ids: string[]) => [...ids].sort();
const tono = (e: Escena, desde: string, hasta: string) =>
  e.conexiones.find((c) => c.desde === desde && c.hasta === hasta)?.tono;
const lineasDePuerta = (e: Escena) => e.conexiones.filter((c) => c.desde.startsWith('puerta:'));

describe('relacionados con el foco', () => {
  it('un Service arrastra su Ingress, sus endpoints listos y no listos y sus puertas NodePort', () => {
    const escena = conFoco({ hover: SVC_BACKEND });

    expect(escena.foco).toBe(SVC_BACKEND);
    expect(escena.relacionados).toEqual(ordenados([SVC_BACKEND, ING_BACKEND, POD_A, POD_B, POD_C, ...NODEPORTS]));
  });

  it('un pod arrastra sus Services y los Ingress y puertas de esos Services, sin los pods hermanos', () => {
    const escena = conFoco({ hover: POD_A });

    expect(escena.relacionados).toEqual(ordenados([POD_A, SVC_BACKEND, ING_BACKEND, ...NODEPORTS]));
  });

  it('un pod con hostPort arrastra sus puertas hostPort', () => {
    const escena = conFoco({ hover: POD_IC });

    expect(escena.relacionados).toEqual(ordenados([POD_IC, ...HOSTPORTS]));
  });

  it('un Ingress arrastra su Service y lo que ese Service arrastra', () => {
    const escena = conFoco({ hover: ING_BACKEND });

    expect(escena.relacionados).toEqual(ordenados([ING_BACKEND, SVC_BACKEND, POD_A, POD_B, POD_C, ...NODEPORTS]));
  });

  it('una puerta NodePort arrastra su Service y lo que ese Service arrastra', () => {
    const escena = conFoco({ hover: 'puerta:lab-worker/30010' });

    expect(escena.relacionados).toEqual(ordenados([SVC_BACKEND, ING_BACKEND, POD_A, POD_B, POD_C, ...NODEPORTS]));
  });

  it('una puerta hostPort arrastra solo su pod', () => {
    const escena = conFoco({ hover: 'puerta:lab-worker2/80' });

    expect(escena.relacionados).toEqual(ordenados(['puerta:lab-worker2/80', POD_IC]));
  });

  it('seleccionar un objeto también es foco, pero el hover manda sobre la selección', () => {
    expect(conFoco({ seleccionado: SVC_DB }).relacionados).toEqual(ordenados([SVC_DB, POD_DB]));
    expect(conFoco({ seleccionado: SVC_DB, hover: POD_DB }).foco).toBe(POD_DB);
  });

  it('seleccionar un nodo o un workload no es foco', () => {
    for (const seleccionado of ['nodo:lab-worker', BACKEND]) {
      const escena = conFoco({ seleccionado });
      expect(escena.foco).toBeNull();
      expect(escena.relacionados).toEqual([]);
    }
  });

  it('un objeto que no está en la escena no es foco', () => {
    const escena = construirEscena(guia(), { hover: POD_IC }); // el sistema está oculto

    expect(escena.foco).toBeNull();
    expect(escena.conexiones.map((c) => c.tono)).not.toContain('fantasma');
  });
});

describe('tono de las conexiones', () => {
  it('con foco, las conexiones entre relacionados son "foco" y las demás "fantasma"', () => {
    const escena = conFoco({ hover: POD_A });

    expect(tono(escena, ING_BACKEND, SVC_BACKEND)).toBe('foco');
    expect(tono(escena, SVC_BACKEND, POD_A)).toBe('foco');
    // los pods hermanos no están relacionados con el pod
    expect(tono(escena, SVC_BACKEND, POD_B)).toBe('fantasma');
    expect(tono(escena, SVC_BACKEND, POD_C)).toBe('fantasma');
    expect(tono(escena, SVC_DB, POD_DB)).toBe('fantasma');
  });

  it('con foco en un Service, también su endpoint no listo va en "foco"', () => {
    const escena = conFoco({ seleccionado: SVC_BACKEND });

    expect(tono(escena, SVC_BACKEND, POD_C)).toBe('foco');
    expect(tono(escena, SVC_DB, POD_DB)).toBe('fantasma');
  });

  it('sin foco ni workload, las conexiones quedan en reposo o no-listo', () => {
    const escena = conFoco({});

    expect(tono(escena, ING_BACKEND, SVC_BACKEND)).toBe('reposo');
    expect(tono(escena, SVC_BACKEND, POD_A)).toBe('reposo');
    expect(tono(escena, SVC_BACKEND, POD_C)).toBe('no-listo');
  });

  it('con un workload seleccionado, solo siguen en reposo las conexiones a sus pods', () => {
    const escena = conFoco({ workloadSeleccionado: BACKEND });

    expect(tono(escena, SVC_BACKEND, POD_A)).toBe('reposo');
    expect(tono(escena, SVC_BACKEND, POD_C)).toBe('no-listo');
    expect(tono(escena, SVC_DB, POD_DB)).toBe('fantasma');
    expect(tono(escena, ING_BACKEND, SVC_BACKEND)).toBe('fantasma');
  });

  it('el foco tiene prioridad sobre el workload seleccionado', () => {
    const escena = conFoco({ workloadSeleccionado: BACKEND, hover: SVC_DB });

    expect(tono(escena, SVC_DB, POD_DB)).toBe('foco');
    expect(tono(escena, SVC_BACKEND, POD_A)).toBe('fantasma');
    expect(tono(escena, ING_BACKEND, SVC_BACKEND)).toBe('fantasma');
  });
});

describe('líneas de puerta', () => {
  it('sin foco no se dibujan, ni con un workload seleccionado', () => {
    expect(lineasDePuerta(conFoco({}))).toEqual([]);
    expect(lineasDePuerta(conFoco({ workloadSeleccionado: BACKEND }))).toEqual([]);
  });

  it('con foco en un Service van de cada puerta NodePort al borde de abajo del Service, en tono "foco"', () => {
    const escena = conFoco({ hover: SVC_BACKEND });
    const svc = escena.services.find((s) => s.id === SVC_BACKEND)!;

    const lineas = lineasDePuerta(escena);
    expect(ordenados(lineas.map((c) => c.desde))).toEqual(ordenados(NODEPORTS));
    for (const c of lineas) {
      const puerta = escena.puertas.find((p) => p.id === c.desde)!;
      expect(c).toMatchObject({ hasta: SVC_BACKEND, tono: 'foco', dePuerta: true });
      expect(c.inicio).toEqual({ x: puerta.x + puerta.ancho / 2, y: puerta.y });
      expect(c.fin.y).toBe(svc.y + svc.alto);
    }
  });

  it('con foco en un pod aparecen las de sus Services', () => {
    expect(ordenados(lineasDePuerta(conFoco({ seleccionado: POD_B })).map((c) => c.desde))).toEqual(ordenados(NODEPORTS));
  });

  it('con foco en un pod con hostPort van de sus puertas al pod', () => {
    const lineas = lineasDePuerta(conFoco({ hover: POD_IC }));

    expect(ordenados(lineas.map((c) => c.desde))).toEqual(ordenados(HOSTPORTS));
    for (const c of lineas) expect(c).toMatchObject({ hasta: POD_IC, tono: 'foco' });
  });

  it('con foco en algo sin relación con las puertas no aparece ninguna', () => {
    expect(lineasDePuerta(conFoco({ hover: SVC_DB }))).toEqual([]);
  });
});

/** Opacidad de cada objeto de la escena (pods, Services, Ingress y puertas), por id. */
const opacidades = (e: Escena) =>
  Object.fromEntries([...e.pods, ...e.services, ...e.ingress, ...e.puertas].map((o) => [o.id, o.opacidad]));
const resaltados = (e: Escena) =>
  ordenados([...e.pods, ...e.services, ...e.ingress, ...e.puertas].filter((o) => o.resaltado).map((o) => o.id));

describe('objetos con foco', () => {
  it('con hover, los objetos no relacionados se aclaran', () => {
    const op = opacidades(conFoco({ hover: SVC_DB }));

    expect(op[SVC_DB]).toBe(1);
    expect(op[POD_DB]).toBe(1);
    for (const id of [SVC_BACKEND, ING_BACKEND, POD_A, POD_C, POD_IC, ...NODEPORTS, ...HOSTPORTS]) expect(op[id]).toBe(0.45);
  });

  it('con selección no se aclara nada', () => {
    const op = opacidades(conFoco({ seleccionado: SVC_DB }));

    expect(Object.values(op).every((o) => o === 1)).toBe(true);
  });

  it('el aclarado del hover se combina con el atenuado del workload seleccionado', () => {
    const op = opacidades(conFoco({ hover: SVC_BACKEND, workloadSeleccionado: DB }));

    expect(op[POD_A]).toBe(0.18); // de otro workload, aunque esté relacionado
    expect(op[POD_DB]).toBe(0.45); // del workload pero sin relación con el foco
    expect(op[SVC_BACKEND]).toBe(1);
  });

  it('con hover se resaltan el foco y sus relacionados; las puertas relacionadas se rellenan', () => {
    expect(resaltados(conFoco({ hover: SVC_BACKEND }))).toEqual(
      ordenados([SVC_BACKEND, ING_BACKEND, POD_A, POD_B, POD_C, ...NODEPORTS]),
    );
  });

  it('con selección se resaltan el foco y las puertas relacionadas', () => {
    expect(resaltados(conFoco({ seleccionado: POD_A }))).toEqual(ordenados([POD_A, ...NODEPORTS]));
  });
});
