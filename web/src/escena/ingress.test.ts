// Costura 2: Ingress y puertas en la escena de la vista de topología.
import { describe, expect, it } from 'vitest';
import type { Endpoint, Foto, Ingress, Pod, Puerta, Service } from '../foto';
import { type Caja, construirEscena } from './escena';

const nodo = (nombre: string) => ({ id: nombre, nombre, internalIP: '172.23.0.2' });
const pod = (nombre: string, nodo: string | null, namespace = 'default'): Pod => ({
  id: `${namespace}/${nombre}`,
  nombre,
  namespace,
  nodo,
  ip: nodo ? '10.244.0.1' : null,
  fase: nodo ? 'Running' : 'Pending',
  estado: nodo ? 'Running' : 'Pending',
  containers: [],
  workload: null,
  hostPorts: [],
});
const listo = (pod: string): Endpoint => ({ ip: '10.244.0.1', pod, nodo: null, listo: true });
const service = (nombre: string, endpoints: Endpoint[] = [], parte: Partial<Service> = {}): Service => ({
  id: `${parte.namespace ?? 'default'}/${nombre}`,
  nombre,
  namespace: 'default',
  tipo: 'ClusterIP',
  clusterIP: '10.96.0.10',
  puertos: [{ nombre: '', protocolo: 'TCP', puerto: 80, targetPort: 8080 }],
  selector: { app: nombre },
  endpoints,
  ...parte,
});
const ingress = (nombre: string, services: string[], parte: Partial<Ingress> = {}): Ingress => ({
  id: `${parte.namespace ?? 'default'}/${nombre}`,
  nombre,
  namespace: 'default',
  clase: 'nginx',
  clasePorDefecto: false,
  rewrite: null,
  reglas: services.map((s) => ({ host: '', path: `/${s}`, tipoPath: 'Prefix', service: s, puerto: 80 })),
  ...parte,
});
const nodePort = (nodo: string, puerto: number, service: string, namespace = 'default'): Puerta => ({
  id: `${nodo}/${puerto}/TCP`,
  nodo,
  puerto,
  protocolo: 'TCP',
  tipo: 'NodePort',
  namespace,
  service,
  pod: null,
  haciaPuerto: 80,
});
const hostPort = (nodo: string, puerto: number, pod: string, namespace = 'ingress-nginx'): Puerta => ({
  id: `${nodo}/${puerto}/TCP`,
  nodo,
  puerto,
  protocolo: 'TCP',
  tipo: 'hostPort',
  namespace,
  service: null,
  pod,
  haciaPuerto: puerto,
});
const foto = (parte: Partial<Foto>): Foto => ({
  contexto: 'kind-lab',
  conectado: true,
  nodos: [],
  pods: [],
  workloads: [],
  services: [],
  ingress: [],
  puertas: [],
  ...parte,
});

const centroX = (c: Caja) => c.x + c.ancho / 2;

/** El cluster de la guía: 3 nodos, Service backend NodePort 30010, Ingress backend con rewrite e IC en lab-worker2. */
const guia = () =>
  foto({
    nodos: [nodo('lab-control-plane'), nodo('lab-worker'), nodo('lab-worker2')],
    pods: [
      pod('backend-79f9ffc666-k2x8p', 'lab-worker'),
      pod('backend-79f9ffc666-7hw4q', 'lab-worker2'),
      pod('ingress-nginx-controller-6b9f-lx2wp', 'lab-worker2', 'ingress-nginx'),
    ],
    services: [
      service('backend', [listo('backend-79f9ffc666-k2x8p'), listo('backend-79f9ffc666-7hw4q')], {
        tipo: 'NodePort',
        clusterIP: '10.96.11.121',
        puertos: [{ nombre: '', protocolo: 'TCP', puerto: 80, targetPort: 8081, nodePort: 30010 }],
      }),
    ],
    ingress: [
      {
        id: 'default/backend',
        nombre: 'backend',
        namespace: 'default',
        clase: 'nginx',
        clasePorDefecto: false,
        rewrite: '/$2',
        reglas: [{ host: '', path: '/backend(/|$)(.*)', tipoPath: 'ImplementationSpecific', service: 'backend', puerto: 80 }],
      },
    ],
    puertas: [
      nodePort('lab-control-plane', 30010, 'backend'),
      nodePort('lab-worker', 30010, 'backend'),
      nodePort('lab-worker2', 30010, 'backend'),
      // por id, como las manda la foto: 443 antes que 80
      hostPort('lab-worker2', 443, 'ingress-nginx-controller-6b9f-lx2wp'),
      hostPort('lab-worker2', 80, 'ingress-nginx-controller-6b9f-lx2wp'),
    ],
  });

