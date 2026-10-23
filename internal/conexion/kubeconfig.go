package conexion

import (
	"fmt"
	"sort"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// KubeconfigDeKubectl es el kubeconfig de kubectl (KUBECONFIG o ~/.kube/config). Se vuelve a leer
// en cada pedido, así kviz ve los clusters que se crean y se borran mientras corre. kviz nunca
// lo modifica: cambiar de contexto en kviz no cambia el contexto actual de kubectl.
func KubeconfigDeKubectl() Kubeconfig { return kubeconfigDeKubectl{} }

type kubeconfigDeKubectl struct{}

func (kubeconfigDeKubectl) Contextos() (string, []string, error) {
	crudo, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		return "", []string{}, err
	}
	todos := []string{}
	for nombre := range crudo.Contexts {
		todos = append(todos, nombre)
	}
	sort.Strings(todos)
	return crudo.CurrentContext, todos, nil
}

func (kubeconfigDeKubectl) Cliente(contexto string) (kubernetes.Interface, error) {
	crudo, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer el kubeconfig: %w", err)
	}
	if _, ok := crudo.Contexts[contexto]; !ok {
		return nil, fmt.Errorf("%w: %q (¿se borró el cluster?)", ErrContextoInexistente, contexto)
	}
	rest, err := clientcmd.NewNonInteractiveClientConfig(*crudo, contexto, &clientcmd.ConfigOverrides{}, nil).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("el contexto %q no sirve: %w", contexto, err)
	}
	return kubernetes.NewForConfig(rest)
}
