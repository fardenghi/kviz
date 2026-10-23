// Costura 2: el recorrido completo de la guía (DNS → puerta → regla de Ingress → Service →
// endpoints → salto → pod) en la vista: el pod del Ingress Controller aparece aunque el sistema
// esté oculto, el salto entre nodos es un arco por debajo de las regiones, y un paso elegido se
// resalta solo.
import { describe, expect, it } from 'vitest';
import type { Foto, Pod, Puerta } from '../foto';
import type { Recorrido } from '../prueba';
import { construirEscena, type Escena, type EstadoInterfaz } from './escena';
import { OPACIDAD_ATENUADO } from './workloads';

const pod = (nombre: string, nodo: string, ip: string, namespace = 'default'): Pod => ({
  id: `${namespace}/${nombre}`,
  nombre,
  namespace,
  nodo,
  ip,
  fase: 'Running',
  estado: 'Running',
  containers: [],
  workload: null,
  hostPorts: [],
});

const IC = 'ingress-nginx-controller-6b9f-lx2wp';
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

/** El cluster de la guía: backend en lab-worker y lab-worker2, el Ingress Controller en lab-worker2. */
const guia = (): Foto => ({
  contexto: 'kind-lab',
  conectado: true,
  nodos: [
    { id: 'lab-control-plane', nombre: 'lab-control-plane', internalIP: '172.23.0.2' },
    { id: 'lab-worker', nombre: 'lab-worker', internalIP: '172.23.0.4' },
    { id: 'lab-worker2', nombre: 'lab-worker2', internalIP: '172.23.0.3' },
  ],
  pods: [
    pod('backend-a', 'lab-worker', '10.244.1.5'),
    pod('backend-b', 'lab-worker2', '10.244.2.4'),
    pod(IC, 'lab-worker2', '10.244.2.2', 'ingress-nginx'),
    pod('coredns-1', 'lab-control-plane', '10.244.0.3', 'kube-system'),
  ],
  workloads: [],
  services: [
    {
      id: 'default/backend',
      nombre: 'backend',
      namespace: 'default',
      tipo: 'NodePort',
      clusterIP: '10.96.11.121',
      puertos: [{ nombre: '', protocolo: 'TCP', puerto: 80, targetPort: 8081, nodePort: 30010 }],
      selector: { app: 'backend' },
      endpoints: [
        { ip: '10.244.1.5', pod: 'backend-a', nodo: 'lab-worker', listo: true },
        { ip: '10.244.2.4', pod: 'backend-b', nodo: 'lab-worker2', listo: true },
      ],
    },
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
    nodePort('lab-control-plane'),
    nodePort('lab-worker'),
    nodePort('lab-worker2'),
    {
      id: 'lab-worker2/80/TCP',
      nodo: 'lab-worker2',
      puerto: 80,
      protocolo: 'TCP',
      tipo: 'hostPort',
      namespace: 'ingress-nginx',
      service: null,
      pod: IC,
      haciaPuerto: 80,
    },
  ],
});

const paso = (n: number, tipo: string, ...objetos: [string, string][]) => ({
  n,
  tipo,
  titulo: '',
  detalle: '',
  objetos: objetos.map(([tipo, id]) => ({ tipo, id })),
});

/** `GET /backend/hello` por lab-worker2:80, como lo calcula kviz (internal/trafico). */
const recorrido: Recorrido = {
  pasos: [
    paso(1, 'dns', ['nodo', 'lab-worker2']),
    paso(2, 'puerta', ['puerta', 'lab-worker2/80/TCP']),
    paso(3, 'ingress', ['ingress', 'default/backend'], ['pod', `ingress-nginx/${IC}`]),
    paso(4, 'service', ['service', 'default/backend']),
    paso(5, 'endpoints', ['pod', 'default/backend-a'], ['pod', 'default/backend-b']),
    paso(6, 'salto', ['nodo', 'lab-worker2'], ['nodo', 'lab-worker']),
    paso(7, 'pod', ['pod', 'default/backend-a']),
  ],
};

