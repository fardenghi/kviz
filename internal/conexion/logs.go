package conexion

// Logs de un container (módulo 8 de la spec): el mismo pedido que kubectl logs, por el cliente
// del cluster conectado.

import (
	"context"
	"errors"
	"io"

	corev1 "k8s.io/api/core/v1"
)

// ErrDesconectado es el error de pedirle algo al cluster mientras kviz no está conectado.
var ErrDesconectado = errors.New("kviz no está conectado al cluster")

// LineasDeLogs es cuántas líneas del final de los logs se traen al abrirlos (kubectl logs --tail).
const LineasDeLogs = 1000

// PedidoDeLogs dice de qué container se quieren los logs.
type PedidoDeLogs struct {
	Namespace string
	Pod       string
	Container string
	// Anterior pide los logs del reinicio anterior (kubectl logs --previous); si no, los actuales,
	// que siguen en vivo (kubectl logs -f) hasta que se cancela ctx o el container termina.
	Anterior bool
	// SinSeguir pide solo lo que ya está en los logs actuales, sin seguir en vivo.
	SinSeguir bool
	// Ultimas es cuántas líneas del final traer; 0 es LineasDeLogs.
	Ultimas int64
}

// Logs abre los logs de un container del cluster conectado, una línea por renglón. Hay que
// cerrarlos; cancelar ctx corta el pedido al cluster.
func (g *Gestor) Logs(ctx context.Context, p PedidoDeLogs) (io.ReadCloser, error) {
	g.mu.Lock()
	c := g.conexion
	g.mu.Unlock()
	if c == nil {
		return nil, ErrDesconectado
	}
	cola := int64(LineasDeLogs)
	if p.Ultimas > 0 {
		cola = p.Ultimas
	}
	opciones := &corev1.PodLogOptions{
		Container: p.Container,
		Follow:    !p.Anterior && !p.SinSeguir,
		Previous:  p.Anterior,
		TailLines: &cola,
	}
	return c.cliente.CoreV1().Pods(p.Namespace).GetLogs(p.Pod, opciones).Stream(ctx)
}
