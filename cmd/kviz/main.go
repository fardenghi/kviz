// kviz: herramienta local para ver el cluster de Kubernetes del contexto actual de kubectl.
// Escucha solo en 127.0.0.1 y abre el navegador en su URL. Ver README.md.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/klog/v2"

	"github.com/fardenghi/kviz/internal/api"
	"github.com/fardenghi/kviz/internal/conexion"
	"github.com/fardenghi/kviz/internal/sondeo"
	"github.com/fardenghi/kviz/web"
)

const puertoPorDefecto = 7777

// version la inyecta el release con -ldflags "-X main.version=..."; compilado a mano es "dev".
var version = "dev"

func main() {
	silenciarClientGo()
	// los flags van en inglés; los nombres en español quedan como alias para no romper a quien ya los usa
	var puerto int
	var noAbrir bool
	flag.IntVar(&puerto, "port", puertoPorDefecto, "port to listen on, on 127.0.0.1; if it is taken, a free one is used")
	flag.IntVar(&puerto, "puerto", puertoPorDefecto, "alias of -port")
	flag.BoolVar(&noAbrir, "no-browser", false, "don't open the browser on start")
	flag.BoolVar(&noAbrir, "no-abrir", false, "alias of -no-browser")
	verVersion := flag.Bool("version", false, "print the version and exit")
	flag.Parse()
	if *verVersion {
		fmt.Printf("kviz %s\n", version)
		return
	}
	log.SetFlags(0)
	log.SetPrefix("kviz: ")

	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	c := conexion.NuevoGestor(conexion.KubeconfigDeKubectl(), conexion.IntervaloDeVigilancia,
		sondeo.EjecutorPorExec(conexion.ConfigDeKubectl))
	c.Iniciar(ctx)
	if c.Contexto() == "" {
		log.Print("sin contexto de kubectl: arranco desconectado")
	} else {
		log.Printf("conectando al contexto %s", c.Contexto())
	}

	if !web.Compilada() {
		log.Print("aviso: este binario no trae la interfaz; compilalo con `make build` desde la raíz del código de kviz")
	}

	ln, err := escuchar(puerto)
	if err != nil {
		log.Fatal(err)
	}
	url := "http://" + ln.Addr().String()
	fmt.Printf("kviz escuchando en %s\n", url)

	srv := &http.Server{
		Handler:           api.Nuevo(c, web.Handler()),
		ReadHeaderTimeout: 10 * time.Second,
		// los pedidos heredan ctx: al apagar kviz se cortan los streams abiertos
		BaseContext: func(net.Listener) context.Context { return ctx },
	}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	if !noAbrir {
		abrirNavegador(url)
	}

	<-ctx.Done()
	apagar, cancelar := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelar()
	_ = srv.Shutdown(apagar)
	// la única escritura de kviz en el cluster se deshace al cerrar
	borrar, cancelarBorrado := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelarBorrado()
	if err := c.Cerrar(borrar); err != nil {
		log.Printf("no pude borrar el pod de sondeo (%v); se limpia la próxima vez que arranque kviz", err)
	}
}

// silenciarClientGo descarta los logs internos de client-go (klog). Son para quien desarrolla
// client-go, no para quien usa kviz: por ejemplo, al cortar el exec de una prueba de conexión apenas
// curl conecta, client-go se queja de la conexión cerrada aunque la prueba salió bien. Los errores
// que importan kviz los muestra en la interfaz.
func silenciarClientGo() {
	klog.SetLogger(logr.Discard())
}

// escuchar abre el puerto pedido en 127.0.0.1 (nunca en otras interfaces). Si está ocupado,
// pide al sistema uno libre y avisa.
func escuchar(puerto int) (net.Listener, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", puerto))
	if err == nil {
		return ln, nil
	}
	ln, err2 := net.Listen("tcp", "127.0.0.1:0")
	if err2 != nil {
		return nil, fmt.Errorf("no pude escuchar en 127.0.0.1: %v; %v", err, err2)
	}
	log.Printf("el puerto %d está ocupado; uso el %d", puerto, ln.Addr().(*net.TCPAddr).Port)
	return ln, nil
}

func abrirNavegador(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.Printf("no pude abrir el navegador (%v); abrí %s a mano", err, url)
		return
	}
	go cmd.Wait() //nolint:errcheck
}
