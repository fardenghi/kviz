// Panel de detalle (superficie flotante, cajón a la derecha encima del lienzo): toda la
// información del objeto seleccionado. Cerrarlo no toca el lienzo (zoom, arrastre, filtros).
// El contenido son las secciones de ./secciones/ (ver secciones.ts).
import type { Foto } from '../foto';
import type { Detalle } from './detalle';
import { SECCIONES } from './secciones';

function titulo(d: Detalle): { tipo: string; nombre: string } {
  switch (d.tipo) {
    case 'Pod':
      return { tipo: 'Pod', nombre: d.pod.nombre };
    case 'Service':
      return { tipo: `Service · ${d.service.tipo}`, nombre: d.service.nombre };
    case 'Ingress':
      return { tipo: 'Ingress', nombre: d.ingress.nombre };
    case 'Nodo':
      return { tipo: 'Nodo', nombre: d.nodo.nombre };
    case 'Workload':
      return { tipo: d.workload.tipo, nombre: d.workload.nombre };
  }
}

export function PanelDeDetalle({
  detalle,
  foto,
  abrir,
  alCerrar,
}: {
  detalle: Detalle;
  foto: Foto;
  abrir: (id: string) => void;
  alCerrar: () => void;
}) {
  const { tipo, nombre } = titulo(detalle);
  return (
    <aside className="flotante panel-detalle" aria-label="Panel de detalle">
      <div className="detalle-cabecera">
        <div>
          <div className="tenue-texto detalle-tipo">{tipo}</div>
          <div className="detalle-titulo">{nombre}</div>
        </div>
        <button className="boton" title="Cerrar (Esc)" onClick={alCerrar}>
          ✕
        </button>
      </div>
      {SECCIONES.map(({ clave, Seccion }) => (
        <Seccion key={clave} detalle={detalle} foto={foto} abrir={abrir} />
      ))}
    </aside>
  );
}
