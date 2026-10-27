# Redirection de ports

Redirigez un port sur lequel Gately écoute vers un autre appareil ou service.

## Fonctions

- TCP et UDP ;
- IPv4 et IPv6, avec redirection entre v4 et v6 ;
- Plages de ports et ports multiples ;
- Cibles multiples avec basculement ;
- Limites de connexions ;
- Statistiques de trafic et journaux d'exécution ;
- Envoi du protocole PROXY (v1 / v2) à la cible, pour transmettre l'adresse réelle du client en aval ;
- Routage SNI : plusieurs services partagent un même port comme le 443, choisis selon le domaine de la poignée de main TLS, sans déchiffrer le TLS.

## Protocole PROXY

La redirection de ports ne fait que relayer le TCP : par défaut, le service en aval ne voit que l'adresse de Gately. Si vous réglez « Protocole PROXY » sur v1 ou v2 dans une règle, Gately envoie d'abord un en-tête PROXY après s'être connecté à la cible ; le service en aval obtient alors l'adresse réelle du client pour ses journaux, sa limitation de débit et son contrôle d'accès.

La cible doit activer l'option de réception correspondante, sinon la connexion sera refusée. Par exemple :

- haproxy : `bind 192.168.1.8:4434 accept-proxy` ;
- nginx : `listen 4435 ssl proxy_protocol;` ;
- Service Web Gately : activez « Accepter le protocole PROXY » sur l'écouteur et ajoutez la source aux sources PROXY de confiance.

Cette option ne concerne que le TCP ; la redirection UDP n'est pas affectée.

## Routage SNI : partager le port 443

Lorsque plusieurs services ont besoin du même port public (par exemple un VPN, un serveur web et un tunnel, tous sur le 443), faites passer le mode de la règle de « Redirection de port » à « Routage SNI ». Gately lit le domaine (SNI) dans le premier message TLS, le ClientHello, et transmet toute la connexion telle quelle au backend choisi selon le domaine. Le TLS n'est pas déchiffré, les certificats restent donc sur les backends. Cela remplit le même rôle que les règles `req_ssl_sni` de haproxy.

Une règle dans ce mode comporte :

- Des routes : chaque route a un nom, un ou plusieurs domaines, des adresses cibles (plusieurs adresses sont essayées tour à tour, avec basculement), un port cible facultatif (vide signifie le port d'écoute) et un réglage du protocole PROXY (désactivé, v1 ou v2). Le réglage PROXY appartient à la route, car certains backends l'exigent et d'autres le rejettent ;
- Une cible par défaut : les adresses cibles, les ports cibles et le réglage du protocole PROXY de la règle elle-même.

Les domaines peuvent être exacts (`vpn.example.com`), un joker (`*.example.com`, à n'importe quelle profondeur) ou `*` (toute connexion qui transporte un domaine), sans distinction de casse. L'ordre de correspondance est : domaine exact, joker le plus long, `*`, puis la cible par défaut. Un domaine ne peut appartenir qu'à une seule route.

Une connexion va vers la cible par défaut lorsqu'aucune route ne correspond, que le client n'envoie pas de SNI (par exemple parce qu'il se connecte par adresse IP), que les données ne sont pas du TLS, que le ClientHello ne peut pas être analysé, ou que le client n'envoie rien pendant 5 secondes (protocoles où le serveur parle en premier, comme SSH). Sans cible par défaut, ces connexions sont fermées et comptées comme « coupées ». Un client qui se déconnecte avant d'avoir terminé le ClientHello est abandonné.

Remarques :

- Le routage SNI ne concerne que le TCP. Le trafic UDP tel que HTTP/3 (QUIC) n'est pas routé ; utilisez une règle distincte pour lui ;
- Le ClientHello peut être réparti sur plusieurs enregistrements TLS et atteindre 64 Ko ;
- La carte de la règle indique combien de connexions chaque route et la cible par défaut ont reçues.

### Exemple : remplacer haproxy

Une configuration haproxy comme celle-ci :

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

devient une seule règle Gately :

| Réglage | Valeur |
| --- | --- |
| Mode / protocole | Routage SNI / TCP |
| Adresse d'écoute / port | `192.0.2.10` / `443` |
| Route `ocserv` | domaine `tz.example.com`, cible `127.0.0.1`, port `443`, PROXY v2 |
| Cible par défaut | `127.0.0.1`, port cible `8443`, PROXY v2 |

Réglez l'adresse d'écoute sur l'adresse publique du serveur afin d'éviter tout conflit avec les backends qui écoutent sur `127.0.0.1:443`. Pour servir des sites avec Gately lui-même, ajoutez un écouteur de [Services Web](/fr/docs/modules/web) sur `127.0.0.1:8443`, avec « Accepter le protocole PROXY » activé et `127.0.0.0/8` dans les sources PROXY de confiance. Arrêtez haproxy avant d'activer la règle, car un seul programme peut écouter sur le port.

## Scénarios typiques

- Exposer les ports des services d'un NAS via un point d'entrée unique ;
- Faire le pont entre le trafic IPv6 et IPv4 ;
- Configurer un backend principal et un backend de secours pour un même service ;
- Partager le port 443 entre un VPN, un serveur web et d'autres services TLS.

## Conseil de déploiement

La redirection de ports écoute directement sur les ports de l'hôte. Pour un déploiement Docker, `--network host` est recommandé.

## Groupes et tri

Vous pouvez attribuer un « Groupe » (64 caractères au plus) à les règles. La liste affiche les groupes sous forme de sections repliables : les groupes apparaissent dans l’ordre de leur première occurrence, les règles sans groupe viennent en dernier, et l’état replié est conservé dans le navigateur. La zone de recherche en haut de la page filtre par nom, groupe, port d’écoute, cible ou domaine SNI.

Cliquez sur « Trier » pour passer en mode tri. Les flèches déplacent les règles vers le haut ou le bas, ou des groupes entiers. Cochez-en plusieurs et faites glisser l’un d’eux pour les déplacer ensemble ; déposés sur un élément ou l’en-tête d’un autre groupe, ils rejoignent ce groupe. La barre d’outils place d’un coup les éléments cochés dans un groupe, ou les retire de leur groupe si le nom est laissé vide. L’ordre et les groupes n’affectent que la liste, et les modifier signifie que le transfert n’est pas redémarré et les connexions ouvertes sont conservées.

Survolez l’en-tête d’un groupe et cliquez sur le crayon pour le renommer : tous ses éléments passent sous le nouveau nom et gardent leur position et leur ordre. Un nom déjà utilisé fusionne les deux groupes après confirmation.
