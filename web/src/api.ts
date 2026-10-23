// Cliente de la API de kviz. La interfaz no habla nunca directo con Kubernetes.
import type { Foto } from './foto';
import type { Mensaje } from './tiempoReal';
import type { PedidoDePrueba, Prueba } from './prueba';

export async function pedirFoto(): Promise<Foto> {
  const res = await fetch('/api/foto');
  if (!res.ok) throw new Error(`GET /api/foto respondió ${res.status}`);
  return res.json();
}

/** Los contextos de kubectl (GET /api/contextos). Espejo de internal/api/contexto.go. */
export interface Contextos {
  /** El contexto de kviz ("" si no hay). Puede faltar en `contextos` si se borró su cluster. */
  actual: string;
  contextos: string[];
  /** Por qué no se pudo leer el kubeconfig. */
  error?: string;
}

export async function pedirContextos(): Promise<Contextos> {
  const res = await fetch('/api/contextos');
  if (!res.ok) throw new Error(`GET /api/contextos respondió ${res.status}`);
  return res.json();
}

/** Cambia de contexto en caliente; la foto del nuevo cluster llega por el stream. */
export async function cambiarContexto(contexto: string): Promise<void> {
  const res = await fetch('/api/contexto', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ contexto }),
  });
  if (!res.ok) throw new Error((await res.text()).trim() || `POST /api/contexto respondió ${res.status}`);
}

const REINTENTO_MS = 2000;

/**
 * Escucha el canal de streaming de la foto (GET /api/stream, Server-Sent Events): primero llega la
 * foto completa y después los cambios. Si se corta, avisa y se reconecta solo; al reconectarse
 * llega otra vez la foto completa. Devuelve la función para dejar de escuchar.
 */
export function escucharFoto(alMensaje: (m: Mensaje) => void, alCortarse: () => void): () => void {
  let fuente: EventSource | null = null;
  let espera: ReturnType<typeof setTimeout> | undefined;
  const abrir = () => {
    fuente = new EventSource('/api/stream');
    fuente.onmessage = (e: MessageEvent<string>) => alMensaje(JSON.parse(e.data) as Mensaje);
    fuente.onerror = () => {
      alCortarse();
      // EventSource reintenta solo, salvo que el servidor haya respondido con un error
      if (fuente?.readyState === EventSource.CLOSED) {
        fuente.close();
        espera = setTimeout(abrir, REINTENTO_MS);
      }
    };
  };
  abrir();
  return () => {
    clearTimeout(espera);
    fuente?.close();
  };
}

/** Un comando sugerido de kubectl (GET /api/comandos). Espejo de internal/comandos. */
export interface ComandoSugerido {
  descripcion: string;
  comando: string;
}

/** Los comandos sugeridos de un objeto: tipo de Kubernetes (Pod, Service, Node, Deployment…), namespace y nombre. */
export async function pedirComandos(o: { tipo: string; namespace: string; nombre: string }): Promise<ComandoSugerido[]> {
  const q = new URLSearchParams({ tipo: o.tipo, namespace: o.namespace, nombre: o.nombre });
  const res = await fetch(`/api/comandos?${q}`);
  if (!res.ok) throw new Error(`GET /api/comandos respondió ${res.status}`);
  return res.json();
}

/**
 * Manda una request de prueba (POST /api/pruebas): sale del pod de sondeo del cluster (la primera
 * vez kviz lo crea, y puede tardar mientras el nodo baja la imagen). Devuelve la respuesta y el recorrido.
 */
export async function mandarPrueba(pedido: PedidoDePrueba): Promise<Prueba> {
  const res = await fetch('/api/pruebas', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(pedido),
  });
  if (!res.ok) throw new Error((await res.text()).trim() || `POST /api/pruebas respondió ${res.status}`);
  return res.json();
}

/** Historial de las requests de prueba de la sesión (GET /api/pruebas), de la más vieja a la más nueva. Vive en la memoria de kviz. */
export async function pedirHistorial(): Promise<Prueba[]> {
  const res = await fetch('/api/pruebas');
  if (!res.ok) throw new Error(`GET /api/pruebas respondió ${res.status}`);
  return res.json();
}

/**
 * Revela el valor de una clave de un Secret (POST /api/secreto). Es el único camino por el que
 * un valor de Secret sale del backend: la foto trae solo las claves.
 */
export async function revelarSecret(o: { namespace: string; nombre: string; clave: string }): Promise<string> {
  const res = await fetch('/api/secreto', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(o),
  });
  if (!res.ok) throw new Error((await res.text()).trim() || `POST /api/secreto respondió ${res.status}`);
  return ((await res.json()) as { valor: string }).valor;
}
