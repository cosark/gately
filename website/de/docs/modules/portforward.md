# Portweiterleitung

Leiten Sie einen Port, auf dem Gately lauscht, an ein anderes Gerät oder einen anderen Dienst weiter.

## Funktionen

- TCP und UDP;
- IPv4 und IPv6, auch mit Weiterleitung zwischen v4 und v6;
- Portbereiche und mehrere Ports;
- Mehrere Ziele mit Failover;
- Verbindungslimits;
- Traffic-Statistiken und Laufzeit-Logs;
- Senden des PROXY-Protokolls (v1 / v2) an das Ziel, damit nachgelagerte Dienste die echte Client-Adresse erhalten;
- SNI-Routing: Mehrere Dienste teilen sich einen Port wie 443, ausgewählt anhand der Domain im TLS-Handshake, ohne TLS zu entschlüsseln.

## PROXY-Protokoll

Die Portweiterleitung leitet TCP nur weiter, daher sieht der nachgelagerte Dienst standardmäßig nur die Adresse von Gately selbst. Setzen Sie in der Regel „PROXY-Protokoll“ auf v1 oder v2, sendet Gately nach dem Verbindungsaufbau zum Ziel zuerst einen PROXY-Header. Der nachgelagerte Dienst erhält so die echte Client-Adresse für Logs, Ratenbegrenzung und Zugriffskontrolle.

Das Ziel muss die passende Empfangsoption aktiviert haben, sonst wird die Verbindung abgelehnt, zum Beispiel:

- haproxy: `bind 192.168.1.8:4434 accept-proxy`;
- nginx: `listen 4435 ssl proxy_protocol;`;
- Gately-Web-Dienst: im Listener „PROXY-Protokoll akzeptieren“ aktivieren und die Quelle zu den vertrauenswürdigen PROXY-Quellen hinzufügen.

Die Option wirkt nur auf TCP; UDP-Weiterleitungen sind nicht betroffen.

## SNI-Routing: Port 443 gemeinsam nutzen

Brauchen mehrere Dienste denselben öffentlichen Port (zum Beispiel ein VPN, ein Webserver und ein Tunnel, alle auf 443), stellen Sie den Modus der Regel von „Portweiterleitung“ auf „SNI-Routing“. Gately liest die Domain (SNI) aus der ersten TLS-Nachricht, dem ClientHello, und reicht die gesamte Verbindung an das nach Domain gewählte Backend durch. TLS wird nicht entschlüsselt, die Zertifikate bleiben also auf den Backends. Das leistet dasselbe wie die `req_ssl_sni`-Regeln von haproxy.

Eine Regel in diesem Modus hat:

- Routen: Jede Route hat einen Namen, eine oder mehrere Domains, Zieladressen (mehrere Adressen werden der Reihe nach probiert, mit Failover), einen optionalen Zielport (leer bedeutet den Listen-Port) und eine PROXY-Protokoll-Einstellung (aus, v1 oder v2). Die PROXY-Einstellung gehört zur Route, weil manche Backends sie verlangen und andere sie ablehnen;
- Ein Standardziel: die eigenen Zieladressen, Zielports und die PROXY-Protokoll-Einstellung der Regel.

Domains können exakt sein (`vpn.example.com`), eine Wildcard (`*.example.com`, beliebige Tiefe) oder `*` (jede Verbindung, die eine Domain mitbringt); Groß- und Kleinschreibung spielt keine Rolle. Die Zuordnungsreihenfolge lautet: exakte Domain, längste Wildcard, `*`, dann das Standardziel. Eine Domain kann nur zu einer Route gehören.

Eine Verbindung geht an das Standardziel, wenn keine Route passt, der Client kein SNI sendet (zum Beispiel weil er sich per IP-Adresse verbindet), die Daten kein TLS sind, das ClientHello sich nicht parsen lässt oder der Client innerhalb von 5 Sekunden nichts sendet (Protokolle, bei denen der Server zuerst spricht, etwa SSH). Ohne Standardziel werden diese Verbindungen geschlossen und als „getrennt“ gezählt. Ein Client, der die Verbindung vor dem Ende des ClientHello abbricht, wird verworfen.