describe('ubicación de los Ingress', () => {
  it('ubica el Ingress arriba del centro de su Service', () => {
    const escena = construirEscena(guia());

    const ing = escena.ingress.find((i) => i.id === 'ingress:default/backend')!;
    const svc = escena.services.find((s) => s.id === 'service:default/backend')!;
    expect(centroX(ing)).toBeCloseTo(centroX(svc));
    expect(ing.y + ing.alto).toBeLessThan(svc.y);
    for (const r of escena.regiones) expect(ing.y + ing.alto).toBeLessThan(r.y);
  });

  it('dice cuándo la clase es la IngressClass por defecto del cluster', () => {
    const f = guia();
    f.ingress.push(ingress('sin-clase', ['backend'], { clasePorDefecto: true }));

    const escena = construirEscena(f);

    expect(escena.ingress.find((i) => i.nombre === 'sin-clase')?.clase).toBe('nginx (por defecto)');
    expect(escena.ingress.find((i) => i.nombre === 'backend')?.clase).toBe('nginx');
  });

  it('muestra nombre, clase y una línea por regla con host, path, Service destino y rewrite', () => {
    const f = guia();
    f.ingress.push(
      ingress('legacy', [], {
        clase: null,
        reglas: [
          { host: 'api.lab.local', path: '/v1', tipoPath: 'Prefix', service: 'old-api', puerto: 'http' },
          { host: '', path: '', tipoPath: '', service: 'pagina-404', puerto: 8080 },
        ],
      }),
    );

    const escena = construirEscena(f);

    expect(escena.ingress.find((i) => i.nombre === 'backend')).toMatchObject({
      clase: 'nginx',
      reglas: ['/backend(/|$)(.*) → backend:80 ⟲ /$2'],
    });
    expect(escena.ingress.find((i) => i.nombre === 'legacy')).toMatchObject({
      clase: 'sin clase',
      reglas: ['api.lab.local/v1 → old-api:http', '(por defecto) → pagina-404:8080'],
    });
  });

  it('une el Ingress con su Service con una línea en reposo, del borde de abajo al de arriba', () => {
    const escena = construirEscena(guia());

    const linea = escena.conexiones.find((c) => c.desde === 'ingress:default/backend')!;
    const [ing, svc] = [escena.ingress[0], escena.services[0]];
    expect(linea).toMatchObject({ hasta: 'service:default/backend', tono: 'reposo' });
    expect(linea.inicio).toEqual({ x: centroX(ing), y: ing.y + ing.alto });
    expect(linea.fin).toEqual({ x: centroX(svc), y: svc.y });
  });

  it('un Ingress con varias reglas al mismo Service tiene una sola línea, y una por cada Service distinto', () => {
    const escena = construirEscena(
      foto({
        nodos: [nodo('lab-worker')],
        pods: [pod('web-a', 'lab-worker'), pod('api-a', 'lab-worker')],
        services: [service('web', [listo('web-a')]), service('api', [listo('api-a')])],
        ingress: [ingress('todo', ['web', 'web', 'api', 'no-existe'])],
      }),
    );

    expect(escena.conexiones.filter((c) => c.desde.startsWith('ingress:')).map((c) => c.hasta).sort()).toEqual([
      'service:default/api',
      'service:default/web',
    ]);
  });

  it('dos Ingress sobre el mismo Service no se pisan', () => {
    const escena = construirEscena(
      foto({
        nodos: [nodo('lab-worker')],
        pods: [pod('web-a', 'lab-worker')],
        services: [service('web', [listo('web-a')])],
        ingress: [ingress('uno', ['web']), ingress('dos', ['web']), ingress('suelto', ['no-existe'])],
      }),
    );

    expect(escena.ingress).toHaveLength(3);
    expect(superpuestos([...escena.ingress, ...escena.services, ...escena.regiones])).toEqual([]);
    for (const c of escena.ingress) expect(adentro(c, { x: 0, y: 0, ancho: escena.ancho, alto: escena.alto })).toBe(true);
  });

  it('sigue la regla de visibilidad de los namespaces, como los Services', () => {
    const f = foto({
      nodos: [nodo('lab-worker')],
      pods: [pod('web-a', 'lab-worker')],
      services: [service('web', [listo('web-a')]), service('api', [], { namespace: 'otro' })],
      ingress: [
        ingress('web', ['web']),
        ingress('api', ['api'], { namespace: 'otro' }),
        ingress('panel', ['dashboard'], { namespace: 'kube-system' }),
        ingress('sondeo', ['sondeo'], { namespace: 'kviz-system' }),
      ],
    });

    const ids = (estado = {}) => construirEscena(f, estado).ingress.map((i) => i.id).sort();
    expect(ids()).toEqual(['ingress:default/web', 'ingress:otro/api']);
    expect(ids({ mostrarSistema: true })).toEqual(['ingress:default/web', 'ingress:kube-system/panel', 'ingress:otro/api']);
    expect(ids({ namespace: 'otro' })).toEqual(['ingress:otro/api']);
    expect(construirEscena(f, { namespace: 'otro' }).conexiones.filter((c) => c.desde.startsWith('ingress:'))).toEqual([
      expect.objectContaining({ desde: 'ingress:otro/api', hasta: 'service:otro/api' }),
    ]);
  });
});

