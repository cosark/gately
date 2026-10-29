# STUN 穿透

当运营商没有提供固定公网端口时，STUN 穿透通过 NAT 打洞获取公网映射地址与端口，让外网可以访问内网服务。

## 能力

- TCP / UDP 穿透与保活；
- UPnP / NAT-PMP 自动在路由器上创建端口映射；
- 内置转发，把穿透得到的公网端口直接转发到内网目标；
- 公网地址或端口变化时触发 Webhook 与脚本，可用于更新 DNS、通知客户端等。

## 搭配使用

STUN 得到的公网端口通常会变化，建议配合：

- [动态域名](/docs/modules/ddns) 或 Webhook / 脚本，把新地址同步到解析；
- [Web 服务](/docs/modules/web) 提供 HTTPS 入口。
- Web 服务的重定向站点可以用 `{STUN_规则名_PORT}` 等变量引用规则当前的公网地址，端口变化后自动跳到新端口，见 [重定向变量](/docs/modules/web#redirect-vars)。

## 限制

路由器 WAN 口位于运营商级 NAT（`100.64.0.0/10`）时，外部无法访问穿透端口，这是网络环境本身的限制。此时可改用 [Tailscale、frp 或 Cloudflare 隧道](/docs/modules/tunnels)。

## 部署提示

STUN 需要直接监听宿主机端口，Docker 部署推荐使用 `--network host`。
