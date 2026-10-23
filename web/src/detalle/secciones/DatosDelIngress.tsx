// Ingress: clase, rewrite, reglas y Services destino.
import { idService, type PropsDeSeccion } from '../detalle';
import { Datos, Enlace, Nada, Seccion } from '../ui';

export const orden = 10;

export default function DatosDelIngress({ detalle, foto, abrir }: PropsDeSeccion) {
  if (detalle.tipo !== 'Ingress') return null;
  const i = detalle.ingress;
  const destinos = [...new Set(i.reglas.map((r) => r.service))];
  return (
    <>
      <Seccion>
        <Datos
          filas={[
            ['namespace', i.namespace],
            [
              'clase',
              i.clase === null ? (
                <Nada>sin clase</Nada>
              ) : (
                <>
                  {i.clase}
                  {i.clasePorDefecto && <Nada> (por defecto)</Nada>}
                </>
              ),
            ],
            ['rewrite', i.rewrite ? <span className="mono-texto">{i.rewrite}</span> : <Nada>sin rewrite</Nada>],
          ]}
        />
      </Seccion>
      <Seccion titulo="Reglas">
        {i.reglas.length === 0 && <Nada>sin reglas</Nada>}
        {i.reglas.map((r, n) => (
          <div key={n} className="mono-texto regla">
            {r.host || '*'}
            {r.path || (r.host ? '' : ' (backend por defecto)')} → {r.service}:{r.puerto}
            {r.tipoPath && <span className="tenue-texto"> · {r.tipoPath}</span>}
          </div>
        ))}
      </Seccion>
      <Seccion titulo="Services destino">
        {destinos.map((nombre) => {
          const existe = foto.services.some((s) => s.namespace === i.namespace && s.nombre === nombre);
          return (
            <div key={nombre}>
              {existe ? (
                <Enlace id={idService({ namespace: i.namespace, nombre })} abrir={abrir}>
                  {nombre}
                </Enlace>
              ) : (
                <>
                  {nombre} <Nada>(no existe en {i.namespace})</Nada>
                </>
              )}
            </div>
          );
        })}
      </Seccion>
    </>
  );
}
