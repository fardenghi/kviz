package api_test

// Lo que la foto trae para el panel de detalle (ticket 08). El Service (tipo, ClusterIP, puertos,
// nodePort, selector y endpoints) y el Ingress (clase, reglas, rewrite y Services destino) ya los
// cubren services_test.go e ingress_test.go.

import (
	"reflect"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type detalleDePod struct {
	Nombre string            `json:"nombre"`
	Labels map[string]string `json:"labels"`
	Creado string            `json:"creado"`
}

type detalleDeNodo struct {
	Nombre    string    `json:"nombre"`
	Capacidad capacidad `json:"capacidad"`
}

type capacidad struct {
	CPU     string `json:"cpu"`
	Memoria string `json:"memoria"`
	Pods    string `json:"pods"`
}

type detalleDeWorkload struct {
	ID         string     `json:"id"`
	Replicas   *replicasW `json:"replicas"`
	Estrategia string     `json:"estrategia"`
}

type replicasW struct {
	Deseadas int32 `json:"deseadas"`
	Listas   int32 `json:"listas"`
}

type fotoParaDetalle struct {
	Pods      []detalleDePod      `json:"pods"`
	Nodos     []detalleDeNodo     `json:"nodos"`
	Workloads []detalleDeWorkload `json:"workloads"`
}

func (k *kvizDePrueba) fotoParaDetalle(t *testing.T) fotoParaDetalle {
	t.Helper()
	var f fotoParaDetalle
	k.getJSON(t, "/api/foto", &f)
	return f
}

func TestElPodTraeSusLabelsYCuandoSeCreoParaCalcularLaEdad(t *testing.T) {
	p := pod("default", "nginx", "lab-worker", "10.244.1.7", corev1.PodRunning)
	p.Labels = map[string]string{"app": "nginx", "tier": "web"}
	p.CreationTimestamp = metav1.NewTime(time.Date(2026, 10, 20, 13, 4, 5, 0, time.UTC))
	sinLabels := pod("default", "shell", "lab-worker", "10.244.1.8", corev1.PodRunning)
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), p, sinLabels)

	f := kviz.fotoParaDetalle(t)

	want := []detalleDePod{
		{Nombre: "nginx", Labels: map[string]string{"app": "nginx", "tier": "web"}, Creado: "2026-10-20T13:04:05Z"},
		{Nombre: "shell", Labels: map[string]string{}, Creado: ""},
	}
	if !reflect.DeepEqual(f.Pods, want) {
		t.Errorf("pods = %s\nwant %s", mostrar(f.Pods), mostrar(want))
	}
}

func TestElNodoTraeSuCapacidad(t *testing.T) {
	n := nodo("lab-worker", "172.23.0.4")
	n.Status.Capacity = corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("8"),
		corev1.ResourceMemory: resource.MustParse("16365916Ki"),
		corev1.ResourcePods:   resource.MustParse("110"),
	}
	kviz := nuevoKviz(t, "kind-lab", n, nodo("sin-datos", "172.23.0.9"))

	f := kviz.fotoParaDetalle(t)

	want := []detalleDeNodo{
		{Nombre: "lab-worker", Capacidad: capacidad{CPU: "8", Memoria: "16365916Ki", Pods: "110"}},
		{Nombre: "sin-datos", Capacidad: capacidad{}},
	}
	if !reflect.DeepEqual(f.Nodos, want) {
		t.Errorf("nodos = %s\nwant %s", mostrar(f.Nodos), mostrar(want))
	}
}

func TestCadaWorkloadTraeSusReplicasYSuEstrategiaDeRollout(t *testing.T) {
	surge, noDisponibles := intstr.FromString("25%"), intstr.FromInt32(0)
	backend := deployment("default", "backend")
	backend.Spec.Replicas = cantidad(3)
	backend.Spec.Strategy = appsv1.DeploymentStrategy{
		Type:          appsv1.RollingUpdateDeploymentStrategyType,
		RollingUpdate: &appsv1.RollingUpdateDeployment{MaxSurge: &surge, MaxUnavailable: &noDisponibles},
	}
	backend.Status.ReadyReplicas = 2
	viejo := deployment("default", "viejo")
	viejo.Spec.Strategy.Type = appsv1.RecreateDeploymentStrategyType
	db := statefulSet("default", "db")
	db.Spec.Replicas = cantidad(1)
	db.Status.ReadyReplicas = 1
	agente := daemonSet("kube-system", "kindnet")
	agente.Status.DesiredNumberScheduled, agente.Status.NumberReady = 3, 2
	agente.Spec.UpdateStrategy.Type = appsv1.OnDeleteDaemonSetStrategyType

	kviz := nuevoKviz(t, "kind-lab", backend, viejo, db, agente, cronJob("default", "backup"))

	got := map[string]detalleDeWorkload{}
	for _, w := range kviz.fotoParaDetalle(t).Workloads {
		got[w.ID] = w
	}
	want := map[string]detalleDeWorkload{
		"Deployment/default/backend": {"Deployment/default/backend", &replicasW{3, 2}, "RollingUpdate (maxSurge 25%, maxUnavailable 0)"},
		// sin spec.replicas Kubernetes usa 1
		"Deployment/default/viejo":      {"Deployment/default/viejo", &replicasW{1, 0}, "Recreate"},
		"StatefulSet/default/db":        {"StatefulSet/default/db", &replicasW{1, 1}, "RollingUpdate"},
		"DaemonSet/kube-system/kindnet": {"DaemonSet/kube-system/kindnet", &replicasW{3, 2}, "OnDelete"},
		"CronJob/default/backup":        {"CronJob/default/backup", nil, ""},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("workloads = %s\nwant %s", mostrar(got), mostrar(want))
	}
}
