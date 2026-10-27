# Redirecionamento de portas

Encaminhe uma porta em que o Gately escuta para outro dispositivo ou serviço.

## Recursos

- TCP e UDP;
- IPv4 e IPv6, com encaminhamento entre v4 e v6;
- Intervalos de portas e várias portas;
- Vários destinos com failover;
- Limite de conexões;
- Estatísticas de tráfego e logs de execução;
- Envio do protocolo PROXY (v1 / v2) ao destino, entregando ao serviço seguinte o endereço real do cliente;
- Roteamento SNI: vários serviços compartilham uma porta como a 443, escolhidos pelo domínio do handshake TLS, sem descriptografar o TLS.

## Protocolo PROXY

O redirecionamento de portas apenas repassa o TCP, então por padrão o serviço seguinte só vê o endereço do próprio Gately. Ao definir "Protocolo PROXY" como v1 ou v2 na regra, o Gately envia primeiro um cabeçalho PROXY depois de conectar ao destino, e o serviço seguinte obtém o endereço real do cliente para logs, limitação de taxa e controle de acesso.

O destino precisa ter a opção de recebimento correspondente ativada, caso contrário a conexão é recusada. Por exemplo:

- haproxy: `bind 192.168.1.8:4434 accept-proxy`;
- nginx: `listen 4435 ssl proxy_protocol;`;
- Serviço Web do Gately: ative "Aceitar protocolo PROXY" no listener e adicione a origem às origens PROXY confiáveis.

A opção vale apenas para TCP; os redirecionamentos UDP não são afetados.

## Roteamento SNI: compartilhando a porta 443

Quando vários serviços precisam da mesma porta pública (por exemplo, uma VPN, um servidor web e um túnel, todos na 443), mude o modo da regra de "Redirecionamento de porta" para "Roteamento SNI". O Gately lê o domínio (SNI) da primeira mensagem TLS, o ClientHello, e repassa a conexão inteira ao backend escolhido pelo domínio. O TLS não é descriptografado, então os certificados continuam nos backends. Ele faz o mesmo trabalho das regras `req_ssl_sni` do haproxy.

Uma regra nesse modo tem:

- Rotas: cada rota tem um nome, um ou mais domínios, endereços de destino (vários endereços são tentados em sequência, com failover), uma porta de destino opcional (vazia significa a porta de escuta) e uma configuração do protocolo PROXY (desativado, v1 ou v2). A configuração PROXY pertence à rota porque alguns backends a exigem e outros a recusam;
- Um destino padrão: os endereços de destino, as portas de destino e a configuração do protocolo PROXY da própria regra.

Os domínios podem ser exatos (`vpn.example.com`), um curinga (`*.example.com`, em qualquer profundidade) ou `*` (qualquer conexão que traga um domínio), e não diferenciam maiúsculas de minúsculas. A ordem de correspondência é domínio exato, o curinga mais longo, `*` e, por fim, o destino padrão. Um domínio só pode pertencer a uma rota.

Uma conexão vai para o destino padrão quando nenhuma rota corresponde, o cliente não envia SNI (por exemplo, conecta pelo endereço IP), os dados não são TLS, o ClientHello não pode ser interpretado ou o cliente não envia nada em 5 segundos (protocolos em que o servidor fala primeiro, como o SSH). Sem destino padrão, essas conexões são encerradas e contadas como "encerradas". Um cliente que se desconecta antes de concluir o ClientHello é descartado.

Observações:

- O roteamento SNI é somente TCP. O tráfego UDP, como o HTTP/3 (QUIC), não é roteado; use uma regra separada para ele;
- O ClientHello pode ser dividido em vários registros TLS e ter até 64 KB;
- O cartão da regra mostra quantas conexões cada rota e o destino padrão receberam.

### Exemplo: substituindo o haproxy

Uma configuração de haproxy como esta:

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

vira uma única regra do Gately:

| Configuração | Valor |
| --- | --- |
| Modo / protocolo | Roteamento SNI / TCP |
| Endereço / porta de escuta | `192.0.2.10` / `443` |
| Rota `ocserv` | domínio `tz.example.com`, destino `127.0.0.1`, porta `443`, PROXY v2 |
| Destino padrão | `127.0.0.1`, porta de destino `8443`, PROXY v2 |

Defina o endereço de escuta como o endereço público do servidor para que não haja conflito com backends que escutam em `127.0.0.1:443`. Para servir sites com o próprio Gately, adicione um listener de [Serviços Web](/pt-BR/docs/modules/web) em `127.0.0.1:8443` com "Aceitar protocolo PROXY" ativado e `127.0.0.0/8` nas origens PROXY confiáveis. Pare o haproxy antes de ativar a regra, pois apenas um programa pode escutar na porta.

## Cenários típicos

- Expor as portas de serviço de um NAS por um único ponto de entrada;
- Fazer a ponte entre tráfego IPv6 e IPv4;
- Configurar um backend principal e outro em espera para um serviço;
- Compartilhar a porta 443 entre uma VPN, um servidor web e outros serviços TLS.

## Dica de implantação

O redirecionamento de portas escuta diretamente nas portas do host. Em implantações com Docker, recomendamos `--network host`.

## Grupos e ordenação

Você pode atribuir um "Grupo" (até 64 caracteres) às regras. A lista mostra os grupos como seções recolhíveis: os grupos aparecem na ordem em que surgem pela primeira vez, os itens sem grupo ficam por último e o estado recolhido é guardado no navegador. A caixa de busca no topo da página filtra por nome, grupo, porta de escuta, destino ou domínio SNI.

Clique em "Ordenar" para entrar no modo de ordenação. Use as setas para subir ou descer itens ou mover grupos inteiros. Marque vários e arraste qualquer um deles para movê-los juntos; ao soltá-los sobre um item ou o cabeçalho de outro grupo, eles passam para esse grupo. A barra de ferramentas move de uma vez os itens marcados para um grupo, ou os tira do grupo quando o nome fica vazio. A ordem e os grupos só afetam a lista, e ao alterá-los o encaminhamento não é reiniciado e as conexões abertas continuam.

Passe o mouse sobre o cabeçalho de um grupo e clique no lápis para renomeá-lo: todos os itens passam para o novo nome e mantêm posição e ordem. Escolher um nome já usado junta os dois grupos após uma confirmação.
