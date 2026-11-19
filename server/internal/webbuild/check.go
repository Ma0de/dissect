package webbuild

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"dissect/internal/model"
)

type Check struct {
	Supported      bool     `json:"supported"`
	Reason         string   `json:"reason,omitempty"`
	Engine         string   `json:"engine"`
	Version        string   `json:"version"`
	Title          string   `json:"title"`
	Live2D         bool     `json:"live2d"`
	WindowsImports []string `json:"windowsImports"`
}

var windowsModules = map[string]bool{
	"ctypes": true, "subprocess": true, "winreg": true, "_winreg": true,
	"msvcrt": true, "winsound": true, "wmi": true, "comtypes": true,
	"pythoncom": true, "pywintypes": true,
	"win32api": true, "win32con": true, "win32gui": true, "win32com": true,
	"win32event": true, "win32file": true, "win32process": true, "win32clipboard": true,
}

var reImport = regexp.MustCompile(`(?m)^\s*(?:import|from)\s+([A-Za-z_][A-Za-z0-9_.]*)`)

var reVersionLine = regexp.MustCompile(`Version\("main",\s*\((\d+),\s*(\d+),\s*(\d+)\)`)

func Inspect(outDir string, m *model.GameModel) Check {
	c := Check{Engine: m.Engine, Title: m.GameName}
	c.Version = versionOf(m)
	c.WindowsImports = scanImports(outDir)
	c.Live2D = hasLive2D(outDir)

	switch {
	case m.Engine != "renpy":
		c.Reason = "只有 Ren'Py 游戏支持转换 Web 版"
	case !strings.HasPrefix(c.Version, "8."):
		c.Reason = "只支持 Ren'Py 8.x 游戏（当前识别为 " + c.Version + "）"
	case c.Live2D:
		c.Reason = "该游戏使用 Live2D。Ren'Py 的 Web 运行需要官方 Live2D Cubism for Web 组件，暂不支持"
	default:
		c.Supported = true
	}
	return c
}

func versionOf(m *model.GameModel) string {
	v := strings.TrimSpace(m.EngineVersion)
	if v != "" {
		return v
	}
	if raw, ok := m.EngineMeta["detect"].(map[string]any); ok {
		if sv, ok := raw["scriptVersion"].(string); ok {
			nums := regexp.MustCompile(`\d+`).FindAllString(sv, 3)
			if len(nums) == 3 {
				return strings.Join(nums, ".")
			}
		}
	}
	if m.SourceDir != "" {
		if b, err := os.ReadFile(filepath.Join(m.SourceDir, "renpy", "versions.py")); err == nil {
			if mt := reVersionLine.FindStringSubmatch(string(b)); mt != nil {
				return mt[1] + "." + mt[2] + "." + mt[3]
			}
		}
	}
	return ""
}

func hasLive2D(outDir string) bool {
	dir := filepath.Join(outDir, "assets", "live2d")
	found := false
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	if found {
		return true
	}
	scripts := filepath.Join(outDir, "scripts")
	ents, err := os.ReadDir(scripts)
	if err != nil {
		return false
	}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".rpy") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(scripts, e.Name()))
		if err != nil || len(b) > 4<<20 {
			continue
		}
		s := strings.ToLower(string(b))
		if strings.Contains(s, "live2d") || strings.Contains(s, "cubism") {
			return true
		}
	}
	return false
}

func scanImports(outDir string) []string {
	scripts := filepath.Join(outDir, "scripts")
	ents, err := os.ReadDir(scripts)
	if err != nil {
		return []string{}
	}
	found := map[string]bool{}
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".rpy") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(scripts, e.Name()))
		if err != nil {
			continue
		}
		for _, mt := range reImport.FindAllStringSubmatch(string(b), -1) {
			top := strings.SplitN(mt[1], ".", 2)[0]
			if windowsModules[top] {
				found[top] = true
			}
		}
	}
	out := make([]string, 0, len(found))
	for k := range found {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
