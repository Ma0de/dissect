// Package extract: rpa.go 实现 Ren'Py .rpa 档案 (RPA-2.0 / RPA-3.0) 的纯 Go 提取。
// 格式:
//   首行: "RPA-3.0 <index_offset_hex> <key_hex>\n"
//   index_offset 处: (zlib 压缩的) pickle, dict{ filename: [(offset,size), ...] }
//   每条 offset/size 需与 key 异或; RPA-3.0 数据块首字节为 key 低字节，需跳过。
package rpa

import (
	"bufio"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unsafe"
)

// RPAIndex: 归档内文件名 → 数据块列表
type RPAIndex map[string][]RPABlock

type RPABlock struct {
	Offset    int64
	Size      int64
	PrefixLen int // RPA-3.0 第三元素（变长前缀）长度；经典 renpy 为 1，第三方打包器可为 0
}

// OpenRPA 解析 .rpa 头部并读取索引。
func OpenRPA(path string) (*RPAArchive, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	br := bufio.NewReader(f)
	line, err := br.ReadString('\n')
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("rpa header: %w", err)
	}
	header := strings.TrimSpace(line)
	var idxOff int64
	var key uint32
	var version int
	switch {
	case strings.HasPrefix(header, "RPA-3.0"):
		version = 3
		var keyHex string
		if _, err := fmt.Sscanf(header, "RPA-3.0 %x %s", &idxOff, &keyHex); err != nil {
			f.Close()
			return nil, fmt.Errorf("bad RPA-3.0 header: %q", header)
		}
		key = parseHexU32(keyHex)
	case strings.HasPrefix(header, "RPA-2.0"):
		version = 2
		if _, err := fmt.Sscanf(header, "RPA-2.0 %x", &idxOff); err != nil {
			f.Close()
			return nil, fmt.Errorf("bad RPA-2.0 header: %q", header)
		}
	default:
		f.Close()
		return nil, fmt.Errorf("unsupported rpa magic: %q", header)
	}

	raw, err := readIndex(f, idxOff, version == 3)
	if err != nil {
		f.Close()
		return nil, err
	}
	obj, err := parsePickle(raw)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("rpa index pickle: %w", err)
	}
	idx, err := normalizeIndex(obj, key)
	if err != nil {
		f.Close()
		return nil, err
	}
	return &RPAArchive{Path: path, f: f, Index: idx, Version: version, Key: key}, nil
}

type RPAArchive struct {
	Path    string
	f       *os.File
	Index   RPAIndex
	Version int
	Key     uint32
}

func (a *RPAArchive) Close() error { return a.f.Close() }

// Names 返回排序后的文件名列表。
func (a *RPAArchive) Names() []string {
	names := make([]string, 0, len(a.Index))
	for n := range a.Index {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ReadFile 读取一个文件的完整内容（多块按序拼接）。
func (a *RPAArchive) ReadFile(name string) ([]byte, error) {
	blocks, ok := a.Index[name]
	if !ok {
		return nil, fmt.Errorf("%s: no such entry in archive", name)
	}
	var out []byte
	for _, b := range blocks {
		buf := make([]byte, b.Size)
		if _, err := a.f.ReadAt(buf, b.Offset); err != nil {
			return nil, err
		}
		// RPA-3.0: 每条目带变长前缀（经典 renpy 为 1 字节 key 低字节；
		// 第三方打包器可能为 0 字节），前缀不属于文件内容
		if b.PrefixLen > 0 && b.PrefixLen <= len(buf) {
			buf = buf[b.PrefixLen:]
		}
		out = append(out, buf...)
	}
	return out, nil
}

// ExtractAll 解出档案内文件（want 过滤器可为 nil = 全部）。
func (a *RPAArchive) ExtractAll(outDir string, want func(name string) bool) (int, error) {
	n := 0
	var firstErr error
	for _, name := range a.Names() {
		if want != nil && !want(name) {
			continue
		}
		dst := filepath.Join(outDir, filepath.FromSlash(sanitize(name)))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return n, err
		}
		data, err := a.ReadFile(name)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", name, err)
			}
			continue
		}
		if err := os.WriteFile(dst, data, 0o644); err != nil {
			return n, err
		}
		n++
	}
	return n, firstErr
}

// Sanitize 清洗归档内路径（防目录穿越）。
func Sanitize(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	parts := strings.Split(name, "/")
	clean := make([]string, 0, len(parts))
	for _, p := range parts {
		switch p {
		case "", ".":
			continue
		case "..":
			clean = append(clean, "_up_")
		default:
			clean = append(clean, p)
		}
	}
	return strings.Join(clean, "/")
}

