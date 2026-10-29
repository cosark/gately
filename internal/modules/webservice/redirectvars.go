package webservice

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/wangwanjie/gately/internal/stunaddr"
)

// 重定向目标支持的变量，写法与 Lucky 一致：
//
//	{host} {port} {hostAndPort}      访客请求的域名、访问端口、域名加端口
//	{path} {PATH} {args} {Args}      请求路径（带/不带开头的 /）、查询串（带/不带开头的 ?）
//	{ClientIP} {RemoteIP} {time}     客户端 IP、当前时间
//	{domainPartN} {pathPartN}        域名第 N 段（以 . 分隔）、路径第 N 段（以 / 分隔），N 从 1 开始
//	{1}                              站点域名 *.example.com 中 * 匹配到的部分
//	{STUN_规则名_PORT/IP/ADDR}        STUN 穿透规则当前的公网端口、IP、IP:端口
//
// 未知的 {xxx} 原样保留。

// redirectRequest 是渲染变量时用到的请求信息。
type redirectRequest struct {
	r *http.Request
	// wildcard 为 {1} 的值
	wildcard string
}

// pathVars 中的变量会带上原路径或查询串，目标里写了它们时不再按「保留路径」追加。
var pathVars = []string{"{path}", "{PATH}", "{args}", "{Args}", "{pathPart"}

func usesPathVars(tmpl string) bool {
	for _, v := range pathVars {
		if strings.Contains(tmpl, v) {
			return true
		}
	}
	return false
}

// expandVars 把 tmpl 中的 {name} 替换为 lookup 的结果；lookup 返回 ok=false 时保留原样，返回错误时中止。
func expandVars(tmpl string, lookup func(name string) (string, bool, error)) (string, error) {
	if !strings.Contains(tmpl, "{") {
		return tmpl, nil
	}
	var b strings.Builder
	s := tmpl
	for {
		start := strings.IndexByte(s, '{')
		if start < 0 {
			b.WriteString(s)
			break
		}
		end := strings.IndexByte(s[start:], '}')
		if end < 0 {
			b.WriteString(s)
			break
		}
		end += start
		// 取离 } 最近的 { 作为变量起点，例如 {{host}
		if inner := strings.LastIndexByte(s[start:end], '{'); inner > 0 {
			b.WriteString(s[:start+inner])
			s = s[start+inner:]
			continue
		}
		b.WriteString(s[:start])
		v, ok, err := lookup(s[start+1 : end])
		if err != nil {
			return "", err
		}
		if ok {
			b.WriteString(v)
		} else {
			b.WriteString(s[start : end+1])
		}
		s = s[end+1:]
	}
	return b.String(), nil
}

// stunVar 解析 STUN_规则名_PORT 这类变量名；规则名中可以含下划线。
func stunVar(name string) (rule, part string, ok bool) {
	rest, found := strings.CutPrefix(name, "STUN_")
	if !found {
		return "", "", false
	}
	for _, p := range []string{"_PORT", "_IP", "_ADDR"} {
		if r, found := strings.CutSuffix(rest, p); found && r != "" {
			return r, p[1:], true
		}
	}
	return "", "", false
}

// partIndex 解析 domainPartN / pathPartN 末尾的序号（从 1 开始）。
func partIndex(name, prefix string) (int, bool) {
	rest, found := strings.CutPrefix(name, prefix)
	if !found {
		return 0, false
	}
	n, err := strconv.Atoi(rest)
	if err != nil || n < 1 || strconv.Itoa(n) != rest {
		return 0, false
	}
	return n, true
}

// nthPart 返回以 sep 分隔、去掉空段后的第 n 段，不存在时为空。
func nthPart(s, sep string, n int) string {
	var parts []string
	for _, p := range strings.Split(s, sep) {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if n > len(parts) {
		return ""
	}
	return parts[n-1]
}

// requestPort 返回访问端口：优先取 Host 中的端口，没有时取连接的本地端口。
func requestPort(r *http.Request) string {
	if _, p, err := net.SplitHostPort(r.Host); err == nil && p != "" {
		return p
	}
	if a, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr); ok {
		if _, p, err := net.SplitHostPort(a.String()); err == nil {
			return p
		}
	}
	if r.TLS != nil {
		return "443"
	}
	return "80"
}

// timeVar 返回 {time} 的值。格式与 Lucky 一致（2006-01-02 15:04:05），中间的空格写成 %20：
// 原样写入 Location 时地址中含空格，不是合法的 URL，部分客户端会拒绝跳转或截断。
// %20 在路径与查询串中都表示空格，接收方解码后得到原来的时间
func timeVar(t time.Time) string {
	return strings.ReplaceAll(t.Format("2006-01-02 15:04:05"), " ", "%20")
}