describe('puertas', () => {
  it('un NodePort pone una puerta en el borde de arriba de cada nodo, y un hostPort una sola en su nodo', () => {
    const escena = construirEscena(guia());

    const porNodo = Object.fromEntries(
      escena.regiones.map((r) => [r.nodo, escena.puertas.filter((p) => p.region === r.id).map((p) => `${p.tipo} ${p.texto}`)]),
    );
    expect(porNodo).toEqual({
      'lab-control-plane': ['NodePort :30010'],
      'lab-worker': ['NodePort :30010'],
      'lab-worker2': ['NodePort :30010', 'hostPort :80', 'hostPort :443'],
    });
    for (const p of escena.puertas) {
      const r = escena.regiones.find((r) => r.id === p.region)!;
      // a caballo del borde superior del nodo, adentro de su ancho
      expect(p.y).toBeLessThan(r.y);
      expect(p.y + p.alto).toBeGreaterThan(r.y);
      expect(p.x).toBeGreaterThanOrEqual(r.x);
      expect(p.x + p.ancho).toBeLessThanOrEqual(r.x + r.ancho);
    }
  });

  it('cada puerta sabe a dónde lleva: su Service, o su pod si es un hostPort', () => {
    const escena = construirEscena(guia());

    const destino = (id: string) => escena.puertas.find((p) => p.id === id)?.destino;
    expect(destino('puerta:lab-worker/30010')).toBe('service:default/backend');
    expect(destino('puerta:lab-worker2/80')).toBe('pod:ingress-nginx/ingress-nginx-controller-6b9f-lx2wp');
    expect(escena.puertas.find((p) => p.id === 'puerta:lab-worker2/443')).toMatchObject({
      tipo: 'hostPort',
      texto: ':443',
      titulo: 'hostPort 443 → ingress-nginx-controller-6b9f-lx2wp:443',
    });
    expect(escena.puertas.find((p) => p.id === 'puerta:lab-worker/30010')!.titulo).toBe('NodePort 30010 → backend:80');
  });

  it('las puertas de un nodo no se pisan entre sí ni con las de otro nodo', () => {
    const muchas = [30001, 30002, 30003, 30004, 30005, 30006, 30007].map((n) => nodePort('lab-worker', n, 'web'));
    const escena = construirEscena(
      foto({
        nodos: [nodo('lab-worker'), nodo('lab-worker2')],
        services: [service('web')],
        puertas: [...muchas, ...muchas.map((p) => ({ ...p, nodo: 'lab-worker2' })), hostPort('lab-worker2', 80, 'ic')],
      }),
    );

    expect(escena.puertas).toHaveLength(15);
    expect(superpuestos(escena.puertas)).toEqual([]);
    for (const p of escena.puertas) expect(adentro(p, { x: 0, y: 0, ancho: escena.ancho, alto: escena.alto })).toBe(true);
    expect(superpuestos([...escena.puertas, ...escena.services, ...escena.ingress])).toEqual([]);
  });

  it('sin Services ni Ingress arriba, las puertas igual entran en la escena', () => {
    const escena = construirEscena(
      foto({ nodos: [nodo('lab-worker')], puertas: [hostPort('lab-worker', 8080, 'web', 'default')], pods: [pod('web', 'lab-worker')] }),
    );

    const [p] = escena.puertas;
    expect(p.y).toBeGreaterThanOrEqual(0);
    expect(p.y).toBeLessThan(escena.regiones[0].y);
  });

  it('en reposo no hay líneas de las puertas a su Service ni a su pod', () => {
    const escena = construirEscena(guia());

    const ids = new Set(escena.puertas.map((p) => p.id));
    expect(ids.size).toBe(5);
    expect(escena.conexiones.filter((c) => ids.has(c.desde) || ids.has(c.hasta))).toEqual([]);
  });

  it('las puertas NodePort siguen la visibilidad de su Service', () => {
    const f = foto({
      nodos: [nodo('lab-worker')],
      services: [service('web'), service('api', [], { namespace: 'otro' })],
      puertas: [nodePort('lab-worker', 30010, 'web'), nodePort('lab-worker', 30020, 'api', 'otro'), nodePort('lab-worker', 30030, 'dash', 'kube-system')],
    });

    const textos = (estado = {}) => construirEscena(f, estado).puertas.map((p) => p.texto);
    expect(textos()).toEqual([':30010', ':30020']);
    expect(textos({ namespace: 'otro' })).toEqual([':30020']);
    expect(textos({ mostrarSistema: true })).toEqual([':30010', ':30020', ':30030']);
  });

  it('el hostPort del Ingress Controller se ve aunque su pod esté oculto con los namespaces del sistema', () => {
    const f = guia();

    const escena = construirEscena(f);

    // el pod del IC (ingress-nginx) no se ve, pero su nodo sí abre los puertos 80 y 443
    expect(escena.pods.map((p) => p.nombre)).not.toContain('ingress-nginx-controller-6b9f-lx2wp');
    expect(escena.puertas.filter((p) => p.tipo === 'hostPort').map((p) => p.texto)).toEqual([':80', ':443']);
    // el filtro por namespace sí las oculta, y las de kviz-system no se ven nunca
    expect(construirEscena(f, { namespace: 'default' }).puertas.map((p) => p.texto)).toEqual([':30010', ':30010', ':30010']);
    const conSondeo = foto({ nodos: [nodo('lab-worker')], puertas: [hostPort('lab-worker', 9999, 'sondeo', 'kviz-system')] });
    expect(construirEscena(conSondeo, { mostrarSistema: true }).puertas).toEqual([]);
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
