package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// compilar arma el binario de kviz con las ldflags dadas, como lo hace GoReleaser.
func compilar(t *testing.T, ldflags string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "kviz")
	args := []string{"build", "-o", bin}
	if ldflags != "" {
		args = append(args, "-ldflags", ldflags)
	}
	out, err := exec.Command("go", append(args, ".")...).CombinedOutput()
	if err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}
	return bin
}

func TestVersionImprimeLaVersionYSale(t *testing.T) {
	casos := []struct {
		nombre, ldflags, quiero string
	}{
		{"sin inyectar", "", "kviz dev\n"},
		{"inyectada por el release", "-X main.version=1.2.3", "kviz 1.2.3\n"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			out, err := exec.Command(compilar(t, c.ldflags), "-version").Output()
			if err != nil {
				t.Fatalf("kviz -version terminó con error: %v", err)
			}
			if string(out) != c.quiero {
				t.Fatalf("kviz -version imprimió %q, quería %q", out, c.quiero)
			}
		})
	}
}

// puertoLibre pide al sistema un puerto libre en 127.0.0.1 y lo suelta.
func puertoLibre(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

// Los flags tienen nombre en inglés (-port, -no-browser) y conservan el español como alias.
func TestFlagsEnInglesYAliasEnEspanol(t *testing.T) {
	bin := compilar(t, "")
	casos := []struct{ nombre, flagPuerto, flagNoAbrir string }{
		{"en inglés", "-port", "-no-browser"},
		{"alias en español", "-puerto", "-no-abrir"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			puerto := puertoLibre(t)
			cmd := exec.Command(bin, c.flagPuerto, strconv.Itoa(puerto), c.flagNoAbrir)
			// sin kubeconfig: kviz arranca desconectado y no toca ningún cluster real
			cmd.Env = append(os.Environ(), "KUBECONFIG="+filepath.Join(t.TempDir(), "no-existe"))
			salida, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()

			quiero := fmt.Sprintf("http://127.0.0.1:%d", puerto)
			lineas := bufio.NewScanner(salida)
			listo := make(chan bool, 1)
			go func() {
				for lineas.Scan() {
					if strings.Contains(lineas.Text(), quiero) {
						listo <- true
						return
					}
				}
				listo <- false
			}()
			select {
			case ok := <-listo:
				if !ok {
					t.Fatalf("kviz terminó sin escuchar en %s", quiero)
				}
			case <-time.After(10 * time.Second):
				t.Fatalf("kviz no escuchó en %s a los 10 s", quiero)
			}
		})
	}
}
