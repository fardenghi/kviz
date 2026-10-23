// Package sondeo es el pod de sondeo (módulo 10 de la spec, ADR 0001): un pod temporal en
// kviz-system desde el que salen las requests de prueba, porque desde la máquina local no se llega
// a las IPs internas del cluster.
//
// Crear y borrar este pod es la única escritura de kviz en el cluster. Se crea la primera vez que
// hace falta (Sondeo.HTTP), se borra al cerrar kviz o al cambiar de contexto (Sondeo.Borrar), y al
// conectarse kviz reutiliza un pod huérfano que siga corriendo o borra los que no sirven
// (Sondeo.Adoptar).
//
// Lo que se ejecuta adentro del pod queda detrás de Ejecutor ("ejecutar y devolver el resultado
// crudo"), que los tests reemplazan por un doble con respuestas guionadas. Para sumar un tipo de
// pedido (la prueba de conexión TCP): un método en Ejecutor, su tipo de pedido y resultado crudo, y
// un método en Sondeo que asegura el pod y lo ejecuta, igual que HTTP.
package sondeo

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Namespace es donde vive el pod de sondeo; la vista de topología nunca lo muestra.
const Namespace = "kviz-system"

// Imagen es una imagen chica que trae curl.
const Imagen = "curlimages/curl:8.11.1"

// Etiquetas marcan los pods de sondeo de kviz, para encontrar los huérfanos al arrancar.
var Etiquetas = map[string]string{
	"app.kubernetes.io/name":       "kviz-sondeo",
	"app.kubernetes.io/managed-by": "kviz",
}

const selector = "app.kubernetes.io/name=kviz-sondeo,app.kubernetes.io/managed-by=kviz"

// Pod es el pod de sondeo donde se ejecuta un pedido.
type Pod struct {
	// Contexto de kubectl del cluster del pod.
	Contexto  string
	Namespace string
	Nombre    string
	// Nodo donde corre ("" si no se sabe).
	Nodo string
}

// Header es un header HTTP. Se guarda como lista para respetar el orden y los repetidos.
type Header struct {
	Nombre string `json:"nombre"`
	Valor  string `json:"valor"`
}

// PedidoHTTP es una request de prueba tal como la ejecuta el pod de sondeo.
type PedidoHTTP struct {
	Metodo  string
	URL     string
	Headers []Header
	Body    string
}

// RespuestaHTTP es el resultado crudo de una request de prueba, sin interpretar.
type RespuestaHTTP struct {
	Status  int
	Headers []Header
	Body    string
	// Duracion es el tiempo total de la request, medido adentro del pod de sondeo.
	Duracion time.Duration
	// IPRemota es la IP a la que se conectó el pod de sondeo (la ClusterIP de un Service, por
	// ejemplo); "" si no se sabe.
	IPRemota string
}

// SinRespuesta es el error de una request de prueba que salió del pod de sondeo pero no recibió
// respuesta: conexión rechazada, timeout, nombre que no resuelve… No es una falla del pod de sondeo.
type SinRespuesta struct {
	Motivo string
}

func (e SinRespuesta) Error() string { return e.Motivo }

// Ejecutor ejecuta pedidos adentro del pod de sondeo y devuelve el resultado crudo. Devuelve
// SinRespuesta si el pedido salió pero no hubo respuesta; cualquier otro error es del pod de sondeo.
type Ejecutor interface {
	HTTP(ctx context.Context, pod Pod, p PedidoHTTP) (RespuestaHTTP, error)
}

// EsperaArranque es cuánto se espera a que el pod de sondeo recién creado esté corriendo (la
// primera vez el nodo tiene que bajar la imagen).
const EsperaArranque = 90 * time.Second

// Sondeo maneja el pod de sondeo de un cluster.
type Sondeo struct {
	cliente  kubernetes.Interface
	contexto string
	ejecutor Ejecutor

	mu  sync.Mutex
	pod *Pod // nil mientras kviz no tiene un pod de sondeo en el cluster
}

// Nuevo arma el pod de sondeo de un cluster, todavía sin crearlo.
func Nuevo(cliente kubernetes.Interface, contexto string, ejecutor Ejecutor) *Sondeo {
	return &Sondeo{cliente: cliente, contexto: contexto, ejecutor: ejecutor}
}

// HTTP manda una request de prueba desde el pod de sondeo; si todavía no hay pod, lo crea.
// Devuelve también el pod desde el que salió.
func (s *Sondeo) HTTP(ctx context.Context, p PedidoHTTP) (RespuestaHTTP, Pod, error) {
	pod, err := s.asegurar(ctx)
	if err != nil {
		return RespuestaHTTP{}, Pod{}, err
	}
	r, err := s.ejecutor.HTTP(ctx, pod, p)
	return r, pod, err
}

// Adoptar se hace cargo de los pods de sondeo que dejó una sesión anterior de kviz (por ejemplo,
// si se cerró de golpe): reutiliza uno que siga corriendo y borra los demás.
func (s *Sondeo) Adoptar(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	lista, err := s.cliente.CoreV1().Pods(Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return err
	}
	var errs []error
	for i := range lista.Items {
		p := &lista.Items[i]
		if s.pod != nil && s.pod.Nombre == p.Name {
			continue
		}
		if s.pod == nil && sirve(p) {
			s.pod = s.podDe(p)
			continue
		}
		errs = append(errs, s.borrar(ctx, p.Name))
	}
	return errors.Join(errs...)
}

