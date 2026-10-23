// Package conexion es la conexión al cluster (módulo 1 de la spec): resuelve el contexto de
// kubectl y mantiene watches (informers) de los objetos que necesita la foto.
//
// kviz nunca se cae por no poder conectarse: sin contexto, o mientras el cluster no responde,
// la conexión entrega una foto desconectada con el motivo.
//
// Para observar un tipo de objeto nuevo: registrar su informer en Iniciar (con el handler de
// avisos, para que sus cambios lleguen en vivo) y copiar sus objetos en Objetos (y sumarlos a
// foto.Objetos).
package conexion

import (
	"context"
	"fmt"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"kviz/internal/foto"
)

// Conexion es la conexión a un cluster, con sus informers.
type Conexion struct {
	contexto string
	// cliente es nil cuando no hay a qué conectarse; motivo dice por qué.
	cliente kubernetes.Interface
	motivo  string

	sincronizada atomic.Bool
	avisos       avisos
	nodos        listerscorev1.NodeLister
	pods         listerscorev1.PodLister
}

// Nueva arma una conexión sobre un cliente ya creado. En producción el cliente sale de
// DesdeKubeconfig; en los tests es el cliente falso de client-go.
func Nueva(cliente kubernetes.Interface, contexto string) *Conexion {
	return &Conexion{cliente: cliente, contexto: contexto}
}

// DesdeKubeconfig arma la conexión al contexto actual de kubectl (KUBECONFIG o ~/.kube/config).
// Si no hay contexto o el kubeconfig no sirve, la conexión queda desconectada con el motivo.
func DesdeKubeconfig() *Conexion {
	reglas := clientcmd.NewDefaultClientConfigLoadingRules()
	cfg := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(reglas, &clientcmd.ConfigOverrides{})
	crudo, err := cfg.RawConfig()
	if err != nil {
		return desconectada("", fmt.Sprintf("no se pudo leer el kubeconfig: %v", err))
	}
	ctx := crudo.CurrentContext
	if ctx == "" {
		return desconectada("", "kubectl no tiene un contexto actual: creá un cluster (kind create cluster) o elegí uno con kubectl config use-context")
	}
	rest, err := cfg.ClientConfig()
	if err != nil {
		return desconectada(ctx, fmt.Sprintf("el contexto %q no sirve: %v", ctx, err))
	}
	cliente, err := kubernetes.NewForConfig(rest)
	if err != nil {
		return desconectada(ctx, fmt.Sprintf("no se pudo crear el cliente para %q: %v", ctx, err))
	}
	return Nueva(cliente, ctx)
}

func desconectada(contexto, motivo string) *Conexion {
	return &Conexion{contexto: contexto, motivo: motivo}
}

// Contexto es el nombre del contexto de kubectl de esta conexión ("" si no hay).
func (c *Conexion) Contexto() string { return c.contexto }

// Iniciar arranca los informers y espera a que sus cachés estén sincronizadas. Mientras el
// cluster no responde, espera (los informers reintentan solos) hasta que se cancela ctx.
// Sin cliente no hace nada. Los informers viven hasta que se cancela ctx.
func (c *Conexion) Iniciar(ctx context.Context) error {
	if c.cliente == nil {
		return nil
	}
	fabrica := informers.NewSharedInformerFactory(c.cliente, 0)
	nodos := fabrica.Core().V1().Nodes()
	pods := fabrica.Core().V1().Pods()
	c.nodos = nodos.Lister()
	c.pods = pods.Lister()
	for _, inf := range []cache.SharedIndexInformer{nodos.Informer(), pods.Informer()} {
		if _, err := inf.AddEventHandler(c.avisos.alCambiar()); err != nil {
			return err
		}
	}

	fabrica.Start(ctx.Done())
	if !cache.WaitForCacheSync(ctx.Done(), nodos.Informer().HasSynced, pods.Informer().HasSynced) {
		return fmt.Errorf("no se pudo sincronizar con el cluster %q", c.contexto)
	}
	c.sincronizada.Store(true)
	c.avisos.avisar()
	return nil
}

// Objetos devuelve los objetos que hay ahora en las cachés de los informers.
// Solo tiene sentido después de que Iniciar terminó bien.
func (c *Conexion) Objetos() foto.Objetos {
	nodos, _ := c.nodos.List(labels.Everything())
	pods, _ := c.pods.List(labels.Everything())
	return foto.Objetos{Nodos: nodos, Pods: pods}
}

// Foto es la foto actual del cluster, o una foto desconectada con el motivo.
func (c *Conexion) Foto() foto.Foto {
	switch {
	case c.cliente == nil:
		return foto.Desconectada(c.contexto, c.motivo)
	case !c.sincronizada.Load():
		return foto.Desconectada(c.contexto, fmt.Sprintf("conectando con el cluster del contexto %q…", c.contexto))
	default:
		return foto.Construir(c.contexto, c.Objetos())
	}
}
