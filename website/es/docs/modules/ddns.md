# DNS dinámico (DDNS)

La IP pública de una conexión de banda ancha doméstica cambia con el tiempo. El DDNS actualiza automáticamente tus registros DNS tras un cambio de IP, de modo que tu dominio siempre apunte a tu dirección actual.

## Proveedores compatibles

Alibaba Cloud, Cloudflare, Tencent Cloud, DNSPod, Huawei Cloud, GoDaddy, Porkbun, dynv6, DuckDNS, Amazon Route 53, OVHcloud, Gandi, NS1, Bunny DNS, Namecheap, DynDNS2 y una devolución de llamada personalizada (para proveedores que no están integrados o para endpoints autoalojados).

| Proveedor | Credenciales | Válido para la validación DNS de certificados |
| --- | --- | --- |
| Amazon Route 53 | Access Key ID / Secret Access Key, ID de zona alojada opcional | Sí |
| OVHcloud | Application Key / Secret y Consumer Key, endpoint ovh-eu / ovh-ca / ovh-us | Sí |
| Gandi | Personal Access Token | Sí |
| NS1 | API Key | Sí |
| Bunny DNS | Account API Key | Sí |
| Namecheap | Contraseña de Dynamic DNS del dominio (solo registros A) | No |
| DynDNS2 | Usuario, contraseña y URL de actualización; para No-IP, Dynu y otros servicios compatibles con el protocolo dyndns2 | No |

## Cómo se obtiene la IP

- **URL**: solicita una dirección de consulta de IP pública (sustitúyela por el servicio que prefieras o aloja el tuyo);
- **Interfaz de red**: lee directamente la dirección de una interfaz de red concreta, adecuado para dispositivos que tienen IP pública;
- **Comando**: ejecuta un comando y usa su salida, adecuado para configuraciones de red poco habituales.

Se admiten tanto IPv4 como IPv6.

## Notificaciones

Se puede llamar a un webhook cuando cambia la IP o falla una sincronización.

## Recomendaciones

- Crea en tu proveedor DNS una clave de subcuenta limitada a la edición de registros DNS. Evita usar la clave de la cuenta principal;
- Tras crear una tarea, pulsa «Forzar sincronización» para confirmar que el registro se ha actualizado;
- La dirección de consulta de IP pública ya no depende de ningún servicio operado por el proyecto y puedes cambiarla en una tarea en cualquier momento.

## Grupos y orden

Puede asignar a las tareas un «Grupo» (hasta 64 caracteres). La lista muestra los grupos como secciones plegables: los grupos aparecen en el orden en que surgen por primera vez, las tareas sin grupo van al final y el estado plegado se guarda en el navegador. El cuadro de búsqueda de la parte superior filtra por nombre, grupo, dominio o proveedor.

Pulse «Ordenar» para entrar en el modo de orden. Use las flechas para subir o bajar las tareas o mover grupos enteros. Marque varias y arrastre cualquiera de ellas para moverlas juntas; al soltarlas sobre un elemento o el encabezado de otro grupo pasan a ese grupo. La barra de herramientas mueve de una vez los elementos marcados a un grupo, o los saca de su grupo si deja el nombre vacío. El orden y los grupos solo afectan a la lista, y al cambiarlos las tareas no se reinician.

Pase el ratón por el encabezado de un grupo y pulse el lápiz para renombrarlo: todos sus elementos pasan al nuevo nombre y conservan su posición y orden. Si elige un nombre ya usado, tras confirmar se fusionan los dos grupos.
