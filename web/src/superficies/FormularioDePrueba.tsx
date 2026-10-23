// Formulario de request de prueba (superficie flotante, arriba a la izquierda, al lado de la lista
// de workloads): destino, método, path, headers y body, como un Postman chico. La request sale del
// pod de sondeo de kviz; abajo se ven status, headers, body y tiempo.
import { type FormEvent, useState } from 'react';
import { mandarPrueba } from '../api';
import { leerHeaders, type Prueba } from '../prueba';

const METODOS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS'];

export function FormularioDePrueba({
  destinoInicial,
  alResponder,
  alCerrar,
}: {
  /** Destino ya completado (desde el panel de un Service), o "" para el campo libre. */
  destinoInicial: string;
  /** Llegó la respuesta: la App muestra su recorrido. */
  alResponder: (p: Prueba) => void;
  alCerrar: () => void;
}) {
  const [destino, setDestino] = useState(destinoInicial);
  const [metodo, setMetodo] = useState('GET');
  const [path, setPath] = useState('/');
  const [headers, setHeaders] = useState('');
  const [body, setBody] = useState('');
  const [enviando, setEnviando] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [prueba, setPrueba] = useState<Prueba | null>(null);

  const enviar = (e: FormEvent) => {
    e.preventDefault();
    setEnviando(true);
    setError(null);
    mandarPrueba({ tipo: 'http', destino, metodo, path, headers: leerHeaders(headers), body: conBody(metodo) ? body : '' })
      .then((p) => {
        setPrueba(p);
        alResponder(p);
      })
      .catch((err: Error) => setError(err.message))
      .finally(() => setEnviando(false));
  };

  return (
    <section className="flotante formulario-prueba" aria-label="Request de prueba">
      <div className="detalle-cabecera">
        <div>
          <div className="tenue-texto detalle-tipo">desde el pod de sondeo</div>
          <div className="detalle-titulo">Request de prueba</div>
        </div>
        <button className="boton" title="Cerrar" onClick={alCerrar}>
          ✕
        </button>
      </div>
      <form onSubmit={enviar}>
        <label>
          destino
          <input
            className="mono-texto"
            value={destino}
            onChange={(e) => setDestino(e.target.value)}
            placeholder="backend.default.svc.cluster.local:80"
            autoFocus={destinoInicial === ''}
            required
          />
        </label>
        <div className="fila-pedido">
          <label>
            método
            <select value={metodo} onChange={(e) => setMetodo(e.target.value)}>
              {METODOS.map((m) => (
                <option key={m}>{m}</option>
              ))}
            </select>
          </label>
          <label>
            path
            <input className="mono-texto" value={path} onChange={(e) => setPath(e.target.value)} placeholder="/hello" />
          </label>
        </div>
        <label>
          headers (uno por línea, `Nombre: valor`)
          <textarea rows={2} value={headers} onChange={(e) => setHeaders(e.target.value)} placeholder="Host: lab.local" />
        </label>
        {conBody(metodo) && (
          <label>
            body
            <textarea rows={3} value={body} onChange={(e) => setBody(e.target.value)} />
          </label>
        )}
        <div className="acciones-pedido">
          <button className="boton boton-primario" type="submit" disabled={enviando || !destino.trim()}>
            Enviar
          </button>
          {enviando && <span className="tenue-texto">Enviando… (la primera vez kviz crea el pod de sondeo)</span>}
        </div>
      </form>
      {error && <div className="respuesta pastilla-mal">{error}</div>}
      {prueba && !enviando && <Respuesta prueba={prueba} />}
    </section>
  );
}

const conBody = (metodo: string) => !['GET', 'HEAD', 'OPTIONS'].includes(metodo);

function Respuesta({ prueba: p }: { prueba: Prueba }) {
  const r = p.respuesta;
  return (
    <div className="respuesta">
      <div className="mono-texto chica tenue-texto">
        {p.metodo} {p.url}
      </div>
      {r ? (
        <>
          <div>
            <span className={r.status < 400 ? 'pastilla pastilla-ok' : 'pastilla pastilla-mal'}>
              {r.status} {r.texto}
            </span>{' '}
            <span className="tenue-texto">
              · {formatearMs(r.ms)} · {r.body.length} B
            </span>
          </div>
          <details>
            <summary className="chica">headers ({r.headers.length})</summary>
            <div className="mono-texto chica">
              {r.headers.map((h, i) => (
                <div key={i}>
                  {h.nombre}: {h.valor}
                </div>
              ))}
            </div>
          </details>
          {r.body ? <pre className="cuerpo-respuesta">{r.body}</pre> : <i className="tenue-texto">sin body</i>}
        </>
      ) : (
        <div>
          <span className="pastilla pastilla-mal">sin respuesta</span> <span>{p.error}</span>
        </div>
      )}
      <div className="tenue-texto chica">Origen: {p.origen}</div>
    </div>
  );
}

export const formatearMs = (ms: number) => (ms < 10 ? `${ms.toFixed(1)} ms` : `${Math.round(ms)} ms`);
