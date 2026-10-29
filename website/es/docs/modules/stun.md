# Traversal STUN

Cuando tu operador no ofrece un puerto público fijo, el traversal STUN usa la perforación de NAT para obtener una dirección y un puerto públicos mapeados, de modo que los servicios de tu red local sean accesibles desde Internet.

## Capacidades

- Traversal TCP / UDP con mantenimiento de la conexión;
- UPnP / NAT-PMP crea automáticamente mapeos de puertos en el router;
- El reenvío integrado envía el puerto público obtenido mediante el traversal directamente a un destino de la red local;
- Webhooks y scripts se disparan cuando cambia la dirección o el puerto públicos, y puedes usarlos para actualizar el DNS, avisar a los clientes, etc.

## Uso junto con otros módulos

El puerto público obtenido con STUN suele cambiar. Combínalo con:

- [DNS dinámico](/es/docs/modules/ddns) o un webhook / script para sincronizar la nueva dirección con tus registros DNS;
- [Servicios web](/es/docs/modules/web) para ofrecer un punto de entrada HTTPS.
- Los sitios de redirección de los Servicios web pueden referenciar la dirección pública actual de la regla con variables como `{STUN_NombreRegla_PORT}` y seguir el puerto nuevo automáticamente cuando cambia; consulta [Variables de redirección](/es/docs/modules/web#redirect-vars).

## Limitaciones

Cuando el puerto WAN del router está detrás de un NAT de operador (`100.64.0.0/10`), el puerto traspasado no es accesible desde el exterior. Esta limitación proviene del propio entorno de red. Usa en su lugar [Tailscale, frp o un túnel de Cloudflare](/es/docs/modules/tunnels).

## Consejo de despliegue

STUN necesita escuchar directamente en puertos del host. En despliegues con Docker se recomienda `--network host`.
