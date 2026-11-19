// Package unity —— Unity IL2CPP 游戏模块。
//
// 设计说明（重要）：
// IL2CPP 的字段类型与继承层级存放在 GameAssembly 二进制的 types 注册表中，
// 纯解析 global-metadata.dat 无法还原完整类型信息。业界标准工具 Il2CppDumper
// (Perfare, MIT) 已完美处理全部元数据版本 + 二进制扫描，因此本模块不重造轮子：
//  1. 调用外部 Il2CppDumper（--il2cppdumper 指定路径）产出 dump.cs / il2cpp.h / script.json
//  2. 解析 dump.cs → dissect 通用模型 (字段表 / 角色无关)
//  3. il2cpp.h / dump.cs 原样收集为头文件交付物（供修改器开发）
//
// 可选：若 python + UnityPy 可用，调用 vendor/unitypy_worker.py 提取图片/音频等资产。
package unity

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"dissect/internal/model"
)

type Options struct {
	GameRoot      string // 游戏根目录（含 GameAssembly.dll 与 *_Data/）
	OutDir        string
	Il2CppDumper  string // Il2CppDumper 可执行文件路径（CLI 版，net6/win 或 net45）
	Python        string // 可选
	UnityPyWorker string // 可选 vendor/unitypy_worker.py 路径
	Progress      func(string, float64)
	Log           func(level, msg string)
	Ctx           context.Context
}

func report(opt *Options, s string, p float64) {
	if opt.Progress != nil {
		opt.Progress(s, p)
	}
}

func logf(opt *Options, level, format string, args ...any) {
	if opt.Log != nil {
		opt.Log(level, fmt.Sprintf(format, args...))
	}
}

func checkCancel(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// Run 执行 Unity IL2CPP 解包管线。
func Run(opt Options) (*model.GameModel, error) {
	if err := checkCancel(opt.Ctx); err != nil {
		return nil, err
	}
	m := &model.GameModel{
		Engine:     "unity",
		SourceDir:  opt.GameRoot,
		EngineMeta: map[string]any{},
		Notes:      []string{},
	}
	if err := os.MkdirAll(opt.OutDir, 0o755); err != nil {
		return nil, err
	}
	exportsDir := filepath.Join(opt.OutDir, "exports")
	if err := os.MkdirAll(exportsDir, 0o755); err != nil {
		return nil, err
	}

	// 定位关键文件
	bin, meta := findIl2Cpp(opt.GameRoot)
	if bin == "" {
		return nil, fmt.Errorf("未找到 GameAssembly（.dll/.so），目录: %s", opt.GameRoot)
	}
	m.EngineMeta["il2cppBinary"] = bin
	if meta == "" {
		return nil, fmt.Errorf("未找到 global-metadata.dat（应在 *_Data/il2cpp_data/Metadata/）")
	}
	m.EngineMeta["globalMetadata"] = meta
	logf(&opt, "info", "定位 IL2CPP 文件：%s", filepath.Base(bin))

	// ---- 1) 调用 Il2CppDumper ----
	dumpDir := filepath.Join(opt.OutDir, "il2cpp_dump")
	if err := os.MkdirAll(dumpDir, 0o755); err != nil {
		return nil, err
	}
	report(&opt, "il2cppdumper", 0.1)
	if opt.Il2CppDumper != "" {
		logf(&opt, "info", "运行 Il2CppDumper：%s", filepath.Base(opt.Il2CppDumper))
		out, err := runDumper(opt.Ctx, opt.Il2CppDumper, bin, meta, dumpDir)
		if err != nil {
			m.Notes = append(m.Notes, fmt.Sprintf("Il2CppDumper 失败: %v\n%s", err, tail(string(out), 600)))
			logf(&opt, "error", "Il2CppDumper 失败: %v", err)
		} else {
			m.Notes = append(m.Notes, "Il2CppDumper 运行成功")
			logf(&opt, "success", "Il2CppDumper 运行成功")
		}
	} else {
		m.Notes = append(m.Notes,
			"未指定 --il2cppdumper：跳过 IL2CPP 转储。请下载 https://github.com/Perfare/Il2CppDumper/releases 并用 --il2cppdumper 指定可执行文件")
		logf(&opt, "warn", "未指定 Il2CppDumper，跳过 IL2CPP 转储")
	}

	// ---- 2) 解析 dump.cs → 模型 ----
	if err := checkCancel(opt.Ctx); err != nil {
		return nil, err
	}
	report(&opt, "parse dump.cs", 0.6)
	dumpPath := filepath.Join(dumpDir, "dump.cs")
	if b, err := os.ReadFile(dumpPath); err == nil {
		parseDumpCS(string(b), m)
		// 收集头文件
		addHeader(m, "il2cpp.h", filepath.Join(dumpDir, "il2cpp.h"), exportsDir)
		addHeader(m, "dump.cs", dumpPath, exportsDir)
		addHeader(m, "script.json", filepath.Join(dumpDir, "script.json"), exportsDir)
		m.Notes = append(m.Notes, fmt.Sprintf("dump.cs 解析: %d 个类型 / %d 个字段",
			m.Stats.Labels+m.Stats.Screens, len(m.Fields)))
		logf(&opt, "info", "dump.cs 解析：%d 个类型 / %d 个字段",
			m.Stats.Labels+m.Stats.Screens, len(m.Fields))
	} else {
		m.Notes = append(m.Notes, "dump.cs 不存在（Il2CppDumper 未运行成功），仅保留检测信息")
		logf(&opt, "warn", "dump.cs 不存在，仅保留检测信息")
	}

	// ---- 3) 可选 UnityPy 资产提取 ----
	if err := checkCancel(opt.Ctx); err != nil {
		return nil, err
	}
	report(&opt, "assets", 0.85)
	runUnityPy(opt, m)
	logf(&opt, "info", "资产文件：%d 个", len(m.Assets))

	// ---- 4) 导出 ----
	if err := checkCancel(opt.Ctx); err != nil {
		return nil, err
	}
	report(&opt, "export", 0.95)
	writeFields(m, exportsDir)
	m.Normalize()
	b, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(opt.OutDir, "model.json"), b, 0o644); err != nil {
		return nil, err
	}
	report(&opt, "done", 1.0)
	logf(&opt, "success", "模型已写入 model.json")
	return m, nil
}

