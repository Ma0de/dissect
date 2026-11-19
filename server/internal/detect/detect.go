// Package detect 识别游戏目录所用引擎。
// 判据保持宽松、可扩展：每个引擎给出 detect 函数，返回置信度与版本线索。
package detect

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Info struct {
	Engine        string // renpy / unity / rpgmaker / unknown
	EngineVersion string
	GameName      string
	GameID        string
	Details       map[string]string
}

var reRenpyLib = regexp.MustCompile(`lib/(py2|py3)-([a-z0-9_-]+)/librenpython\.(so|dll)`)

var reVerTuple = regexp.MustCompile(`\((\d+),\s*(\d+),\s*(\d+)\)`)

// Detect 在给定目录（已解包的游戏根目录）内识别引擎。
func Detect(root string) Info {
	inf := Info{Engine: "unknown", Details: map[string]string{}}
	// --- Ren'Py: game/ 目录 + lib/*-linux-x86_64/librenpython.so ---
	renpyConfidence := 0
	if dirExists(filepath.Join(root, "game")) {
		renpyConfidence += 1
		inf.GameName = filepath.Base(root)
	}
	// 递归找 librenpython
	found := ""
	filepath.WalkDir(filepath.Join(root, "lib"), func(p string, d os.DirEntry, err error) error {
		if err == nil && d != nil && !d.IsDir() && strings.HasPrefix(d.Name(), "librenpython") {
			found = p
		}
		return nil
	})
	if found != "" {
		renpyConfidence += 2
		// py3-linux-x86_64 → Python 3; 再从 lib 目录名猜位数
		m := reRenpyLib.FindStringSubmatch(found)
		if m != nil {
			inf.Details["python"] = m[1]
			inf.Details["platform"] = m[2]
		}
		if bi := readJSONFile(filepath.Join(root, "game", "cache", "build_info.json")); bi != nil {
			if n, ok := bi["name"].(string); ok {
				inf.GameID = n
				inf.GameName = n
			}
			if v, ok := bi["version"].(string); ok {
				inf.Details["gameVersion"] = v
			}
		}
		if sv := readSmall(filepath.Join(root, "game", "script_version.txt")); sv != "" {
			inf.Details["scriptVersion"] = strings.TrimSpace(sv)
			// "(8, 6, 0)" → 8.6.0
			if m := reVerTuple.FindStringSubmatch(sv); m != nil {
				inf.EngineVersion = m[1] + "." + m[2] + "." + m[3]
			}
		}
	}
	// .rpa 存在也加分
	rpa, _ := filepath.Glob(filepath.Join(root, "game", "*.rpa"))
	if len(rpa) > 0 {
		renpyConfidence += 1
		inf.Details["archives"] = strings.Join(baseNames(rpa), ",")
	}
	if renpyConfidence >= 2 {
		inf.Engine = "renpy"
		if inf.GameID == "" {
			inf.GameID = guessGameID(root)
		}
		return inf
	}

	// --- Unity IL2CPP: GameAssembly.dll + *_Data/il2cpp_data/Metadata/global-metadata.dat ---
	ga := findFirst(root, "GameAssembly.dll", "GameAssembly.so", "il2cpp.arm64.bc", "GameAssembly.dylib")
	if ga != "" {
		inf.Engine = "unity"
		inf.Details["il2cppBinary"] = ga
		md := findFirstDeep(root, "global-metadata.dat")
		if md != "" {
			inf.Details["globalMetadata"] = md
		}
		dataDir := findFirstDirSuffix(root, "_Data")
		if dataDir != "" {
			inf.Details["dataDir"] = dataDir
			inf.GameID = filepath.Base(strings.TrimSuffix(dataDir, "_Data"))
		}
		return inf
	}

	// --- Unity Mono ---
	if dirExists(filepath.Join(root, "*_Data")) || len(globOne(root, "*_Data")) > 0 {
		inf.Engine = "unity"
		inf.Details["variant"] = "mono"
		return inf
	}

	// --- RPG Maker MV/MZ ---
	if dirExists(filepath.Join(root, "www", "js")) || dirExists(filepath.Join(root, "js")) {
		inf.Engine = "rpgmaker"
		inf.Details["variant"] = "mv/mz"
		return inf
	}
	return inf
}

// --- helpers ---

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

func globOne(root, pattern string) []string {
	g, _ := filepath.Glob(filepath.Join(root, pattern))
	return g
}

func baseNames(ps []string) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, filepath.Base(p))
	}
	return out
}

func findFirst(root string, names ...string) string {
	var hit string
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || hit != "" {
			return nil
		}
		if d != nil && !d.IsDir() {
			for _, n := range names {
				if d.Name() == n {
					hit = p
					return filepath.SkipAll
				}
			}
		}
		return nil
	})
	return hit
}

func findFirstDeep(root, name string) string { return findFirst(root, name) }

func findFirstDirSuffix(root, suffix string) string {
	var hit string
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || hit != "" {
			return nil
		}
		if d != nil && d.IsDir() && strings.HasSuffix(d.Name(), suffix) {
			hit = p
			return filepath.SkipAll
		}
		return nil
	})
	return hit
}

func readSmall(p string) string {
	b, err := os.ReadFile(p)
	if err != nil || len(b) > 4096 {
		return ""
	}
	return string(b)
}

func guessGameID(root string) string {
	// lib/py3-xxx/ 里以游戏名命名的启动器可执行文件
	ents, err := os.ReadDir(filepath.Join(root, "lib"))
	if err != nil {
		return ""
	}
	for _, e := range ents {
		if e.IsDir() {
			sub, _ := os.ReadDir(filepath.Join(root, "lib", e.Name()))
			for _, f := range sub {
				n := f.Name()
				if f.Type().IsRegular() && n != "python" && n != "pythonw" &&
					!strings.HasPrefix(n, "lib") && !strings.HasSuffix(n, ".so") {
					return strings.TrimSuffix(n, ".exe")
				}
			}
		}
	}
	return ""
}

// readJSONFile 极简 JSON 对象读取（避免仅为此引入编码依赖）。
func readJSONFile(p string) map[string]any {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	return simpleJSON(string(b))
}

// simpleJSON 只解析 build_info.json 这类平面对象 {"k":"v", ...}
func simpleJSON(s string) map[string]any {
	out := map[string]any{}
	i := 0
	n := len(s)
	skipWS := func() {
		for i < n && (s[i] == ' ' || s[i] == '\n' || s[i] == '\t' || s[i] == '\r') {
			i++
		}
	}
	readStr := func() string {
		if i >= n || s[i] != '"' {
			return ""
		}
		i++
		var sb strings.Builder
		for i < n && s[i] != '"' {
			if s[i] == '\\' && i+1 < n {
				i++
			}
			sb.WriteByte(s[i])
			i++
		}
		i++ // closing quote
		return sb.String()
	}
	skipWS()
	if i >= n || s[i] != '{' {
		return nil
	}
	i++
	for {
		skipWS()
		if i >= n {
			break
		}
		if s[i] == '}' {
			break
		}
		if s[i] == ',' {
			i++
			continue
		}
		k := readStr()
		skipWS()
		if i < n && s[i] == ':' {
			i++
		}
		skipWS()
		if i < n && s[i] == '"' {
			out[k] = readStr()
		} else {
			j := i
			for j < n && s[j] != ',' && s[j] != '}' && s[j] != '\n' {
				j++
			}
			out[k] = strings.TrimSpace(s[i:j])
			i = j
		}
	}
	return out
}
