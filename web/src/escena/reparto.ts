// Reparto de una request de prueba repetida N veces: una pastilla `n/N` sobre cada pod que
// respondió. Las respuestas sin pod identificado y las que no llegaron solo se ven en la tarjeta
// del recorrido. No hay etiquetas de reparto sobre las líneas (ruido que se probó y se sacó).
import type { Escena, EstadoInterfaz } from './escena';
import { idPod } from './escena';

export function aplicarReparto(escena: Escena, estado: EstadoInterfaz): Escena {
  const reparto = estado.reparto;
  if (!reparto) return escena;
  const pastillas = new Map(
    reparto.pods.map(({ pod, n }) => {
      const corte = pod.indexOf('/');
      return [idPod({ namespace: pod.slice(0, corte), nombre: pod.slice(corte + 1) }), `${n}/${reparto.total}`];
    }),
  );
  return { ...escena, pods: escena.pods.map((p) => (pastillas.has(p.id) ? { ...p, reparto: pastillas.get(p.id) } : p)) };
}
