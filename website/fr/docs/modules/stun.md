# Traversée STUN

Lorsque votre FAI ne fournit pas de port public fixe, la traversée STUN utilise le percement de NAT pour obtenir une adresse et un port publics mappés, afin que les services de votre réseau local soient accessibles depuis internet.

## Fonctions

- Traversée TCP / UDP avec maintien de session ;
- UPnP / NAT-PMP crée automatiquement les mappages de ports sur le routeur ;
- La redirection intégrée envoie directement le port public obtenu par la traversée vers une cible du réseau local ;
- Des Webhooks et des scripts se déclenchent quand l'adresse ou le port public change, ce qui permet de mettre à jour le DNS, de notifier des clients, etc.

## Utilisation avec d'autres modules

Le port public obtenu via STUN change généralement. Associez-le à :

- [DNS dynamique](/fr/docs/modules/ddns) ou à un Webhook / script pour synchroniser la nouvelle adresse avec vos enregistrements DNS ;
- [Services Web](/fr/docs/modules/web) pour fournir un point d'entrée HTTPS.
- Les sites de redirection des Services Web peuvent référencer l'adresse publique actuelle de la règle avec des variables comme `{STUN_NomRègle_PORT}` et suivent automatiquement le nouveau port quand il change ; voir [Variables de redirection](/fr/docs/modules/web#redirect-vars).

## Limitations

Lorsque le port WAN du routeur se trouve derrière un NAT de niveau opérateur (`100.64.0.0/10`), le port de traversée est inaccessible depuis l'extérieur. Cette limitation provient de l'environnement réseau lui-même. Utilisez plutôt [Tailscale, frp ou un tunnel Cloudflare](/fr/docs/modules/tunnels).

## Conseil de déploiement

STUN doit écouter directement sur les ports de l'hôte. Pour un déploiement Docker, `--network host` est recommandé.