func findIl2Cpp(root string) (string, string) {
	bin := ""
	meta := ""
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d == nil {
			return nil
		}
		n := d.Name()
		if !d.IsDir() {
			switch {
			case n == "GameAssembly.dll", n == "GameAssembly.so", strings.HasPrefix(n, "GameAssembly"):
				if bin == "" {
					bin = p
				}
			case n == "global-metadata.dat":
				if meta == "" {
					meta = p
				}
			}
		}
		return nil
	})
	return bin, meta
}

func runDumper(ctx context.Context, exe, bin, meta, outDir string) ([]byte, error) {
	if _, err := os.Stat(exe); err != nil {
		return nil, fmt.Errorf("Il2CppDumper 路径无效: %s", exe)
	}
	abs, _ := filepath.Abs(outDir)
	// Il2CppDumper CLI: Il2CppDumper <il2cpp bundle> <metadata> <output dir>
	// 交互确认仅在参数缺失时出现；给足参数则直接运行
	var cmd *exec.Cmd
	if ctx != nil {
		cmd = exec.CommandContext(ctx, exe, bin, meta, abs)
	} else {
		cmd = exec.Command(exe, bin, meta, abs)
	}
	cmd.Dir = filepath.Dir(exe)
	return cmd.CombinedOutput()
}

func runUnityPy(opt Options, m *model.GameModel) {
	py := opt.Python
	if py == "" {
		for _, c := range []string{"python3", "python"} {
			if p, err := exec.LookPath(c); err == nil {
				py = p
				break
			}
		}
	}
	if py == "" || opt.UnityPyWorker == "" {
		m.Notes = append(m.Notes, "（可选）UnityPy 资产提取未启用：需要 python + pip install UnityPy")
		return
	}
	assetsDir := filepath.Join(opt.OutDir, "assets")
	_ = os.MkdirAll(assetsDir, 0o755)
	var cmd *exec.Cmd
	if opt.Ctx != nil {
		cmd = exec.CommandContext(opt.Ctx, py, opt.UnityPyWorker, opt.GameRoot, assetsDir)
	} else {
		cmd = exec.Command(py, opt.UnityPyWorker, opt.GameRoot, assetsDir)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		m.Notes = append(m.Notes, fmt.Sprintf("UnityPy 提取失败: %v\n%s", err, tail(string(out), 400)))
		return
	}
	m.Notes = append(m.Notes, "UnityPy 资产提取完成: "+tail(strings.TrimSpace(string(out)), 200))
}

