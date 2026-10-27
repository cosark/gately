# Reenvío de puertos

Reenvía un puerto en el que escucha Gately a otro dispositivo o servicio.

## Capacidades

- TCP y UDP;
- IPv4 e IPv6, con reenvío entre v4 y v6;
- Rangos de puertos y varios puertos;
- Varios destinos con conmutación por error;
- Límites de conexiones;
- Estadísticas de tráfico y registros de ejecución;
- Envío del protocolo PROXY (v1 / v2) al destino, para entregar la dirección real del cliente al siguiente salto;
- Enrutamiento SNI: varios servicios comparten un puerto como el 443, elegidos según el dominio del handshake TLS, sin descifrar TLS.

## Protocolo PROXY

El reenvío de puertos solo retransmite TCP, así que por defecto el servicio de destino solo ve la dirección de Gately. Si en una regla defines «Protocolo PROXY» como v1 o v2, Gately envía primero una cabecera PROXY tras conectar con el destino, y este obtiene la dirección real del cliente para sus registros, límites de peticiones y control de acceso.

El destino debe tener activada la opción de recepción correspondiente; de lo contrario, rechazará la conexión. Por ejemplo:

- haproxy: `bind 192.168.1.8:4434 accept-proxy`;
- nginx: `listen 4435 ssl proxy_protocol;`;
- Servicio web de Gately: activa «Aceptar protocolo PROXY» en el listener y añade el origen a los orígenes PROXY de confianza.

Esta opción solo afecta a TCP; el reenvío UDP no cambia.

## Enrutamiento SNI: compartir el puerto 443

Cuando varios servicios necesitan el mismo puerto público (por ejemplo, una VPN, un servidor web y un túnel en el 443), cambia el modo de la regla de «Reenvío de puertos» a «Enrutamiento SNI». Gately lee el dominio (SNI) del primer mensaje TLS, el ClientHello, y pasa la conexión completa al backend elegido según el dominio. TLS no se descifra, de modo que los certificados permanecen en los backends. Hace el mismo trabajo que las reglas `req_ssl_sni` de haproxy.

Una regla en este modo tiene:

- Rutas: cada ruta tiene un nombre, uno o más dominios, direcciones de destino (si hay varias, se prueban por turno, con conmutación por error), un puerto de destino opcional (vacío significa el puerto de escucha) y un ajuste de protocolo PROXY (desactivado, v1 o v2). El ajuste PROXY pertenece a la ruta porque algunos backends lo exigen y otros lo rechazan;
- Un destino predeterminado: las direcciones de destino, los puertos de destino y el ajuste de protocolo PROXY propios de la regla.

Los dominios pueden ser exactos (`vpn.example.com`), un comodín (`*.example.com`, de cualquier profundidad) o `*` (cualquier conexión que lleve un dominio), y no distinguen mayúsculas de minúsculas. El orden de coincidencia es dominio exacto, el comodín más largo, `*` y, por último, el destino predeterminado. Un dominio solo puede pertenecer a una ruta.

Una conexión va al destino predeterminado cuando ninguna ruta coincide, el cliente no envía SNI (por ejemplo, se conecta por dirección IP), los datos no son TLS, no se puede analizar el ClientHello o el cliente no envía nada en 5 segundos (protocolos en los que habla primero el servidor, como SSH). Sin destino predeterminado, estas conexiones se cierran y se cuentan como «cerradas». Un cliente que se desconecta antes de terminar el ClientHello se descarta.

Notas:

- El enrutamiento SNI es solo TCP. El tráfico UDP, como HTTP/3 (QUIC), no se enruta; usa una regla aparte para él;
- El ClientHello puede estar dividido en varios registros TLS y tener hasta 64 KB;
- La tarjeta de la regla muestra cuántas conexiones ha recibido cada ruta y el destino predeterminado.

### Ejemplo: sustituir haproxy

Una configuración de haproxy como esta:

```text
frontend https
    bind 192.0.2.10:443
    mode tcp
    tcp-request inspect-delay 5s
    tcp-request content accept if { req_ssl_hello_type 1 }
    use_backend ocserv if { req_ssl_sni -i tz.example.com }
    default_backend web
backend ocserv
    server ocserv 127.0.0.1:443 send-proxy-v2
backend web
    server web 127.0.0.1:8443 send-proxy-v2
```

se convierte en una única regla de Gately:

| Ajuste | Valor |
| --- | --- |
| Modo / protocolo | Enrutamiento SNI / TCP |
| Dirección / puerto de escucha | `192.0.2.10` / `443` |
| Ruta `ocserv` | dominio `tz.example.com`, destino `127.0.0.1`, puerto `443`, PROXY v2 |
| Destino predeterminado | `127.0.0.1`, puerto de destino `8443`, PROXY v2 |

Define la dirección de escucha como la dirección pública del servidor para que no choque con los backends que escuchan en `127.0.0.1:443`. Para servir sitios web con el propio Gately, añade un listener de [Servicios web](/es/docs/modules/web) en `127.0.0.1:8443` con «Aceptar protocolo PROXY» activado y `127.0.0.0/8` en los orígenes PROXY de confianza. Detén haproxy antes de activar la regla, porque solo un programa puede escuchar en el puerto.

## Escenarios típicos

- Exponer los puertos de los servicios de un NAS a través de un único punto de entrada;
- Conectar el tráfico IPv6 con IPv4;
- Configurar un backend principal y otro de reserva para un mismo servicio;
- Compartir el puerto 443 entre una VPN, un servidor web y otros servicios TLS.

## Consejo de despliegue

El reenvío de puertos escucha directamente en puertos del host. En despliegues con Docker se recomienda `--network host`.

## Grupos y orden

Puede asignar a las reglas un «Grupo» (hasta 64 caracteres). La lista muestra los grupos como secciones plegables: los grupos aparecen en el orden en que surgen por primera vez, las reglas sin grupo van al final y el estado plegado se guarda en el navegador. El cuadro de búsqueda de la parte superior filtra por nombre, grupo, puerto de escucha, destino o dominio SNI.

Pulse «Ordenar» para entrar en el modo de orden. Use las flechas para subir o bajar las reglas o mover grupos enteros. Marque varias y arrastre cualquiera de ellas para moverlas juntas; al soltarlas sobre un elemento o el encabezado de otro grupo pasan a ese grupo. La barra de herramientas mueve de una vez los elementos marcados a un grupo, o los saca de su grupo si deja el nombre vacío. El orden y los grupos solo afectan a la lista, y al cambiarlos el reenvío no se reinicia y las conexiones abiertas se mantienen.

Pase el ratón por el encabezado de un grupo y pulse el lápiz para renombrarlo: todos sus elementos pasan al nuevo nombre y conservan su posición y orden. Si elige un nombre ya usado, tras confirmar se fusionan los dos grupos.
