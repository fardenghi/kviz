// Costura 2: Services y sus conexiones con los pods en la escena de la vista de topología.
import { describe, expect, it } from 'vitest';
import type { Endpoint, Foto, Pod, Service } from '../foto';
import { encuadrar } from '../lienzo/vista';
import { type Caja, construirEscena } from './escena';

const nodo = (nombre: string) => ({ nombre, internalIP: '172.23.0.2' });
const pod = (nombre: string, nodo: string | null): Pod => ({
  nombre,
  namespace: 'default',
  nodo,
  ip: nodo ? '10.244.0.1' : null,
  fase: nodo ? 'Running' : 'Pending',
  hostPorts: [],
});
const listo = (pod: string): Endpoint => ({ ip: '10.244.0.1', pod, nodo: null, listo: true });
const noListo = (pod: string): Endpoint => ({ ip: '10.244.0.1', pod, nodo: null, listo: false });
const service = (nombre: string, endpoints: Endpoint[] = [], parte: Partial<Service> = {}): Service => ({
  nombre,
  namespace: 'default',
  tipo: 'ClusterIP',
  clusterIP: '10.96.0.10',
  puertos: [{ nombre: '', protocolo: 'TCP', puerto: 80, targetPort: 8080 }],
  selector: { app: nombre },
  endpoints,
  ...parte,
});
const foto = (parte: Partial<Foto>): Foto => ({ contexto: 'kind-lab', conectado: true, nodos: [], pods: [], services: [], ingress: [], puertas: [], ...parte });

const centroX = (c: Caja) => c.x + c.ancho / 2;

describe('ubicación de los Services', () => {
  it('ubica cada Service arriba del centro de sus pods', () => {
    const escena = construirEscena(
      foto({
        nodos: [nodo('lab-worker'), nodo('lab-worker2')],
        pods: [pod('backend-a', 'lab-worker'), pod('nginx', 'lab-worker'), pod('backend-b', 'lab-worker2')],
        services: [service('backend', [listo('backend-a'), noListo('backend-b')])],
      }),
    );

    const svc = escena.services.find((s) => s.nombre === 'backend')!;
    const [a, b] = ['backend-a', 'backend-b'].map((n) => escena.pods.find((p) => p.nombre === n)!);
    expect(centroX(svc)).toBeCloseTo((centroX(a) + centroX(b)) / 2);
    for (const r of escena.regiones) expect(svc.y + svc.alto).toBeLessThan(r.y);
  });
});

describe('Services que se pisan', () => {
  it('si dos Services se pisan, el segundo baja a otra fila, y los nodos quedan debajo de todas', () => {
    const escena = construirEscena(
      foto({
        nodos: [nodo('lab-worker')],
        pods: [pod('web-a', 'lab-worker'), pod('web-b', 'lab-worker')],
        services: [service('web', [listo('web-a'), listo('web-b')]), service('web-interno', [listo('web-a'), listo('web-b')])],
      }),
    );

    const [web, interno] = ['web', 'web-interno'].map((n) => escena.services.find((s) => s.nombre === n)!);
    expect(interno.y).toBeGreaterThan(web.y + web.alto);
    expect(centroX(interno)).toBeCloseTo(centroX(web));
    const region = escena.regiones[0];
    expect(region.y).toBeGreaterThan(interno.y + interno.alto);
  });

  it('usa todas las filas que hagan falta', () => {
    const nombres = ['a', 'b', 'c', 'd'];
    const escena = construirEscena(
      foto({
        nodos: [nodo('lab-worker')],
        pods: [pod('web-a', 'lab-worker')],
        services: nombres.map((n) => service(n, [listo('web-a')])),
      }),
    );

    expect(new Set(escena.services.map((s) => s.y)).size).toBe(4);
    expect(superpuestos(escena.services)).toEqual([]);
  });

  it('dos Services sobre pods distintos que no se pisan comparten fila', () => {
    const escena = construirEscena(
      foto({
        nodos: [nodo('lab-worker'), nodo('lab-worker2')],
        pods: [pod('a', 'lab-worker'), pod('b', 'lab-worker2')],
        services: [service('sa', [listo('a')]), service('sb', [listo('b')])],
      }),
    );

    const [sa, sb] = escena.services;
    expect(sa.y).toBe(sb.y);
  });

  it('un Service sin pods queda ubicado sin superponerse con otros', () => {
    const escena = construirEscena(
      foto({
        nodos: [nodo('lab-worker'), nodo('lab-worker2')],
        pods: [pod('a', 'lab-worker'), pod('b', 'lab-worker2')],
        services: [service('frontend'), service('sa', [listo('a')]), service('sb', [listo('b')]), service('vacio')],
      }),
    );

    expect(escena.services.map((s) => s.nombre).sort()).toEqual(['frontend', 'sa', 'sb', 'vacio']);
    expect(superpuestos([...escena.services, ...escena.regiones])).toEqual([]);
    for (const s of escena.services) expect(adentro(s, { x: 0, y: 0, ancho: escena.ancho, alto: escena.alto })).toBe(true);
  });
});

describe('texto de los Services', () => {
  it('muestra tipo, ClusterIP y puertos con su targetPort y su nodePort', () => {
    const escena = construirEscena(
      foto({
        services: [
          service('backend', [], {
            tipo: 'NodePort',
            clusterIP: '10.96.11.121',
            puertos: [{ nombre: '', protocolo: 'TCP', puerto: 80, targetPort: 8081, nodePort: 30010 }],
          }),
          service('dns', [], {
            puertos: [
              { nombre: 'dns', protocolo: 'UDP', puerto: 53, targetPort: 53 },
              { nombre: 'metrics', protocolo: 'TCP', puerto: 9153, targetPort: 'metrics' },
            ],
          }),
        ],
      }),
    );

    expect(escena.services[0]).toMatchObject({ tipo: 'NodePort', clusterIP: '10.96.11.121', puertos: ['80 → 8081 · :30010'] });
    expect(escena.services[1].puertos).toEqual(['53 → 53/UDP', '9153 → metrics']);
  });
});