const POD_IC = `pod:ingress-nginx/${IC}`;
const POD_A = 'pod:default/backend-a';
const POD_B = 'pod:default/backend-b';
const SVC = 'service:default/backend';
const ING = 'ingress:default/backend';
const PUERTA_IC = 'puerta:lab-worker2/80';

const conRecorrido = (estado: EstadoInterfaz = {}) => construirEscena(guia(), { recorrido, ...estado });
const objetos = (e: Escena) => [...e.pods, ...e.services, ...e.ingress, ...e.puertas];
const objeto = (e: Escena, id: string) => objetos(e).find((o) => o.id === id)!;
const pasosDe = (e: Escena) =>
  Object.fromEntries([...objetos(e), ...e.regiones].filter((o) => o.pasos.length > 0).map((o) => [o.id, o.pasos]));

describe('excepción del Ingress Controller', () => {
  it('su pod aparece durante el recorrido aunque los namespaces del sistema estén ocultos', () => {
    expect(construirEscena(guia()).pods.map((p) => p.id)).not.toContain(POD_IC);

    const e = conRecorrido();

    expect(objeto(e, POD_IC).pasos).toEqual([3]);
    expect(objeto(e, POD_IC).opacidad).toBe(1);
    // los demás pods del sistema siguen ocultos
    expect(e.pods.map((p) => p.id)).not.toContain('pod:kube-system/coredns-1');
    // y la línea de su puerta llega a él
    const lineas = e.conexiones.filter((c) => c.dePuerta).map((c) => [c.desde, c.hasta, c.tono]);
    expect(lineas).toContainEqual([PUERTA_IC, POD_IC, 'recorrido']);
  });

  it('aparece también con el filtro por namespace, junto con su puerta', () => {
    const e = conRecorrido({ namespace: 'default' });

    expect(e.pods.map((p) => p.id)).toContain(POD_IC);
    expect(objeto(e, PUERTA_IC).pasos).toEqual([2]);
  });

  it('al cerrar el recorrido vuelve a ocultarse', () => {
    const e = construirEscena(guia(), { recorrido: null });

    expect(e.pods.map((p) => p.id)).not.toContain(POD_IC);
  });
});