Hinweise:

- SNI-Routing gilt nur für TCP. UDP-Verkehr wie HTTP/3 (QUIC) wird nicht geroutet; verwenden Sie dafür eine eigene Regel;
- Das ClientHello darf auf mehrere TLS-Records verteilt und bis zu 64 KB groß sein;
- Die Regelkarte zeigt, wie viele Verbindungen jede Route und das Standardziel erhalten haben.

### Beispiel: haproxy ersetzen

Ein haproxy-Setup wie dieses:

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

wird zu einer Gately-Regel:

| Einstellung | Wert |
| --- | --- |
| Modus / Protokoll | SNI-Routing / TCP |
| Listen-Adresse / Port | `192.0.2.10` / `443` |
| Route `ocserv` | Domain `tz.example.com`, Ziel `127.0.0.1`, Port `443`, PROXY v2 |
| Standardziel | `127.0.0.1`, Zielport `8443`, PROXY v2 |

Setzen Sie die Listen-Adresse auf die öffentliche Adresse des Servers, damit es nicht zu Konflikten mit Backends kommt, die auf `127.0.0.1:443` lauschen. Um Websites mit Gately selbst auszuliefern, fügen Sie auf `127.0.0.1:8443` einen Listener der [Web-Dienste](/de/docs/modules/web) hinzu, mit aktiviertem „PROXY-Protokoll akzeptieren“ und `127.0.0.0/8` in den vertrauenswürdigen PROXY-Quellen. Beenden Sie haproxy, bevor Sie die Regel aktivieren, denn nur ein Programm kann den Port belegen.

## Typische Szenarien

- Dienstports eines NAS über einen einzigen Einstiegspunkt bereitstellen;
- Datenverkehr zwischen IPv6 und IPv4 überbrücken;
- Für einen Dienst ein primäres und ein Standby-Backend einrichten;
- Port 443 zwischen einem VPN, einem Webserver und weiteren TLS-Diensten teilen.

## Hinweis zur Bereitstellung

Die Portweiterleitung lauscht direkt auf Ports des Hosts. Bei Docker-Bereitstellungen empfehlen wir `--network host`.

## Gruppen und Sortierung

Regeln können eine „Gruppe“ erhalten (höchstens 64 Zeichen). Die Liste zeigt Gruppen als einklappbare Abschnitte: Gruppen erscheinen in der Reihenfolge ihres ersten Auftretens, Regeln ohne Gruppe stehen am Ende, und der eingeklappte Zustand wird im Browser gespeichert. Das Suchfeld oben auf der Seite filtert nach Name, Gruppe, Listen-Port, Ziel oder SNI-Domain.

Mit „Sortieren“ wechseln Sie in den Sortiermodus. Mit den Pfeilen verschieben Sie Regeln nach oben oder unten oder ganze Gruppen. Markieren Sie mehrere und ziehen Sie einen davon, um alle zusammen zu verschieben; auf einem Eintrag oder dem Kopf einer anderen Gruppe abgelegt, wechseln sie in diese Gruppe. Die Werkzeugleiste verschiebt die markierten Einträge in einem Schritt in eine Gruppe oder entfernt sie aus ihrer Gruppe, wenn der Name leer bleibt. Reihenfolge und Gruppen wirken sich nur auf die Liste aus; Änderungen starten die Weiterleitung nicht neu, offene Verbindungen bleiben bestehen.

Fahren Sie über einen Gruppenkopf und klicken Sie auf den Stift, um die Gruppe umzubenennen: Alle Einträge erhalten den neuen Namen und behalten Position und Reihenfolge. Ein bereits verwendeter Name führt die beiden Gruppen nach einer Bestätigung zusammen.
