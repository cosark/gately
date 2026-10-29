package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// 测试在包目录下运行，相对路径指向 deploy/openwrt/luci
const (
	testPoDir  = "../po"
	testOutDir = "../root/usr/lib/lua/luci/i18n"
	testViewJS = "../root/www/luci-static/resources/view/gately/main.js"
)

// 期望值由 LuCI lib/lmo.c 中的 sfh_hash 编译运行得出；Save / Services / Status 同时核对过 OpenWrt 自带 base.zh-cn.lmo 的索引。
// 覆盖 4 字节整除与余 1～3 字节的分支，以及余数部分含高位字节（按 signed char 运算）的情况
func TestSfhHashMatchesLuCI(t *testing.T) {
	cases := map[string]uint32{
		"Save":     0x219966c7,
		"Services": 0x246f4de5,
		"Status":   0x663359c9,
		"a":        0x115ea782,
		"ab":       0x516b8b44,
		"abc":      0xd2be198a,
		"中":        0xb073298a,
		"é":        0xc4fba554,
		"aé":       0xe3d9c55b,
		"":         0,
	}
	for in, want := range cases {
		if got := sfhHash([]byte(in)); got != want {
			t.Errorf("sfhHash(%q) = %#08x, want %#08x", in, got, want)
		}
	}
}

// lookup 按 LuCI lmo_lookup 的方式在 LMO 中二分查找译文，用于验证生成的文件能被正确读取
func lookup(t *testing.T, data []byte, key string) (string, bool) {
	t.Helper()
	if len(data) < 4 {
		t.Fatalf("LMO 过短: %d 字节", len(data))
	}
	idx := binary.BigEndian.Uint32(data[len(data)-4:])
	n := (uint32(len(data)) - 4 - idx) / 16
	h := sfhHash([]byte(key))
	i := sort.Search(int(n), func(i int) bool {
		return binary.BigEndian.Uint32(data[idx+uint32(i)*16:]) >= h
	})
	if i >= int(n) {
		return "", false
	}
	e := data[idx+uint32(i)*16:]
	if binary.BigEndian.Uint32(e) != h {
		return "", false
	}
	off, l := binary.BigEndian.Uint32(e[8:]), binary.BigEndian.Uint32(e[12:])
	return string(data[off : off+l]), true
}

func TestBuildLMOLookup(t *testing.T) {
	msgs := []message{
		{id: "Start", str: "启动"},
		{id: "Running (PID %d)", str: "运行中（PID %d）"},
		{id: "Same", str: "Same"},
		{id: "abc", str: "a"},
	}
	data := buildLMO(msgs)
	for _, m := range msgs {
		got, ok := lookup(t, data, m.id)
		if m.id == m.str {
			if ok {
				t.Errorf("原文与译文相同的条目不应写入: %q", m.id)
			}
			continue
		}
		if !ok || got != m.str {
			t.Errorf("lookup(%q) = %q, %v; want %q", m.id, got, ok, m.str)
		}
	}
	// 译文按 4 字节对齐，索引起始偏移也应对齐
	if idx := binary.BigEndian.Uint32(data[len(data)-4:]); idx%4 != 0 {
		t.Errorf("索引偏移 %d 未按 4 字节对齐", idx)
	}
	if buildLMO([]message{{id: "x", str: "x"}}) != nil {
		t.Error("没有有效条目时应返回 nil")
	}
}

func TestParsePO(t *testing.T) {
	src := []byte(`# 注释
msgid ""
msgstr ""
"Content-Type: text/plain; charset=UTF-8\n"

msgid "Say \"hi\""
msgstr "说\"你好\""

msgid "Long "
"line"
msgstr "长"
"行\n"

msgid "Untranslated"
msgstr ""
`)
	msgs, err := parsePO(src)
	if err != nil {
		t.Fatal(err)
	}
	want := []message{
		{id: `Say "hi"`, str: `说"你好"`},
		// 与 po2lmo 一致，\n 之类的转义原样保留
		{id: "Long line", str: `长行\n`},
	}
	if len(msgs) != len(want) {
		t.Fatalf("parsePO = %+v, want %+v", msgs, want)
	}
	for i := range want {
		if msgs[i] != want[i] {
			t.Errorf("parsePO[%d] = %+v, want %+v", i, msgs[i], want[i])
		}
	}
	if _, err := parsePO([]byte("msgctxt \"x\"\nmsgid \"a\"\nmsgstr \"b\"\n")); err == nil {
		t.Error("msgctxt 应报错")
	}
	if _, err := parsePO([]byte("msgstr \"b\"\n")); err == nil {
		t.Error("缺少 msgid 应报错")
	}
}

