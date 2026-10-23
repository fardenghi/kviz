// Logs de un container por el canal de streaming (GET /api/logs, Server-Sent Events).
// Espejo de internal/api/logs.go.

export interface PedidoDeLogs {
  namespace: string;
  pod: string;
  container: string;
  /** Los logs del reinicio anterior (kubectl logs --previous); si no, los actuales, en vivo. */
  anterior: boolean;
}

export type MensajeDeLogs =
  | { tipo: 'lineas'; lineas: string[] }
  /** Los logs terminaron: el container terminó, o eran los del reinicio anterior. */
  | { tipo: 'fin' }
  /** Kubernetes no dio los logs (container esperando para arrancar…) o se cortó la conexión. */
  | { tipo: 'error'; error: string };

/**
 * Abre los logs de un container y avisa cada mensaje. Los actuales siguen en vivo hasta que se
 * llama a la función devuelta, que corta el stream (y con él el pedido al cluster). Después de
 * `fin` o `error` no llega nada más. `alAbrir` avisa que el stream quedó abierto (aunque todavía no
 * haya líneas).
 */
export function escucharLogs(
  p: PedidoDeLogs,
  alMensaje: (m: MensajeDeLogs) => void,
  alAbrir?: () => void,
): () => void {
  const q = new URLSearchParams({ namespace: p.namespace, pod: p.pod, container: p.container });
  if (p.anterior) q.set('anterior', 'true');
  const fuente = new EventSource(`/api/logs?${q}`);
  let terminado = false;
  const terminar = (m: MensajeDeLogs) => {
    terminado = true;
    fuente.close(); // si no, EventSource se reconecta solo y repite los logs
    alMensaje(m);
  };
  fuente.onopen = () => alAbrir?.();
  fuente.onmessage = (e: MessageEvent<string>) => {
    const m = JSON.parse(e.data) as MensajeDeLogs;
    if (m.tipo === 'lineas') alMensaje(m);
    else terminar(m);
  };
  fuente.onerror = () => {
    if (!terminado) terminar({ tipo: 'error', error: 'se cortó la conexión con kviz o no hay logs de ese container' });
  };
  return () => {
    terminado = true;
    fuente.close();
  };
}
