// Comandos sugeridos de kubectl del objeto, completos con nombre, namespace y contexto, con su
// botón de copiar. Los arma el backend (GET /api/comandos); kviz no ejecuta ninguno.
import { useEffect, useState } from 'react';
import { type ComandoSugerido, pedirComandos } from '../../api';
import { Comando } from '../Comando';
import { fichaDe, type PropsDeSeccion } from '../detalle';
import { Nada, Seccion } from '../ui';

export const orden = 90;

export default function ComandosSugeridos({ detalle, foto }: PropsDeSeccion) {
  const objeto = fichaDe(detalle).api;
  // se vuelven a pedir si cambia el objeto, el contexto o algo del objeto que entra en los comandos
  // (réplicas, puertos, containers): la clave es el objeto mismo
  const clave = JSON.stringify([objeto, foto.contexto, detalle]);
  const [comandos, setComandos] = useState<{ clave: string; lista: ComandoSugerido[] | null }>({ clave: '', lista: null });
  useEffect(() => {
    let vigente = true;
    pedirComandos(objeto)
      .then((lista) => vigente && setComandos({ clave, lista }))
      .catch(() => vigente && setComandos({ clave, lista: [] }));
    return () => {
      vigente = false;
    };
  }, [clave]); // eslint-disable-line react-hooks/exhaustive-deps

  const lista = comandos.clave === clave ? comandos.lista : null;
  return (
    <Seccion titulo="Comandos sugeridos">
      {lista === null && <Nada>Cargando…</Nada>}
      {lista?.length === 0 && <Nada>sin comandos</Nada>}
      {lista?.map((c) => <Comando key={c.comando} comando={c.comando} descripcion={c.descripcion} />)}
    </Seccion>
  );
}
