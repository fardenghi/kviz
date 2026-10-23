// Costura 2: la escena de la vista de topología. Se llama a la función pura con una foto y un
// estado de interfaz y se revisa la escena, sin navegador.
import { describe, expect, it } from 'vitest';
import type { Foto, Pod } from '../foto';
import { type Caja, construirEscena, namespacesDelFiltro } from './escena';

const nodo = (nombre: string, internalIP: string) => ({ nombre, internalIP });
const pod = (nombre: string, nodo: string | null, ip: string | null = '10.244.0.1'): Pod => ({
  nombre,
  namespace: 'default',
  nodo,
  ip,
  fase: nodo ? 'Running' : 'Pending',
});
const foto = (parte: Partial<Foto>): Foto => ({ contexto: 'kind-lab', conectado: true, nodos: [], pods: [], services: [], ...parte });

describe('regiones de nodo', () => {
  it('ubica los nodos lado a lado, en el orden de la foto y sin pisarse', () => {
    const escena = construirEscena(
      foto({ nodos: [nodo('lab-control-plane', '172.23.0.2'), nodo('lab-worker', '172.23.0.4'), nodo('lab-worker2', '172.23.0.3')] }),
    );

    const [a, b, c] = escena.regiones;
    expect(escena.regiones.map((r) => r.nodo)).toEqual(['lab-control-plane', 'lab-worker', 'lab-worker2']);
    expect(b.y).toBe(a.y);
    expect(c.y).toBe(a.y);
    expect(b.x).toBeGreaterThan(a.x + a.ancho);
    expect(c.x).toBeGreaterThan(b.x + b.ancho);
  });

  it('rotula cada nodo con su nombre y su InternalIP', () => {
    const escena = construirEscena(foto({ nodos: [nodo('lab-worker', '172.23.0.4')] }));

    expect(escena.regiones[0]).toMatchObject({ titulo: 'lab-worker', subtitulo: '172.23.0.4' });
  });
});

describe('pods', () => {
  it('pone los pods adentro de su nodo, en una grilla de 2 columnas', () => {
    const escena = construirEscena(
      foto({
        nodos: [nodo('lab-worker', '172.23.0.4'), nodo('lab-worker2', '172.23.0.3')],
        pods: [pod('a', 'lab-worker2'), pod('b', 'lab-worker2'), pod('c', 'lab-worker2'), pod('d', 'lab-worker')],
      }),
    );

    const region = escena.regiones.find((r) => r.nodo === 'lab-worker2')!;
    const [a, b, c] = ['a', 'b', 'c'].map((n) => escena.pods.find((p) => p.nombre === n)!);
    // a y b comparten fila; c baja a la fila siguiente, debajo de a
    expect(b.y).toBe(a.y);
    expect(b.x).toBeGreaterThan(a.x + a.ancho);
    expect(c.x).toBe(a.x);
    expect(c.y).toBeGreaterThan(a.y + a.alto);
    for (const p of [a, b, c]) {
      expect(p.region).toBe(region.id);
      expect(adentro(p, region)).toBe(true);
    }
    const d = escena.pods.find((p) => p.nombre === 'd')!;
    expect(adentro(d, escena.regiones.find((r) => r.nodo === 'lab-worker')!)).toBe(true);
  });

  it('muestra en cada pod su nombre y su IP', () => {
    const escena = construirEscena(
      foto({ nodos: [nodo('lab-worker', '172.23.0.4')], pods: [pod('nginx', 'lab-worker', '10.244.1.5')] }),
    );

    expect(escena.pods[0]).toMatchObject({ nombre: 'nginx', ip: '10.244.1.5' });
  });
});

describe('región "Sin nodo asignado"', () => {
  it('no aparece si todos los pods tienen nodo', () => {
    const escena = construirEscena(foto({ nodos: [nodo('lab-worker', '172.23.0.4')], pods: [pod('nginx', 'lab-worker')] }));

    expect(escena.regiones.map((r) => r.titulo)).toEqual(['lab-worker']);
  });

  it('junta a la derecha de los nodos los pods Pending sin nodo', () => {
    const escena = construirEscena(
      foto({
        nodos: [nodo('lab-worker', '172.23.0.4'), nodo('lab-worker2', '172.23.0.3')],
        pods: [pod('grande', null, null), pod('nginx', 'lab-worker'), pod('enorme', null, null)],
      }),
    );

    const sinNodo = escena.regiones.at(-1)!;
    expect(sinNodo).toMatchObject({ nodo: null, titulo: 'Sin nodo asignado' });
    const ultimoNodo = escena.regiones.find((r) => r.nodo === 'lab-worker2')!;
    expect(sinNodo.y).toBe(ultimoNodo.y);
    expect(sinNodo.x).toBeGreaterThan(ultimoNodo.x + ultimoNodo.ancho);
    for (const nombre of ['grande', 'enorme']) {
      const p = escena.pods.find((x) => x.nombre === nombre)!;
      expect(p.region).toBe(sinNodo.id);
      expect(adentro(p, sinNodo)).toBe(true);
    }
  });
});

