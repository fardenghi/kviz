// Piezas comunes de las secciones del panel de detalle.
import type { ReactNode } from 'react';

/** Una sección con título. */
export function Seccion({ titulo, children }: { titulo?: string; children: ReactNode }) {
  return (
    <section className="detalle-seccion">
      {titulo && <h3 className="detalle-seccion-titulo">{titulo}</h3>}
      {children}
    </section>
  );
}

/** Pares nombre → valor; las filas con valor null no se muestran. */
export function Datos({ filas }: { filas: [string, ReactNode][] }) {
  return (
    <dl className="datos">
      {filas
        .filter(([, v]) => v !== null)
        .map(([k, v]) => (
          <div key={k} className="datos-fila">
            <dt>{k}</dt>
            <dd>{v}</dd>
          </div>
        ))}
    </dl>
  );
}

/** Un objeto relacionado que se abre en el panel al hacerle clic. */
export function Enlace({ id, abrir, children }: { id: string; abrir: (id: string) => void; children: ReactNode }) {
  return (
    <button className="enlace" onClick={() => abrir(id)}>
      {children}
    </button>
  );
}

export const Nada = ({ children }: { children: ReactNode }) => <i className="tenue-texto">{children}</i>;

/** Lista `clave=valor` en mono, uno por línea; "—" si no hay. */
export function Pares({ pares }: { pares: string[] }) {
  if (pares.length === 0) return <Nada>—</Nada>;
  return (
    <span className="mono-texto">
      {pares.map((p) => (
        <div key={p}>{p}</div>
      ))}
    </span>
  );
}
