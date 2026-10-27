package portforward

import (
	"fmt"
	"net"
	"strings"

	"github.com/wangwanjie/gately/internal/crud"
)

// 支持的协议，语义与 Go 网络类型一致：tcp/udp 为双栈，tcp4/udp4 仅 IPv4，tcp6/udp6 仅 IPv6。
var validProtocols = map[string]bool{
	"tcp": true, "tcp4": true, "tcp6": true,
	"udp": true, "udp4": true, "udp6": true,
}

// Rule 是一条端口转发规则。
type Rule struct {
	Key    string `json:"key"`
	Name   string `json:"name"`
	Enable bool   `json:"enable"`
	// Group 为列表中的分组名称，只影响管理界面的展示，留空为未分组
	Group           string   `json:"group"`
	Protocols       []string `json:"protocols"`
	ListenAddress   string   `json:"listenAddress"`
	ListenPorts     string   `json:"listenPorts"`
	TargetAddresses []string `json:"targetAddresses"`
	TargetPorts     string   `json:"targetPorts"`
	// MaxConnections 是该规则每个 TCP 端口的最大并发连接数，0 表示使用全局默认值
	MaxConnections int `json:"maxConnections"`
	// UDPSessionTimeout 是 UDP 会话空闲超时（秒），0 表示使用全局默认值
	UDPSessionTimeout int `json:"udpSessionTimeout"`
	// ProxyProtocol 非空时，TCP 连接目标后先发送 PROXY 协议头（v1 或 v2），把真实客户端地址告诉下游
	// （haproxy accept-proxy、nginx proxy_protocol、Gately Web 服务"接收 PROXY 协议"）。目标必须支持，否则连接会被拒绝。
	ProxyProtocol string `json:"proxyProtocol"`
	// Mode 为空时按端口直接转发；为 "sni" 时读取 TLS ClientHello 中的 SNI，按 SNIRoutes 分流（仅 TCP，TLS 不终止）。
	// SNI 模式下 TargetAddresses/TargetPorts/ProxyProtocol 作为默认目标，未命中路由或客户端未携带 SNI 时使用，可留空表示直接断开。
	Mode      string     `json:"mode"`
	SNIRoutes []SNIRoute `json:"sniRoutes"`
	Remark    string     `json:"remark"`
}

// ModeSNI 是按 SNI 分流的规则模式。
const ModeSNI = "sni"

// SNIRoute 是 SNI 分流规则中的一条路由，作用相当于 haproxy 的一个 backend。
type SNIRoute struct {
	Name string `json:"name"`
	// Domains 为匹配的域名：精确域名、*.example.com 通配（匹配任意层级子域名）或 *（匹配所有携带 SNI 的连接），不区分大小写
	Domains         []string `json:"domains"`
	TargetAddresses []string `json:"targetAddresses"`
	// TargetPort 为目标端口，0 表示与监听端口相同
	TargetPort int `json:"targetPort"`
	// ProxyProtocol 为发给该目标的 PROXY 协议版本（v1/v2），空为不发送
	ProxyProtocol string `json:"proxyProtocol"`
}

// Settings 是模块级配置。
type Settings struct {
	Enable                   bool `json:"enable"`
	DefaultMaxConnections    int  `json:"defaultMaxConnections"`
	DefaultUDPSessionTimeout int  `json:"defaultUDPSessionTimeout"`
	MaxUDPSessions           int  `json:"maxUDPSessions"`
	// DialTimeout 是连接目标的超时（秒）
	DialTimeout int `json:"dialTimeout"`
}

// fileConfig 是 portforward.json 的结构。
type fileConfig struct {
	Settings Settings `json:"settings"`
	Rules    []Rule   `json:"rules"`
}

func defaultSettings() Settings {
	return Settings{
		Enable:                   true,
		DefaultMaxConnections:    256,
		DefaultUDPSessionTimeout: 60,
		MaxUDPSessions:           1024,
		DialTimeout:              5,
	}
}

// normalize 填充缺省值，保证运行时不出现 0 值导致的异常行为。
func (s *Settings) normalize() {
	d := defaultSettings()
	if s.DefaultMaxConnections <= 0 {
		s.DefaultMaxConnections = d.DefaultMaxConnections
	}
	if s.DefaultUDPSessionTimeout <= 0 {
		s.DefaultUDPSessionTimeout = d.DefaultUDPSessionTimeout
	}
	if s.MaxUDPSessions <= 0 {
		s.MaxUDPSessions = d.MaxUDPSessions
	}
	if s.DialTimeout <= 0 {
		s.DialTimeout = d.DialTimeout
	}
}