describe('tamaño de la escena', () => {
  it('abarca todas las regiones, para poder encuadrarla', () => {
    const escena = construirEscena(
      foto({
        nodos: [nodo('lab-control-plane', '172.23.0.2'), nodo('lab-worker', '172.23.0.4')],
        pods: [pod('a', 'lab-worker'), pod('b', 'lab-worker'), pod('c', 'lab-worker'), pod('p', null, null)],
      }),
    );

    for (const r of escena.regiones) {
      expect(adentro(r, { x: 0, y: 0, ancho: escena.ancho, alto: escena.alto })).toBe(true);
    }
  });
});

describe('filtros de namespace', () => {
  const lab = () =>
    foto({
      nodos: [nodo('lab-control-plane', '172.23.0.2'), nodo('lab-worker', '172.23.0.4'), nodo('lab-worker2', '172.23.0.3')],
      pods: [
        podEn('kube-system', 'coredns-7db6d8ff4d-x2v9q', 'lab-control-plane'),
        podEn('kube-system', 'kube-proxy-4hk2m', 'lab-worker'),
        podEn('ingress-nginx', 'ingress-nginx-controller-5c9f8', 'lab-worker2'),
        podEn('local-path-storage', 'local-path-provisioner-6f8956', 'lab-control-plane'),
        podEn('kviz-system', 'kviz-sondeo', 'lab-worker'),
        podEn('default', 'backend-79f9ffc666-k2x8p', 'lab-worker'),
        podEn('default', 'nginx', 'lab-worker'),
        podEn('datos', 'db-0', 'lab-worker2'),
      ],
    });
  const nombres = (e: ReturnType<typeof construirEscena>) => e.pods.map((p) => p.nombre).sort();

  it('oculta por defecto los pods de los namespaces del sistema', () => {
    const escena = construirEscena(lab());

    expect(nombres(escena)).toEqual(['backend-79f9ffc666-k2x8p', 'db-0', 'nginx']);
  });

  it('con el interruptor muestra los namespaces del sistema, salvo kviz-system', () => {
    const escena = construirEscena(lab(), { mostrarSistema: true });

    expect(nombres(escena)).toEqual([
      'backend-79f9ffc666-k2x8p',
      'coredns-7db6d8ff4d-x2v9q',
      'db-0',
      'ingress-nginx-controller-5c9f8',
      'kube-proxy-4hk2m',
      'local-path-provisioner-6f8956',
      'nginx',
    ]);
  });

  it('el filtro por namespace deja solo los pods de ese namespace', () => {
    const escena = construirEscena(lab(), { namespace: 'datos' });

    expect(nombres(escena)).toEqual(['db-0']);
    // los nodos siguen todos, aunque queden vacíos
    expect(escena.regiones.map((r) => r.nodo)).toEqual(['lab-control-plane', 'lab-worker', 'lab-worker2']);
  });

  it('el filtro ofrece los namespaces con pods, sin los del sistema mientras están ocultos', () => {
    expect(namespacesDelFiltro(lab(), {})).toEqual(['datos', 'default']);
    expect(namespacesDelFiltro(lab(), { namespace: 'datos' })).toEqual(['datos', 'default']);
    expect(namespacesDelFiltro(lab(), { mostrarSistema: true })).toEqual([
      'datos',
      'default',
      'ingress-nginx',
      'kube-system',
      'local-path-storage',
    ]);
  });

  it('un pod del recorrido se ve aunque los filtros lo oculten, salvo en kviz-system', () => {
    const ic = 'pod:ingress-nginx/ingress-nginx-controller-5c9f8';
    const sondeo = 'pod:kviz-system/kviz-sondeo';

    const escena = construirEscena(lab(), { namespace: 'default', podsDelRecorrido: [ic, sondeo] });

    expect(nombres(escena)).toEqual(['backend-79f9ffc666-k2x8p', 'ingress-nginx-controller-5c9f8', 'nginx']);
    const pod = escena.pods.find((p) => p.id === ic)!;
    expect(adentro(pod, escena.regiones.find((r) => r.nodo === 'lab-worker2')!)).toBe(true);
  });
});

function podEn(namespace: string, nombre: string, nodo: string | null): Pod {
  return { ...pod(nombre, nodo), namespace };
}

const adentro =(c: Caja, region: Caja) =>
  c.x >= region.x && c.y >= region.y && c.x + c.ancho <= region.x + region.ancho && c.y + c.alto <= region.y + region.alto;
