# STUN-Traversal

Vergibt Ihr Provider keinen festen öffentlichen Port, nutzt das STUN-Traversal NAT-Hole-Punching, um eine öffentliche zugeordnete Adresse und einen Port zu erhalten, sodass Dienste in Ihrem LAN aus dem Internet erreichbar sind.

## Funktionen

- TCP-/UDP-Traversal mit Keep-alive;
- UPnP / NAT-PMP legt Portzuordnungen automatisch am Router an;
- Die integrierte Weiterleitung schickt den per Traversal erhaltenen öffentlichen Port direkt an ein Ziel im LAN;
- Webhooks und Skripte werden ausgelöst, wenn sich die öffentliche Adresse oder der Port ändert, etwa um DNS zu aktualisieren, Clients zu benachrichtigen und mehr.

## Zusammenspiel mit anderen Modulen

Der per STUN erhaltene öffentliche Port ändert sich meist. Kombinieren Sie ihn mit:

- [Dynamischem DNS](/de/docs/modules/ddns) oder einem Webhook / Skript, um die neue Adresse in Ihre DNS-Einträge zu übernehmen;
- [Web-Diensten](/de/docs/modules/web), um einen HTTPS-Einstiegspunkt bereitzustellen.
- Weiterleitungs-Sites der Web-Dienste können die aktuelle öffentliche Adresse der Regel mit Variablen wie `{STUN_Regelname_PORT}` referenzieren und folgen bei einer Änderung automatisch dem neuen Port; siehe [Weiterleitungsvariablen](/de/docs/modules/web#redirect-vars).

## Einschränkungen

Liegt der WAN-Port des Routers hinter Carrier-Grade-NAT (`100.64.0.0/10`), ist der Traversal-Port von außen nicht erreichbar. Diese Einschränkung stammt aus der Netzwerkumgebung selbst. Nutzen Sie stattdessen [Tailscale, frp oder einen Cloudflare Tunnel](/de/docs/modules/tunnels).

## Hinweis zur Bereitstellung

STUN muss direkt auf Ports des Hosts lauschen. Bei Docker-Bereitstellungen empfehlen wir `--network host`.
