# kviz

Herramienta local para ver de forma gráfica todo lo que corre en un cluster de Kubernetes y depurar cómo viaja el tráfico dentro de él. Observa el cluster sin modificarlo.

## Vista

**Vista de topología**:
Diagrama del cluster agrupado por nodo, con los pods (y sus containers) dentro de cada nodo y los Services e Ingress afuera, conectados a los pods a los que apuntan.
_Avoid_: mapa, grafo, dashboard

**Foco**:
Objeto de la vista de topología (Service, pod, Ingress o puerta) que el usuario apunta o selecciona; sus conexiones y objetos relacionados se marcan y el resto se atenúa. Seleccionar un workload no es foco: solo resalta sus pods.
_Avoid_: hover, highlight, resaltado

**Puerta**:
Marca en el borde de un nodo que representa un puerto abierto en ese nodo hacia el cluster: un NodePort (presente en todos los nodos) o el hostPort del Ingress Controller (solo en su nodo).
_Avoid_: entrada, puerto de nodo

**Advertencia de configuración**:
Señal que kviz marca sobre un objeto cuando su configuración tiene un error típico que impide que el tráfico llegue, como un selector que no matchea pods o un targetPort que ningún container expone.
_Avoid_: error, alerta, lint

**Panel de detalle**:
Panel que muestra toda la información de un objeto seleccionado en la vista de topología.
_Avoid_: sidebar, inspector

**Estado de probe**:
Salud de una probe que kviz deduce de señales indirectas (container Ready, Started, reinicios y events de fallo), ya que Kubernetes no guarda el resultado de cada ejecución.
_Avoid_: resultado de probe, último chequeo

**Comando sugerido**:
Comando de `kubectl` que la herramienta muestra, listo para copiar, para realizar una acción que ella misma no ejecuta.
_Avoid_: acción, botón

## Depuración de tráfico

**Request de prueba**:
Request HTTP real que la herramienta lanza desde dentro del cluster hacia un destino, para ver cómo responde y por dónde pasa.
_Avoid_: ping, test, llamada

**Prueba de conexión**:
Intento de abrir una conexión TCP hacia un destino, sin hablar ningún protocolo, para saber si el puerto acepta, rechaza o no responde. Sirve para lo que no es HTTP (por ejemplo, una base de datos).
_Avoid_: ping, telnet, port check

**Destino**:
Punto al que se dirige una request de prueba o una prueba de conexión: una ruta de Ingress, un Service, un NodePort, la IP de un pod o un nombre DNS interno.
_Avoid_: target, endpoint (en Kubernetes "endpoint" ya significa la IP de un pod detrás de un Service)

**Recorrido**:
Camino que siguió una request de prueba o una prueba de conexión (resolución DNS → entrada → regla de Ingress → Service → endpoint → pod), resaltado en la vista de topología.
_Avoid_: traza, trace (sugieren rastreo a nivel de paquetes, que la herramienta no hace)

**Reparto**:
Distribución de las respuestas entre los pods cuando una request de prueba se repite N veces, para ver el balanceo de carga.
_Avoid_: distribución, stats

**Pod de sondeo**:
Pod temporal que la herramienta crea en el cluster para originar las requests de prueba, ya que desde la máquina local no se llega a las IPs internas.
_Avoid_: pod de debug (se confunde con los ephemeral containers de `kubectl debug`)
