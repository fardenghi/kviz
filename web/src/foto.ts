// Contrato de la foto del cluster, tal como la entrega GET /api/foto y el stream (ver tiempoReal.ts).
// Es el espejo de internal/foto/foto.go: si se agrega un campo allá, se agrega acá.
// Cada objeto de una lista lleva un `id` único en su lista, y las listas van ordenadas por id.
// (*) Los campos que solo usa el panel de detalle son opcionales acá para que los fixtures de la
// escena no tengan que llevarlos; el backend los manda siempre.

export interface Foto {
  /** Contexto de kubectl de kviz, por ejemplo "kind-lab"; "" si kubectl no tiene contexto. */
  contexto: string;
  /** false cuando kviz no ve el cluster; entonces nodos y pods vienen vacíos. */
  conectado: boolean;
  /** Por qué kviz está desconectado, en español. */
  motivo?: string;
  /** Ordenados por id (el nombre). */
  nodos: Nodo[];
  /** Ordenados por id (namespace y nombre). */
  pods: Pod[];
  /** Workloads de arriba de cada cadena de dueños, ordenados por id. */
  workloads: Workload[];
  /** Ordenados por id (namespace y nombre). */
  services: Service[];
  /** Ordenados por id (namespace y nombre). */
  ingress: Ingress[];
  /** Ordenadas por id (nodo, puerto y protocolo: el orden del texto, no el numérico). */
  puertas: Puerta[];
  /** ConfigMaps con su contenido, ordenados por id. Panel de detalle (*). */
  configMaps?: ConfigMap[];
  /** Secrets sin valores (solo sus claves), ordenados por id. Panel de detalle (*). */
  secrets?: Secret[];
}

export interface Nodo {
  /** El nombre del nodo. */
  id: string;
  nombre: string;
  internalIP: string;
  /** status.capacity del nodo. Solo la usa el panel de detalle (*). */
  capacidad?: Capacidad;
}

/** Capacidad de un nodo en unidades de Kubernetes ("8", "16365916Ki", "110"); "" si no la informa. */
export interface Capacidad {
  cpu: string;
  memoria: string;
  pods: string;
}

export interface Pod {
  /** `<namespace>/<nombre>`. */
  id: string;
  nombre: string;
  namespace: string;
  /** Labels del pod ({} si no tiene). Solo los usa el panel de detalle (*). */
  labels?: Record<string, string>;
  /** Hora de creación en RFC 3339 (UTC), para la edad; "" si no se sabe. Panel de detalle (*). */
  creado?: string;
  /** null cuando el scheduler todavía no le asignó un nodo. */
  nodo: string | null;
  /** null mientras el pod no tiene IP. */
  ip: string | null;
  /** status.phase: Pending, Running, Succeeded, Failed o Unknown. */
  fase: string;
  /** Columna STATUS de kubectl get pods: Running, Init:0/1, CrashLoopBackOff, Completed, Terminating… */
  estado: string;
  /** En orden de vida: init, normales y ephemeral. */
  containers: Container[];
  /** id del workload dueño del pod; null si es un Pod suelto. */
  workload: string | null;
  /** Puertos de sus containers que se abren en su nodo. */
  hostPorts: HostPort[];
  /** Volúmenes del pod, en el orden de spec.volumes. Panel de detalle (*). */
  volumenes?: Volumen[];
}

export type TipoDeWorkload = 'Deployment' | 'ReplicaSet' | 'StatefulSet' | 'DaemonSet' | 'Job' | 'CronJob';

/**
 * Workload de arriba de una cadena de dueños: Deployment, StatefulSet, DaemonSet, CronJob, o un
 * ReplicaSet o Job sin dueño.
 */
export interface Workload {
  /** `<Tipo>/<namespace>/<nombre>`, por ejemplo `Deployment/default/backend`. */
  id: string;
  tipo: TipoDeWorkload;
  nombre: string;
  namespace: string;
  /** Color CSS propio del workload, estable entre actualizaciones. */
  color: string;
  /**
   * Cadena de dueños de arriba hacia abajo: el workload y después sus ReplicaSets o Jobs, del más
   * nuevo al más viejo. Durante un rollout trae los dos ReplicaSets.
   */
  cadena: Eslabon[];
  /** Réplicas deseadas y listas; null en un Job o un CronJob. Panel de detalle (*). */
  replicas?: Replicas | null;
  /** Estrategia de rollout ("RollingUpdate (maxSurge 25%, maxUnavailable 25%)", "Recreate", "OnDelete"…); "" si no tiene. Panel de detalle (*). */
  estrategia?: string;
}

export interface Replicas {
  deseadas: number;
  listas: number;
}

/** Objeto de la cadena de dueños con los pods que crea directamente. */
export interface Eslabon {
  tipo: TipoDeWorkload;
  nombre: string;
  /** Nombres de sus pods (mismo namespace que el workload). */
  pods: string[];
}

/** normal (spec.containers), init (spec.initContainers) o ephemeral (agregado por kubectl debug). */
export type TipoContainer = 'normal' | 'init' | 'ephemeral';

export interface Container {
  nombre: string;
  tipo: TipoContainer;
  imagen: string;
  puertos: Puerto[];
  /** Running, el motivo de la espera (CrashLoopBackOff, PodInitializing…) o de la terminación (Completed, Error…). */
  estado: string;
  ready: boolean;
  started: boolean;
  reinicios: number;
  /** Variables de entorno con su origen: las de envFrom y después las de env. Panel de detalle (*). */
  env?: Variable[];
  /** Volúmenes del pod montados en el container. Panel de detalle (*). */
  montajes?: Montaje[];
}

