package conexion

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"kviz/internal/foto"
	"kviz/internal/sondeo"
)

// Kubeconfig es de dónde salen los contextos y sus clientes. En producción es el kubeconfig de
// kubectl (KubeconfigDeKubectl); en los tests, clusters falsos.
type Kubeconfig interface {
	// Contextos devuelve el contexto actual de kubectl ("" si no hay) y todos los disponibles, ordenados.
	Contextos() (actual string, todos []string, err error)
	// Cliente arma un cliente para el contexto. Se pide de nuevo en cada reconexión: un cluster
	// recreado (kind delete + kind create) vuelve con otra dirección y otros certificados.
	Cliente(contexto string) (kubernetes.Interface, error)
}

// ErrContextoInexistente es el error de cambiar a un contexto que no está en el kubeconfig.
var ErrContextoInexistente = errors.New("el contexto no está en el kubeconfig")

// IntervaloDeVigilancia es cada cuánto se revisa que el cluster responda, y cada cuánto se
// reintenta mientras no responde.
const IntervaloDeVigilancia = 2 * time.Second

// esperaRespuesta es cuánto se espera a que el cluster conteste la revisión.
const esperaRespuesta = 5 * time.Second

// Gestor mantiene la conexión al cluster del contexto elegido: la arma, revisa que el cluster
// siga respondiendo, la vuelve a armar cuando el cluster vuelve, y cambia de contexto en caliente.
// Mientras no hay conexión, su foto es una foto desconectada con el motivo.
type Gestor struct {
	kubeconfig Kubeconfig
	intervalo  time.Duration
	avisos     avisos
	// ejecutor corre los pedidos adentro del pod de sondeo de cada cluster.
	ejecutor sondeo.Ejecutor

	mu  sync.Mutex
	ctx context.Context
	// sesion cuenta los cambios de contexto: una vigilancia de un contexto anterior no toca el estado.
	sesion   int
	parar    context.CancelFunc
	contexto string
	conexion *Conexion // sincronizada; nil mientras no hay conexión
	motivo   string
	// sondeo es el pod de sondeo del cluster conectado; nil mientras no hay conexión.
	sondeo *sondeo.Sondeo
}

// NuevoGestor arma el gestor; revisa el cluster cada intervalo (IntervaloDeVigilancia en producción).
// El ejecutor corre las requests de prueba adentro del pod de sondeo (sondeo.EjecutorPorExec en
// producción).
func NuevoGestor(kc Kubeconfig, intervalo time.Duration, ejecutor sondeo.Ejecutor) *Gestor {
	return &Gestor{kubeconfig: kc, intervalo: intervalo, ejecutor: ejecutor}
}

// Sondeo es el pod de sondeo del cluster conectado (se crea recién con la primera request de prueba).
func (g *Gestor) Sondeo() (*sondeo.Sondeo, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.sondeo == nil {
		return nil, ErrDesconectado
	}
	return g.sondeo, nil
}

// Cerrar borra el pod de sondeo del cluster conectado, si kviz creó o adoptó uno. Se llama al cerrar kviz.
func (g *Gestor) Cerrar(ctx context.Context) error {
	g.mu.Lock()
	s := g.sondeo
	g.sondeo = nil
	g.mu.Unlock()
	if s == nil {
		return nil
	}
	return s.Borrar(ctx)
}

// esperaBorrado es cuánto se espera a que el cluster que se deja borre el pod de sondeo.
const esperaBorrado = 10 * time.Second

// Iniciar se conecta al contexto actual de kubectl, en segundo plano, hasta que se cancela ctx.
// Sin contexto, espera a que kubectl tenga uno. No bloquea.
func (g *Gestor) Iniciar(ctx context.Context) {
	actual, _, err := g.kubeconfig.Contextos()
	g.mu.Lock()
	g.ctx = ctx
	g.mu.Unlock()
	if err != nil {
		g.empezar("", fmt.Sprintf("no se pudo leer el kubeconfig: %v", err))
		return
	}
	g.empezar(actual, "")
}

// Contexto es el contexto de kubectl al que kviz está conectado (o intenta conectarse); "" si no hay.
func (g *Gestor) Contexto() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.contexto
}

// Contextos devuelve el contexto de kviz y todos los contextos del kubeconfig.
func (g *Gestor) Contextos() (actual string, todos []string, err error) {
	_, todos, err = g.kubeconfig.Contextos()
	return g.Contexto(), todos, err
}

// CambiarContexto deja el cluster actual y se conecta al del contexto pedido. La foto pasa a
// ser la del nuevo cluster cuando se sincroniza.
func (g *Gestor) CambiarContexto(contexto string) error {
	_, todos, err := g.kubeconfig.Contextos()
	if err != nil {
		return err
	}
	if !slices.Contains(todos, contexto) {
		return fmt.Errorf("%w: %q", ErrContextoInexistente, contexto)
	}
	if contexto != g.Contexto() {
		g.empezar(contexto, "")
	}
	return nil
}

// Suscribir devuelve un canal que avisa cada vez que la foto puede haber cambiado (un objeto
// observado se creó, cambió o se borró, o cambió el estado de la conexión), y la función para
// dejar de escuchar. Hay que pedir la foto después de suscribirse, así no se pierde ningún cambio.
func (g *Gestor) Suscribir() (<-chan struct{}, func()) { return g.avisos.suscribir() }