describe('conexiones Service → pod', () => {
  it('une cada Service con sus endpoints y distingue los no listos', () => {
    const escena = construirEscena(
      foto({
        nodos: [nodo('lab-worker')],
        pods: [pod('web-a', 'lab-worker'), pod('web-b', 'lab-worker'), pod('suelto', 'lab-worker')],
        services: [service('web', [listo('web-a'), noListo('web-b')])],
      }),
    );

    expect(escena.conexiones.map((c) => [c.desde, c.hasta, c.tono])).toEqual([
      ['service:default/web', 'pod:default/web-a', 'reposo'],
      ['service:default/web', 'pod:default/web-b', 'no-listo'],
    ]);
  });

  it('va del borde de abajo del Service al borde de arriba del pod', () => {
    const escena = construirEscena(
      foto({ nodos: [nodo('lab-worker')], pods: [pod('web-a', 'lab-worker')], services: [service('web', [listo('web-a')])] }),
    );

    const [c] = escena.conexiones;
    const [svc, p] = [escena.services[0], escena.pods[0]];
    expect(c.inicio).toEqual({ x: centroX(svc), y: svc.y + svc.alto });
    expect(c.fin).toEqual({ x: centroX(p), y: p.y });
  });

  it('no une endpoints sin pod ni pods que no se ven', () => {
    const escena = construirEscena(
      foto({
        nodos: [nodo('lab-worker')],
        pods: [pod('web-a', 'lab-worker')],
        services: [
          service('web', [listo('web-a'), listo('no-existe'), { ip: '192.168.0.9', pod: null, nodo: null, listo: true }]),
        ],
      }),
    );

    expect(escena.conexiones.map((c) => c.hasta)).toEqual(['pod:default/web-a']);
  });
});

describe('filtros de namespace', () => {
  it('oculta los Services de los namespaces ocultos, como sus pods', () => {
    const f = foto({
      nodos: [nodo('lab-worker')],
      pods: [pod('web-a', 'lab-worker'), { ...pod('coredns', 'lab-worker'), namespace: 'kube-system' }],
      services: [
        service('web', [listo('web-a')]),
        service('kube-dns', [listo('coredns')], { namespace: 'kube-system' }),
        service('api', [], { namespace: 'otro' }),
      ],
    });

    expect(construirEscena(f).services.map((s) => s.id).sort()).toEqual(['service:default/web', 'service:otro/api']);
    expect(construirEscena(f, { mostrarSistema: true }).services.map((s) => s.id)).toContain('service:kube-system/kube-dns');
    expect(construirEscena(f, { namespace: 'otro' }).services.map((s) => s.id)).toEqual(['service:otro/api']);
    expect(construirEscena(f, { namespace: 'otro' }).conexiones).toEqual([]);
  });
});

describe('criterio de encuadre', () => {
  it('con 10 nodos y 20 Services todo entra en el encuadre y ningún objeto se superpone', () => {
    const nodos = Array.from({ length: 10 }, (_, i) => nodo(`nodo-${i}`));
    const pods = nodos.flatMap((n) => [0, 1, 2, 3].map((j) => pod(`${n.nombre}-p${j}`, n.nombre)));
    const services = Array.from({ length: 20 }, (_, i) => {
      // casi todos sobre 2 pods de un nodo, algunos repartidos entre dos nodos, uno sin pods
      if (i === 19) return service(`s${i}`);
      const n = i % 10;
      const otro = i >= 15 ? (n + 1) % 10 : n;
      return service(`s${i}`, [listo(`nodo-${n}-p${i % 4}`), noListo(`nodo-${otro}-p${(i + 1) % 4}`)]);
    });

    const escena = construirEscena(foto({ nodos, pods, services }));

    expect(escena.regiones).toHaveLength(10);
    expect(escena.services).toHaveLength(20);
    expect(superpuestos([...escena.regiones, ...escena.services])).toEqual([]);
    expect(superpuestos(escena.pods)).toEqual([]);
    const todo = { x: 0, y: 0, ancho: escena.ancho, alto: escena.alto };
    for (const c of [...escena.regiones, ...escena.services, ...escena.pods]) expect(adentro(c, todo)).toBe(true);

    // en una pantalla de notebook, "encuadrar" muestra toda la escena sin arrastrar
    const lienzo = { ancho: 1280, alto: 720 };
    const v = encuadrar(escena, lienzo);
    expect(v.x).toBeGreaterThanOrEqual(0);
    expect(v.y).toBeGreaterThanOrEqual(0);
    expect(v.x + escena.ancho * v.escala).toBeLessThanOrEqual(lienzo.ancho);
    expect(v.y + escena.alto * v.escala).toBeLessThanOrEqual(lienzo.alto);
  });
});

const adentro = (c: Caja, region: Caja) =>
  c.x >= region.x && c.y >= region.y && c.x + c.ancho <= region.x + region.ancho && c.y + c.alto <= region.y + region.alto;

const pisan = (a: Caja, b: Caja) => a.x < b.x + b.ancho && b.x < a.x + a.ancho && a.y < b.y + b.alto && b.y < a.y + a.alto;

/** Pares de cajas que se pisan, por id (vacío si ninguna se superpone). */
function superpuestos(cajas: (Caja & { id: string })[]): string[][] {
  const pares: string[][] = [];
  cajas.forEach((a, i) => cajas.slice(i + 1).forEach((b) => pisan(a, b) && pares.push([a.id, b.id])));
  return pares;
}
