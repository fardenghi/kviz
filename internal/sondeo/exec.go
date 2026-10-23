package sondeo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// EsperaRequest es el tiempo máximo de una request de prueba (curl --max-time).
const EsperaRequest = 10 * time.Second

// MaxBody es cuánto del body de la respuesta se guarda; el resto se descarta.
const MaxBody = 256 << 10

// EjecutorPorExec ejecuta los pedidos con curl adentro del pod de sondeo, como kubectl exec.
// config arma la configuración de la API para un contexto de kubectl.
func EjecutorPorExec(config func(contexto string) (*rest.Config, error)) Ejecutor {
	return porExec{config: config}
}

type porExec struct {
	config func(contexto string) (*rest.Config, error)
}

// separador divide, en la salida de errores de curl, sus mensajes de lo que escribe -w.
const separador = "\n<<kviz>>"

func (e porExec) HTTP(ctx context.Context, pod Pod, p PedidoHTTP) (RespuestaHTTP, error) {
	comando := ComandoCurl(p)
	var stdin io.Reader
	if p.Body != "" {
		stdin = strings.NewReader(p.Body)
	}
	var stdout limitado
	var stderr bytes.Buffer
	if err := e.exec(ctx, pod, comando, stdin, &stdout, &stderr); err != nil {
		// curl termina con código distinto de 0 cuando no hay respuesta: eso lo dice su salida
		if _, ok := err.(interface{ ExitStatus() int }); !ok {
			return RespuestaHTTP{}, fmt.Errorf("no se pudo ejecutar curl en el pod de sondeo: %w", err)
		}
	}
	return LeerCurl(stdout.Bytes(), stderr.String())
}

// ComandoCurl es el curl que ejecuta el pod de sondeo: el body de la respuesta sale por stdout y,
// por stderr, los errores de curl y después del separador los datos de la respuesta en JSON.
// Los argumentos van sin shell, así nada del pedido se interpreta.
func ComandoCurl(p PedidoHTTP) []string {
	metodo := p.Metodo
	if metodo == "" {
		metodo = http.MethodGet
	}
	c := []string{"curl", "-sS", "--max-time", fmt.Sprint(int(EsperaRequest.Seconds())), "-X", metodo,
		"-w", "%{stderr}" + separador + "%{json}" + separador + "%{header_json}"}
	for _, h := range p.Headers {
		c = append(c, "-H", h.Nombre+": "+h.Valor)
	}
	if p.Body != "" {
		c = append(c, "--data-binary", "@-")
	}
	return append(c, "--", p.URL)
}

// LeerCurl interpreta la salida de ComandoCurl.
func LeerCurl(stdout []byte, stderr string) (RespuestaHTTP, error) {
	partes := strings.Split(stderr, separador)
	if len(partes) < 3 {
		return RespuestaHTTP{}, fmt.Errorf("salida de curl inesperada: %q", stderr)
	}
	var datos struct {
		Codigo    int     `json:"response_code"`
		Total     float64 `json:"time_total"`
		IPRemota  string  `json:"remote_ip"`
		Salida    int     `json:"exitcode"`
		MensajeDe string  `json:"errormsg"`
	}
	if err := json.Unmarshal([]byte(partes[1]), &datos); err != nil {
		return RespuestaHTTP{}, fmt.Errorf("salida de curl inesperada: %w", err)
	}
	if datos.Salida != 0 || datos.Codigo == 0 {
		motivo := strings.TrimSpace(datos.MensajeDe)
		if motivo == "" {
			motivo = strings.TrimSpace(strings.TrimPrefix(partes[0], "curl: "))
		}
		return RespuestaHTTP{}, SinRespuesta{Motivo: motivo}
	}
	var crudos map[string][]string
	_ = json.Unmarshal([]byte(partes[2]), &crudos)
	headers := []Header{}
	for nombre, valores := range crudos {
		for _, v := range valores {
			headers = append(headers, Header{Nombre: nombre, Valor: v})
		}
	}
	sort.SliceStable(headers, func(i, j int) bool { return headers[i].Nombre < headers[j].Nombre })
	return RespuestaHTTP{
		Status:   datos.Codigo,
		Headers:  headers,
		Body:     string(stdout),
		Duracion: time.Duration(datos.Total * float64(time.Second)),
		IPRemota: datos.IPRemota,
	}, nil
}

// exec corre un comando en el container del pod de sondeo, como kubectl exec (websocket, y SPDY
// si el cluster no lo soporta).
func (e porExec) exec(ctx context.Context, pod Pod, comando []string, stdin io.Reader, stdout, stderr io.Writer) error {
	cfg, err := e.config(pod.Contexto)
	if err != nil {
		return err
	}
	cliente, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}
	req := cliente.CoreV1().RESTClient().Post().Resource("pods").Namespace(pod.Namespace).Name(pod.Nombre).
		SubResource("exec").VersionedParams(&corev1.PodExecOptions{
		Container: "curl", Command: comando, Stdin: stdin != nil, Stdout: true, Stderr: true,
	}, scheme.ParameterCodec)
	ws, err := remotecommand.NewWebSocketExecutor(cfg, "GET", req.URL().String())
	if err != nil {
		return err
	}
	spdy, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
	if err != nil {
		return err
	}
	ejecutor, err := remotecommand.NewFallbackExecutor(ws, spdy, func(err error) bool {
		return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
	})
	if err != nil {
		return err
	}
	return ejecutor.StreamWithContext(ctx, remotecommand.StreamOptions{Stdin: stdin, Stdout: stdout, Stderr: stderr})
}

// limitado guarda hasta MaxBody bytes y descarta el resto.
type limitado struct{ bytes.Buffer }

func (l *limitado) Write(p []byte) (int, error) {
	if falta := MaxBody - l.Len(); falta > 0 {
		l.Buffer.Write(p[:min(len(p), falta)])
	}
	return len(p), nil
}
