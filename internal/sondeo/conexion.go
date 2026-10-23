package sondeo

// La prueba de conexión TCP por exec: curl con telnet:// abre la conexión sin hablar ningún
// protocolo. Como telnet se queda conectado hasta --max-time, kviz corta el exec apenas curl avisa
// (en su salida -v) que se conectó; si no se conecta, curl termina solo y su -w dice por qué.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

func (e porExec) TCP(ctx context.Context, pod Pod, p PedidoTCP) (ResultadoTCP, error) {
	ctx, cortar := context.WithCancel(ctx)
	defer cortar()
	stderr := &avisaConexion{alConectar: cortar}
	err := e.exec(ctx, pod, ComandoConexion(p), nil, io.Discard, stderr)
	salida, conecto := stderr.leer()
	if err != nil && !conecto {
		// curl termina con código distinto de 0 cuando no conecta: eso lo dice su salida
		if _, ok := err.(interface{ ExitStatus() int }); !ok {
			return ResultadoTCP{}, fmt.Errorf("no se pudo ejecutar curl en el pod de sondeo: %w", err)
		}
	}
	return LeerConexion(salida)
}

// ComandoConexion es el curl de una prueba de conexión: por stderr, el detalle de -v con la hora
// de cada línea (de ahí sale el tiempo hasta conectar) y, si termina solo, después del separador
// los datos del intento en JSON.
func ComandoConexion(p PedidoTCP) []string {
	segundos := int(EsperaConexion.Seconds())
	return []string{"curl", "-sS", "-v", "--trace-time",
		"--connect-timeout", fmt.Sprint(segundos),
		"--max-time", fmt.Sprint(segundos + 2),
		"-w", "%{stderr}" + separador + "%{json}",
		"--", "telnet://" + net.JoinHostPort(p.Host, fmt.Sprint(p.Puerto))}
}

// LeerConexion interpreta la salida de errores de ComandoConexion, completa o cortada apenas
// conectó.
func LeerConexion(stderr string) (ResultadoTCP, error) {
	partes := strings.SplitN(stderr, separador, 2)
	var inicio, conectado time.Time
	ip := ""
	for _, linea := range strings.Split(partes[0], "\n") {
		hora, texto, ok := lineaConHora(linea)
		if !ok {
			continue
		}
		if strings.HasPrefix(texto, "Trying ") && inicio.IsZero() {
			inicio = hora
			ip = ipDeTrying(strings.TrimPrefix(texto, "Trying "))
		}
		if conecto(texto) {
			conectado = hora
			break
		}
	}
	if !conectado.IsZero() {
		return ResultadoTCP{Resultado: Abre, Duracion: entre(inicio, conectado), IPRemota: ip}, nil
	}
	if len(partes) < 2 {
		return ResultadoTCP{}, fmt.Errorf("salida de curl inesperada: %q", stderr)
	}
	var datos struct {
		Salida    int     `json:"exitcode"`
		MensajeDe string  `json:"errormsg"`
		Conexion  float64 `json:"time_connect"`
		DNS       float64 `json:"time_namelookup"`
		Total     float64 `json:"time_total"`
		IPRemota  string  `json:"remote_ip"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(partes[1])), &datos); err != nil {
		return ResultadoTCP{}, fmt.Errorf("salida de curl inesperada: %w", err)
	}
	if datos.IPRemota != "" {
		ip = datos.IPRemota
	}
	motivo := strings.TrimSpace(datos.MensajeDe)
	segundos := func(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }
	switch {
	case datos.Conexion > 0: // conectó y terminó solo (el destino cerró enseguida)
		return ResultadoTCP{Resultado: Abre, Duracion: segundos(datos.Conexion - datos.DNS), IPRemota: ip}, nil
	case datos.Salida == 7: // CURLE_COULDNT_CONNECT: nadie escucha (RST) o no hay ruta
		return ResultadoTCP{Resultado: Rechaza, Duracion: segundos(datos.Total - datos.DNS), IPRemota: ip, Motivo: motivo}, nil
	case datos.Salida == 28: // CURLE_OPERATION_TIMEDOUT, sin haber conectado
		return ResultadoTCP{Resultado: Timeout, Duracion: segundos(datos.Total - datos.DNS), IPRemota: ip, Motivo: motivo}, nil
	}
	if motivo == "" {
		motivo = fmt.Sprintf("curl terminó con código %d", datos.Salida)
	}
	return ResultadoTCP{}, SinRespuesta{Motivo: motivo}
}

// conecto dice si una línea de curl -v (sin la hora ni el `*`) avisa que se abrió la conexión.
// curl 8.11 dice `Connected to host (ip) port N`; versiones más nuevas, `Established connection…`.
func conecto(texto string) bool {
	return strings.HasPrefix(texto, "Connected to ") || strings.HasPrefix(texto, "Established connection")
}

// lineaConHora separa una línea de curl -v --trace-time: `18:05:21.276695 *   Trying 10.96.0.1:80...`.
func lineaConHora(linea string) (time.Time, string, bool) {
	hora, resto, ok := strings.Cut(strings.TrimSpace(linea), " ")
	if !ok {
		return time.Time{}, "", false
	}
	t, err := time.Parse("15:04:05.999999", hora)
	if err != nil {
		return time.Time{}, "", false
	}
	resto = strings.TrimSpace(resto)
	if !strings.HasPrefix(resto, "*") {
		return time.Time{}, "", false
	}
	return t, strings.TrimSpace(strings.TrimPrefix(resto, "*")), true
}

// ipDeTrying saca la IP de `10.96.0.1:80...` o `[fd00::1]:80...`.
func ipDeTrying(s string) string {
	s = strings.TrimSuffix(strings.TrimSpace(s), "...")
	if host, _, err := net.SplitHostPort(s); err == nil {
		return host
	}
	return ""
}

func entre(desde, hasta time.Time) time.Duration {
	if desde.IsZero() {
		return 0
	}
	d := hasta.Sub(desde)
	if d < 0 { // pasó la medianoche
		d += 24 * time.Hour
	}
	return d
}

// avisaConexion guarda la salida de errores de curl y avisa la primera vez que dice que conectó.
type avisaConexion struct {
	alConectar func()

	mu      sync.Mutex
	buf     bytes.Buffer
	conecto bool
}

func (a *avisaConexion) Write(p []byte) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.buf.Write(p)
	if !a.conecto {
		for _, linea := range strings.Split(a.buf.String(), "\n") {
			if _, texto, ok := lineaConHora(linea); ok && conecto(texto) {
				a.conecto = true
				a.alConectar()
				break
			}
		}
	}
	return len(p), nil
}

func (a *avisaConexion) leer() (string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.buf.String(), a.conecto
}
