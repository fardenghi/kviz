// Advertencias de configuración en la escena: cada pod, Service o Ingress lleva los títulos de sus
// advertencias, que el lienzo dibuja como una marca roja. Las calcula el backend (viajan en la foto).
import type { Advertencia, Foto } from '../foto';
import { idIngress, idPod, idService } from './ids';

/** id de escena del objeto de una advertencia. */
export function idDeAdvertencia(a: Pick<Advertencia, 'tipo' | 'namespace' | 'nombre'>): string {
  switch (a.tipo) {
    case 'Pod':
      return idPod(a);
    case 'Service':
      return idService(a);
    case 'Ingress':
      return idIngress(a);
  }
}

/** Las advertencias de la foto por id de escena de su objeto, en el orden de la foto. */
export function advertenciasPorObjeto(foto: Foto): Map<string, Advertencia[]> {
  const porObjeto = new Map<string, Advertencia[]>();
  for (const a of foto.advertencias ?? []) {
    const id = idDeAdvertencia(a);
    porObjeto.set(id, [...(porObjeto.get(id) ?? []), a]);
  }
  return porObjeto;
}

/** Títulos de las advertencias de un objeto de la escena; vacío si no tiene. */
export const titulosDe = (porObjeto: Map<string, Advertencia[]>, id: string): string[] =>
  (porObjeto.get(id) ?? []).map((a) => a.titulo);
