# STUN Traversal

When your ISP does not provide a fixed public port, STUN traversal uses NAT hole punching to obtain a public mapped address and port, so that services on your LAN can be reached from the internet.

## Capabilities

- TCP / UDP traversal with keep-alive;
- UPnP / NAT-PMP creates port mappings on the router automatically;
- Built-in forwarding sends the public port obtained through traversal directly to a LAN target;
- Webhooks and scripts fire when the public address or port changes, which you can use to update DNS, notify clients, and more.

## Using it with other modules

The public port obtained through STUN usually changes. Combine it with:

- [Dynamic DNS](/en/docs/modules/ddns) or a Webhook / script to sync the new address to your DNS records;
- [Web Services](/en/docs/modules/web) to provide an HTTPS entry point.
- Redirect sites in Web Services can reference the rule's current public address with variables such as `{STUN_RuleName_PORT}` and follow the new port automatically when it changes; see [Redirect variables](/en/docs/modules/web#redirect-vars).

## Limitations

When the router's WAN port sits behind carrier-grade NAT (`100.64.0.0/10`), the traversal port is unreachable from outside. This limitation comes from the network environment itself. Use [Tailscale, frp or a Cloudflare Tunnel](/en/docs/modules/tunnels) instead.

## Deployment tip

STUN needs to listen on host ports directly. For Docker deployments, `--network host` is recommended.
