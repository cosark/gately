# Travessia STUN

Quando a sua operadora não oferece uma porta pública fixa, a travessia STUN usa NAT hole punching para obter um endereço e uma porta públicos mapeados, permitindo que serviços da rede local sejam acessados pela internet.

## Recursos

- Travessia TCP / UDP com keep-alive;
- UPnP / NAT-PMP cria mapeamentos de porta no roteador automaticamente;
- O encaminhamento embutido envia a porta pública obtida pela travessia diretamente para um destino da rede local;
- Webhooks e scripts são disparados quando o endereço ou a porta pública muda, e podem ser usados para atualizar o DNS, avisar clientes e mais.

## Uso combinado com outros módulos

A porta pública obtida via STUN costuma mudar. Combine com:

- [DNS dinâmico](/pt-BR/docs/modules/ddns) ou um Webhook / script para sincronizar o novo endereço com os registros DNS;
- [Serviços Web](/pt-BR/docs/modules/web) para oferecer um ponto de entrada HTTPS.
- Os sites de redirecionamento dos Serviços Web podem referenciar o endereço público atual da regra com variáveis como `{STUN_NomeRegra_PORT}` e seguem a porta nova automaticamente quando ela muda; veja [Variáveis de redirecionamento](/pt-BR/docs/modules/web#redirect-vars).

## Limitações

Quando a porta WAN do roteador está atrás de um CGNAT da operadora (`100.64.0.0/10`), a porta de travessia não é acessível de fora. Essa limitação vem do próprio ambiente de rede. Use [Tailscale, frp ou Cloudflare Tunnel](/pt-BR/docs/modules/tunnels).

## Dica de implantação

O STUN precisa escutar diretamente nas portas do host. Em implantações com Docker, recomendamos `--network host`.
