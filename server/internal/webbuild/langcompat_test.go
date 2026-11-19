package webbuild

import (
	"os"
	"path/filepath"
	"testing"
)

func writeScript(t *testing.T, dir, name, body string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNeedsLangCompat(t *testing.T) {
	t.Run("category language", func(t *testing.T) {
		dir := t.TempDir()
		writeScript(t, dir, "scripts/0016_toggle_language.rpy", "init -999 python:\n    def is_japanese_ui():\n        return _preferences.image_language == \"japanese\"\n")
		if !needsLangCompat(dir) {
			t.Fatal("expected compat to be requested")
		}
	})

	t.Run("category language changers", func(t *testing.T) {
		dir := t.TempDir()
		writeScript(t, dir, "scripts/prefs.rpy", "init python:\n    renpy.translation.change_voice_language(\"japanese\")\n")
		if !needsLangCompat(dir) {
			t.Fatal("expected compat to be requested")
		}
	})

	t.Run("main language only", func(t *testing.T) {
		dir := t.TempDir()
		writeScript(t, dir, "scripts/0031_preferences_screen.rpy", "screen preferences():\n    if _preferences.language == \"japanese\":\n        pass\n")
		if needsLangCompat(dir) {
			t.Fatal("main language alone must not request compat")
		}
	})

	t.Run("no scripts", func(t *testing.T) {
		if needsLangCompat(t.TempDir()) {
			t.Fatal("missing scripts directory must not request compat")
		}
	})
}
