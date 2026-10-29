// po2lmo 把 luci-app-gately 的翻译（po/<语言>/gately.po）编译为 LuCI 使用的 LMO 文件
// （root/usr/lib/lua/luci/i18n/gately.<语言>.lmo）。
//
// LuCI 自带的 po2lmo 是随 SDK 编译的宿主机工具，打包环境中没有，这里按其源码
// （openwrt/luci modules/luci-base/src/po2lmo.c 与 lib/lmo.c）用 Go 重新实现，生成结果逐字节一致。
// 生成的 .lmo 提交到仓库，打包脚本直接复制；修改 .po 后在仓库根目录执行：
//
//	go run ./deploy/openwrt/luci/po2lmo
//
// 测试会核对已提交的 .lmo 与 .po 一致，并检查页面中的每条文案在各语言中都有译文。
package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// 相对仓库根目录的默认路径
const (
	defaultPoDir  = "deploy/openwrt/luci/po"
	defaultOutDir = "deploy/openwrt/luci/root/usr/lib/lua/luci/i18n"
	catalog       = "gately"
)

func main() {
	poDir, outDir := defaultPoDir, defaultOutDir
	if len(os.Args) == 3 {
		poDir, outDir = os.Args[1], os.Args[2]
	} else if len(os.Args) != 1 {
		fmt.Fprintln(os.Stderr, "用法: po2lmo [po 目录 输出目录]")
		os.Exit(2)
	}
	files, err := buildAll(poDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(outDir, name), data, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("已生成", filepath.Join(outDir, name))
	}
}

// buildAll 编译 poDir 下每个语言目录中的 gately.po，返回 文件名 → LMO 内容。templates 目录是 .pot 模板，跳过
func buildAll(poDir string) (map[string][]byte, error) {
	entries, err := os.ReadDir(poDir)
	if err != nil {
		return nil, err
	}
	out := map[string][]byte{}
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "templates" {
			continue
		}
		src, err := os.ReadFile(filepath.Join(poDir, e.Name(), catalog+".po"))
		if err != nil {
			return nil, err
		}
		msgs, err := parsePO(src)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		data := buildLMO(msgs)
		if data == nil {
			continue
		}
		out[catalog+"."+e.Name()+".lmo"] = data
	}
	return out, nil
}

type message struct {
	id, str string
}

// parsePO 解析 .po 中的 msgid / msgstr 对（含多行字符串续行）。本插件不使用 msgctxt 与复数形式，遇到时报错
func parsePO(src []byte) ([]message, error) {
	var (
		msgs     []message
		cur      *string
		id, str  string
		haveID   bool
		haveStr  bool
		lineNo   int
		flushMsg = func() {
			if haveID && haveStr && id != "" && str != "" {
				msgs = append(msgs, message{id: id, str: str})
			}
			id, str, haveID, haveStr, cur = "", "", false, false, nil
		}
	)
	sc := bufio.NewScanner(bytes.NewReader(src))
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
			continue
		case strings.HasPrefix(line, "msgctxt "), strings.HasPrefix(line, "msgid_plural "), strings.HasPrefix(line, "msgstr["):
			return nil, fmt.Errorf("第 %d 行：不支持 msgctxt 与复数形式", lineNo)
		case strings.HasPrefix(line, "msgid "):
			flushMsg()
			haveID = true
			cur = &id
			line = strings.TrimPrefix(line, "msgid ")
		case strings.HasPrefix(line, "msgstr "):
			if !haveID {
				return nil, fmt.Errorf("第 %d 行：msgstr 之前缺少 msgid", lineNo)
			}
			haveStr = true
			cur = &str
			line = strings.TrimPrefix(line, "msgstr ")
		}
		if cur == nil || !strings.HasPrefix(line, `"`) {
			return nil, fmt.Errorf("第 %d 行：无法解析 %q", lineNo, line)
		}
		*cur += extractString(line)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	flushMsg()
	return msgs, nil
}

// extractString 取出引号内的内容，转义规则与 po2lmo.c 的 extract_string 相同：
// 只把 \" 与 \\ 还原为单个字符，其他转义（如 \n）原样保留
func extractString(line string) string {
	var b strings.Builder
	s := line[1:]
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			next := s[i+1]
			if next == '"' || next == '\\' {
				b.WriteByte(next)
			} else {
				b.WriteByte(c)
				b.WriteByte(next)
			}
			i++
			continue
		}
		if c == '"' {
			break
		}
		b.WriteByte(c)
	}
	return b.String()
}

type lmoEntry struct {
	keyID, valID, offset, length uint32
}

// buildLMO 生成 LMO：依次写入各条译文（按 4 字节补零对齐），然后是按 key_id 排序的索引
// （key_id、val_id、偏移、长度，均为大端 uint32），最后 4 字节为索引的起始偏移。没有任何条目时返回 nil（po2lmo 此时不生成文件）
func buildLMO(msgs []message) []byte {
	var (
		buf     bytes.Buffer
		entries []lmoEntry
	)
	for _, m := range msgs {
		keyID := sfhHash([]byte(m.id))
		valID := sfhHash([]byte(m.str))
		// 原文与译文相同时不写入
		if keyID == valID {
			continue
		}
		// val_id 在 po2lmo 中为 plural_num + 1，非复数条目的 plural_num 为 0
		entries = append(entries, lmoEntry{keyID: keyID, valID: 1, offset: uint32(buf.Len()), length: uint32(len(m.str))})
		buf.WriteString(m.str)
		buf.Write(make([]byte, (4-len(m.str)%4)%4))
	}
	if buf.Len() == 0 {
		return nil
	}
	indexOffset := uint32(buf.Len())
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].keyID < entries[j].keyID
	})
	for _, e := range entries {
		for _, v := range []uint32{e.keyID, e.valID, e.offset, e.length} {
			_ = binary.Write(&buf, binary.BigEndian, v)
		}
	}
	_ = binary.Write(&buf, binary.BigEndian, indexOffset)
	return buf.Bytes()
}

// sfhHash 是 LuCI 使用的 Paul Hsieh SuperFastHash，初始值为数据长度。
// 剩余 1～3 字节时按 signed char 参与运算，与 C 实现一致
func sfhHash(data []byte) uint32 {
	n := len(data)
	if n == 0 {
		return 0
	}
	hash := uint32(n)
	get16 := func(p []byte) uint32 {
		return uint32(p[0]) | uint32(p[1])<<8
	}
	rem := n & 3
	for i := n >> 2; i > 0; i-- {
		hash += get16(data)
		tmp := (get16(data[2:]) << 11) ^ hash
		hash = (hash << 16) ^ tmp
		data = data[4:]
		hash += hash >> 11
	}
	switch rem {
	case 3:
		hash += get16(data)
		hash ^= hash << 16
		hash ^= uint32(int32(int8(data[2]))) << 18
		hash += hash >> 11
	case 2:
		hash += get16(data)
		hash ^= hash << 11
		hash += hash >> 17
	case 1:
		hash += uint32(int32(int8(data[0])))
		hash ^= hash << 10
		hash += hash >> 1
	}
	hash ^= hash << 3
	hash += hash >> 5
	hash ^= hash << 4
	hash += hash >> 17
	hash ^= hash << 25
	hash += hash >> 6
	return hash
}