// Borrar borra el pod de sondeo, si kviz tiene uno. Se llama al cerrar kviz y al dejar el cluster.
func (s *Sondeo) Borrar(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pod == nil {
		return nil
	}
	nombre := s.pod.Nombre
	s.pod = nil
	return s.borrar(ctx, nombre)
}

func (s *Sondeo) borrar(ctx context.Context, nombre string) error {
	var ya int64 // no hace falta esperar: el pod solo duerme
	err := s.cliente.CoreV1().Pods(Namespace).Delete(ctx, nombre, metav1.DeleteOptions{GracePeriodSeconds: &ya})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// asegurar devuelve el pod de sondeo corriendo; si no hay (o lo borraron), crea uno y espera a que arranque.
func (s *Sondeo) asegurar(ctx context.Context) (Pod, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pod != nil {
		p, err := s.cliente.CoreV1().Pods(Namespace).Get(ctx, s.pod.Nombre, metav1.GetOptions{})
		if err == nil && sirve(p) {
			return *s.pod, nil
		}
		if err != nil && !apierrors.IsNotFound(err) {
			return Pod{}, fmt.Errorf("no se pudo revisar el pod de sondeo: %w", err)
		}
		if err == nil { // terminó (lleva un día durmiendo) o se está borrando: se reemplaza
			_ = s.borrar(ctx, p.Name)
		}
		s.pod = nil
	}
	if err := s.asegurarNamespace(ctx); err != nil {
		return Pod{}, err
	}
	creado, err := s.cliente.CoreV1().Pods(Namespace).Create(ctx, especificacion(), metav1.CreateOptions{})
	if err != nil {
		return Pod{}, fmt.Errorf("no se pudo crear el pod de sondeo en %s: %w", Namespace, err)
	}
	s.pod = s.podDe(creado)
	listo, err := s.esperarArranque(ctx, creado)
	if err != nil {
		return Pod{}, err
	}
	s.pod = s.podDe(listo)
	return *s.pod, nil
}

// asegurarNamespace crea kviz-system si no existe: el pod de sondeo necesita dónde vivir.
func (s *Sondeo) asegurarNamespace(ctx context.Context) error {
	_, err := s.cliente.CoreV1().Namespaces().Get(ctx, Namespace, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return fmt.Errorf("no se pudo revisar el namespace %s: %w", Namespace, err)
	}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: Namespace, Labels: map[string]string{"app.kubernetes.io/managed-by": "kviz"}}}
	if _, err := s.cliente.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("no se pudo crear el namespace %s: %w", Namespace, err)
	}
	return nil
}

// esperarArranque espera a que el pod esté corriendo, revisándolo cada tanto.
func (s *Sondeo) esperarArranque(ctx context.Context, p *corev1.Pod) (*corev1.Pod, error) {
	ctx, cancelar := context.WithTimeout(ctx, EsperaArranque)
	defer cancelar()
	revisar := time.NewTicker(250 * time.Millisecond)
	defer revisar.Stop()
	for {
		if sirve(p) {
			return p, nil
		}
		if p.Status.Phase == corev1.PodFailed || p.Status.Phase == corev1.PodSucceeded {
			return nil, fmt.Errorf("el pod de sondeo %s/%s terminó (%s)", Namespace, p.Name, p.Status.Phase)
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("el pod de sondeo %s/%s no arrancó: %s", Namespace, p.Name, porQueNoArranca(p))
		case <-revisar.C:
		}
		actual, err := s.cliente.CoreV1().Pods(Namespace).Get(ctx, p.Name, metav1.GetOptions{})
		if err != nil {
			if ctx.Err() != nil {
				continue
			}
			return nil, fmt.Errorf("no se pudo revisar el pod de sondeo: %w", err)
		}
		p = actual
	}
}

func porQueNoArranca(p *corev1.Pod) string {
	for _, c := range p.Status.ContainerStatuses {
		if w := c.State.Waiting; w != nil && w.Reason != "" {
			return w.Reason + " " + w.Message
		}
	}
	if p.Status.Phase != "" {
		return string(p.Status.Phase)
	}
	return "sigue sin estado"
}

// sirve dice si un pod de sondeo está corriendo y no se está borrando.
func sirve(p *corev1.Pod) bool {
	return p.DeletionTimestamp == nil && p.Status.Phase == corev1.PodRunning
}

func (s *Sondeo) podDe(p *corev1.Pod) *Pod {
	return &Pod{Contexto: s.contexto, Namespace: p.Namespace, Nombre: p.Name, Nodo: p.Spec.NodeName}
}

// especificacion es el pod de sondeo: curl que duerme hasta que kviz le hace exec, sin permisos
// sobre la API y con pocos recursos.
func especificacion() *corev1.Pod {
	no := false
	var ya int64
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: Namespace, Name: "kviz-sondeo-" + sufijo(), Labels: Etiquetas},
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken:  &no,
			TerminationGracePeriodSeconds: &ya,
			RestartPolicy:                 corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:            "curl",
				Image:           Imagen,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Command:         []string{"sleep", "86400"},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10m"), corev1.ResourceMemory: resource.MustParse("16Mi")},
					Limits:   corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("64Mi")},
				},
			}},
		},
	}
}

// sufijo son 5 letras o números al azar, como los de los pods de un ReplicaSet.
func sufijo() string {
	const letras = "bcdfghjklmnpqrstvwxz2456789"
	b := make([]byte, 5)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = letras[int(b[i])%len(letras)]
	}
	return string(b)
}
