package main

import (
	"os/exec"
	"path/filepath"
	"testing"
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
