package webbuild

import (
	"os"
	"path/filepath"
	"testing"

	"dissect/internal/model"
)

func TestInspectLive2D(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "assets", "live2d", "m.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := Inspect(dir, &model.GameModel{Engine: "renpy", EngineVersion: "8.6.0", GameName: "g"})
	if c.Supported {
		t.Fatal("live2d game should not be supported")
	}
	if !c.Live2D {
		t.Fatal("live2d not detected")
	}
}

func TestInspectWindowsImports(t *testing.T) {
	dir := t.TempDir()
	scripts := filepath.Join(dir, "scripts")
	if err := os.MkdirAll(scripts, 0o755); err != nil {
		t.Fatal(err)
	}
	code := "init python:\n    import ctypes\n    from subprocess import Popen\n"
	if err := os.WriteFile(filepath.Join(scripts, "001_x.rpy"), []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	c := Inspect(dir, &model.GameModel{Engine: "renpy", EngineVersion: "8.6.0", GameName: "g"})
	if !c.Supported {
		t.Fatalf("unexpected reason: %s", c.Reason)
	}
	if len(c.WindowsImports) != 2 || c.WindowsImports[0] != "ctypes" || c.WindowsImports[1] != "subprocess" {
		t.Fatalf("imports = %v", c.WindowsImports)
	}
}

func TestInspectVersionFallback(t *testing.T) {
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "renpy"), 0o755); err != nil {
		t.Fatal(err)
	}
	code := `Version("main", (8, 5, 9), "x")`
	if err := os.WriteFile(filepath.Join(src, "renpy", "versions.py"), []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	c := Inspect(t.TempDir(), &model.GameModel{Engine: "renpy", SourceDir: src, GameName: "g"})
	if c.Version != "8.5.9" {
		t.Fatalf("version = %q", c.Version)
	}
}