// Foto es la foto del cluster conectado, o una foto desconectada con el motivo.
func (g *Gestor) Foto() foto.Foto {
	g.mu.Lock()
	c, contexto, motivo := g.conexion, g.contexto, g.motivo
	g.mu.Unlock()
	if c == nil {
		return foto.Desconectada(contexto, motivo)
	}
	return c.Foto()
}

// empezar corta la vigilancia anterior y vigila el contexto dado. Con motivo, no intenta conectarse.
func (g *Gestor) empezar(contexto, motivo string) {
	g.mu.Lock()
	if g.parar != nil {
		g.parar()
	}
	g.sesion++
	sesion := g.sesion
	ctx, parar := context.WithCancel(g.ctx)
	g.parar = parar
	// al dejar un cluster por otro se borra su pod de sondeo, sin demorar el cambio
	if anterior := g.sondeo; anterior != nil {
		go func() {
			ctx, cancelar := context.WithTimeout(context.Background(), esperaBorrado)
			defer cancelar()
			_ = anterior.Borrar(ctx)
		}()
	}
	g.contexto, g.conexion, g.sondeo = contexto, nil, nil
	switch {
	case motivo != "":
		g.motivo = motivo
	case contexto == "":
		g.motivo = sinContexto
	default:
		g.motivo = conectando(contexto)
	}
	g.mu.Unlock()
	g.avisos.avisar()
	if motivo == "" {
		go g.vigilar(ctx, sesion, contexto)
	}
}

const sinContexto = "kubectl no tiene un contexto actual: creá un cluster (kind create cluster) o elegí uno con kubectl config use-context"

func conectando(contexto string) string {
	return fmt.Sprintf("conectando con el cluster del contexto %q…", contexto)
}

// vigilar se conecta al cluster del contexto y lo vuelve a intentar cada vez que deja de
// responder, hasta que se cancela ctx. Sin contexto, espera a que kubectl tenga uno.
func (g *Gestor) vigilar(ctx context.Context, sesion int, contexto string) {
	for ctx.Err() == nil {
		if contexto == "" {
			if actual, _, err := g.kubeconfig.Contextos(); err == nil && actual != "" {
				contexto = actual
				g.estado(sesion, func() { g.contexto, g.motivo = actual, conectando(actual) })
				continue
			}
		} else if cliente, err := g.responde(ctx, contexto); err != nil {
			g.estado(sesion, func() { g.conexion, g.motivo = nil, noResponde(contexto, err) })
		} else {
			g.mantener(ctx, sesion, contexto, cliente)
			continue
		}
		if !esperar(ctx, g.intervalo) {
			return
		}
	}
}

// mantener arma la conexión al cluster y la sostiene mientras el cluster responda; cuando deja
// de responder, apaga sus informers y vuelve.
func (g *Gestor) mantener(ctx context.Context, sesion int, contexto string, cliente kubernetes.Interface) {
	ctxConexion, apagar := context.WithCancel(ctx)
	defer apagar()
	c := nueva(cliente, contexto, &g.avisos)
	sincronizada := make(chan error, 1)
	go func() { sincronizada <- c.Iniciar(ctxConexion) }()

	revisar := time.NewTicker(g.intervalo)
	defer revisar.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case err := <-sincronizada:
			if err != nil {
				return
			}
			// un pod de sondeo que dejó una sesión anterior se reutiliza o se borra; si falla, el
			// pod de sondeo se crea igual cuando haga falta
			s := sondeo.Nuevo(cliente, contexto, g.ejecutor)
			_ = s.Adoptar(ctx)
			g.estado(sesion, func() { g.conexion, g.motivo, g.sondeo = c, "", s })
		case <-revisar.C:
			if _, err := g.responde(ctx, contexto); err != nil {
				// el pod de sondeo queda en el cluster; al reconectarse se adopta
				g.estado(sesion, func() { g.conexion, g.motivo, g.sondeo = nil, noResponde(contexto, err), nil })
				return
			}
		}
	}
}

// responde arma un cliente para el contexto y revisa que el cluster conteste.
func (g *Gestor) responde(ctx context.Context, contexto string) (kubernetes.Interface, error) {
	cliente, err := g.kubeconfig.Cliente(contexto)
	if err != nil {
		return nil, err
	}
	ctx, cancelar := context.WithTimeout(ctx, esperaRespuesta)
	defer cancelar()
	if _, err := cliente.CoreV1().Nodes().List(ctx, metav1.ListOptions{Limit: 1}); err != nil {
		return nil, err
	}
	return cliente, nil
}

func noResponde(contexto string, err error) string {
	return fmt.Sprintf("el cluster del contexto %q no responde (%v); reintentando…", contexto, err)
}

// estado cambia el estado del gestor, salvo que la sesión ya no sea la actual, y avisa.
func (g *Gestor) estado(sesion int, cambiar func()) {
	g.mu.Lock()
	vigente := sesion == g.sesion
	if vigente {
		cambiar()
	}
	g.mu.Unlock()
	if vigente {
		g.avisos.avisar()
	}
}

func esperar(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
