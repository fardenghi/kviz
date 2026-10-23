// Costura 2, tiempo real: la foto llega por el stream (completa y después como cambios) y la
// escena marca los pods que aparecen y atenúa los Terminating.
import { describe, expect, it } from 'vitest';
import { construirEscena } from './escena/escena';
import type { Foto, Pod } from './foto';
import { BRILLO_POD_NUEVO, EN_VIVO_INICIAL, type EnVivo, finDelBrillo, type Mensaje, podsNuevos, recibir } from './tiempoReal';

const pod = (nombre: string, parte: Partial<Pod> = {}): Pod => ({
  id: `default/${nombre}`,
  nombre,
  namespace: 'default',
  nodo: 'lab-worker',
  ip: '10.244.1.5',
  fase: 'Running',
  estado: 'Running',
  containers: [],
  workload: null,
  hostPorts: [],
  ...parte,
});
const foto = (pods: Pod[]): Foto => ({
  contexto: 'kind-lab',
  conectado: true,
  nodos: [{ id: 'lab-worker', nombre: 'lab-worker', internalIP: '172.23.0.4' }],
  pods,
  services: [],
  workloads: [],
  ingress: [],
  puertas: [],
});

const recibirTodos = (mensajes: Mensaje[], ahora = 0): EnVivo =>
  mensajes.reduce((e, m) => recibir(e, m, ahora), EN_VIVO_INICIAL);

const escenaEn = (e: EnVivo, ahora: number) =>
  construirEscena(e.foto!, { fotoAnterior: e.anterior ?? undefined, podsNuevos: podsNuevos(e, ahora) });
const enEscena = (e: EnVivo, ahora: number, nombre: string) => escenaEn(e, ahora).pods.find((p) => p.nombre === nombre)!;

describe('pods nuevos', () => {
  it('marca como nuevo el pod que aparece en una actualización, y no a los que ya estaban', () => {
    const e = recibirTodos([
      { tipo: 'foto', foto: foto([pod('nginx')]) },
      { tipo: 'cambios', cambios: [{ seccion: 'pods', id: 'default/web-2', valor: pod('web-2') }] },
    ]);

    expect(enEscena(e, 100, 'web-2').nuevo).toBe(true);
    expect(enEscena(e, 100, 'nginx').nuevo).toBe(false);
  });

  it('brilla exactamente 3 veces de 1,4 s y después deja de ser nuevo', () => {
    const e = recibirTodos(
      [
        { tipo: 'foto', foto: foto([]) },
        { tipo: 'cambios', cambios: [{ seccion: 'pods', id: 'default/web-2', valor: pod('web-2') }] },
      ],
      10_000,
    );

    expect(BRILLO_POD_NUEVO).toEqual({ veces: 3, msPorVez: 1400 });
    expect(enEscena(e, 10_000 + 4_199, 'web-2').nuevo).toBe(true);
    expect(enEscena(e, 10_000 + 4_200, 'web-2').nuevo).toBe(false);
    expect(finDelBrillo(e, 10_000)).toBe(14_200);
    expect(finDelBrillo(e, 14_200)).toBeNull();
  });

  it('al recibir la foto completa (al cargar o reconectar) no brilla nada', () => {
    const e = recibirTodos([
      { tipo: 'foto', foto: foto([pod('nginx')]) },
      { tipo: 'cambios', cambios: [{ seccion: 'pods', id: 'default/web-2', valor: pod('web-2') }] },
      { tipo: 'foto', foto: foto([pod('nginx'), pod('web-2'), pod('web-3')]) },
    ]);

    expect(escenaEn(e, 100).pods.map((p) => p.nuevo)).toEqual([false, false, false]);
  });
});

describe('pods Terminating', () => {
  it('atenúa el pod que pasa a Terminating hasta que desaparece de la foto', () => {
    const terminando = recibirTodos([
      { tipo: 'foto', foto: foto([pod('nginx'), pod('web-1')]) },
      { tipo: 'cambios', cambios: [{ seccion: 'pods', id: 'default/web-1', valor: pod('web-1', { estado: 'Terminating' }) }] },
    ]);

    expect(enEscena(terminando, 0, 'web-1')).toMatchObject({ terminando: true, opacidad: 0.4 });
    expect(enEscena(terminando, 0, 'nginx')).toMatchObject({ terminando: false, opacidad: 1 });

    const borrado = recibir(terminando, { tipo: 'cambios', cambios: [{ seccion: 'pods', id: 'default/web-1', borrado: true }] }, 0);
    expect(escenaEn(borrado, 0).pods.map((p) => p.nombre)).toEqual(['nginx']);
  });
});

describe('cambios de la foto', () => {
  it('modifica un objeto en su lugar, agrega en orden de id y reemplaza secciones sin id', () => {
    const e = recibirTodos([
      { tipo: 'foto', foto: foto([pod('a'), pod('c', { ip: null })]) },
      {
        tipo: 'cambios',
        cambios: [
          { seccion: 'pods', id: 'default/c', valor: pod('c', { ip: '10.244.1.9' }) },
          { seccion: 'pods', id: 'default/b', valor: pod('b') },
          { seccion: 'conectado', valor: false },
          { seccion: 'motivo', valor: 'el cluster no responde' },
        ],
      },
    ]);

    expect(e.foto!.pods.map((p) => [p.nombre, p.ip])).toEqual([
      ['a', '10.244.1.5'],
      ['b', '10.244.1.5'],
      ['c', '10.244.1.9'],
    ]);
    expect(e.foto).toMatchObject({ conectado: false, motivo: 'el cluster no responde' });
  });
});

describe('foto anterior', () => {
  it('recuerda la foto de antes de la última actualización, para que titilen los reinicios que aumentan', () => {
    const conReinicios = (n: number) =>
      pod('api', {
        containers: [
          { nombre: 'api', tipo: 'normal', imagen: 'api:1', puertos: [], estado: 'Running', ready: true, started: true, reinicios: n },
        ],
      });
    const e = recibirTodos([
      { tipo: 'foto', foto: foto([conReinicios(1), pod('nginx')]) },
      { tipo: 'cambios', cambios: [{ seccion: 'pods', id: 'default/api', valor: conReinicios(2) }] },
    ]);

    expect(enEscena(e, 0, 'api')).toMatchObject({ reinicios: 2, reiniciosTitilan: true });

    const otro = recibir(e, { tipo: 'cambios', cambios: [{ seccion: 'pods', id: 'default/nginx', valor: pod('nginx', { ip: null }) }] }, 0);
    expect(enEscena(otro, 0, 'api').reiniciosTitilan).toBe(false);
  });
});
