package rpa

import (
	"bytes"
	"compress/zlib"
	"os"
	"path/filepath"
	"testing"
)

// 合成 RPA-3.0：手工构造 pickle (协议2) 索引，验证解析/异或/提取全链路。
func TestOpenRPA3(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "test.rpa")
	idxOffset := 0x20
	// pickle: {"a.txt": [(32, 3)], "dir/b.txt": [(40, 5)]}  key=0
	var pk []byte
	pk = append(pk, 0x80, 0x02) // PROTO 2
	pk = append(pk, '}')        // EMPTY_DICT
	// entry 1: mark + key + list(mark+tuple(32,3)+appends) + setitems
	pk = append(pk, '(')
	pk = append(pk, 0x8c, 5)
	pk = append(pk, []byte("a.txt")...)
	pk = append(pk, ']')                     // EMPTY_LIST
	pk = append(pk, '(')                     // mark (appends)
	pk = append(pk, '(')                     // mark (tuple)
	pk = append(pk, 'M', 0x00, 0x01, 'K', 4) // (256,3)
	pk = append(pk, 't')                     // TUPLE
	pk = append(pk, 'e')                     // APPENDS
	pk = append(pk, 'u')                     // SETITEMS
	// entry 2
	pk = append(pk, '(')
	pk = append(pk, 0x8c, 9)
	pk = append(pk, []byte("dir/b.txt")...)
	pk = append(pk, ']')
	pk = append(pk, '(')
	pk = append(pk, '(')
	pk = append(pk, 'M', 0x04, 0x01, 'K', 5) // (260,5)
	pk = append(pk, 0x8c, 0)                 // 第三元素 b'' 空前缀（第三方打包器风格）
	pk = append(pk, 't', 'e', 'u')
	pk = append(pk, '.')

	f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	hdr := "RPA-3.0 " + "20" + " 0\n"
	for len(hdr) < idxOffset {
		hdr += " "
	}
	f.WriteString(hdr)
	// RPA-3.0 索引是 zlib 压缩的 pickle
	var zwBuf bytes.Buffer
	zw := zlib.NewWriter(&zwBuf)
	_, _ = zw.Write(pk)
	_ = zw.Close()
	f.Write(zwBuf.Bytes())
	// 数据区: a.txt 经典格式(1字节前缀+内容); b.txt 第三方格式(空前缀+内容)
	buf := make([]byte, 512)
	copy(buf[256:], []byte{0, 'a', 'b', 'c'}) // (256, 4) prefix=1 → "abc"
	copy(buf[260:], []byte("hello"))          // (260, 5) prefix=0 → "hello"
	f.Seek(256, 0)
	f.Write(buf[256:265])
	f.Close()

	a, err := OpenRPA(p)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer a.Close()
	names := a.Names()
	if len(names) != 2 {
		t.Fatalf("names = %v", names)
	}
	d1, err := a.ReadFile("a.txt")
	if err != nil || string(d1) != "abc" {
		t.Fatalf("a.txt = %q err=%v", d1, err)
	}
	d2, err := a.ReadFile("dir/b.txt")
	if err != nil || string(d2) != "hello" {
		t.Fatalf("b.txt = %q err=%v", d2, err)
	}
}

func TestSanitize(t *testing.T) {
	cases := map[string]string{
		"../evil.txt":     "_up_/evil.txt",
		"a//b/../c.png":   "a/b/_up_/c.png",
		"":                "",
		"\\win\\path.ogg": "win/path.ogg",
	}
	for in, want := range cases {
		if got := Sanitize(in); got != want {
			t.Errorf("Sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}
