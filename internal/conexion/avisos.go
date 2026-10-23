package conexion

import (
	"sync"

	"k8s.io/client-go/tools/cache"
)

// avisos reparte a cada suscriptor un aviso, sin datos, de que la foto puede haber cambiado.
// Los avisos se juntan: si el suscriptor todavía no leyó el anterior, no se acumula otro.
type avisos struct {
	mu   sync.Mutex
	subs map[chan struct{}]struct{}
}

// Suscribir devuelve un canal que avisa cada vez que la foto puede haber cambiado (un objeto
// observado se creó, cambió o se borró, o cambió el estado de la conexión), y la función para
// dejar de escuchar. Hay que pedir la foto después de suscribirse, así no se pierde ningún cambio.
func (c *Conexion) Suscribir() (<-chan struct{}, func()) {
	a := &c.avisos
	ch := make(chan struct{}, 1)
	a.mu.Lock()
	if a.subs == nil {
		a.subs = map[chan struct{}]struct{}{}
	}
	a.subs[ch] = struct{}{}
	a.mu.Unlock()
	return ch, func() {
		a.mu.Lock()
		delete(a.subs, ch)
		a.mu.Unlock()
	}
}

func (a *avisos) avisar() {
	a.mu.Lock()
	defer a.mu.Unlock()
	for ch := range a.subs {
		select {
		case ch <- struct{}{}:
		default: // ya tiene un aviso pendiente
		}
	}
}

// alCambiar es el handler de los informers: cualquier alta, modificación o baja avisa.
func (a *avisos) alCambiar() cache.ResourceEventHandler {
	avisar := func() { a.avisar() }
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(any) { avisar() },
		UpdateFunc: func(_, _ any) { avisar() },
		DeleteFunc: func(any) { avisar() },
	}
}
