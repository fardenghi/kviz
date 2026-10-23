package foto

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Replicas son las réplicas deseadas y las listas de un workload.
type Replicas struct {
	Deseadas int32 `json:"deseadas"`
	Listas   int32 `json:"listas"`
}

// completarRollout llena las réplicas y la estrategia de rollout de cada workload. Un Job o un
// CronJob no tiene réplicas ni estrategia; un ReplicaSet suelto tiene réplicas pero no estrategia.
func completarRollout(ws []Workload, o Objetos) {
	type rollout struct {
		replicas   *Replicas
		estrategia string
	}
	porID := map[string]rollout{}
	for _, d := range o.Deployments {
		r := Replicas{Deseadas: deseadas(d.Spec.Replicas), Listas: d.Status.ReadyReplicas}
		porID[ref{"Deployment", d.Namespace, d.Name}.id()] = rollout{&r, estrategiaDeDeployment(d.Spec.Strategy)}
	}
	for _, s := range o.StatefulSets {
		r := Replicas{Deseadas: deseadas(s.Spec.Replicas), Listas: s.Status.ReadyReplicas}
		porID[ref{"StatefulSet", s.Namespace, s.Name}.id()] = rollout{&r, estrategiaDeStatefulSet(s.Spec.UpdateStrategy)}
	}
	for _, d := range o.DaemonSets {
		r := Replicas{Deseadas: d.Status.DesiredNumberScheduled, Listas: d.Status.NumberReady}
		porID[ref{"DaemonSet", d.Namespace, d.Name}.id()] = rollout{&r, estrategiaDeDaemonSet(d.Spec.UpdateStrategy)}
	}
	for _, rs := range o.ReplicaSets {
		r := Replicas{Deseadas: deseadas(rs.Spec.Replicas), Listas: rs.Status.ReadyReplicas}
		porID[ref{"ReplicaSet", rs.Namespace, rs.Name}.id()] = rollout{replicas: &r}
	}
	for i := range ws {
		if r, ok := porID[ws[i].ID]; ok {
			ws[i].Replicas, ws[i].Estrategia = r.replicas, r.estrategia
		}
	}
}

// deseadas es spec.replicas; sin valor, Kubernetes usa 1.
func deseadas(n *int32) int32 {
	if n == nil {
		return 1
	}
	return *n
}

// estrategiaDeDeployment es "Recreate" o "RollingUpdate (maxSurge 25%, maxUnavailable 25%)".
// Sin tipo, Kubernetes usa RollingUpdate.
func estrategiaDeDeployment(s appsv1.DeploymentStrategy) string {
	if s.Type == appsv1.RecreateDeploymentStrategyType {
		return string(s.Type)
	}
	if s.RollingUpdate == nil {
		return string(appsv1.RollingUpdateDeploymentStrategyType)
	}
	return string(appsv1.RollingUpdateDeploymentStrategyType) + limites(s.RollingUpdate.MaxSurge, s.RollingUpdate.MaxUnavailable)
}

func estrategiaDeStatefulSet(s appsv1.StatefulSetUpdateStrategy) string {
	if s.Type == appsv1.OnDeleteStatefulSetStrategyType {
		return string(s.Type)
	}
	texto := string(appsv1.RollingUpdateStatefulSetStrategyType)
	if s.RollingUpdate != nil && s.RollingUpdate.Partition != nil && *s.RollingUpdate.Partition > 0 {
		texto += fmt.Sprintf(" (partition %d)", *s.RollingUpdate.Partition)
	}
	return texto
}

func estrategiaDeDaemonSet(s appsv1.DaemonSetUpdateStrategy) string {
	if s.Type == appsv1.OnDeleteDaemonSetStrategyType {
		return string(s.Type)
	}
	if s.RollingUpdate == nil {
		return string(appsv1.RollingUpdateDaemonSetStrategyType)
	}
	return string(appsv1.RollingUpdateDaemonSetStrategyType) + limites(s.RollingUpdate.MaxSurge, s.RollingUpdate.MaxUnavailable)
}

// limites es " (maxSurge X, maxUnavailable Y)" con los que estén puestos.
func limites(maxSurge, maxUnavailable *intstr.IntOrString) string {
	var partes []string
	if maxSurge != nil {
		partes = append(partes, "maxSurge "+maxSurge.String())
	}
	if maxUnavailable != nil {
		partes = append(partes, "maxUnavailable "+maxUnavailable.String())
	}
	if len(partes) == 0 {
		return ""
	}
	texto := " (" + partes[0]
	for _, p := range partes[1:] {
		texto += ", " + p
	}
	return texto + ")"
}
