// Costura 2: marcas por pod en la tarjeta (ticket 03): estado si no es Running, reinicios si hay
// y los containers como chips según su tipo.
import { describe, expect, it } from 'vitest';
import type { Container, Foto, Pod } from '../foto';
import { construirEscena, type PodEnEscena } from './escena';

const container = (nombre: string, parte: Partial<Container> = {}): Container => ({
  nombre,
  tipo: 'normal',
  imagen: `${nombre}:1`,
  puertos: [],
  estado: 'Running',
  ready: true,
  started: true,
  reinicios: 0,
  ...parte,
});
const pod = (nombre: string, parte: Partial<Pod> = {}): Pod => ({
  id: `default/${nombre}`,
  nombre,
  namespace: 'default',
  nodo: 'lab-worker',
  ip: '10.244.1.5',
  fase: 'Running',
  estado: 'Running',
  containers: [container('app')],
  workload: null,
  hostPorts: [],
  ...parte,
});
const foto = (...pods: Pod[]): Foto => ({
  contexto: 'kind-lab',
  conectado: true,
  nodos: [{ id: 'lab-worker', nombre: 'lab-worker', internalIP: '172.23.0.4' }],
  pods,
  services: [],
  workloads: [],
  ingress: [],
  puertas: [],
});
const tarjeta = (escena: { pods: PodEnEscena[] }, nombre: string) => escena.pods.find((p) => p.nombre === nombre)!;

describe('estado del pod', () => {
  it('no escribe el estado de un pod Running', () => {
    const escena = construirEscena(foto(pod('nginx')));

    expect(tarjeta(escena, 'nginx').estado).toBeNull();
  });

  it('muestra el estado cuando no es Running', () => {
    const escena = construirEscena(
      foto(
        pod('api-5d8f', { estado: 'CrashLoopBackOff' }),
        pod('report-2891', { fase: 'Succeeded', estado: 'Completed' }),
        pod('ml-train', { nodo: null, ip: null, fase: 'Pending', estado: 'Pending' }),
      ),
    );

    expect(tarjeta(escena, 'api-5d8f').estado).toBe('CrashLoopBackOff');
    expect(tarjeta(escena, 'report-2891').estado).toBe('Completed');
    expect(tarjeta(escena, 'ml-train').estado).toBe('Pending');
  });

  it('colorea el estado: mal si falla, aviso si espera, terminado si ya no corre', () => {
    const tonos = {
      CrashLoopBackOff: 'mal',
      ImagePullBackOff: 'mal',
      'Init:Error': 'mal',
      OOMKilled: 'mal',
      Pending: 'aviso',
      ContainerCreating: 'aviso',
      'Init:0/1': 'aviso',
      NotReady: 'aviso',
      Completed: 'terminado',
      Terminating: 'terminado',
    };
    const escena = construirEscena(foto(...Object.keys(tonos).map((estado, i) => pod(`p${i}`, { estado }))));

    expect(Object.fromEntries(escena.pods.map((p) => [p.estado, p.tonoEstado]))).toEqual(tonos);
  });
});

describe('reinicios', () => {
  it('no aparecen si el pod nunca se reinició', () => {
    const escena = construirEscena(foto(pod('nginx')));

    expect(tarjeta(escena, 'nginx').reinicios).toBeNull();
  });

  it('suman los reinicios de todos los containers del pod', () => {
    const escena = construirEscena(
      foto(
        pod('api-5d8f', {
          estado: 'CrashLoopBackOff',
          containers: [
            container('api', { estado: 'CrashLoopBackOff', ready: false, reinicios: 7 }),
            container('envoy', { reinicios: 1 }),
          ],
        }),
      ),
    );

    expect(tarjeta(escena, 'api-5d8f').reinicios).toBe(8);
  });

  const shell = (reinicios: number) =>
    pod('shell', { containers: [container('shell', { imagen: 'busybox:latest', reinicios })] });

  it('titilan cuando aumentan respecto de la foto anterior', () => {
    const escena = construirEscena(foto(shell(4)), { fotoAnterior: foto(shell(3)) });

    expect(tarjeta(escena, 'shell')).toMatchObject({ reinicios: 4, reiniciosTitilan: true });
  });

  it('no titilan si no cambiaron', () => {
    const escena = construirEscena(foto(shell(3)), { fotoAnterior: foto(shell(3)) });

    expect(tarjeta(escena, 'shell')).toMatchObject({ reinicios: 3, reiniciosTitilan: false });
  });

  it('no titilan en la primera foto ni en un pod que recién aparece', () => {
    expect(tarjeta(construirEscena(foto(shell(3))), 'shell').reiniciosTitilan).toBe(false);
    expect(tarjeta(construirEscena(foto(shell(3)), { fotoAnterior: foto() }), 'shell').reiniciosTitilan).toBe(false);
  });

  it('distingue pods con el mismo nombre en distintos namespaces', () => {
    const otro = { ...shell(9), namespace: 'lab' };
    const escena = construirEscena(foto(shell(4), otro), { fotoAnterior: foto(shell(4), { ...otro, containers: [] }) });

    expect(escena.pods.find((p) => p.namespace === 'default')!.reiniciosTitilan).toBe(false);
    expect(escena.pods.find((p) => p.namespace === 'lab')!.reiniciosTitilan).toBe(true);
  });
});