func sanitize(name string) string { return Sanitize(name) }

func readIndex(f *os.File, off int64, compressed bool) ([]byte, error) {
	r := io.NewSectionReader(f, off, 1<<62)
	if compressed {
		zr, err := zlib.NewReader(r)
		if err != nil {
			return nil, err
		}
		defer zr.Close()
		return io.ReadAll(zr)
	}
	return io.ReadAll(r)
}

func parseHexU32(s string) uint32 {
	var v uint64
	fmt.Sscanf(strings.ToLower(strings.TrimPrefix(s, "0x")), "%x", &v)
	return uint32(v)
}

func normalizeIndex(obj any, key uint32) (RPAIndex, error) {
	dict, ok := obj.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("index is not a dict (%T)", obj)
	}
	idx := RPAIndex{}
	for name, v := range dict {
		list, ok := v.([]any)
		if !ok {
			if t, ok := v.(pTuple); ok && len(t) >= 1 {
				if l, ok := t[0].([]any); ok {
					list = l
				}
			}
		}
		if list == nil {
			continue
		}
		blocks := make([]RPABlock, 0, len(list))
		for _, item := range list {
			if blk, ok := blockOf(item); ok {
				blk.Offset = int64(uint32(blk.Offset) ^ key)
				blk.Size = int64(uint32(blk.Size) ^ key)
				if blk.Size < 0 || blk.Offset < 0 {
					continue
				}
				blocks = append(blocks, blk)
			}
		}
		if len(blocks) > 0 {
			idx[strings.TrimSpace(name)] = blocks
		}
	}
	if len(idx) == 0 {
		return nil, fmt.Errorf("empty index")
	}
	return idx, nil
}

func blockOf(item any) (RPABlock, bool) {
	get := func(p []any) (RPABlock, bool) {
		if len(p) < 2 {
			return RPABlock{}, false
		}
		o1, k1 := toI64(p[0])
		s1, k2 := toI64(p[1])
		b := RPABlock{Offset: o1, Size: s1}
		// 第三元素 = 变长前缀（bytes/str），缺省视为 1 字节（经典 renpy）
		if len(p) >= 3 {
			if px, ok := p[2].(string); ok {
				b.PrefixLen = len(px)
				if len(p) >= 4 {
					b.PrefixLen = len(px) // 保持简单：前缀即该字符串
				}
			} else {
				b.PrefixLen = 1 // 2 元组以上但无字符串前缀 → 经典格式
			}
		} else {
			b.PrefixLen = 1 // 2 元组 → 经典 renpy 1 字节前缀
		}
		return b, k1 && k2
	}
	switch p := item.(type) {
	case pTuple:
		return get(p)
	case []any:
		return get(p)
	}
	return RPABlock{}, false
}

func toI64(v any) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case int:
		return int64(x), true
	}
	return 0, false
}

// ---------------------------------------------------------------------------
// 最小 pickle 解析器：仅覆盖 RPA 索引所需子集 (dict/list/tuple, str/int/bool/None,
// 协议 0~5 常见操作码)。遇到未知操作码返回错误而不是静默错解。
// ---------------------------------------------------------------------------

// pTuple 区分于 []any，以正确还原索引中的 tuple 结构。
type pTuple []any

type pickleVM struct {
	stack []any
	memo  map[int64]any
}

func (v *pickleVM) push(x any) { v.stack = append(v.stack, x) }
func (v *pickleVM) pop() (any, error) {
	if len(v.stack) == 0 {
		return nil, fmt.Errorf("pickle stack underflow")
	}
	x := v.stack[len(v.stack)-1]
	v.stack = v.stack[:len(v.stack)-1]
	return x, nil
}

