// kviz: herramienta local para ver el cluster de Kubernetes del contexto actual de kubectl.
// Escucha solo en 127.0.0.1 y abre el navegador en su URL. Ver kviz/README.md.
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

	"kviz/internal/api"
	"kviz/internal/conexion"
	"kviz/web"
)

const puertoPorDefecto = 7777

func main() {
	puerto := flag.Int("puerto", puertoPorDefecto, "puerto en 127.0.0.1; si está ocupado se usa otro libre")
	noAbrir := flag.Bool("no-abrir", false, "no abrir el navegador al arrancar")
	flag.Parse()
	log.SetFlags(0)
	log.SetPrefix("kviz: ")

	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	c := conexion.DesdeKubeconfig()
	if c.Contexto() == "" {
		log.Print("sin contexto de kubectl: arranco desconectado")
	} else {
		log.Printf("conectando al contexto %s", c.Contexto())
	}
	go func() {
		if err := c.Iniciar(ctx); err == nil && c.Contexto() != "" {
			log.Printf("conectado a %s", c.Contexto())
		}
	}()

	if !web.Compilada() {
		log.Print("aviso: este binario no trae la interfaz; compilalo con `make build` desde kviz/")
	}

	ln, err := escuchar(*puerto)
	if err != nil {
		log.Fatal(err)
	}
	url := "http://" + ln.Addr().String()
	fmt.Printf("kviz escuchando en %s\n", url)

	srv := &http.Server{Handler: api.Nuevo(c, web.Handler()), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	if !*noAbrir {
		abrirNavegador(url)
	}

	<-ctx.Done()
	apagar, cancelar := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelar()
	_ = srv.Shutdown(apagar)
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
