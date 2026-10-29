package stunaddr

import (
	"net/netip"
	"testing"
)

func TestSetLookupDelete(t *testing.T) {
	a := netip.MustParseAddrPort("203.0.113.1:40000")
	b := netip.MustParseAddrPort("203.0.113.2:40001")
	Set("k2", "web", b)
	Set("k1", "web", a)
	Set("k3", "", a)
	defer Delete("k1")
	defer Delete("k2")
	defer Delete("k3")

	// 同名规则取 key 排序最小的一条
	if got, ok := Lookup("web"); !ok || got != a {
		t.Fatalf("同名规则应取 k1: %v %v", got, ok)
	}
	// 没有同名规则时按 key 查找
	if got, ok := Lookup("k3"); !ok || got != a {
		t.Fatalf("应能按 key 查找: %v %v", got, ok)
	}
	if _, ok := Lookup("none"); ok {
		t.Fatal("不存在的规则不应查到地址")
	}

	// 规则改名后按 key 覆盖，旧名字不再残留
	Set("k1", "web2", a)
	if got, _ := Lookup("web"); got != b {
		t.Fatalf("改名后 web 只剩 k2: %v", got)
	}
	Delete("k2")
	if _, ok := Lookup("web"); ok {
		t.Fatal("删除后不应再查到")
	}
	if got, ok := Lookup("web2"); !ok || got != a {
		t.Fatalf("新名字应可查到: %v %v", got, ok)
	}
}