describe('containers como chips', () => {
  const puerto = (numero: number) => ({ numero, protocolo: 'TCP' });

  it('muestra un chip por container, con su tipo y su puerto', () => {
    const escena = construirEscena(
      foto(
        pod('backend', {
          containers: [
            container('application', { puertos: [puerto(8080)] }),
            container('debugger-7xq9z', { tipo: 'ephemeral' }),
          ],
        }),
        pod('web-7c9d', {
          estado: 'Init:0/1',
          containers: [
            container('cache-warmer', { tipo: 'init' }),
            container('web', { puertos: [puerto(80), puerto(443)], estado: 'PodInitializing', ready: false }),
          ],
        }),
      ),
    );

    expect(tarjeta(escena, 'backend').containers).toMatchObject([
      { nombre: 'application', tipo: 'normal', texto: ':8080' },
      { nombre: 'debugger-7xq9z', tipo: 'ephemeral', texto: 'debugge' },
    ]);
    expect(tarjeta(escena, 'web-7c9d').containers).toMatchObject([
      { nombre: 'cache-warmer', tipo: 'init', texto: 'cache-w' },
      { nombre: 'web', tipo: 'normal', texto: ':80' },
    ]);
  });

  it('marca el container normal que no está listo', () => {
    const escena = construirEscena(
      foto(
        pod('api-5d8f', {
          estado: 'CrashLoopBackOff',
          containers: [
            container('migrar', { tipo: 'init', estado: 'Completed', ready: false }),
            container('api', { estado: 'CrashLoopBackOff', ready: false, reinicios: 7 }),
          ],
        }),
        pod('nginx'),
        pod('report-28910', {
          fase: 'Succeeded',
          estado: 'Completed',
          containers: [container('report', { estado: 'Completed', ready: false })],
        }),
      ),
    );

    expect(tarjeta(escena, 'api-5d8f').containers.map((c) => [c.nombre, c.noListo])).toEqual([
      ['migrar', false],
      ['api', true],
    ]);
    expect(tarjeta(escena, 'nginx').containers[0].noListo).toBe(false);
    // un container que terminó bien no está "no listo": ya hizo su trabajo
    expect(tarjeta(escena, 'report-28910').containers[0].noListo).toBe(false);
  });

  it('ubica los chips dentro de la tarjeta, en fila y sin pisarse', () => {
    const escena = construirEscena(
      foto(pod('nginx', { containers: [container('init', { tipo: 'init' }), container('nginx'), container('debug', { tipo: 'ephemeral' })] })),
    );

    const t = tarjeta(escena, 'nginx');
    const [a, b, c] = t.containers;
    for (const chip of [a, b, c]) expect(adentro(chip, t)).toBe(true);
    expect(b.y).toBe(a.y);
    expect(b.x).toBeGreaterThan(a.x + a.ancho);
    expect(c.x).toBeGreaterThan(b.x + b.ancho);
  });

  it('los chips siguen dentro de la tarjeta cuando los Services bajan los nodos', () => {
    const conService: Foto = {
      ...foto(pod('backend-79f9ffc666-k2x8p', { containers: [container('application', { puertos: [puerto(8081)] })] })),
      services: [
        {
          id: 'default/backend',
          nombre: 'backend',
          namespace: 'default',
          tipo: 'NodePort',
          clusterIP: '10.96.11.121',
          puertos: [{ nombre: '', protocolo: 'TCP', puerto: 80, targetPort: 8081, nodePort: 30010 }],
          selector: { app: 'backend' },
          endpoints: [{ ip: '10.244.1.5', pod: 'backend-79f9ffc666-k2x8p', nodo: 'lab-worker', listo: true }],
        },
      ],
    };
    const escena = construirEscena(conService);

    const t = tarjeta(escena, 'backend-79f9ffc666-k2x8p');
    expect(t.y).toBeGreaterThan(0);
    expect(adentro(t.containers[0], t)).toBe(true);
  });

  it('si no entran todos, el último chip dice cuántos faltan', () => {
    const muchos = ['a', 'b', 'c', 'd', 'e'].map((n) => container(n));
    const escena = construirEscena(foto(pod('grande', { containers: muchos })));

    const t = tarjeta(escena, 'grande');
    const ultimo = t.containers.at(-1)!;
    const visibles = t.containers.slice(0, -1);
    expect(ultimo.texto).toBe(`+${muchos.length - visibles.length}`);
    for (const chip of t.containers) expect(adentro(chip, t)).toBe(true);
  });
});

const adentro = (c: { x: number; y: number; ancho: number; alto: number }, t: { x: number; y: number; ancho: number; alto: number }) =>
  c.x >= t.x && c.y >= t.y && c.x + c.ancho <= t.x + t.ancho && c.y + c.alto <= t.y + t.alto;

describe('nombre del pod', () => {
  it('entra entero si el pod es Running', () => {
    const escena = construirEscena(foto(pod('backend-79f9ffc666-k2x8p')));

    expect(tarjeta(escena, 'backend-79f9ffc666-k2x8p').nombreVisible).toBe('backend-79f9ffc666-k2x8p');
  });

  it('se recorta para dejarle lugar al estado', () => {
    const escena = construirEscena(foto(pod('backend-79f9ffc666-k2x8p', { estado: 'CrashLoopBackOff' })));

    const visible = tarjeta(escena, 'backend-79f9ffc666-k2x8p').nombreVisible;
    expect(visible).toMatch(/…$/);
    expect('backend-79f9ffc666-k2x8p'.startsWith(visible.slice(0, -1))).toBe(true);
    expect(visible.length).toBeLessThan(15);
  });

  it('no se recorta si el nombre es corto, aunque haya estado', () => {
    const escena = construirEscena(foto(pod('shell', { estado: 'CrashLoopBackOff' })));

    expect(tarjeta(escena, 'shell').nombreVisible).toBe('shell');
  });

  it('no lleva el namespace', () => {
    const escena = construirEscena(foto(pod('postgres', { namespace: 'datos' })));

    expect(tarjeta(escena, 'postgres').nombreVisible).toBe('postgres');
  });
});
