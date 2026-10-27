package portforward

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/wangwanjie/gately/internal/crud"
)

// muxRouter 把 http.ServeMux 适配为 module.Router，路径前缀与正式注册一致。
type muxRouter struct{ mux *http.ServeMux }

func (r muxRouter) HandleFunc(p string, h http.HandlerFunc) {
	method, path, _ := strings.Cut(p, " ")
	r.mux.HandleFunc(method+" /api/portforward"+path, h)
}

// 分组名称去掉首尾空白，超过 64 个字符时拒绝保存。
func TestRuleGroupValidate(t *testing.T) {
	rule := Rule{Protocols: []string{"tcp"}, ListenPorts: "8080", TargetAddresses: []string{"127.0.0.1"}, Group: "  家里  "}
	if err := rule.Validate(); err != nil || rule.Group != "家里" {
		t.Fatalf("分组应去掉首尾空白: %q %v", rule.Group, err)
	}
	rule.Group = strings.Repeat("组", crud.MaxGroupLen+1)
	if rule.Validate() == nil {
		t.Fatal("超长分组名应被拒绝")
	}
}

// 批量设置分组只改展示用的分组字段并保存，不重启转发，已建立的连接保持可用。
func TestSetGroupKeepsConnections(t *testing.T) {
	echoPort, closeEcho := tcpEcho(t)
	defer closeEcho()
	listenPort := freePort(t, "tcp")
	m := startedModule(t, Rule{
		Key:             "k1",
		Enable:          true,
		Protocols:       []string{"tcp4"},
		ListenAddress:   "127.0.0.1",
		ListenPorts:     strconv.Itoa(listenPort),
		TargetAddresses: []string{"127.0.0.1"},
		TargetPorts:     strconv.Itoa(echoPort),
	})
	conn, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(listenPort)))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := echoOnce(t, conn, []byte("before")); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	m.Routes(muxRouter{mux})
	req := httptest.NewRequest("PUT", "/api/portforward/rules-group", strings.NewReader(`{"keys":["k1"],"group":"家里"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("批量设置分组失败: %d %s", rec.Code, rec.Body.String())
	}
	m.mu.Lock()
	group := m.cfg.Rules[0].Group
	m.mu.Unlock()
	if group != "家里" {
		t.Fatalf("分组未保存: %q", group)
	}
	if err := echoOnce(t, conn, []byte("after")); err != nil {
		t.Fatalf("设置分组后已有连接被断开: %v", err)
	}
}
