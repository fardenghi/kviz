// Secciones del panel de detalle. Cada sección es un archivo de ./secciones/ que exporta:
//
//   export const orden = 30;                                   // dónde va en el panel
//   export default function MiSeccion(props: PropsDeSeccion) // null si no aplica a ese objeto
//
// Se cargan solas: para sumar una sección (probes, env, logs, events…) alcanza con agregar su
// archivo, sin tocar el panel ni esta lista. Orden de referencia:
//
//   10 datos del objeto · 20 advertencias de configuración · 30 probes y recursos
//   40 env, ConfigMaps, Secrets y volúmenes · 50 events · 60 logs · 90 comandos sugeridos
import type { ComponentType } from 'react';
import type { PropsDeSeccion } from './detalle';

interface ModuloDeSeccion {
  orden: number;
  default: ComponentType<PropsDeSeccion>;
}

const modulos = import.meta.glob<ModuloDeSeccion>('./secciones/*.tsx', { eager: true });

/** Las secciones en orden; a igual orden, por nombre de archivo. */
export const SECCIONES: { clave: string; Seccion: ComponentType<PropsDeSeccion> }[] = Object.entries(modulos)
  .sort(([a, ma], [b, mb]) => ma.orden - mb.orden || a.localeCompare(b))
  .map(([clave, m]) => ({ clave, Seccion: m.default }));
