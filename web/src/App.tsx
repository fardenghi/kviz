import { useEffect, useMemo, useState } from 'react';
import { pedirFoto } from './api';
import { construirEscena, namespacesDelFiltro } from './escena/escena';
import type { Foto } from './foto';
import { Lienzo } from './lienzo/Lienzo';

// Mientras kviz está desconectado vuelve a pedir la foto cada tanto. Las actualizaciones en vivo
// llegan con el canal de streaming (ticket de tiempo real).
const REINTENTO_MS = 2000;

export function App() {
  // la foto anterior va a la escena para marcar lo que cambió (reinicios que aumentaron)
  const [{ foto, fotoAnterior }, setFotos] = useState<{ foto: Foto | null; fotoAnterior?: Foto }>({ foto: null });
  const setFoto = (f: Foto) => setFotos((s) => ({ foto: f, fotoAnterior: s.foto ?? undefined }));
  const [error, setError] = useState<string | null>(null);
  const [mostrarSistema, setMostrarSistema] = useState(false);
  const [namespace, setNamespace] = useState<string | null>(null);

  useEffect(() => {
    let vivo = true;
    let espera: ReturnType<typeof setTimeout>;
    const pedir = () =>
      pedirFoto().then(
        (f) => {
          if (!vivo) return;
          setFoto(f);
          setError(null);
          if (!f.conectado) espera = setTimeout(pedir, REINTENTO_MS);
        },
        (e: Error) => {
          if (!vivo) return;
          setError(e.message);
          espera = setTimeout(pedir, REINTENTO_MS);
        },
      );
    pedir();
    return () => {
      vivo = false;
      clearTimeout(espera);
    };
  }, []);

  const namespaces = useMemo(() => (foto ? namespacesDelFiltro(foto, { mostrarSistema }) : []), [foto, mostrarSistema]);
  // si el namespace elegido deja de ofrecerse (se ocultó el sistema o se fue), el filtro vuelve a "todos"
  const filtro = namespace !== null && namespaces.includes(namespace) ? namespace : null;
  const escena = useMemo(
    () => (foto?.conectado ? construirEscena(foto, { fotoAnterior, mostrarSistema, namespace: filtro }) : null),
    [foto, fotoAnterior, mostrarSistema, filtro],
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
      {error && <div className="aviso">No se pudo hablar con kviz: {error}</div>}
      {!error && !foto && <div className="aviso">Cargando…</div>}
      {!error && foto && !foto.conectado && <div className="aviso">{foto.motivo ?? 'Desconectado del cluster.'}</div>}
      {escena && <Lienzo escena={escena} />}
    </div>
  );
}
