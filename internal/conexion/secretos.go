package conexion

import (
	"errors"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// ErrSecretInexistente es el error de revelar un Secret, o una clave de un Secret, que no existe.
var ErrSecretInexistente = errors.New("no existe ese Secret o esa clave")

// ErrDesconectado es el error de pedir algo al cluster mientras kviz no lo ve.
var ErrDesconectado = errors.New("kviz no está conectado al cluster")

// ValorDeSecret revela el valor de una clave de un Secret. Es el único camino por el que un
// valor de Secret sale de kviz: la foto trae solo las claves.
func (g *Gestor) ValorDeSecret(namespace, nombre, clave string) (string, error) {
	g.mu.Lock()
	c := g.conexion
	g.mu.Unlock()
	if c == nil {
		return "", ErrDesconectado
	}
	return c.valorDeSecret(namespace, nombre, clave)
}

func (c *Conexion) valorDeSecret(namespace, nombre, clave string) (string, error) {
	s, err := c.secrets.Secrets(namespace).Get(nombre)
	if apierrors.IsNotFound(err) {
		return "", fmt.Errorf("%w: Secret %s/%s", ErrSecretInexistente, namespace, nombre)
	}
	if err != nil {
		return "", err
	}
	valor, ok := s.Data[clave]
	if !ok {
		return "", fmt.Errorf("%w: el Secret %s/%s no tiene la clave %q", ErrSecretInexistente, namespace, nombre, clave)
	}
	return string(valor), nil
}
