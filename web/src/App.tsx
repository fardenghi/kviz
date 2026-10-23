import { useEffect, useMemo, useState } from 'react';
import { cambiarContexto, escucharFoto, pedirContextos } from './api';
import { construirEscena, namespacesDelFiltro, workloadsVisibles } from './escena/escena';
import { detalleDe, idDeWorkload } from './detalle/detalle';
import { PanelDeDetalle } from './detalle/PanelDeDetalle';
import { Lienzo } from './lienzo/Lienzo';
import { ListaDeWorkloads } from './superficies/ListaDeWorkloads';
import { EN_VIVO_INICIAL, type EnVivo, finDelBrillo, podsNuevos, recibir } from './tiempoReal';

export function App() {
  // la foto llega por el canal de streaming y se actualiza sola (ver tiempoReal.ts)
  const [enVivo, setEnVivo] = useState<EnVivo>(EN_VIVO_INICIAL);
  const [cortado, setCortado] = useState(false);
  // se cambia cuando termina el brillo de los pods nuevos, para volver a armar la escena sin él
  const [finBrillo, setFinBrillo] = useState(0);
  const [mostrarSistema, setMostrarSistema] = useState(false);
  const [namespace, setNamespace] = useState<string | null>(null);
  const [workload, setWorkload] = useState<string | null>(null);
  // objeto apuntado con el mouse: es el foco (ver escena/foco.ts)
  const [hover, setHover] = useState<string | null>(null);
  // objeto elegido con un clic (id de escena, o `workload:<id>` desde la lista): da el foco y abre
  // el panel de detalle (ver detalle/detalle.ts)
  const [seleccionado, setSeleccionado] = useState<string | null>(null);
  // contextos de kubectl para el selector; se vuelven a pedir al abrirlo y cuando cambia la conexión
  const [contextos, setContextos] = useState<string[]>([]);
  const [errorContexto, setErrorContexto] = useState<string | null>(null);
  const refrescarContextos = () =>
    pedirContextos()
      .then((c) => setContextos(c.contextos))
      .catch(() => {}); // el selector queda con la lista anterior

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
  useEffect(() => {
    refrescarContextos();
  }, [foto?.contexto, foto?.conectado]); // eslint-disable-line react-hooks/exhaustive-deps
  const elegirContexto = (nombre: string) => {
    setErrorContexto(null);
    setNamespace(null);
    setWorkload(null);
    setSeleccionado(null);
    cambiarContexto(nombre).catch((e: Error) => setErrorContexto(e.message));
  };
  // el contexto de kviz va siempre en el selector, aunque su cluster se haya borrado del kubeconfig
  const opcionesDeContexto =
    foto && foto.contexto && !contextos.includes(foto.contexto) ? [foto.contexto, ...contextos] : contextos;
  const namespaces = useMemo(() => (foto ? namespacesDelFiltro(foto, { mostrarSistema }) : []), [foto, mostrarSistema]);
  // si el namespace elegido deja de ofrecerse (se ocultó el sistema o se fue), el filtro vuelve a "todos"
  const filtro = namespace !== null && namespaces.includes(namespace) ? namespace : null;
  const workloads = useMemo(
    () => (foto ? workloadsVisibles(foto, { mostrarSistema, namespace: filtro }) : []),
    [foto, mostrarSistema, filtro],
  );
  // si el workload seleccionado se borra o queda oculto por los filtros, se deselecciona
  const workloadSeleccionado = workload !== null && workloads.some((w) => w.id === workload) ? workload : null;
  const escena = useMemo(
    () =>
      foto?.conectado
        ? construirEscena(foto, {
            fotoAnterior: enVivo.anterior ?? undefined,
            mostrarSistema,
            namespace: filtro,
            workloadSeleccionado,
            hover,
            seleccionado,
            podsNuevos: podsNuevos(enVivo, Date.now()),
          })
        : null,
    [enVivo, finBrillo, mostrarSistema, filtro, workloadSeleccionado, hover, seleccionado], // eslint-disable-line react-hooks/exhaustive-deps
  );

  // el panel muestra el objeto mientras siga en la foto; si se borra, se cierra solo
  const detalle = foto?.conectado && seleccionado ? detalleDe(foto, seleccionado) : null;
  useEffect(() => {
    const alTeclear = (e: KeyboardEvent) => e.key === 'Escape' && setSeleccionado(null);
    window.addEventListener('keydown', alTeclear);
    return () => window.removeEventListener('keydown', alTeclear);
  }, []);
  // elegir un workload en la lista lo resalta y lo abre en el panel; deselegirlo cierra su panel
  const elegirWorkload = (id: string | null) => {
    setWorkload(id);
    if (id) setSeleccionado(idDeWorkload({ id }));
    else setSeleccionado((s) => (s?.startsWith('workload:') ? null : s));
  };
  // un objeto abierto desde el panel (el nodo de un pod, su workload…); un workload, además, se resalta
  const abrir = (id: string) => (id.startsWith('workload:') ? elegirWorkload(id.slice('workload:'.length)) : setSeleccionado(id));

  return (
    <div className="app">
      <header className="barra">
        <b className="marca">kviz</b>
        {foto && (
          <span className="contexto" title="Contexto de kubectl">
            <span className={foto.conectado ? 'punto punto-conectado' : 'punto punto-desconectado'} />
            {opcionesDeContexto.length > 0 ? (
              <select
                aria-label="Contexto de kubectl"
                value={foto.contexto}
                onFocus={refrescarContextos}
                onChange={(e) => elegirContexto(e.target.value)}
              >
                {!foto.contexto && <option value="">sin contexto</option>}
                {opcionesDeContexto.map((c) => (
                  <option key={c} value={c}>
                    {c}
                  </option>
                ))}
              </select>
            ) : (
              foto.contexto || 'sin contexto'
            )}
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
      {errorContexto && <div className="aviso">No se pudo cambiar de contexto: {errorContexto}</div>}
      {cortado && <div className="aviso">Se cortó la conexión con kviz; reintentando…</div>}
      {!cortado && !foto && <div className="aviso">Cargando…</div>}
      {!cortado && foto && !foto.conectado && <div className="aviso">{foto.motivo ?? 'Desconectado del cluster.'}</div>}
      {escena && (
        <Lienzo
          escena={escena}
          alClicEnVacio={() => {
            setWorkload(null);
            setSeleccionado(null);
          }}
          alApuntar={setHover}
          alClicEnObjeto={setSeleccionado}
        >
          <ListaDeWorkloads workloads={workloads} seleccionado={workloadSeleccionado} alSeleccionar={elegirWorkload} />
          {detalle && foto && (
            <PanelDeDetalle detalle={detalle} foto={foto} abrir={abrir} alCerrar={() => setSeleccionado(null)} />
          )}
        </Lienzo>
      )}
    </div>
  );
}
