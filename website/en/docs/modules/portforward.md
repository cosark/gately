# Port Forwarding

Forward a port that Gately listens on to another device or service.

## Capabilities

- TCP and UDP;
- IPv4 and IPv6, with forwarding between v4 and v6;
- Port ranges and multiple ports;
- Multiple targets with failover;
- Connection limits;
- Traffic statistics and runtime logs;
- Sending the PROXY protocol (v1 / v2) to targets, passing the real client address downstream;
- SNI routing: several services share one port such as 443, chosen by the domain in the TLS handshake, without decrypting TLS.

## PROXY protocol

Port forwarding only relays TCP, so by default the downstream sees only Gately's own address. Set "PROXY protocol" to v1 or v2 in a rule, and Gately sends a PROXY header right after connecting to the target. The downstream then gets the real client address for logging, rate limiting and access control.

The target must enable the matching receive option, otherwise the connection is rejected. For example:

- haproxy: `bind 192.168.1.8:4434 accept-proxy`;
- nginx: `listen 4435 ssl proxy_protocol;`;
- Gately Web Services: enable "Accept PROXY protocol" on the listener, and add the source to the trusted PROXY sources.

This option applies to TCP only; UDP forwarding is unaffected.

## SNI routing: sharing port 443

When several services need the same public port (for example a VPN, a web server and a tunnel all on 443), change the rule's mode from "Port forward" to "SNI routing". Gately reads the domain (SNI) from the first TLS message, the ClientHello, and passes the whole connection through to the backend chosen by domain. TLS is not decrypted, so certificates stay on the backends. It does the same job as haproxy's `req_ssl_sni` rules.

A rule in this mode has:

- Routes: each route has a name, one or more domains, target addresses (several addresses are tried in turn, with failover), an optional target port (empty means the listening port) and a PROXY protocol setting (off, v1 or v2). The PROXY setting belongs to the route because some backends require it and others reject it;
- A default target: the rule's own target addresses, target ports and PROXY protocol setting.

Domains can be exact (`vpn.example.com`), a wildcard (`*.example.com`, any depth) or `*` (any connection that carries a domain), and they are case-insensitive. Matching order is exact domain, the longest wildcard, `*`, then the default target. A domain can belong to one route only.

A connection goes to the default target when no route matches, the client sends no SNI (for example it connects by IP address), the data is not TLS, the ClientHello cannot be parsed, or the client sends nothing within 5 seconds (protocols where the server speaks first, such as SSH). Without a default target these connections are closed and counted as "rejected". A client that disconnects before finishing the ClientHello is dropped.

Notes:

- SNI routing is TCP only. UDP traffic such as HTTP/3 (QUIC) is not routed; use a separate rule for it;
- The ClientHello may be split across several TLS records and may be up to 64 KB;
- The rule card shows how many connections each route and the default target have received.

### Example: replacing haproxy

An haproxy setup like this:

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

becomes one Gately rule:

| Setting | Value |
| --- | --- |
| Mode / protocol | SNI routing / TCP |
| Listen address / port | `192.0.2.10` / `443` |
| Route `ocserv` | domain `tz.example.com`, target `127.0.0.1`, port `443`, PROXY v2 |
| Default target | `127.0.0.1`, target port `8443`, PROXY v2 |

Set the listen address to the server's public address so that it does not clash with backends that listen on `127.0.0.1:443`. To serve websites with Gately itself, add a [Web Services](/en/docs/modules/web) listener on `127.0.0.1:8443` with "Accept PROXY protocol" on and `127.0.0.0/8` in the trusted PROXY sources. Stop haproxy before enabling the rule, because only one program can listen on the port.

## Typical scenarios

- Expose NAS service ports through a single entry point;
- Bridge IPv6 and IPv4 traffic;
- Configure a primary and a standby backend for one service;
- Share port 443 between a VPN, a web server and other TLS services.

## Deployment tip

Port forwarding listens on host ports directly. For Docker deployments, `--network host` is recommended.

## Groups and sorting

You can give rules a "Group" (up to 64 characters). The list shows groups as collapsible sections: groups appear in the order they first occur, ungrouped rules come last, and the collapsed state is kept in the browser. The search box at the top of the page filters by name, group, listen port, target or SNI domain.

Click "Sort" to enter sort mode. Use the arrows to move rules up and down or to move whole groups. Check several rules and drag any one of them to move them together; dropping onto rules or the header of another group moves them into that group. The toolbar moves the checked rules into a group in one step, or out of their group when the name is left empty. Order and groups only affect the list, and changing them means forwarding is not restarted and open connections stay up.

Hover over a group header and click the pencil button to rename the group: everything in it moves to the new name and keeps its position and order. Choosing a name that is already in use merges the two groups after a confirmation.
