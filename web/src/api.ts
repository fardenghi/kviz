// Cliente de la API de kviz. La interfaz no habla nunca directo con Kubernetes.
import type { Foto } from './foto';
import type { Mensaje } from './tiempoReal';

export async function pedirFoto(): Promise<Foto> {
  const res = await fetch('/api/foto');
  if (!res.ok) throw new Error(`GET /api/foto respondió ${res.status}`);
  return res.json();
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
