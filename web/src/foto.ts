// Contrato de la foto del cluster, tal como la entrega GET /api/foto.
// Es el espejo de internal/foto/foto.go: si se agrega un campo allá, se agrega acá.

export interface Foto {
  /** Contexto de kubectl de kviz, por ejemplo "kind-lab"; "" si kubectl no tiene contexto. */
  contexto: string;
  /** false cuando kviz no ve el cluster; entonces nodos y pods vienen vacíos. */
  conectado: boolean;
  /** Por qué kviz está desconectado, en español. */
  motivo?: string;
  /** Ordenados por nombre. */
  nodos: Nodo[];
  /** Ordenados por namespace y nombre. */
  pods: Pod[];
  /** Ordenados por namespace y nombre. */
  services: Service[];
  /** Ordenados por namespace y nombre. */
  ingress: Ingress[];
  /** Ordenadas por nodo, con las NodePort primero, y por puerto. */
  puertas: Puerta[];
}

export interface Nodo {
  nombre: string;
  internalIP: string;
}

export interface Pod {
  nombre: string;
  namespace: string;
  /** null cuando el scheduler todavía no le asignó un nodo. */
  nodo: string | null;
  /** null mientras el pod no tiene IP. */
  ip: string | null;
  /** status.phase: Pending, Running, Succeeded, Failed o Unknown. */
  fase: string;
  /** Puertos de sus containers que se abren en su nodo. */
  hostPorts: HostPort[];
}

export interface HostPort {
  /** El hostPort: el número que se abre en el nodo. */
  puerto: number;
  protocolo: string;
  container: string;
  containerPort: number;
}

export interface Service {
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
