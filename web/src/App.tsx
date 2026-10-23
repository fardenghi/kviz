import { useEffect, useMemo, useState } from 'react';
import { escucharFoto } from './api';
import { construirEscena } from './escena/escena';
import { Lienzo } from './lienzo/Lienzo';
import { EN_VIVO_INICIAL, type EnVivo, finDelBrillo, podsNuevos, recibir } from './tiempoReal';

export function App() {
  // la foto llega por el canal de streaming y se actualiza sola (ver tiempoReal.ts)
  const [enVivo, setEnVivo] = useState<EnVivo>(EN_VIVO_INICIAL);
  const [cortado, setCortado] = useState(false);
  // se cambia cuando termina el brillo de los pods nuevos, para volver a armar la escena sin él
  const [finBrillo, setFinBrillo] = useState(0);

  useEffect(
    () =>
      escucharFoto(
        (m) => {
          setEnVivo((e) => recibir(e, m, Date.now()));
          setCortado(false);
        },
        () => setCortado(true),
      ),
    [],
  );

  useEffect(() => {
    const fin = finDelBrillo(enVivo, Date.now());
    if (fin === null) return;
    const espera = setTimeout(() => setFinBrillo(fin), fin - Date.now());
    return () => clearTimeout(espera);
  }, [enVivo, finBrillo]);

  const foto = enVivo.foto;
  const escena = useMemo(
    () => (foto?.conectado ? construirEscena(foto, { podsNuevos: podsNuevos(enVivo, Date.now()) }) : null),
    [enVivo, finBrillo], // eslint-disable-line react-hooks/exhaustive-deps
  );

  return (
    <div className="app">
      <header className="barra">
        <b className="marca">kviz</b>
        {foto && (
          <span className="contexto" title="Contexto de kubectl">
            <span className={foto.conectado ? 'punto punto-conectado' : 'punto punto-desconectado'} />
            {foto.contexto || 'sin contexto'}
            {!foto.conectado && <span className="tenue-texto">· desconectado</span>}
          </span>
        )}
      </header>
      {cortado && <div className="aviso">Se cortó la conexión con kviz; reintentando…</div>}
      {!cortado && !foto && <div className="aviso">Cargando…</div>}
      {!cortado && foto && !foto.conectado && <div className="aviso">{foto.motivo ?? 'Desconectado del cluster.'}</div>}
      {escena && <Lienzo escena={escena} />}
    </div>
  );
}
