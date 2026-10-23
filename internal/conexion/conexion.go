// Package conexion es la conexión al cluster (módulo 1 de la spec): resuelve el contexto de
// kubectl y mantiene watches (informers) de los objetos que necesita la foto.
//
// kviz nunca se cae por no poder conectarse: sin contexto, o mientras el cluster no responde,
// la conexión entrega una foto desconectada con el motivo.
//
// Para observar un tipo de objeto nuevo: registrar su informer en Iniciar y copiar sus objetos
// en Objetos (y sumarlos a foto.Objetos).
package conexion

import (
	"context"
	"fmt"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	listersappsv1 "k8s.io/client-go/listers/apps/v1"
	listersbatchv1 "k8s.io/client-go/listers/batch/v1"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
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
	nodos        listerscorev1.NodeLister
	pods         listerscorev1.PodLister
	deployments  listersappsv1.DeploymentLister
	replicaSets  listersappsv1.ReplicaSetLister
	statefulSets listersappsv1.StatefulSetLister
	daemonSets   listersappsv1.DaemonSetLister
	jobs         listersbatchv1.JobLister
	cronJobs     listersbatchv1.CronJobLister
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
	apps, batch := fabrica.Apps().V1(), fabrica.Batch().V1()
	c.deployments = apps.Deployments().Lister()
	c.replicaSets = apps.ReplicaSets().Lister()
	c.statefulSets = apps.StatefulSets().Lister()
	c.daemonSets = apps.DaemonSets().Lister()
	c.jobs = batch.Jobs().Lister()
	c.cronJobs = batch.CronJobs().Lister()

	fabrica.Start(ctx.Done())
	sincronizados := fabrica.WaitForCacheSync(ctx.Done())
	todos := len(sincronizados) > 0
	for _, ok := range sincronizados {
		todos = todos && ok
	}
	if !todos {
		return fmt.Errorf("no se pudo sincronizar con el cluster %q", c.contexto)
	}
	c.sincronizada.Store(true)
	return nil
}

// Objetos devuelve los objetos que hay ahora en las cachés de los informers.
// Solo tiene sentido después de que Iniciar terminó bien.
func (c *Conexion) Objetos() foto.Objetos {
	nodos, _ := c.nodos.List(labels.Everything())
	pods, _ := c.pods.List(labels.Everything())
	o := foto.Objetos{Nodos: nodos, Pods: pods}
	o.Deployments, _ = c.deployments.List(labels.Everything())
	o.ReplicaSets, _ = c.replicaSets.List(labels.Everything())
	o.StatefulSets, _ = c.statefulSets.List(labels.Everything())
	o.DaemonSets, _ = c.daemonSets.List(labels.Everything())
	o.Jobs, _ = c.jobs.List(labels.Everything())
	o.CronJobs, _ = c.cronJobs.List(labels.Everything())
	return o
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
