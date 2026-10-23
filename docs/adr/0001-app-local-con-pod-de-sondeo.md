# kviz corre en la máquina local y usa un pod de sondeo para las requests

kviz es un binario que corre en la máquina del usuario y se conecta al cluster con el contexto actual de kubectl, en vez de instalarse como un Deployment dentro del cluster. Desde la máquina local (en Mac, con kind) no se puede llegar a las IPs de pods, Services ni nodos, así que las requests de prueba las origina un pod de sondeo temporal que kviz crea en el namespace `kviz-system` y borra al cerrarse. Así kviz sobrevive a borrar y recrear el cluster sin reinstalarse, y sus propios pods no ensucian la vista.

## Considered Options

- **Instalado dentro del cluster:** llega a todas las IPs sin pod extra, pero hay que reinstalarlo tras cada `kind delete`, igual necesita port-forward para entrar desde la Mac y aparece en su propia vista.

## Consequences

- La única escritura de kviz en el cluster es crear y borrar el pod de sondeo; todo lo demás es solo lectura.
- Si kviz se cierra de forma abrupta, el pod de sondeo puede quedar huérfano; kviz debe reutilizarlo o limpiarlo al volver a arrancar.
