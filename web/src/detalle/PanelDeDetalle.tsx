// Panel de detalle (superficie flotante, cajón a la derecha encima del lienzo): toda la
// información del objeto seleccionado. Cerrarlo no toca el lienzo (zoom, arrastre, filtros).
// El contenido son las secciones de ./secciones/ (ver secciones.ts).
import type { Foto } from '../foto';
import { type Detalle, fichaDe } from './detalle';
import { SECCIONES } from './secciones';

export function PanelDeDetalle({
  detalle,
  foto,
  abrir,
  probar,
  alCerrar,
}: {
  detalle: Detalle;
  foto: Foto;
  abrir: (id: string) => void;
  /** Abre el formulario de request de prueba con el destino ya completado. */
  probar?: (destino: string) => void;
  alCerrar: () => void;
}) {
  const { tipo, nombre } = fichaDe(detalle).titulo;
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
        <Seccion key={clave} detalle={detalle} foto={foto} abrir={abrir} probar={probar} />
      ))}
    </aside>
  );
}
