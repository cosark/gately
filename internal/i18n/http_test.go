package i18n

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// serveJSON 以语言 xx 经 transWriter 返回 body，模拟中间件处理 path 的 GET 响应。
func serveJSON(t *testing.T, path, body string) map[string]any {
	t.Helper()
	rec := httptest.NewRecorder()
	tw := &transWriter{ResponseWriter: rec, lang: "xx", path: path, get: true}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(body))
	})
	h.ServeHTTP(tw, httptest.NewRequest(http.MethodGet, path, nil))
	tw.finish()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("响应不是 JSON: %s", rec.Body.String())
	}
	return out
}

// 配置类 GET 响应只翻译程序生成的说明字段：用户填写的备注、名称、命令即使包含译文表里的词也原样返回，
// 客户端读出后原样保存时不会把用户内容改写成译文。
func TestMiddlewareKeepsUserContent(t *testing.T) {
	testCatalog(map[string]string{"端口转发": "Port forwarding", "监听失败": "listen failed", "未找到程序": "program not found"})
	body := `{"remark":"由端口转发 SNI 分流送达","name":"端口转发","group":"端口转发","text":"端口转发","error":"监听失败",` +
		`"sites":[{"remark":"端口转发","details":["监听失败"],"state":{"lastError":"监听失败","value":"端口转发"}}],` +
		`"frpc":{"error":"未找到程序","path":"端口转发"}}`
	for _, path := range []string{"/api/webservice/listeners", "/api/ddns/tasks", "/api/cron/tasks", "/api/history"} {
		out := serveJSON(t, path, body)
		for _, k := range []string{"remark", "name", "group", "text"} {
			if out[k] != "端口转发" && !strings.HasPrefix(out[k].(string), "由端口转发") {
				t.Errorf("%s 的用户字段 %s 被翻译: %v", path, k, out[k])
			}
		}
		if out["error"] != "listen failed" {
			t.Errorf("%s 的 error 未翻译: %v", path, out["error"])
		}
		site := out["sites"].([]any)[0].(map[string]any)
		state := site["state"].(map[string]any)
		if site["remark"] != "端口转发" || state["value"] != "端口转发" {
			t.Errorf("%s 嵌套的用户字段被翻译: %v", path, site)
		}
		if site["details"].([]any)[0] != "listen failed" || state["lastError"] != "listen failed" {
			t.Errorf("%s 嵌套的说明字段未翻译: %v", path, site)
		}
		frpc := out["frpc"].(map[string]any)
		if frpc["error"] != "program not found" || frpc["path"] != "端口转发" {
			t.Errorf("%s 的 frpc 字段处理不对: %v", path, frpc)
		}
	}
	// 完全由程序生成的接口（状态、服务商说明等）全部翻译
	out := serveJSON(t, "/api/status", body)
	if out["remark"] != "由Port forwarding SNI 分流送达" || out["name"] != "Port forwarding" {
		t.Errorf("状态接口应全部翻译: %v", out)
	}
	if out = serveJSON(t, "/api/ddns/providers", `{"title":"端口转发","placeholder":"端口转发"}`); out["placeholder"] != "Port forwarding" {
		t.Errorf("服务商说明应全部翻译: %v", out)
	}
}

// Localize 与 GET 响应一致：只翻译说明字段与指定的额外字段，名称、备注等原样保留。
func TestLocalizeKeepsUserContent(t *testing.T) {
	testCatalog(map[string]string{"端口转发": "Port forwarding", "该监听已有默认站点": "the listener already has a default site"})
	req := httptest.NewRequest(http.MethodPost, "/api/x", nil)
	req.Header.Set("Accept-Language", "en")
	v := map[string]any{"name": "端口转发", "remark": "端口转发", "skipped": "该监听已有默认站点", "warnings": []any{"端口转发"}}
	// 请求语言 en 走真实译文表，这里直接用 xx 验证字段选择
	out := TrFields("xx", v, "skipped").(map[string]any)
	if out["name"] != "端口转发" || out["remark"] != "端口转发" {
		t.Errorf("用户字段被翻译: %v", out)
	}
	if out["skipped"] != "the listener already has a default site" || out["warnings"].([]any)[0] != "Port forwarding" {
		t.Errorf("说明字段未翻译: %v", out)
	}
	if got := Localize(req, map[string]any{"name": "端口转发"}).(map[string]any); got["name"] != "端口转发" {
		t.Errorf("Localize 不应翻译名称: %v", got)
	}
}

// mapping、unsupported 为字符串时是程序生成的说明（STUN 端口映射结果、托管程序不支持当前平台），要翻译；
// 证书的 mapping 是用户填写的目录与脚本对象，保持原样。
func TestMiddlewareStringMessageKeys(t *testing.T) {
	testCatalog(map[string]string{"端口转发": "Port forwarding", "失败": "failed", "映射失败": "mapping failed"})
	out := serveJSON(t, "/api/stun/rules", `{"state":{"mapping":"UPnP 映射失败"},"unsupported":"失败",`+
		`"cert":{"mapping":{"dir":"端口转发","script":"echo 失败"}}}`)
	if got := out["state"].(map[string]any)["mapping"]; got != "UPnP mapping failed" {
		t.Errorf("STUN 映射说明未翻译: %v", got)
	}
	if out["unsupported"] != "failed" {
		t.Errorf("unsupported 未翻译: %v", out["unsupported"])
	}
	m := out["cert"].(map[string]any)["mapping"].(map[string]any)
	if m["dir"] != "端口转发" || m["script"] != "echo 失败" {
		t.Errorf("证书目录映射配置被翻译: %v", m)
	}
}
