package crud

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type groupItem struct {
	Key   string
	On    bool
	Group string
}

// 批量设置分组：去掉首尾空白、顺序不变、不调用 Changed；未知条目、空列表与超长名称被拒绝且不修改任何条目。
func TestCollectionSetGroup(t *testing.T) {
	var mu sync.Mutex
	items := []groupItem{{Key: "a"}, {Key: "b", Group: "旧"}, {Key: "c"}}
	changed := 0
	c := &Collection[groupItem]{
		Mu:       &mu,
		Items:    &items,
		Save:     func() error { return nil },
		Key:      func(it *groupItem) *string { return &it.Key },
		Enabled:  func(it *groupItem) *bool { return &it.On },
		Validate: func(*groupItem) error { return nil },
		Changed:  func(string) { changed++ },
		Group:    func(it *groupItem) *string { return &it.Group },
	}
	mux := http.NewServeMux()
	c.Register(muxRouter{mux}, "/items")
	put := func(body string) int {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/items-group", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		mux.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := put(`{"keys":["c","a"],"group":"  家里  "}`); code != 200 {
		t.Fatalf("批量设置分组应成功，实际 %d", code)
	}
	if items[0].Key != "a" || items[0].Group != "家里" || items[1].Group != "旧" || items[2].Group != "家里" {
		t.Fatalf("分组或顺序不对: %+v", items)
	}
	if changed != 0 {
		t.Fatalf("修改分组不应重启实例，Changed 调用了 %d 次", changed)
	}
	if code := put(`{"keys":["b"],"group":""}`); code != 200 || items[1].Group != "" {
		t.Fatalf("留空应移出分组: %d %+v", code, items)
	}
	if code := put(`{"keys":["a","x"],"group":"新"}`); code != 404 || items[0].Group != "家里" {
		t.Fatalf("包含未知条目时应返回 404 且不修改: %d %+v", code, items)
	}
	if code := put(`{"keys":[],"group":"新"}`); code != 400 {
		t.Fatalf("空列表应返回 400，实际 %d", code)
	}
	if code := put(`{"keys":["a"],"group":"` + strings.Repeat("组", MaxGroupLen+1) + `"}`); code != 400 || items[0].Group != "家里" {
		t.Fatalf("超长分组名应返回 400: %d %+v", code, items)
	}

	// 未提供 Group 时不注册该接口
	c.Group = nil
	mux = http.NewServeMux()
	c.Register(muxRouter{mux}, "/items")
	if code := put(`{"keys":["a"],"group":"新"}`); code == 200 {
		t.Fatalf("未提供 Group 时不应注册批量设置分组接口")
	}
}
