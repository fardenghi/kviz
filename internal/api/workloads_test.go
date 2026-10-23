package api_test

import (
	"context"
	"reflect"
	"regexp"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUnPodSinDuenoEsPodSuelto(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab",
		nodo("lab-worker", "172.23.0.4"),
		pod("default", "shell", "lab-worker", "10.244.1.9", corev1.PodRunning),
	)

	f := kviz.foto(t)

	if got := f.Pods[0].Workload; got != nil {
		t.Errorf("workload de shell = %q, want null (Pod suelto)", *got)
	}
	if len(f.Workloads) != 0 {
		t.Errorf("workloads = %s, want []", mostrar(f.Workloads))
	}
	if string(kviz.fotoCruda(t)["workloads"]) != "[]" {
		t.Errorf("workloads en el JSON = %s, want []", kviz.fotoCruda(t)["workloads"])
	}
}

func TestUnDeploymentEnRolloutMuestraLosDosReplicaSetsEnLaCadena(t *testing.T) {
	backend := deployment("default", "backend")
	nuevo := replicaSet(backend, "backend-6c7d9f8b5", 2, 2)
	viejo := replicaSet(backend, "backend-5d8f7c9b4", 1, 1)
	kviz := nuevoKviz(t, "kind-lab",
		nodo("lab-worker", "172.23.0.4"), nodo("lab-worker2", "172.23.0.3"),
		backend, nuevo, viejo,
		podDe(nuevo, "backend-6c7d9f8b5-p8r2c", "lab-worker"),
		podDe(nuevo, "backend-6c7d9f8b5-mn3vt", "lab-worker2"),
		podDe(viejo, "backend-5d8f7c9b4-h4j2w", "lab-worker2"),
	)

	f := kviz.foto(t)

	w := workloadConID(t, f, "Deployment/default/backend")
	want := workloadEnFoto{
		ID: "Deployment/default/backend", Tipo: "Deployment", Nombre: "backend", Namespace: "default",
		Cadena: []eslabonEnFoto{
			{Tipo: "Deployment", Nombre: "backend", Pods: []string{}},
			{Tipo: "ReplicaSet", Nombre: "backend-6c7d9f8b5", Pods: []string{"backend-6c7d9f8b5-mn3vt", "backend-6c7d9f8b5-p8r2c"}},
			{Tipo: "ReplicaSet", Nombre: "backend-5d8f7c9b4", Pods: []string{"backend-5d8f7c9b4-h4j2w"}},
		},
	}
	sinColor(&w)
	if !reflect.DeepEqual(w, want) {
		t.Fatalf("workload = %s\nwant %s", mostrar(w), mostrar(want))
	}
	if len(f.Workloads) != 1 {
		t.Errorf("workloads = %s, want solo el Deployment (sus ReplicaSets van en la cadena)", mostrar(f.Workloads))
	}
	for _, p := range f.Pods {
		if p.Workload == nil || *p.Workload != "Deployment/default/backend" {
			t.Errorf("workload de %s = %s, want Deployment/default/backend", p.Nombre, mostrar(p.Workload))
		}
	}
}

func workloadConID(t *testing.T, f fotoJSON, id string) workloadEnFoto {
	t.Helper()
	for _, w := range f.Workloads {
		if w.ID == id {
			return w
		}
	}
	t.Fatalf("no hay workload %q en %s", id, mostrar(f.Workloads))
	return workloadEnFoto{}
}

// sinColor borra el color para comparar el resto del workload; el color tiene sus propios tests.
func sinColor(w *workloadEnFoto) { w.Color = "" }

