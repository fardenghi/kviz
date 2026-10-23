// Advertencias de configuración del pod, Service o Ingress: qué impide que el tráfico llegue, por
// qué, y el comando sugerido para revisarlo. Vienen en la foto (las calcula el backend).
import { advertenciasPorObjeto } from '../../escena/advertencias';
import { Comando } from '../Comando';
import { fichaDe, type PropsDeSeccion } from '../detalle';
import { Seccion } from '../ui';

export const orden = 20;

export default function Advertencias({ detalle, foto }: PropsDeSeccion) {
  // solo los pods, Services e Ingress tienen advertencias
  const suyas = advertenciasPorObjeto(foto).get(fichaDe(detalle).id) ?? [];
  if (suyas.length === 0) return null;
  return (
    <Seccion titulo={suyas.length === 1 ? 'Advertencia de configuración' : `Advertencias de configuración · ${suyas.length}`}>
      {suyas.map((a) => (
        <div key={a.id} className="advertencia-detalle">
          <div className="advertencia-titulo">{a.titulo}</div>
          <p className="advertencia-explicacion">{a.explicacion}</p>
          <Comando comando={a.comando.comando} descripcion={a.comando.descripcion} />
        </div>
      ))}
    </Seccion>
  );
}