func addHeader(m *model.GameModel, name, src, exportsDir string) {
	b, err := os.ReadFile(src)
	if err != nil {
		return
	}
	dst := filepath.Join(exportsDir, name)
	_ = os.WriteFile(dst, b, 0o644)
	note := map[string]string{
		"il2cpp.h":    "Il2CppDumper 生成的 C++ 结构体头文件（字段偏移注释），供修改器/内存工具使用",
		"dump.cs":     "IL2CPP 转储的 C# 伪源码（全部类型/字段/方法与偏移）",
		"script.json": "Il2CppDumper 脚本索引（方法地址表，可用于 Frida/Ghidra）",
	}[name]
	m.Headers = append(m.Headers, model.HeaderFile{
		Name: name, Engine: "unity", Note: note, Path: "exports/" + name,
	})
}

// ---- dump.cs 解析 ----

var (
	reClass     = regexp.MustCompile(`^\s*(?:public|private|internal|protected|static|\s)*class\s+(\w+)(?:<[^>]*>)?\s*(?::\s*([^\r\n{]+))?\s*(?:\/\/.*)?$`)
	reStruct    = regexp.MustCompile(`^\s*(?:public|private|internal|protected|static|\s)*struct\s+(\w+)`)
	reEnum      = regexp.MustCompile(`^\s*(?:public|private|internal|protected|static|\s)*enum\s+(\w+)`)
	reNamespace = regexp.MustCompile(`^(?:public|private)?\s*namespace\s+([\w.]+)`)
	reField     = regexp.MustCompile(`^\s*(?:public|private|protected|internal|static|readonly|new|extern|volatile|const|\s)*([\w.<>\[\],\s]+?)\s+(\w+)\s*(?:=\s*[^;]+)?;\s*(?:\/\/\s*(.*))?$`)
	reMethod    = regexp.MustCompile(`^\s*(?:public|private|protected|internal|static|virtual|override|abstract|new|async|extern|unsafe|\s)*[\w.<>\[\],]+\s+(\w+)\s*\(([^)]*)\)\s*(?:\{|\s*;)`)
)

// parseDumpCS 把 dump.cs 行解析进通用模型（字段聚焦，方法计数）。
func parseDumpCS(src string, m *model.GameModel) {
	ns := ""
	class := ""
	base := ""
	inClass := false
	line := 0
	for _, raw := range strings.Split(src, "\n") {
		line++
		l := strings.TrimSpace(raw)
		if l == "" {
			continue
		}
		if mm := reNamespace.FindStringSubmatch(l); mm != nil {
			ns = mm[1]
			class = ""
			inClass = false
			continue
		}
		if mm := reClass.FindStringSubmatch(l); mm != nil && !reMethod.MatchString(l) {
			class = mm[1]
			base = strings.TrimSpace(mm[2])
			inClass = true
			m.Stats.Labels++
			continue
		}
		if reStruct.MatchString(l) || reEnum.MatchString(l) {
			m.Stats.Screens++
			inClass = true
			continue
		}
		if l == "}" {
			if inClass {
				class = ""
				base = ""
				inClass = false
			}
			continue
		}
		if !inClass || class == "" {
			continue
		}
		if mm := reField.FindStringSubmatch(l); mm != nil {
			typ := strings.TrimSpace(mm[1])
			name := mm[2]
			off := strings.TrimSpace(mm[3])
			if strings.Contains(typ, "(") { // 方法/构造函数误匹配
				continue
			}
			scope := "class:" + class
			if ns != "" {
				scope = "ns:" + ns + "." + class
			}
			f := model.Field{
				Name: name, Scope: scope, ClassOf: class, Type: typ,
				Keyword: "field", File: "dump.cs", Line: line,
			}
			if off != "" && strings.HasPrefix(off, "0x") {
				f.Default = off
			}
			if base != "" {
				f.ClassOf = class + " : " + base
			}
			m.Fields = append(m.Fields, f)
		} else if reMethod.MatchString(l) {
			m.Stats.ChoiceCnt++
		}
	}
	sort.Slice(m.Fields, func(i, j int) bool { return m.Fields[i].Scope < m.Fields[j].Scope })
}

func writeFields(m *model.GameModel, exportsDir string) {
	b, _ := json.MarshalIndent(m.Fields, "", "  ")
	_ = os.WriteFile(filepath.Join(exportsDir, "fields.json"), b, 0o644)
}

func tail(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}

var _ = strconv.Itoa
