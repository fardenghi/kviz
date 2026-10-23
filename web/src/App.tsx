import { useEffect, useMemo, useState } from 'react';
import { escucharFoto } from './api';
import { construirEscena, namespacesDelFiltro } from './escena/escena';
import { Lienzo } from './lienzo/Lienzo';
import { EN_VIVO_INICIAL, type EnVivo, finDelBrillo, podsNuevos, recibir } from './tiempoReal';

export function App() {
  // la foto llega por el canal de streaming y se actualiza sola (ver tiempoReal.ts)
  const [enVivo, setEnVivo] = useState<EnVivo>(EN_VIVO_INICIAL);
  const [cortado, setCortado] = useState(false);
  // se cambia cuando termina el brillo de los pods nuevos, para volver a armar la escena sin él
  const [finBrillo, setFinBrillo] = useState(0);
  const [mostrarSistema, setMostrarSistema] = useState(false);
  const [namespace, setNamespace] = useState<string | null>(null);

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
  const namespaces = useMemo(() => (foto ? namespacesDelFiltro(foto, { mostrarSistema }) : []), [foto, mostrarSistema]);
  // si el namespace elegido deja de ofrecerse (se ocultó el sistema o se fue), el filtro vuelve a "todos"
  const filtro = namespace !== null && namespaces.includes(namespace) ? namespace : null;
  const escena = useMemo(
    () =>
      foto?.conectado
        ? construirEscena(foto, {
            fotoAnterior: enVivo.anterior ?? undefined,
            mostrarSistema,
            namespace: filtro,
            podsNuevos: podsNuevos(enVivo, Date.now()),
          })
        : null,
    [enVivo, finBrillo, mostrarSistema, filtro], // eslint-disable-line react-hooks/exhaustive-deps
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
        {foto?.conectado && (
          <>
            <label className="campo">
              namespace
              <select value={filtro ?? ''} onChange={(e) => setNamespace(e.target.value || null)}>
                <option value="">todos</option>
                {namespaces.map((n) => (
                  <option key={n} value={n}>
                    {n}
                  </option>
                ))}
              </select>
            </label>
            <label className="campo">
              <input type="checkbox" checked={mostrarSistema} onChange={(e) => setMostrarSistema(e.target.checked)} />
              namespaces del sistema
            </label>
          </>
        )}
      </header>
      {cortado && <div className="aviso">Se cortó la conexión con kviz; reintentando…</div>}
      {!cortado && !foto && <div className="aviso">Cargando…</div>}
      {!cortado && foto && !foto.conectado && <div className="aviso">{foto.motivo ?? 'Desconectado del cluster.'}</div>}
      {escena && <Lienzo escena={escena} />}
    </div>
  );
}
