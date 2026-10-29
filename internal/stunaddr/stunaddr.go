// Package stunaddr 保存各条 STUN 穿透规则当前的公网地址，
// 供 Web 服务的重定向目标等按规则名引用（{STUN_规则名_PORT} 等变量，与 Lucky 写法一致）。
package stunaddr

import (
	"net/netip"
	"sort"
	"sync"
)

type entry struct {
	name string
	addr netip.AddrPort
}

var (
	mu sync.RWMutex
	// addrs 以规则 key 为键：规则改名或重启时按 key 覆盖、删除，不会残留旧名字
	addrs = map[string]entry{}
)

// Set 记录规则当前的公网地址。
func Set(key, name string, addr netip.AddrPort) {
	mu.Lock()
	defer mu.Unlock()
	addrs[key] = entry{name: name, addr: addr}
}

// Delete 移除规则的地址（规则停止或穿透失败且不再重试时调用）。
func Delete(key string) {
	mu.Lock()
	defer mu.Unlock()
	delete(addrs, key)
}

// Lookup 按规则名查找公网地址；没有同名规则时再按 key 查找。
// 多条规则同名时取 key 排序最小的一条，保证结果稳定。
func Lookup(name string) (netip.AddrPort, bool) {
	mu.RLock()
	defer mu.RUnlock()
	var keys []string
	for k, e := range addrs {
		if e.name == name {
			keys = append(keys, k)
		}
	}
	if len(keys) > 0 {
		sort.Strings(keys)
		return addrs[keys[0]].addr, true
	}
	if e, ok := addrs[name]; ok {
		return e.addr, true
	}
	return netip.AddrPort{}, false
}