/** De dónde sale una variable de entorno. */
export type OrigenDeVariable = 'valor' | 'ConfigMap' | 'Secret' | 'DownwardAPI';

/**
 * Una variable de entorno tal como la ve el proceso: las de env pisan a las de envFrom con el
 * mismo nombre. Espejo de internal/foto/env.go.
 */
export interface Variable {
  /** "" en un envFrom cuyo ConfigMap o Secret no existe (no se sabe qué claves trae). */
  nombre: string;
  origen: OrigenDeVariable;
  /** El ConfigMap o el Secret, o el campo del Downward API (metadata.name, limits.cpu…); "" en un valor. */
  fuente: string;
  /** La clave del ConfigMap o del Secret; "" en los demás orígenes. */
  clave: string;
  /**
   * El valor resuelto. Siempre null si sale de un Secret: se revela a pedido (revelarSecret en
   * api.ts). También null si falta la fuente o es un campo del Downward API que la foto no resuelve.
   */
  valor: string | null;
  /** Viene de un envFrom (todas las claves de un ConfigMap o Secret). */
  envFrom: boolean;
  /** El ConfigMap, el Secret o la clave no existen. */
  falta: boolean;
}

/** Un volumen del pod (spec.volumes). */
export interface Volumen {
  nombre: string;
  /** ConfigMap, Secret, emptyDir, hostPath, PersistentVolumeClaim, projected, downwardAPI… u "otro". */
  tipo: string;
  /** El ConfigMap, el Secret o el PVC, o la ruta del nodo de un hostPath; "" si no tiene. */
  fuente: string;
}

/** Un volumen montado en un container (volumeMounts). */
export interface Montaje {
  volumen: string;
  ruta: string;
  soloLectura: boolean;
  /** Archivo o carpeta del volumen que se monta; "" si se monta entero. */
  subPath: string;
}

export interface ConfigMap {
  /** `<namespace>/<nombre>`. */
  id: string;
  nombre: string;
  namespace: string;
  /** El contenido (data). */
  datos: Record<string, string>;
}

/** Un Secret sin valores: solo sus claves. Los valores se revelan a pedido, de a uno. */
export interface Secret {
  /** `<namespace>/<nombre>`. */
  id: string;
  nombre: string;
  namespace: string;
  /** Opaque, kubernetes.io/tls… */
  tipo: string;
  claves: string[];
}

export interface Puerto {
  numero: number;
  /** Nombre del puerto ("http"), si tiene. */
  nombre?: string;
  protocolo: string;
}

export interface HostPort {
  /** El hostPort: el número que se abre en el nodo. */
  puerto: number;
  protocolo: string;
  container: string;
  containerPort: number;
}

export interface Service {
  /** `<namespace>/<nombre>`. */
  id: string;
  nombre: string;
  namespace: string;
  /** spec.type: ClusterIP, NodePort, LoadBalancer o ExternalName. */
  tipo: string;
  /** "None" en un Service headless; "" si no tiene (ExternalName). */
  clusterIP: string;
  puertos: PuertoService[];
  selector: Record<string, string>;
  /** Endpoints listos y no listos, de las EndpointSlices del Service; ordenados por IP. */
  endpoints: Endpoint[];
}

export interface PuertoService {
  nombre: string;
  protocolo: string;
  puerto: number;
  /** Número, o el nombre de un puerto de los containers. */
  targetPort: number | string;
  /** Solo si el Service abre un puerto en cada nodo. */
  nodePort?: number;
}

export interface Endpoint {
  ip: string;
  /** Nombre del pod (en el namespace del Service) detrás del endpoint, si se sabe. */
  pod: string | null;
  nodo: string | null;
  /** conditions.ready de la EndpointSlice. */
  listo: boolean;
}

export interface Ingress {
  /** `<namespace>/<nombre>`. */
  id: string;
  nombre: string;
  namespace: string;
  /** spec.ingressClassName (o la anotación vieja kubernetes.io/ingress.class); null si no tiene. */
  clase: string | null;
  /** Anotación nginx.ingress.kubernetes.io/rewrite-target; vale para todas las reglas. */
  rewrite: string | null;
  /** En el orden del Ingress; el backend por defecto, si hay, va al final sin host ni path. */
  reglas: ReglaIngress[];
}

export interface ReglaIngress {
  /** "" si la regla vale para cualquier host. */
  host: string;
  path: string;
  /** Exact, Prefix o ImplementationSpecific; "" en el backend por defecto. */
  tipoPath: string;
  /** Service destino, en el namespace del Ingress. */
  service: string;
  /** Puerto del Service: número, o el nombre de un puerto del Service. */
  puerto: number | string;
}

/**
 * Puerto abierto en un nodo hacia el cluster: un NodePort (una puerta en cada nodo) o un hostPort
 * (una puerta solo en el nodo de su pod).
 */
export interface Puerta {
  /** `<nodo>/<puerto>/<protocolo>`: en un nodo, un puerto y protocolo abre una sola puerta. */
  id: string;
  nodo: string;
  puerto: number;
  protocolo: string;
  tipo: 'NodePort' | 'hostPort';
  /** Namespace del Service o del pod al que lleva. */
  namespace: string;
  /** Service al que lleva una puerta NodePort; null en las de hostPort. */
  service: string | null;
  /** Pod al que lleva una puerta de hostPort; null en las NodePort. */
  pod: string | null;
  /** Puerto del Service (NodePort) o del container (hostPort) al que llega. */
  haciaPuerto: number;
}
