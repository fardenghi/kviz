// Service: botón "Enviar request" por cada puerto TCP, que abre el formulario de request de prueba
// con el destino ya completado (el nombre DNS del Service y el puerto).
import type { PropsDeSeccion } from '../detalle';
import { Seccion } from '../ui';

export const orden = 15;

export default function RequestDePrueba({ detalle, probar }: PropsDeSeccion) {
  if (detalle.tipo !== 'Service' || !probar) return null;
  const s = detalle.service;
  const puertos = s.puertos.filter((p) => !p.protocolo || p.protocolo === 'TCP');
  if (puertos.length === 0) return null;
  const dns = `${s.nombre}.${s.namespace}.svc.cluster.local`;
  return (
    <Seccion>
      <div className="acciones-pedido">
        {puertos.map((p) => (
          <button key={p.puerto} className="boton" onClick={() => probar(`${dns}:${p.puerto}`)} title={`Request de prueba a ${dns}:${p.puerto}`}>
            Enviar request{puertos.length > 1 ? ` a :${p.puerto}` : ''}
          </button>
        ))}
      </div>
    </Seccion>
  );
}