// lookup 返回请求变量的值。
func (q redirectRequest) lookup(name string) (string, bool, error) {
	r := q.r
	switch name {
	case "host":
		return hostOnly(r.Host), true, nil
	case "port":
		return requestPort(r), true, nil
	case "hostAndPort":
		return strings.ToLower(r.Host), true, nil
	case "path":
		return r.URL.EscapedPath(), true, nil
	case "PATH":
		return strings.TrimPrefix(r.URL.EscapedPath(), "/"), true, nil
	case "args":
		if r.URL.RawQuery == "" {
			return "", true, nil
		}
		return "?" + r.URL.RawQuery, true, nil
	case "Args":
		return r.URL.RawQuery, true, nil
	case "ClientIP", "RemoteIP":
		// 两者都取 PROXY 协议与可信代理处理后的客户端地址
		if a := clientAddr(r).Addr(); a.IsValid() {
			return a.String(), true, nil
		}
		return "", true, nil
	case "time":
		return timeVar(time.Now()), true, nil
	case "1":
		return q.wildcard, true, nil
	}
	if n, ok := partIndex(name, "domainPart"); ok {
		return nthPart(hostOnly(r.Host), ".", n), true, nil
	}
	if n, ok := partIndex(name, "pathPart"); ok {
		return nthPart(r.URL.EscapedPath(), "/", n), true, nil
	}
	if rule, part, ok := stunVar(name); ok {
		addr, found := stunaddr.Lookup(rule)
		if !found {
			// 规则不存在、未启用或还没有穿透成功
			return "", false, fmt.Errorf("STUN 规则“%s”暂无公网地址", rule)
		}
		switch part {
		case "PORT":
			return strconv.Itoa(int(addr.Port())), true, nil
		case "IP":
			return addr.Addr().String(), true, nil
		}
		return addr.String(), true, nil
	}
	return "", false, nil
}

// sampleVar 为保存时校验提供示例值：替换后的地址应能解析为完整 URL。
func sampleVar(name string) (string, bool, error) {
	switch name {
	case "host":
		return "example.com", true, nil
	case "port":
		return "443", true, nil
	case "hostAndPort":
		return "example.com:443", true, nil
	case "path":
		return "/a", true, nil
	case "PATH":
		return "a", true, nil
	case "args":
		return "?b=1", true, nil
	case "Args":
		return "b=1", true, nil
	case "ClientIP", "RemoteIP":
		return "192.0.2.1", true, nil
	case "time":
		return timeVar(time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC)), true, nil
	case "1":
		return "a", true, nil
	}
	if _, ok := partIndex(name, "domainPart"); ok {
		return "a", true, nil
	}
	if _, ok := partIndex(name, "pathPart"); ok {
		return "a", true, nil
	}
	if _, part, ok := stunVar(name); ok {
		switch part {
		case "PORT":
			return "40000", true, nil
		case "IP":
			return "192.0.2.1", true, nil
		}
		return "192.0.2.1:40000", true, nil
	}
	// 只支持一个通配符，{2} 及以后的序号没有对应的值
	if n, err := strconv.Atoi(name); err == nil && n > 1 {
		return "", false, fmt.Errorf("站点域名只支持一个通配符，不能使用 {%d}", n)
	}
	return "", false, nil
}

// wildcardPart 返回访客域名在站点域名规则中被 * 匹配到的部分：
// 命中 *.example.com 时为前面的子域名，命中 * 或默认站点时为完整域名，精确匹配时为空。
func wildcardPart(domains []string, host string) string {
	best, bestLen := "", -1
	for _, d := range domains {
		if d == host {
			return ""
		}
		if strings.HasPrefix(d, "*.") && strings.HasSuffix(host, d[1:]) && len(d) > bestLen {
			best, bestLen = strings.TrimSuffix(host, d[1:]), len(d)
		}
	}
	if bestLen >= 0 {
		return best
	}
	return host
}

// redirectTarget 计算重定向站点的跳转地址。
func redirectTarget(s *Site, r *http.Request) (string, error) {
	q := redirectRequest{r: r, wildcard: wildcardPart(s.Domains, hostOnly(r.Host))}
	to, err := expandVars(s.RedirectTo, q.lookup)
	if err != nil {
		return "", err
	}
	if s.KeepPath && !usesPathVars(s.RedirectTo) {
		to = strings.TrimRight(to, "/") + r.URL.RequestURI()
	}
	return to, nil
}