func parsePickle(data []byte) (any, error) {
	v := &pickleVM{memo: map[int64]any{}}
	marks := []int{}
	i, n := 0, len(data)
	odd := func(m int) bool { return i+m > n }

	for i < n {
		op := data[i]
		i++
		switch op {
		case 0x80: // PROTO
			i++
		case 0x95: // FRAME
			i += 8
		case 0x28: // MARK '('
			marks = append(marks, len(v.stack))
		case 0x7d: // EMPTY_DICT '}'
			v.push(map[string]any{})
		case 0x5d: // EMPTY_LIST ']'
			v.push([]any{})
		case 0x6c: // LIST 'l'（文本形式，容错）
			m := marks[len(marks)-1]
			marks = marks[:len(marks)-1]
			items := append([]any{}, v.stack[m:]...)
			v.stack = v.stack[:m]
			v.push(items)
		case 0x43: // SHORT_BINBYTES 'C'
			l := int(data[i])
			i++
			v.push(string(data[i : i+l]))
			i += l
		case 0x58: // BINBYTES 'X'
			if odd(4) {
				return nil, fmt.Errorf("pickle truncated")
			}
			l := int(binary.LittleEndian.Uint32(data[i:]))
			i += 4
			v.push(string(data[i : i+l]))
			i += l
		case 0x42: // BINBYTES8? no — 'B' = BINBYTES (proto3)? 容错按 4 字节长度
			if odd(4) {
				return nil, fmt.Errorf("pickle truncated")
			}
			l := int(binary.LittleEndian.Uint32(data[i:]))
			i += 4
			v.push(string(data[i : i+l]))
			i += l
		case 0x47: // BINFLOAT 'G'
			if odd(8) {
				return nil, fmt.Errorf("pickle truncated")
			}
			bits := binary.BigEndian.Uint64(data[i:])
			f := *(*float64)(unsafe.Pointer(&bits))
			v.push(f)
			i += 8
		case 0x54: // TRUE 'T' (proto 0)
			v.push(true)
		case 0x46: // FALSE 'F' (proto 0)
			v.push(false)
		case 0x53: // STRING 'S' (proto 0 带引号)
			j := i
			for j < n && data[j] != '\n' {
				j++
			}
			s := string(data[i:j])
			s = strings.Trim(s, "\"'")
			v.push(s)
			i = j + 1
		case 0x74: // TUPLE 't'（MARK 之后到栈顶合成 tuple）
			m := marks[len(marks)-1]
			marks = marks[:len(marks)-1]
			items := append([]any{}, v.stack[m:]...)
			v.stack = v.stack[:m]
			v.push(pTuple(items))
		case 0x94: // MEMOIZE (协议 4/5): 存入 memo[len(memo)]，与 BINGET 索引对齐
			if top, err := v.peek(); err == nil {
				v.memo[int64(len(v.memo))] = top
			}
		case 0x85: // TUPLE1
			a, err := v.pop()
			if err != nil {
				return nil, err
			}
			v.push(pTuple{a})
		case 0x86: // TUPLE2
			b, err := v.pop()
			if err != nil {
				return nil, err
			}
			a, err := v.pop()
			if err != nil {
				return nil, err
			}
			v.push(pTuple{a, b})
		case 0x87: // TUPLE3
			c, err := v.pop()
			if err != nil {
				return nil, err
			}
			b, err := v.pop()
			if err != nil {
				return nil, err
			}
			a, err := v.pop()
			if err != nil {
				return nil, err
			}
			v.push(pTuple{a, b, c})
		case 0x65: // APPENDS 'e'（MARK..top 依次 append 到列表）
			m := marks[len(marks)-1]
			marks = marks[:len(marks)-1]
			items := append([]any{}, v.stack[m:]...)
			v.stack = v.stack[:m]
			lst, ok := v.stack[len(v.stack)-1].([]any)
			if !ok {
				return nil, fmt.Errorf("pickle APPENDS onto %T", v.stack[len(v.stack)-1])
			}
			v.stack[len(v.stack)-1] = append(lst, items...)
		case 0x61: // APPEND 'a'
			x, err := v.pop()
			if err != nil {
				return nil, err
			}
			lst, ok := v.stack[len(v.stack)-1].([]any)
			if !ok {
				return nil, fmt.Errorf("pickle APPEND onto %T", v.stack[len(v.stack)-1])
			}
			v.stack[len(v.stack)-1] = append(lst, x)
		case 0x71: // BINPUT 'q'
			k := int64(data[i])
			i++
			top, _ := v.peek()
			v.memo[k] = top
		case 0x68: // BINGET 'h'
			k := int64(data[i])
			i++
			v.push(v.memo[k])
		case 0x72: // LONG_BINPUT 'r'
			if odd(4) {
				return nil, fmt.Errorf("pickle truncated")
			}
			k := int64(binary.LittleEndian.Uint32(data[i:]))
			i += 4
			top, _ := v.peek()
			v.memo[k] = top
		case 0x6a: // LONG_BINGET 'j'
			if odd(4) {
				return nil, fmt.Errorf("pickle truncated")
			}
			k := int64(binary.LittleEndian.Uint32(data[i:]))
			i += 4
			v.push(v.memo[k])
		case 0x8f: // EMPTY_SET (协议4, 不应出现在 RPA)
			return nil, fmt.Errorf("pickle op 0x8f unsupported")
		case 0x69: // LONG_BINGET 'j'? actually 0x69='i' INST
			return nil, fmt.Errorf("pickle op 'i' (INST) unsupported")
		case 0x4e: // NONE 'N'
			v.push(nil)
		case 0x88: // NEWTRUE
			v.push(true)
		case 0x89: // NEWFALSE
			v.push(false)
		case 0x4b: // BININT1 'K'
			v.push(int64(data[i]))
			i++
		case 0x4a: // BININT2? 'J' = BININT(4)
			if odd(4) {
				return nil, fmt.Errorf("pickle truncated")
			}
			v.push(int64(int32(binary.LittleEndian.Uint32(data[i:]))))
			i += 4
		case 0x4d: // BININT2 'M'
			if odd(2) {
				return nil, fmt.Errorf("pickle truncated")
			}
			v.push(int64(binary.LittleEndian.Uint16(data[i:])))
			i += 2
		case 0x49: // INT 'I' (text)
			j := i
			for j < n && data[j] != '\n' {
				j++
			}
			var x int64
			fmt.Sscanf(string(data[i:j]), "%d", &x)
			v.push(x)
			i = j + 1
		case 0x4c: // LONG 'L' (text)
			j := i
			for j < n && data[j] != '\n' {
				j++
			}
			var x int64
			fmt.Sscanf(string(data[i:j]), "%d", &x)
			v.push(x)
			i = j + 1
		case 0x8a: // LONG1
			l := int(data[i])
			i++
			var x int64
			for k := 0; k < l && k < 8; k++ {
				x |= int64(data[i+k]) << (8 * k)
			}
			i += l
			v.push(x)
		case 0x8b: // LONG4
			if odd(4) {
				return nil, fmt.Errorf("pickle truncated")
			}
			l := int(binary.LittleEndian.Uint32(data[i:]))
			i += 4
			var x int64
			for k := 0; k < l && k < 8; k++ {
				x |= int64(data[i+k]) << (8 * k)
			}
			i += l
			v.push(x)
		case 0x8c: // SHORT_BINUNICODE
			l := int(data[i])
			i++
			v.push(string(data[i : i+l]))
			i += l
		case 0x8d: // BINUNICODE
			if odd(4) {
				return nil, fmt.Errorf("pickle truncated")
			}
			l := int(binary.LittleEndian.Uint32(data[i:]))
			i += 4
			v.push(string(data[i : i+l]))
			i += l
		case 0x8e: // BINUNICODE8
			if odd(8) {
				return nil, fmt.Errorf("pickle truncated")
			}
			l := int(binary.LittleEndian.Uint64(data[i:]))
			i += 8
			v.push(string(data[i : i+l]))
			i += l
		case 0x75: // SETITEMS 'u'（MARK..top 两两为 key/value 写入 dict）
			m := marks[len(marks)-1]
			marks = marks[:len(marks)-1]
			items := append([]any{}, v.stack[m:]...)
			v.stack = v.stack[:m]
			d, ok := v.stack[len(v.stack)-1].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("pickle SETITEMS onto %T", v.stack[len(v.stack)-1])
			}
			for k := 0; k+1 < len(items); k += 2 {
				ks, ok := items[k].(string)
				if !ok {
					return nil, fmt.Errorf("pickle dict key %T not string", items[k])
				}
				d[ks] = items[k+1]
			}
		case 0x73: // SETITEM 's'
			val, err := v.pop()
			if err != nil {
				return nil, err
			}
			key, err := v.pop()
			if err != nil {
				return nil, err
			}
			d, ok := v.stack[len(v.stack)-1].(map[string]any)
			if !ok {
				return nil, fmt.Errorf("pickle SETITEM onto %T", v.stack[len(v.stack)-1])
			}
			ks, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("pickle dict key %T not string", key)
			}
			d[ks] = val
		case 0x2e: // STOP
			return v.pop()
		case 0x63: // GLOBAL 'c'
			// 跳过 module\nname\n 两行（RPA 不应出现，容错）
			for cnt := 0; cnt < 2 && i < n; {
				if data[i] == '\n' {
					cnt++
				}
				i++
			}
			v.push(nil)
		default:
			return nil, fmt.Errorf("pickle op 0x%02x (%c) unsupported", op, rune(op))
		}
	}
	return nil, fmt.Errorf("pickle no STOP")
}

func (v *pickleVM) peek() (any, error) {
	if len(v.stack) == 0 {
		return nil, fmt.Errorf("pickle stack underflow")
	}
	return v.stack[len(v.stack)-1], nil
}
