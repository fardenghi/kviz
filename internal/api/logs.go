package api

// Logs de un container por el canal de streaming:
//
//	GET /api/logs?namespace=default&pod=web-7d4b9-x2k&container=web[&anterior=true]
//
// Responde un stream de Server-Sent Events (el mismo escritor que /api/stream) con mensajes:
//
//	{"tipo":"lineas","lineas":["…","…"]}  líneas nuevas, en orden
//	{"tipo":"fin"}                        los logs terminaron (el container terminó, o eran los del reinicio anterior)
//	{"tipo":"error","error":"…"}          Kubernetes no dio los logs (container esperando para arrancar…)
//
// Después de fin o error el servidor cierra el stream. Los actuales siguen en vivo hasta que la
// interfaz cierra la conexión: eso corta también el pedido al cluster. Un pod o container que no
// está en la foto, o los logs anteriores de un container que no se reinició, dan 404.

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"

	"kviz/internal/conexion"
	"kviz/internal/foto"
)

// Tipos de mensaje del stream de logs.
const (
	MensajeLineas = "lineas"
	MensajeFin    = "fin"
	MensajeError  = "error"
)

// lineasPorMensaje es cuántas líneas viajan como mucho en un mensaje: al abrir los logs llegan
// muchas juntas (las últimas conexion.LineasDeLogs) y se mandan de a tandas.
const lineasPorMensaje = 500

type mensajeDeLogs struct {
	Tipo   string   `json:"tipo"`
	Lineas []string `json:"lineas,omitempty"`
	Error  string   `json:"error,omitempty"`
}

func transmitirLogs(cluster Cluster) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		anterior, _ := strconv.ParseBool(q.Get("anterior"))
		pedido := conexion.PedidoDeLogs{Namespace: q.Get("namespace"), Pod: q.Get("pod"), Container: q.Get("container"), Anterior: anterior}
		if motivo := sinLogs(cluster.Foto(), pedido); motivo != "" {
			http.Error(w, motivo, http.StatusNotFound)
			return
		}

		logs, err := cluster.Logs(r.Context(), pedido)
		if err == nil {
			defer logs.Close()
		}
		s, errSSE := abrirSSE(w)
		if errSSE != nil {
			return
		}
		if err != nil {
			s.enviar(mensajeDeLogs{Tipo: MensajeError, Error: err.Error()})
			return
		}

		lineas, fin := leerLineas(r.Context(), logs)
		for {
			var tanda []string
			select {
			case <-r.Context().Done():
				return
			case l, ok := <-lineas:
				if !ok {
					terminar(s, r.Context(), *fin)
					return
				}
				tanda = append(tanda, l)
			}
			// junta en el mismo mensaje las líneas que ya están leídas
		juntar:
			for len(tanda) < lineasPorMensaje {
				select {
				case l, ok := <-lineas:
					if !ok {
						break juntar
					}
					tanda = append(tanda, l)
				default:
					break juntar
				}
			}
			if s.enviar(mensajeDeLogs{Tipo: MensajeLineas, Lineas: tanda}) != nil {
				return
			}
		}
	}
}

// terminar avisa que los logs se terminaron, o por qué se cortaron.
func terminar(s *sse, ctx context.Context, err error) {
	switch {
	case ctx.Err() != nil:
		// la interfaz cerró la conexión: no hay a quién avisarle
	case err != nil && !errors.Is(err, io.EOF):
		s.enviar(mensajeDeLogs{Tipo: MensajeError, Error: err.Error()})
	default:
		s.enviar(mensajeDeLogs{Tipo: MensajeFin})
	}
}

// leerLineas lee los logs renglón por renglón (sin límite de largo) hasta el final o hasta que
// se cancela ctx. Cuando cierra el canal, *fin dice por qué terminó (io.EOF si terminó bien).
func leerLineas(ctx context.Context, logs io.Reader) (<-chan string, *error) {
	lineas := make(chan string, lineasPorMensaje)
	var fin error
	go func() {
		defer close(lineas)
		lector := bufio.NewReader(logs)
		for {
			l, err := lector.ReadString('\n')
			if len(l) > 0 {
				if l[len(l)-1] == '\n' {
					l = l[:len(l)-1]
				}
				select {
				case lineas <- l:
				case <-ctx.Done():
					fin = ctx.Err()
					return
				}
			}
			if err != nil {
				fin = err
				return
			}
		}
	}()
	return lineas, &fin
}

// sinLogs dice por qué no hay logs para el pedido según la foto, o "" si los hay.
func sinLogs(f foto.Foto, p conexion.PedidoDeLogs) string {
	for _, pod := range f.Pods {
		if pod.Namespace != p.Namespace || pod.Nombre != p.Pod {
			continue
		}
		for _, c := range pod.Containers {
			if c.Nombre != p.Container {
				continue
			}
			if p.Anterior && c.Reinicios == 0 {
				return "el container no se reinició: no hay logs del reinicio anterior"
			}
			return ""
		}
		return "el pod no tiene ese container"
	}
	return "no hay ese pod en la foto"
}
