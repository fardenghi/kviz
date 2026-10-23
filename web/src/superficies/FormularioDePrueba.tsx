// Formulario de request de prueba (superficie flotante, arriba a la izquierda, al lado de la lista
// de workloads): destino, método, path, headers y body, como un Postman chico. La request sale del
// pod de sondeo de kviz; abajo se ven status, headers, body y tiempo. Con "TCP" es una prueba de
// conexión: solo el destino `host:puerto`, y abajo si abre, rechaza o da timeout.
import { type FormEvent, useState } from 'react';
import { mandarPrueba } from '../api';
import { leerHeaders, lineaDelPedido, type Prueba, type TipoDePrueba } from '../prueba';

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
  const [tipo, setTipo] = useState<TipoDePrueba>('http');
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
    const pedido =
      tipo === 'tcp'
        ? { tipo, destino }
        : { tipo, destino, metodo, path, headers: leerHeaders(headers), body: conBody(metodo) ? body : '' };
    mandarPrueba(pedido)
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
          <div className="detalle-titulo">{tipo === 'tcp' ? 'Prueba de conexión' : 'Request de prueba'}</div>
        </div>
        <button className="boton" title="Cerrar" onClick={alCerrar}>
          ✕
        </button>
      </div>
      <form onSubmit={enviar}>
        <div className="tipo-prueba" role="group" aria-label="Tipo de prueba">
          {TIPOS.map((t) => (
            <button
              key={t.tipo}
              type="button"
              className={t.tipo === tipo ? 'boton boton-primario' : 'boton'}
              aria-pressed={t.tipo === tipo}
              title={t.ayuda}
              onClick={() => setTipo(t.tipo)}
            >
              {t.nombre}
            </button>
          ))}
        </div>
        <label>
          destino{tipo === 'tcp' && ' (host:puerto)'}
          <input
            className="mono-texto"
            value={destino}
            onChange={(e) => setDestino(e.target.value)}
            placeholder={tipo === 'tcp' ? 'postgres.default.svc.cluster.local:5432' : 'backend.default.svc.cluster.local:80'}
            autoFocus={destinoInicial === ''}
            required
          />
        </label>
        {tipo === 'http' && (
          <>
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
          </>
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

const TIPOS: { tipo: TipoDePrueba; nombre: string; ayuda: string }[] = [
  { tipo: 'http', nombre: 'HTTP', ayuda: 'Request de prueba: método, path, headers y body' },
  { tipo: 'tcp', nombre: 'TCP', ayuda: 'Prueba de conexión: solo abre una conexión TCP, para lo que no habla HTTP' },
];

const conBody = (metodo: string) => !['GET', 'HEAD', 'OPTIONS'].includes(metodo);

function Respuesta({ prueba: p }: { prueba: Prueba }) {
  const r = p.respuesta;
  return (
    <div className="respuesta">
      <div className="mono-texto chica tenue-texto">{lineaDelPedido(p)}</div>
      {p.tipo === 'tcp' ? (
        <div>
          <PastillaDelResultado prueba={p} /> {p.conexion?.motivo ?? (p.conexion ? '' : p.error)}
        </div>
      ) : r ? (
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

/** El resultado de una prueba en una pastilla: `200 OK · 14 ms`, `abre · 1.5 ms`, `sin respuesta`. */
export function PastillaDelResultado({ prueba: p }: { prueba: Prueba }) {
  if (p.tipo === 'tcp') {
    const c = p.conexion;
    if (!c) return <span className="pastilla pastilla-mal">sin resultado</span>;
    return (
      <span className={c.resultado === 'abre' ? 'pastilla pastilla-ok' : 'pastilla pastilla-mal'}>
        {c.resultado} · {formatearMs(c.ms)}
      </span>
    );
  }
  const r = p.respuesta;
  if (!r) return <span className="pastilla pastilla-mal">sin respuesta</span>;
  return (
    <span className={r.status < 400 ? 'pastilla pastilla-ok' : 'pastilla pastilla-mal'}>
      {r.status} {r.texto} · {formatearMs(r.ms)}
    </span>
  );
}

export const formatearMs = (ms: number) => (ms < 10 ? `${ms.toFixed(1)} ms` : `${Math.round(ms)} ms`);
