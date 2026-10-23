// Cliente de la API de kviz. La interfaz no habla nunca directo con Kubernetes.
import type { Foto } from './foto';

export async function pedirFoto(): Promise<Foto> {
  const res = await fetch('/api/foto');
  if (!res.ok) throw new Error(`GET /api/foto respondió ${res.status}`);
  return res.json();
}
