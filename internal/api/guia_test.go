package api_test

// El cluster de la guía como fixture (spec, "Datos de prueba"): kind `lab` con 3 nodos, el
// Service `backend` NodePort 30010 (7b-service-nodeport.yaml), el Ingress `backend` con rewrite
// (7d-ingress.yaml) y el Ingress Controller en lab-worker2. El controller no está en los YAML de
// la guía: se arma a mano como lo instala kind (Deployment → ReplicaSet → Pod en `ingress-nginx`,
// con hostPort 80 y 443) junto con su IngressClass `nginx`.

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func clusterDeLaGuia() []runtime.Object {
	objetos := []runtime.Object{
		nodo("lab-control-plane", "172.23.0.2"),
		nodo("lab-worker", "172.23.0.4"),
		nodo("lab-worker2", "172.23.0.3"),
		pod("default", "backend-79f9ffc666-k2x8p", "lab-worker", "10.244.1.5", corev1.PodRunning),
		pod("default", "backend-79f9ffc666-7hw4q", "lab-worker2", "10.244.2.4", corev1.PodRunning),
		service("default", "backend", corev1.ServiceTypeNodePort, "10.96.11.121", map[string]string{"app": "backend"},
			corev1.ServicePort{Protocol: corev1.ProtocolTCP, Port: 80, TargetPort: intstr.FromInt32(8081), NodePort: 30010}),
		endpointSlice("default", "backend-x7k2p", "backend",
			endpoint("10.244.1.5", "backend-79f9ffc666-k2x8p", "lab-worker", ptrBool(true)),
			endpoint("10.244.2.4", "backend-79f9ffc666-7hw4q", "lab-worker2", ptrBool(true)),
		),
		ingressDeLaGuia(),
	}
	return append(objetos, ingressController()...)
}

// ingressDeLaGuia es 7d-ingress.yaml.
func ingressDeLaGuia() *networkingv1.Ingress {
	clase := "nginx"
	tipo := networkingv1.PathTypeImplementationSpecific
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      "backend",
			Annotations: map[string]string{
				"nginx.ingress.kubernetes.io/use-regex":      "true",
				"nginx.ingress.kubernetes.io/rewrite-target": "/$2",
			},
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &clase,
			Rules:            []networkingv1.IngressRule{reglaIngress("", "/backend(/|$)(.*)", &tipo, backendIngress("backend", 80, ""))},
		},
	}
}

// ingressController es ingress-nginx como lo instala kind: un Deployment con un pod en
// lab-worker2 que abre los puertos 80 y 443 del nodo (hostPort), y la IngressClass `nginx`.
func ingressController() []runtime.Object {
	const ns = "ingress-nginx"
	etiquetas := map[string]string{"app.kubernetes.io/name": "ingress-nginx", "app.kubernetes.io/component": "controller"}
	contenedor := corev1.Container{
		Name:  "controller",
		Image: "registry.k8s.io/ingress-nginx/controller:v1.11.2",
		Ports: []corev1.ContainerPort{
			{Name: "http", ContainerPort: 80, HostPort: 80, Protocol: corev1.ProtocolTCP},
			{Name: "https", ContainerPort: 443, HostPort: 443, Protocol: corev1.ProtocolTCP},
			{Name: "webhook", ContainerPort: 8443, Protocol: corev1.ProtocolTCP},
		},
	}
	plantilla := corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{Labels: etiquetas},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{contenedor}},
	}
	verdadero := true
	dueno := func(kind, nombre string) []metav1.OwnerReference {
		return []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: kind, Name: nombre, Controller: &verdadero}}
	}

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "ingress-nginx-controller", Labels: etiquetas},
		Spec:       appsv1.DeploymentSpec{Selector: &metav1.LabelSelector{MatchLabels: etiquetas}, Template: plantilla},
	}
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "ingress-nginx-controller-6b9f", Labels: etiquetas,
			OwnerReferences: dueno("Deployment", deploy.Name)},
		Spec: appsv1.ReplicaSetSpec{Selector: &metav1.LabelSelector{MatchLabels: etiquetas}, Template: plantilla},
	}
	p := pod(ns, "ingress-nginx-controller-6b9f-lx2wp", "lab-worker2", "10.244.2.2", corev1.PodRunning)
	p.Labels = etiquetas
	p.OwnerReferences = dueno("ReplicaSet", rs.Name)
	p.Spec.Containers = []corev1.Container{contenedor}

	clase := &networkingv1.IngressClass{
		ObjectMeta: metav1.ObjectMeta{Name: "nginx"},
		Spec:       networkingv1.IngressClassSpec{Controller: "k8s.io/ingress-nginx"},
	}
	return []runtime.Object{deploy, rs, p, clase}
}
