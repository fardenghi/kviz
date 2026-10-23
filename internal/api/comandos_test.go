package api_test

// Comandos sugeridos (ticket 08): GET /api/comandos?tipo=…&namespace=…&nombre=… devuelve los
// comandos de kubectl de un objeto, completos con nombre, namespace y contexto. kviz solo los
// muestra: no ejecuta ninguno.

import (
	"net/http"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

type comandoSugerido struct {
	Descripcion string `json:"descripcion"`
	Comando     string `json:"comando"`
}

func (k *kvizDePrueba) comandos(t *testing.T, tipo, namespace, nombre string) []string {
	t.Helper()
	var cs []comandoSugerido
	k.getJSON(t, "/api/comandos?tipo="+tipo+"&namespace="+namespace+"&nombre="+nombre, &cs)
	textos := []string{}
	for _, c := range cs {
		if c.Descripcion == "" {
			t.Errorf("el comando %q no tiene descripción", c.Comando)
		}
		textos = append(textos, c.Comando)
	}
	return textos
}

func mismosComandos(t *testing.T, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("comandos:\n%s\nwant:\n%s", mostrarLineas(got), mostrarLineas(want))
	}
}

// cantidad es un *int32 para spec.replicas y parecidos.
func cantidad(n int32) *int32 { return &n }

func mostrarLineas(ls []string) string {
	s := ""
	for _, l := range ls {
		s += "  " + l + "\n"
	}
	return s
}

// podConPuertos arma un pod corriendo con un container por nombre, cada uno con sus puertos TCP.
func podConContainer(namespace, nombre, container string, puertos ...int32) *corev1.Pod {
	p := pod(namespace, nombre, "lab-worker", "10.244.1.7", corev1.PodRunning)
	c := corev1.Container{Name: container, Image: "nginx:1.27"}
	for _, n := range puertos {
		c.Ports = append(c.Ports, corev1.ContainerPort{ContainerPort: n, Protocol: corev1.ProtocolTCP})
	}
	p.Spec.Containers = append(p.Spec.Containers, c)
	return p
}

func TestUnPodTraeLogsExecDescribeDeletePortForwardYDebug(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), podConContainer("default", "nginx", "nginx", 80))

	mismosComandos(t, kviz.comandos(t, "Pod", "default", "nginx"), []string{
		"kubectl logs nginx -n default --context kind-lab",
		"kubectl exec -it nginx -n default --context kind-lab -- sh",
		"kubectl describe pod nginx -n default --context kind-lab",
		"kubectl delete pod nginx -n default --context kind-lab",
		"kubectl port-forward pod/nginx 8080:80 -n default --context kind-lab",
		"kubectl debug -it nginx -n default --context kind-lab --image=busybox:1.36 --target=nginx",
	})
}

func TestUnObjetoQueNoEstaEnLaFotoNoTieneComandos(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab")

	res, err := http.Get(kviz.srv.URL + "/api/comandos?tipo=Pod&namespace=default&nombre=nginx")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", res.StatusCode)
	}
}

func TestEnUnPodConVariosContainersLogsYExecVanPorContainerYPortForwardAbreTodosLosPuertos(t *testing.T) {
	web := podConContainer("tienda", "web", "app", 80, 8081)
	web.Spec.Containers = append(web.Spec.Containers, corev1.Container{Name: "log", Image: "busybox"})
	web.Spec.InitContainers = []corev1.Container{{Name: "migrar", Image: "busybox"}}
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), web)

	mismosComandos(t, kviz.comandos(t, "Pod", "tienda", "web"), []string{
		"kubectl logs web -c app -n tienda --context kind-lab",
		"kubectl logs web -c log -n tienda --context kind-lab",
		"kubectl exec -it web -c app -n tienda --context kind-lab -- sh",
		"kubectl exec -it web -c log -n tienda --context kind-lab -- sh",
		"kubectl describe pod web -n tienda --context kind-lab",
		"kubectl delete pod web -n tienda --context kind-lab",
		"kubectl port-forward pod/web 8080:80 8081:8081 -n tienda --context kind-lab",
		"kubectl debug -it web -n tienda --context kind-lab --image=busybox:1.36 --target=app",
	})
}

func TestUnPodSinPuertosDeclaradosNoTienePortForward(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", nodo("lab-worker", "172.23.0.4"), podConContainer("default", "shell", "shell"))

	mismosComandos(t, kviz.comandos(t, "Pod", "default", "shell"), []string{
		"kubectl logs shell -n default --context kind-lab",
		"kubectl exec -it shell -n default --context kind-lab -- sh",
		"kubectl describe pod shell -n default --context kind-lab",
		"kubectl delete pod shell -n default --context kind-lab",
		"kubectl debug -it shell -n default --context kind-lab --image=busybox:1.36 --target=shell",
	})
}