// 已提交的 .lmo 必须与 .po 一致：修改 .po 后忘记执行 go run ./deploy/openwrt/luci/po2lmo 时失败
func TestLMOFilesUpToDate(t *testing.T) {
	files, err := buildAll(testPoDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("没有生成任何 LMO")
	}
	for name, data := range files {
		got, err := os.ReadFile(filepath.Join(testOutDir, name))
		if err != nil {
			t.Errorf("%s: %v（请执行 go run ./deploy/openwrt/luci/po2lmo）", name, err)
			continue
		}
		if !bytes.Equal(got, data) {
			t.Errorf("%s 与 .po 不一致，请执行 go run ./deploy/openwrt/luci/po2lmo", name)
		}
	}
	existing, _ := filepath.Glob(filepath.Join(testOutDir, "*.lmo"))
	for _, p := range existing {
		if _, ok := files[filepath.Base(p)]; !ok {
			t.Errorf("%s 没有对应的 .po，应删除", p)
		}
	}
}

// 页面中每条 _('...') 文案都要出现在模板与各语言 .po 中且有译文（与原文相同的除外，如产品名），.po 中也不能残留页面已不用的条目
func TestTranslationsComplete(t *testing.T) {
	js, err := os.ReadFile(testViewJS)
	if err != nil {
		t.Fatal(err)
	}
	used := map[string]bool{}
	for _, m := range regexp.MustCompile(`_\('((?:[^'\\]|\\.)*)'\)`).FindAllStringSubmatch(string(js), -1) {
		used[strings.ReplaceAll(m[1], `\'`, `'`)] = true
	}
	if len(used) < 10 {
		t.Fatalf("只从页面中找到 %d 条文案，正则可能失效", len(used))
	}
	// 与原文相同、无需翻译的文案
	same := map[string]bool{"Gately": true}

	pot, err := os.ReadFile(filepath.Join(testPoDir, "templates", catalog+".pot"))
	if err != nil {
		t.Fatal(err)
	}
	potIDs := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^msgid "(.+)"$`).FindAllStringSubmatch(string(pot), -1) {
		potIDs[m[1]] = true
	}

	dirs, err := os.ReadDir(testPoDir)
	if err != nil {
		t.Fatal(err)
	}
	langs := 0
	for _, d := range dirs {
		if !d.IsDir() || d.Name() == "templates" {
			continue
		}
		langs++
		src, err := os.ReadFile(filepath.Join(testPoDir, d.Name(), catalog+".po"))
		if err != nil {
			t.Fatal(err)
		}
		msgs, err := parsePO(src)
		if err != nil {
			t.Fatalf("%s: %v", d.Name(), err)
		}
		have := map[string]string{}
		for _, m := range msgs {
			have[m.id] = m.str
			if !used[m.id] {
				t.Errorf("%s: 页面已不再使用 %q", d.Name(), m.id)
			}
			// 格式占位符必须保留
			for _, verb := range []string{"%s", "%d"} {
				if strings.Count(m.id, verb) != strings.Count(m.str, verb) {
					t.Errorf("%s: %q 的译文占位符 %s 数量不一致", d.Name(), m.id, verb)
				}
			}
		}
		for id := range used {
			if same[id] {
				continue
			}
			if have[id] == "" {
				t.Errorf("%s: 缺少 %q 的译文", d.Name(), id)
			}
		}
	}
	if langs < 9 {
		t.Errorf("只有 %d 种语言的 .po", langs)
	}
	for id := range used {
		if !same[id] && !potIDs[id] {
			t.Errorf("模板 gately.pot 缺少 %q", id)
		}
	}
}
