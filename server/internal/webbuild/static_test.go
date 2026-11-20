package webbuild

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func makeSite(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "icons"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "renpy.js"), []byte("console.log(1)"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "icons", "icon-72x72.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestExportStaticFolder(t *testing.T) {
	src := filepath.Join(t.TempDir(), "web")
	dest := t.TempDir()
	makeSite(t, src)

	res, err := ExportStatic(ExportOptions{SrcDir: src, Dest: dest, Title: "测试：游戏", Version: "8.6.0"})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dest, "测试：游戏 Web"); res.Path != want {
		t.Fatalf("path = %q, want %q", res.Path, want)
	}
	if res.Files != 4 {
		t.Fatalf("files = %d, want 4", res.Files)
	}
	for _, rel := range []string{"index.html", "renpy.js", "icons/icon-72x72.png", "README.txt"} {
		if _, err := os.Stat(filepath.Join(res.Path, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("%s 缺失：%v", rel, err)
		}
	}
	b, err := os.ReadFile(filepath.Join(res.Path, "README.txt"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, want := range []string{"测试：游戏", "Ren'Py 8.6.0", "application/wasm", "双击 index.html"} {
		if !strings.Contains(text, want) {
			t.Fatalf("README 缺少 %q", want)
		}
	}
	if _, err := ExportStatic(ExportOptions{SrcDir: src, Dest: dest, Title: "测试：游戏"}); err == nil {
		t.Fatal("目标已存在时应报错")
	}
}

func TestExportStaticZip(t *testing.T) {
	src := filepath.Join(t.TempDir(), "web")
	dest := t.TempDir()
	makeSite(t, src)

	res, err := ExportStatic(ExportOptions{SrcDir: src, Dest: dest, Title: "Test Game", Zip: true})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dest, "Test Game Web.zip"); res.Path != want {
		t.Fatalf("path = %q, want %q", res.Path, want)
	}
	r, err := zip.OpenReader(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	got := map[string]bool{}
	for _, f := range r.File {
		got[f.Name] = true
	}
	for _, name := range []string{"README.txt", "index.html", "renpy.js", "icons/icon-72x72.png"} {
		if !got[name] {
			t.Fatalf("zip 缺少 %s（实际 %v）", name, got)
		}
	}
	if len(got) != res.Files {
		t.Fatalf("zip 条目 %d != files %d", len(got), res.Files)
	}
}

func TestExportStaticRejectsMissingSite(t *testing.T) {
	dest := t.TempDir()
	if _, err := ExportStatic(ExportOptions{SrcDir: filepath.Join(dest, "nope"), Dest: dest}); err == nil {
		t.Fatal("没有 Web 版时应报错")
	}
	if _, err := ExportStatic(ExportOptions{SrcDir: dest, Dest: "relative"}); err == nil {
		t.Fatal("相对路径应报错")
	}
}

func TestSafeName(t *testing.T) {
	cases := [][2]string{
		{"  小小的身影，重叠的内心  ", "小小的身影，重叠的内心"},
		{`a/b:c*d?e"f<g>h|i\j`, "a_b_c_d_e_f_g_h_i_j"},
		{"trailing. .", "trailing"},
		{"", ""},
	}
	for _, c := range cases {
		if got := safeName(c[0]); got != c[1] {
			t.Fatalf("safeName(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}
