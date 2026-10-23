// Costura 2: las advertencias de configuración de la foto se ven como una marca roja sobre su
// objeto (pod, Service o Ingress) en la escena de la vista de topología.
import { describe, expect, it } from 'vitest';
import type { Advertencia, Foto, Ingress, Pod, Service } from '../foto';
import { construirEscena } from './escena';

const pod = (nombre: string, namespace = 'default'): Pod => ({
  id: `${namespace}/${nombre}`,
  nombre,
  namespace,
  nodo: 'lab-worker',
  ip: '10.244.1.5',
  fase: 'Running',
  estado: 'Running',
  containers: [],
  workload: null,
  hostPorts: [],
});
const service = (nombre: string): Service => ({
  id: `default/${nombre}`,
  nombre,
  namespace: 'default',
  tipo: 'ClusterIP',
  clusterIP: '10.96.0.10',
  puertos: [{ nombre: '', protocolo: 'TCP', puerto: 80, targetPort: 8080 }],
  selector: { app: nombre },
  endpoints: [],
});
const ingress = (nombre: string): Ingress => ({
  id: `default/${nombre}`,
  nombre,
  namespace: 'default',
  clase: null,
  clasePorDefecto: false,
  rewrite: null,
  reglas: [{ host: '', path: '/', tipoPath: 'Prefix', service: 'backend', puerto: 80 }],
});
const advertencia = (tipo: Advertencia['tipo'], nombre: string, regla: string, titulo: string, namespace = 'default'): Advertencia => ({
  id: `${tipo}/${namespace}/${nombre}/${regla}`,
  tipo,
  namespace,
  nombre,
  regla,
  titulo,
  explicacion: 'explicación',
  comando: { descripcion: 'revisar', comando: 'kubectl describe' },
});
const foto = (parte: Partial<Foto>): Foto => ({
  contexto: 'kind-lab',
  conectado: true,
  nodos: [{ id: 'lab-worker', nombre: 'lab-worker', internalIP: '172.23.0.4' }],
  pods: [pod('api-1'), pod('nginx'), pod('coredns', 'kube-system')],
  services: [service('backend'), service('web')],
  workloads: [],
  ingress: [ingress('backend'), ingress('web')],
  puertas: [],
  ...parte,
});

describe('marca de advertencia', () => {
  it('el pod, el Service y el Ingress con advertencias llevan sus títulos; los demás, ninguno', () => {
    const escena = construirEscena(
      foto({
        advertencias: [
          advertencia('Ingress', 'backend', 'ingress-sin-clase', 'Ingress sin IngressClass'),
          advertencia('Pod', 'api-1', 'probe-puerto-no-expuesto/app/liveness', 'La liveness probe apunta a un puerto que el container no expone'),
          advertencia('Pod', 'api-1', 'probe-puerto-no-expuesto/app/readiness', 'La readiness probe apunta a un puerto que el container no expone'),
          advertencia('Service', 'backend', 'service-sin-pods', 'El selector no matchea ningún pod'),
        ],
      }),
    );

    const de = <T extends { nombre: string; advertencias: string[] }>(lista: T[], nombre: string) =>
      lista.find((o) => o.nombre === nombre)!.advertencias;
    expect(de(escena.pods, 'api-1')).toEqual([
      'La liveness probe apunta a un puerto que el container no expone',
      'La readiness probe apunta a un puerto que el container no expone',
    ]);
    expect(de(escena.pods, 'nginx')).toEqual([]);
    expect(de(escena.services, 'backend')).toEqual(['El selector no matchea ningún pod']);
    expect(de(escena.services, 'web')).toEqual([]);
    expect(de(escena.ingress, 'backend')).toEqual(['Ingress sin IngressClass']);
    expect(de(escena.ingress, 'web')).toEqual([]);
  });

  it('una advertencia de otro objeto con el mismo nombre (otro tipo u otro namespace) no marca', () => {
    const escena = construirEscena(
      foto({
        advertencias: [
          advertencia('Pod', 'backend', 'pending-sin-recursos', 'Pending: ningún nodo tiene lugar'),
          advertencia('Service', 'web', 'service-sin-pods', 'El selector no matchea ningún pod', 'otro'),
        ],
      }),
    );

    for (const o of [...escena.pods, ...escena.services, ...escena.ingress]) expect(o.advertencias).toEqual([]);
  });

  it('sin advertencias en la foto no hay marcas, y las de objetos ocultos no aparecen', () => {
    expect(construirEscena(foto({})).pods.every((p) => p.advertencias.length === 0)).toBe(true);

    const escena = construirEscena(
      foto({ advertencias: [advertencia('Pod', 'coredns', 'imagen-local-sin-bajar/coredns', 'ImagePullBackOff', 'kube-system')] }),
    );
    expect(escena.pods.map((p) => p.nombre)).not.toContain('coredns');
    expect(escena.pods.every((p) => p.advertencias.length === 0)).toBe(true);
  });
});
