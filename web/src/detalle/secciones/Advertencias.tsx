// Advertencias de configuración del pod, Service o Ingress: qué impide que el tráfico llegue, por
// qué, y el comando sugerido para revisarlo. Vienen en la foto (las calcula el backend).
import { advertenciasPorObjeto } from '../../escena/advertencias';
import { Comando } from '../Comando';
import { idIngress, idPod, idService, type PropsDeSeccion } from '../detalle';
import { Seccion } from '../ui';

export const orden = 20;

export default function Advertencias({ detalle, foto }: PropsDeSeccion) {
  let id: string;
  switch (detalle.tipo) {
    case 'Pod':
      id = idPod(detalle.pod);
      break;
    case 'Service':
      id = idService(detalle.service);
      break;
    case 'Ingress':
      id = idIngress(detalle.ingress);
      break;
    default:
      return null;
  }
  const suyas = advertenciasPorObjeto(foto).get(id) ?? [];
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
