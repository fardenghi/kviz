// Un comando sugerido con su botón de copiar. kviz nunca lo ejecuta: solo lo muestra.
import { useEffect, useState } from 'react';

export function Comando({ comando, descripcion }: { comando: string; descripcion?: string }) {
  const [copiado, setCopiado] = useState(false);
  useEffect(() => {
    if (!copiado) return;
    const t = setTimeout(() => setCopiado(false), 1500);
    return () => clearTimeout(t);
  }, [copiado]);
  const copiar = () =>
    navigator.clipboard
      .writeText(comando)
      .then(() => setCopiado(true))
      .catch(() => {}); // sin permiso para el portapapeles, queda el texto para seleccionarlo a mano
  return (
    <div className="comando">
      {descripcion && <div className="tenue-texto comando-descripcion">{descripcion}</div>}
      <div className="comando-linea">
        <code>{comando}</code>
        <button className="boton" title="Copiar al portapapeles" onClick={copiar}>
          {copiado ? 'Copiado' : 'Copiar'}
        </button>
      </div>
    </div>
  );
}
