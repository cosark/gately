# STUN 穿透

當電信業者沒有提供固定公網連接埠時，STUN 穿透透過 NAT 打洞取得公網對應位址與連接埠，讓外網可以存取內網服務。

## 能力

- TCP / UDP 穿透與保活；
- UPnP / NAT-PMP 自動在路由器上建立連接埠對應；
- 內建轉發，把穿透取得的公網連接埠直接轉發到內網目標；
- 公網位址或連接埠變動時觸發 Webhook 與指令碼，可用於更新 DNS、通知用戶端等。

## 搭配使用

STUN 取得的公網連接埠通常會變動，建議搭配：

- [動態網域](/zh-TW/docs/modules/ddns) 或 Webhook / 指令碼，把新位址同步到解析；
- [Web 服務](/zh-TW/docs/modules/web) 提供 HTTPS 入口。
- Web 服務的重新導向站台可以用 `{STUN_規則名_PORT}` 等變數引用規則目前的公網位址，連接埠變化後自動跳到新連接埠，見 [重新導向變數](/zh-TW/docs/modules/web#redirect-vars)。

## 限制

路由器 WAN 口位於電信級 NAT（`100.64.0.0/10`）時，外部無法存取穿透連接埠，這是網路環境本身的限制。此時可改用 [Tailscale、frp 或 Cloudflare 通道](/zh-TW/docs/modules/tunnels)。

## 部署提示

STUN 需要直接監聽主機的連接埠，Docker 部署建議使用 `--network host`。