// Validate 校验规则并清理输入中的空白。
func (r *Rule) Validate() error {
	r.Name = strings.TrimSpace(r.Name)
	if err := crud.NormalizeGroup(&r.Group); err != nil {
		return err
	}
	r.ListenAddress = strings.Trim(strings.TrimSpace(r.ListenAddress), "[]")
	if len(r.Protocols) == 0 {
		return fmt.Errorf("至少选择一种协议")
	}
	seen := map[string]bool{}
	for _, p := range r.Protocols {
		if !validProtocols[p] {
			return fmt.Errorf("不支持的协议 %q", p)
		}
		if seen[p] {
			return fmt.Errorf("协议 %q 重复", p)
		}
		seen[p] = true
	}
	if r.ListenAddress != "" && net.ParseIP(r.ListenAddress) == nil {
		return fmt.Errorf("监听地址 %q 不是有效 IP", r.ListenAddress)
	}
	if _, err := MapPorts(r.ListenPorts, r.TargetPorts); err != nil {
		return err
	}
	r.TargetAddresses = cleanTargets(r.TargetAddresses)
	if err := validateProxyProtocol(r.ProxyProtocol); err != nil {
		return err
	}
	switch r.Mode {
	case "":
		if len(r.TargetAddresses) == 0 {
			return fmt.Errorf("至少填写一个目标地址")
		}
		// 普通转发不使用路由表，清掉避免残留数据误导
		r.SNIRoutes = nil
	case ModeSNI:
		if err := r.validateSNI(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("不支持的转发模式 %q", r.Mode)
	}
	if r.MaxConnections < 0 || r.UDPSessionTimeout < 0 {
		return fmt.Errorf("连接数与超时不能为负数")
	}
	return nil
}

// validateSNI 校验 SNI 分流规则的协议与路由表，并规范化域名。
func (r *Rule) validateSNI() error {
	for _, p := range r.Protocols {
		if !strings.HasPrefix(p, "tcp") {
			return fmt.Errorf("SNI 分流仅支持 TCP")
		}
	}
	if len(r.SNIRoutes) == 0 && len(r.TargetAddresses) == 0 {
		return fmt.Errorf("至少添加一条 SNI 路由或填写默认目标")
	}
	// 同一域名只能属于一条路由，否则命中结果取决于书写顺序，容易配错
	owner := map[string]string{}
	for i := range r.SNIRoutes {
		rt := &r.SNIRoutes[i]
		rt.Name = strings.TrimSpace(rt.Name)
		label := rt.Name
		if label == "" {
			label = fmt.Sprintf("第 %d 条路由", i+1)
		}
		var domains []string
		for _, d := range rt.Domains {
			d = normalizeSNI(d)
			if d == "" {
				continue
			}
			if err := validateSNIDomain(d); err != nil {
				return fmt.Errorf("%s: %v", label, err)
			}
			if prev, ok := owner[d]; ok {
				return fmt.Errorf("域名 %s 同时出现在 %s 与 %s", d, prev, label)
			}
			owner[d] = label
			domains = append(domains, d)
		}
		if len(domains) == 0 {
			return fmt.Errorf("%s: 至少填写一个域名", label)
		}
		rt.Domains = domains
		rt.TargetAddresses = cleanTargets(rt.TargetAddresses)
		if len(rt.TargetAddresses) == 0 {
			return fmt.Errorf("%s: 至少填写一个目标地址", label)
		}
		if rt.TargetPort < 0 || rt.TargetPort > 65535 {
			return fmt.Errorf("%s: 无效端口 %d", label, rt.TargetPort)
		}
		if err := validateProxyProtocol(rt.ProxyProtocol); err != nil {
			return fmt.Errorf("%s: %v", label, err)
		}
	}
	return nil
}

// validateSNIDomain 检查域名写法：精确域名、*.后缀 或单独的 *。
func validateSNIDomain(d string) error {
	if d == "*" {
		return nil
	}
	name := strings.TrimPrefix(d, "*.")
	if name == "" || strings.Contains(name, "*") || strings.ContainsAny(name, " /:") || strings.HasPrefix(name, ".") || strings.Contains(name, "..") {
		return fmt.Errorf("域名 %q 格式无效，支持 example.com、*.example.com 或 *", d)
	}
	return nil
}

// cleanTargets 去掉目标地址两侧的空白与 IPv6 方括号，并丢弃空项。
func cleanTargets(in []string) []string {
	var out []string
	for _, t := range in {
		t = strings.Trim(strings.TrimSpace(t), "[]")
		if t == "" {
			continue
		}
		out = append(out, t)
	}
	return out
}

func validateProxyProtocol(v string) error {
	switch v {
	case "", "v1", "v2":
		return nil
	}
	return fmt.Errorf("不支持的 PROXY 协议版本 %q", v)
}