func TestUnServiceTraePortForwardYSusEndpointSlices(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", clusterDeLaGuia()...)

	mismosComandos(t, kviz.comandos(t, "Service", "default", "backend"), []string{
		"kubectl port-forward service/backend 8080:80 -n default --context kind-lab",
		"kubectl get endpointslices -l kubernetes.io/service-name=backend -n default --context kind-lab",
		"kubectl describe service backend -n default --context kind-lab",
	})
}

func TestElPortForwardDeUnServiceNoRepiteUnPuertoLocal(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", service("default", "web", corev1.ServiceTypeClusterIP, "10.96.0.20", map[string]string{"app": "web"},
		corev1.ServicePort{Name: "http", Protocol: corev1.ProtocolTCP, Port: 80},
		corev1.ServicePort{Name: "alt", Protocol: corev1.ProtocolTCP, Port: 8080},
		corev1.ServicePort{Name: "dns", Protocol: corev1.ProtocolUDP, Port: 53},
	))

	got := kviz.comandos(t, "Service", "default", "web")
	if want := "kubectl port-forward service/web 8080:80 8081:8080 -n default --context kind-lab"; got[0] != want {
		t.Errorf("port-forward = %q, want %q (UDP no se puede reenviar)", got[0], want)
	}
}

func TestUnDeploymentTraeScaleConSusReplicasYLosComandosDeRollout(t *testing.T) {
	backend := deployment("default", "backend")
	backend.Spec.Replicas = cantidad(2)
	kviz := nuevoKviz(t, "kind-lab", backend)

	mismosComandos(t, kviz.comandos(t, "Deployment", "default", "backend"), []string{
		"kubectl scale deployment/backend --replicas=2 -n default --context kind-lab",
		"kubectl rollout restart deployment/backend -n default --context kind-lab",
		"kubectl rollout undo deployment/backend -n default --context kind-lab",
		"kubectl rollout history deployment/backend -n default --context kind-lab",
		"kubectl describe deployment backend -n default --context kind-lab",
	})
}

func TestUnCronJobTraeCreateJobFrom(t *testing.T) {
	kviz := nuevoKviz(t, "kind-lab", cronJob("default", "backup"))

	mismosComandos(t, kviz.comandos(t, "CronJob", "default", "backup"), []string{
		"kubectl create job backup-manual --from=cronjob/backup -n default --context kind-lab",
		"kubectl describe cronjob backup -n default --context kind-lab",
	})
}

func TestLosDemasWorkloadsElIngressYElNodoTraenDescribe(t *testing.T) {
	db := statefulSet("default", "db")
	kviz := nuevoKviz(t, "kind-lab", append(clusterDeLaGuia(), db, daemonSet("kube-system", "kindnet"), job("default", "migrar"))...)

	casos := []struct {
		tipo, namespace, nombre string
		want                    []string
	}{
		{"StatefulSet", "default", "db", []string{"kubectl describe statefulset db -n default --context kind-lab"}},
		{"DaemonSet", "kube-system", "kindnet", []string{"kubectl describe daemonset kindnet -n kube-system --context kind-lab"}},
		{"Job", "default", "migrar", []string{"kubectl describe job migrar -n default --context kind-lab"}},
		{"Ingress", "default", "backend", []string{"kubectl describe ingress backend -n default --context kind-lab"}},
		{"Node", "", "lab-worker", []string{
			"kubectl describe node lab-worker --context kind-lab",
			"kubectl get pods -A -o wide --field-selector spec.nodeName=lab-worker --context kind-lab",
		}},
	}
	for _, c := range casos {
		t.Run(c.tipo, func(t *testing.T) {
			mismosComandos(t, kviz.comandos(t, c.tipo, c.namespace, c.nombre), c.want)
		})
	}
}

func TestUnContextoConEspaciosVaEntreComillasParaPegarloEnLaTerminal(t *testing.T) {
	kviz := nuevoKviz(t, "mi cluster's", nodo("lab-worker", "172.23.0.4"))

	got := kviz.comandos(t, "Node", "", "lab-worker")
	if want := `kubectl describe node lab-worker --context 'mi cluster'\''s'`; got[0] != want {
		t.Errorf("describe = %q, want %q", got[0], want)
	}
}

func TestSinContextoDeKubectlLosComandosNoLlevanContext(t *testing.T) {
	kviz := nuevoKviz(t, "", clusterDeLaGuia()...)

	mismosComandos(t, kviz.comandos(t, "Service", "default", "backend"), []string{
		"kubectl port-forward service/backend 8080:80 -n default",
		"kubectl get endpointslices -l kubernetes.io/service-name=backend -n default",
		"kubectl describe service backend -n default",
	})
}