describe('arco de salto entre nodos', () => {
  it('va del pod del Ingress Controller al pod que respondió, por debajo de las regiones', () => {
    const e = conRecorrido();

    expect(e.saltos).toHaveLength(1);
    const [s] = e.saltos;
    const ic = objeto(e, POD_IC);
    const a = objeto(e, POD_A);
    expect(s.etiqueta).toBe('salto lab-worker2 → lab-worker');
    expect(s.paso).toBe(6);
    expect(s.desde).toEqual({ x: ic.x + ic.ancho / 2, y: ic.y + ic.alto });
    expect(s.hasta).toEqual({ x: a.x + a.ancho / 2, y: a.y + a.alto });
    // el punto más bajo (donde va la etiqueta) queda debajo de todas las regiones y dentro de la escena
    const fondo = Math.max(...e.regiones.map((r) => r.y + r.alto));
    expect(s.etiquetaEn.y).toBeGreaterThan(fondo);
    expect(s.etiquetaEn.x).toBe((s.desde.x + s.hasta.x) / 2);
    expect(e.alto).toBeGreaterThan(s.etiquetaEn.y);
    expect(s.opacidad).toBe(1);
  });

  it('el punto medio de la curva es el de la etiqueta', () => {
    const [s] = conRecorrido().saltos;

    // cúbica con los dos controles a la altura yControl: en t = 0.5, y = (y1 + y2)/8 + 3/4 · yControl
    expect((s.desde.y + s.hasta.y) / 8 + (3 / 4) * s.yControl).toBeCloseTo(s.etiquetaEn.y);
  });

  it('entrando por un NodePort, el arco sale de la puerta del nodo de entrada', () => {
    const porNodePort: Recorrido = {
      pasos: [
        paso(1, 'puerta', ['puerta', 'lab-worker/30010/TCP']),
        paso(2, 'service', ['service', 'default/backend']),
        paso(3, 'endpoints', ['pod', 'default/backend-a'], ['pod', 'default/backend-b']),
        paso(4, 'salto', ['nodo', 'lab-worker'], ['nodo', 'lab-worker2']),
        paso(5, 'pod', ['pod', 'default/backend-b']),
      ],
    };
    const e = construirEscena(guia(), { recorrido: porNodePort });

    const puerta = objeto(e, 'puerta:lab-worker/30010');
    const b = objeto(e, POD_B);
    expect(e.saltos.map((s) => [s.etiqueta, s.desde, s.hasta])).toEqual([
      ['salto lab-worker → lab-worker2', { x: puerta.x + puerta.ancho / 2, y: puerta.y + puerta.alto }, { x: b.x + b.ancho / 2, y: b.y + b.alto }],
    ]);
  });

  it('sin paso de salto (el pod está en el nodo de entrada) no hay arco', () => {
    const sinSalto: Recorrido = { pasos: recorrido.pasos.filter((p) => p.tipo !== 'salto') };

    expect(construirEscena(guia(), { recorrido: sinSalto }).saltos).toEqual([]);
    expect(construirEscena(guia()).saltos).toEqual([]);
  });

  it('las regiones de los nodos del DNS y del salto llevan los números de sus pasos', () => {
    const e = conRecorrido();

    expect(e.regiones.find((r) => r.id === 'nodo:lab-worker2')!.pasos).toEqual([1, 6]);
    expect(e.regiones.find((r) => r.id === 'nodo:lab-worker')!.pasos).toEqual([6]);
    expect(e.regiones.find((r) => r.id === 'nodo:lab-control-plane')!.pasos).toEqual([]);
  });
});

describe('paso elegido', () => {
  it('con un clic en un paso se resalta solo ese paso', () => {
    const e = conRecorrido({ pasoElegido: 3 });

    expect(pasosDe(e)).toEqual({ [ING]: [3], [POD_IC]: [3] });
    for (const id of [ING, POD_IC]) expect(objeto(e, id).opacidad, id).toBe(1);
    for (const id of [SVC, POD_A, POD_B, PUERTA_IC]) expect(objeto(e, id).opacidad, id).toBe(OPACIDAD_ATENUADO);
    // las líneas de otros pasos dejan de ser del recorrido
    expect(e.conexiones.find((c) => c.desde === ING && c.hasta === SVC)?.tono).toBe('fantasma');
    expect(e.conexiones.filter((c) => c.tono === 'recorrido')).toEqual([]);
    expect(e.saltos[0].opacidad).toBe(OPACIDAD_ATENUADO);
  });

  it('el paso del salto resalta el arco y sus dos nodos', () => {
    const e = conRecorrido({ pasoElegido: 6 });

    expect(e.saltos[0].opacidad).toBe(1);
    expect(pasosDe(e)).toEqual({ 'nodo:lab-worker2': [6], 'nodo:lab-worker': [6] });
    expect(objeto(e, POD_A).opacidad).toBe(OPACIDAD_ATENUADO);
  });

  it('el paso de los endpoints marca solo sus pods', () => {
    const e = conRecorrido({ pasoElegido: 5 });

    expect(pasosDe(e)).toEqual({ [POD_A]: [5], [POD_B]: [5] });
    // el pod del Ingress Controller sigue a la vista aunque no sea de este paso
    expect(e.pods.map((p) => p.id)).toContain(POD_IC);
  });

  it('sin paso elegido (o uno que no existe) se ven todos los pasos', () => {
    for (const pasoElegido of [null, 99]) {
      const e = conRecorrido({ pasoElegido });

      expect(objeto(e, POD_A).pasos).toEqual([5, 7]);
      expect(objeto(e, SVC).pasos).toEqual([4]);
      expect(e.saltos[0].opacidad).toBe(1);
    }
  });
});
