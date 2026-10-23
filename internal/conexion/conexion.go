// Package conexion es la conexión al cluster (módulo 1 de la spec): resuelve el contexto de
// kubectl, lista los contextos y cambia de contexto en caliente (Gestor, en gestor.go), y mantiene
// watches (informers) de los objetos que necesita la foto (Conexion).
//
// kviz nunca se cae por no poder conectarse: sin contexto, o mientras el cluster no responde,
// el Gestor entrega una foto desconectada con el motivo y reintenta solo.
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
	listersappsv1 "k8s.io/client-go/listers/apps/v1"
	listersbatchv1 "k8s.io/client-go/listers/batch/v1"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
	listersdiscoveryv1 "k8s.io/client-go/listers/discovery/v1"
	listersnetworkingv1 "k8s.io/client-go/listers/networking/v1"
	"k8s.io/client-go/tools/cache"

	"kviz/internal/foto"
)

// Conexion es la conexión a un cluster que responde, con sus informers. La arma el Gestor.
type Conexion struct {
	contexto string
	cliente  kubernetes.Interface

	sincronizada atomic.Bool
	avisos       *avisos
	nodos        listerscorev1.NodeLister
	pods         listerscorev1.PodLister
	deployments  listersappsv1.DeploymentLister
	replicaSets  listersappsv1.ReplicaSetLister
	statefulSets listersappsv1.StatefulSetLister
	daemonSets   listersappsv1.DaemonSetLister
	jobs         listersbatchv1.JobLister
	cronJobs     listersbatchv1.CronJobLister
	services     listerscorev1.ServiceLister
	slices       listersdiscoveryv1.EndpointSliceLister
	ingress      listersnetworkingv1.IngressLister
	configMaps   listerscorev1.ConfigMapLister
	secrets      listerscorev1.SecretLister
}

// nueva arma una conexión sobre un cliente ya creado; los cambios de sus objetos avisan a a.
func nueva(cliente kubernetes.Interface, contexto string, a *avisos) *Conexion {
	return &Conexion{cliente: cliente, contexto: contexto, avisos: a}
}

// Contexto es el nombre del contexto de kubectl de esta conexión.
func (c *Conexion) Contexto() string { return c.contexto }

// Iniciar arranca los informers y espera a que sus cachés estén sincronizadas. Mientras el
// cluster no responde, espera (los informers reintentan solos) hasta que se cancela ctx.
// Los informers viven hasta que se cancela ctx.
func (c *Conexion) Iniciar(ctx context.Context) error {
	fabrica := informers.NewSharedInformerFactory(c.cliente, 0)
	nodos := fabrica.Core().V1().Nodes()
	pods := fabrica.Core().V1().Pods()
	c.nodos = nodos.Lister()
	c.pods = pods.Lister()
	services := fabrica.Core().V1().Services()
	slices := fabrica.Discovery().V1().EndpointSlices()
	c.services = services.Lister()
	c.slices = slices.Lister()
	ingress := fabrica.Networking().V1().Ingresses()
	c.ingress = ingress.Lister()
	configMaps, secrets := fabrica.Core().V1().ConfigMaps(), fabrica.Core().V1().Secrets()
	c.configMaps, c.secrets = configMaps.Lister(), secrets.Lister()
	apps, batch := fabrica.Apps().V1(), fabrica.Batch().V1()
	c.deployments = apps.Deployments().Lister()
	c.replicaSets = apps.ReplicaSets().Lister()
	c.statefulSets = apps.StatefulSets().Lister()
	c.daemonSets = apps.DaemonSets().Lister()
	c.jobs = batch.Jobs().Lister()
	c.cronJobs = batch.CronJobs().Lister()

	// cada alta, modificación o baja de un objeto observado avisa al stream
	for _, inf := range []cache.SharedIndexInformer{
		nodos.Informer(), pods.Informer(), services.Informer(), slices.Informer(), ingress.Informer(),
		apps.Deployments().Informer(), apps.ReplicaSets().Informer(), apps.StatefulSets().Informer(),
		apps.DaemonSets().Informer(), batch.Jobs().Informer(), batch.CronJobs().Informer(),
		configMaps.Informer(), secrets.Informer(),
	} {
		if _, err := inf.AddEventHandler(c.avisos.alCambiar()); err != nil {
			return err
		}
	}

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
	c.avisos.avisar()
	return nil
}

// Objetos devuelve los objetos que hay ahora en las cachés de los informers.
// Solo tiene sentido después de que Iniciar terminó bien.
func (c *Conexion) Objetos() foto.Objetos {
	nodos, _ := c.nodos.List(labels.Everything())
	pods, _ := c.pods.List(labels.Everything())
	services, _ := c.services.List(labels.Everything())
	slices, _ := c.slices.List(labels.Everything())
	ingress, _ := c.ingress.List(labels.Everything())
	o := foto.Objetos{Nodos: nodos, Pods: pods, Services: services, EndpointSlices: slices, Ingress: ingress}
	o.Deployments, _ = c.deployments.List(labels.Everything())
	o.ReplicaSets, _ = c.replicaSets.List(labels.Everything())
	o.StatefulSets, _ = c.statefulSets.List(labels.Everything())
	o.DaemonSets, _ = c.daemonSets.List(labels.Everything())
	o.Jobs, _ = c.jobs.List(labels.Everything())
	o.CronJobs, _ = c.cronJobs.List(labels.Everything())
	o.ConfigMaps, _ = c.configMaps.List(labels.Everything())
	o.Secrets, _ = c.secrets.List(labels.Everything())
	return o
}

// Foto es la foto actual del cluster, o una foto desconectada mientras se sincroniza.
func (c *Conexion) Foto() foto.Foto {
	if !c.sincronizada.Load() {
		return foto.Desconectada(c.contexto, conectando(c.contexto))
	}
	return foto.Construir(c.contexto, c.Objetos())
}
