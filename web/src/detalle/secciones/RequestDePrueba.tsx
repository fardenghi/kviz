// Service: botón "Enviar request" por cada puerto TCP, que abre el formulario de request de prueba
// con el destino ya completado (el nombre DNS del Service y el puerto), y uno por nodePort para
// entrar por un nodo. Ingress: uno por cada puerta de hostPort :80 (la del Ingress Controller),
// con el path de su primera regla.
import type { Foto, Ingress } from '../../foto';
import type { PropsDeSeccion } from '../detalle';
import { Seccion } from '../ui';

export const orden = 15;

export default function RequestDePrueba({ detalle, foto, probar }: PropsDeSeccion) {
  if (detalle.tipo === 'Ingress' && probar) return <PorElIngress ingress={detalle.ingress} foto={foto} probar={probar} />;
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
        {puertos
          .filter((p) => p.nodePort)
          .map((p) => (
            <button
              key={`nodo-${p.nodePort}`}
              className="boton"
              onClick={() => probar(`${foto.nodos[0]?.nombre ?? 'nodo'}:${p.nodePort}`)}
              title={`Request de prueba al NodePort ${p.nodePort}, entrando por un nodo (se elige en el formulario)`}
            >
              Por un nodo (:{p.nodePort})
            </button>
          ))}
      </div>
    </Seccion>
  );
}

/** Una ruta de Ingress se prueba entrando por la puerta de hostPort :80 de un nodo (la del Ingress Controller). */
function PorElIngress({ ingress, foto, probar }: { ingress: Ingress; foto: Foto; probar: (destino: string) => void }) {
  const puertas = foto.puertas.filter((p) => p.tipo === 'hostPort' && p.puerto === 80 && p.protocolo === 'TCP');
  if (puertas.length === 0) return null;
  const path = ejemploDePath(ingress.reglas.find((r) => r.path)?.path ?? '/');
  return (
    <Seccion>
      <div className="acciones-pedido">
        {puertas.map((p) => (
          <button key={p.id} className="boton" onClick={() => probar(`${p.nodo}:80${path}`)} title={`Request de prueba a ${p.nodo}:80${path}`}>
            Enviar request por {p.nodo}:80
          </button>
        ))}
      </div>
    </Seccion>
  );
}

/** Un path concreto para una regla que puede ser una expresión regular: lo que va antes del primer carácter especial. */
const ejemploDePath = (path: string) => path.split(/[()[\]{}*+?|^$\\.]/)[0] || '/';