// fotoHastaQue pide la foto hasta que cumple la condición: los informers tardan un poco en ver
// los cambios hechos en el cliente falso.
func (k *kvizDePrueba) fotoHastaQue(t *testing.T, cumple func(fotoJSON) bool) fotoJSON {
	t.Helper()
	limite := time.Now().Add(5 * time.Second)
	for {
		f := k.foto(t)
		if cumple(f) {
			return f
		}
		if time.Now().After(limite) {
			t.Fatalf("la foto no llegó al estado esperado: %s", mostrar(f))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestTerminadoElRolloutElReplicaSetViejoSinReplicasNoAparece(t *testing.T) {
	backend := deployment("default", "backend")
	nuevo := replicaSet(backend, "backend-6c7d9f8b5", 1, 2)
	viejo := replicaSet(backend, "backend-5d8f7c9b4", 0, 1)
	kviz := nuevoKviz(t, "kind-lab",
		nodo("lab-worker", "172.23.0.4"),
		backend, nuevo, viejo,
		podDe(nuevo, "backend-6c7d9f8b5-p8r2c", "lab-worker"),
	)

	w := workloadConID(t, kviz.foto(t), "Deployment/default/backend")

	want := []eslabonEnFoto{
		{Tipo: "Deployment", Nombre: "backend", Pods: []string{}},
		{Tipo: "ReplicaSet", Nombre: "backend-6c7d9f8b5", Pods: []string{"backend-6c7d9f8b5-p8r2c"}},
	}
	if !reflect.DeepEqual(w.Cadena, want) {
		t.Fatalf("cadena = %s\nwant %s", mostrar(w.Cadena), mostrar(want))
	}
}

func TestUnStatefulSetEsDuenoDirectoDeSusPods(t *testing.T) {
	db := statefulSet("default", "db")
	kviz := nuevoKviz(t, "kind-lab",
		nodo("lab-worker", "172.23.0.4"), nodo("lab-worker2", "172.23.0.3"),
		db, podDe(db, "db-1", "lab-worker2"), podDe(db, "db-0", "lab-worker"),
	)

	f := kviz.foto(t)

	w := workloadConID(t, f, "StatefulSet/default/db")
	want := []eslabonEnFoto{{Tipo: "StatefulSet", Nombre: "db", Pods: []string{"db-0", "db-1"}}}
	if !reflect.DeepEqual(w.Cadena, want) {
		t.Fatalf("cadena = %s\nwant %s", mostrar(w.Cadena), mostrar(want))
	}
	for _, p := range f.Pods {
		if p.Workload == nil || *p.Workload != "StatefulSet/default/db" {
			t.Errorf("workload de %s = %s, want StatefulSet/default/db", p.Nombre, mostrar(p.Workload))
		}
	}
}

func TestUnDaemonSetTieneUnPodPorNodo(t *testing.T) {
	agente := daemonSet("observabilidad", "log-agent")
	kviz := nuevoKviz(t, "kind-lab",
		nodo("lab-control-plane", "172.23.0.2"), nodo("lab-worker", "172.23.0.4"), nodo("lab-worker2", "172.23.0.3"),
		agente,
		podDe(agente, "log-agent-8xq2n", "lab-control-plane"),
		podDe(agente, "log-agent-r7tfm", "lab-worker"),
		podDe(agente, "log-agent-wd94k", "lab-worker2"),
	)

	f := kviz.foto(t)

	w := workloadConID(t, f, "DaemonSet/observabilidad/log-agent")
	want := []eslabonEnFoto{{Tipo: "DaemonSet", Nombre: "log-agent", Pods: []string{"log-agent-8xq2n", "log-agent-r7tfm", "log-agent-wd94k"}}}
	if !reflect.DeepEqual(w.Cadena, want) {
		t.Fatalf("cadena = %s\nwant %s", mostrar(w.Cadena), mostrar(want))
	}
	nodos := map[string]bool{}
	for _, p := range f.Pods {
		if p.Workload != nil && *p.Workload == w.ID {
			nodos[*p.Nodo] = true
		}
	}
	if len(nodos) != 3 {
		t.Errorf("los pods del DaemonSet están en %v, want uno en cada uno de los 3 nodos", nodos)
	}
}

func TestUnCronJobConDosJobsArmaLaCadenaCronJobJobPod(t *testing.T) {
	backup := cronJob("default", "backup")
	primero := job("default", "backup-28745130", backup)
	segundo := job("default", "backup-28745160", backup)
	primero.CreationTimestamp = metav1.NewTime(time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC))
	segundo.CreationTimestamp = metav1.NewTime(time.Date(2026, 10, 1, 10, 30, 0, 0, time.UTC))
	kviz := nuevoKviz(t, "kind-lab",
		nodo("lab-worker", "172.23.0.4"),
		backup, primero, segundo,
		podDe(primero, "backup-28745130-7xk2d", "lab-worker"),
		podDe(segundo, "backup-28745160-c4mfn", "lab-worker"),
	)

	f := kviz.foto(t)

	w := workloadConID(t, f, "CronJob/default/backup")
	want := []eslabonEnFoto{
		{Tipo: "CronJob", Nombre: "backup", Pods: []string{}},
		{Tipo: "Job", Nombre: "backup-28745160", Pods: []string{"backup-28745160-c4mfn"}},
		{Tipo: "Job", Nombre: "backup-28745130", Pods: []string{"backup-28745130-7xk2d"}},
	}
	if !reflect.DeepEqual(w.Cadena, want) {
		t.Fatalf("cadena = %s\nwant %s", mostrar(w.Cadena), mostrar(want))
	}
	if len(f.Workloads) != 1 {
		t.Errorf("workloads = %s, want solo el CronJob (sus Jobs van en la cadena)", mostrar(f.Workloads))
	}
	for _, p := range f.Pods {
		if p.Workload == nil || *p.Workload != "CronJob/default/backup" {
			t.Errorf("workload de %s = %s, want CronJob/default/backup", p.Nombre, mostrar(p.Workload))
		}
	}
}

func TestUnJobOUnReplicaSetSinDuenoSonWorkloadsPropios(t *testing.T) {
	migracion := job("default", "migracion")
	rs := &appsv1.ReplicaSet{TypeMeta: tipoReplicaSet, ObjectMeta: meta("default", "cache")}
	kviz := nuevoKviz(t, "kind-lab",
		nodo("lab-worker", "172.23.0.4"),
		migracion, rs,
		podDe(migracion, "migracion-x2k9q", "lab-worker"),
		podDe(rs, "cache-7tq2p", "lab-worker"),
	)

	f := kviz.foto(t)

	var ids []string
	for _, w := range f.Workloads {
		ids = append(ids, w.ID)
	}
	want := []string{"Job/default/migracion", "ReplicaSet/default/cache"} // ordenados por id
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("workloads = %v, want %v", ids, want)
	}
	got := workloadConID(t, f, "Job/default/migracion").Cadena
	if !reflect.DeepEqual(got, []eslabonEnFoto{{Tipo: "Job", Nombre: "migracion", Pods: []string{"migracion-x2k9q"}}}) {
		t.Errorf("cadena del Job = %s", mostrar(got))
	}
}

func TestUnWorkloadSinPodsIgualAparece(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", cronJob("default", "backup"), statefulSet("default", "db"))

	f := kviz.foto(t)

	workloadConID(t, f, "CronJob/default/backup")
	got := workloadConID(t, f, "StatefulSet/default/db").Cadena
	if !reflect.DeepEqual(got, []eslabonEnFoto{{Tipo: "StatefulSet", Nombre: "db", Pods: []string{}}}) {
		t.Errorf("cadena = %s", mostrar(got))
	}
}

func TestCadaWorkloadTieneUnColorEstableEntreActualizaciones(t *testing.T) {
	backend := deployment("default", "backend")
	kviz := nuevoKviz(t, "kind-lab", backend, statefulSet("default", "db"))
	antes := map[string]string{}
	for _, w := range kviz.foto(t).Workloads {
		if !regexp.MustCompile(`^#[0-9a-f]{6}$`).MatchString(w.Color) {
			t.Errorf("color de %s = %q, want un color CSS #rrggbb", w.ID, w.Color)
		}
		antes[w.ID] = w.Color
	}

	// el cluster cambia: aparecen otros workloads y se borra uno
	ctx := context.Background()
	if _, err := kviz.cliente.AppsV1().Deployments("default").Create(ctx, deployment("default", "web"), metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := kviz.cliente.BatchV1().CronJobs("default").Create(ctx, cronJob("default", "backup"), metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := kviz.cliente.AppsV1().StatefulSets("default").Delete(ctx, "db", metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	f := kviz.fotoHastaQue(t, func(f fotoJSON) bool { return len(f.Workloads) == 3 })

	if got := workloadConID(t, f, "Deployment/default/backend").Color; got != antes["Deployment/default/backend"] {
		t.Errorf("color de backend = %q después de actualizar, want %q", got, antes["Deployment/default/backend"])
	}
	// y es el mismo si kviz arranca de nuevo con el mismo cluster
	otraVez := nuevoKviz(t, "kind-lab", backend)
	if got := workloadConID(t, otraVez.foto(t), "Deployment/default/backend").Color; got != antes["Deployment/default/backend"] {
		t.Errorf("color de backend = %q en otro arranque, want %q", got, antes["Deployment/default/backend"])
	}
}

func TestUnPodConUnDuenoQueNoEsWorkloadEsPodSuelto(t *testing.T) {
	estatico := pod("kube-system", "etcd-lab-control-plane", "lab-control-plane", "172.23.0.2", corev1.PodRunning)
	estatico.OwnerReferences = []metav1.OwnerReference{{APIVersion: "v1", Kind: "Node", Name: "lab-control-plane", Controller: ptrBool(true)}}
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-control-plane", "172.23.0.2"), estatico)

	f := kviz.foto(t)

	if f.Pods[0].Workload != nil || len(f.Workloads) != 0 {
		t.Errorf("pod = %s, workloads = %s; want Pod suelto y sin workloads", mostrar(f.Pods[0]), mostrar(f.Workloads))
	}
}
