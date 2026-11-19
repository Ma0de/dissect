// Package renpy: pipeline.go —— Ren'Py 解包总管线：
// 检测 → RPA 提取 → unrpyc 反编译 → 脚本解析 → 资产索引 → 导出 → 模型落盘。
package renpy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"dissect/internal/detect"
	"dissect/internal/model"
	"dissect/internal/rpa"
)

type Options struct {
	GameRoot  string                          // 已解包的游戏根目录（含 game/）
	OutDir    string                          // 输出目录
	Python    string                          // python 解释器路径，留空自动探测
	UnrpycDir string                          // unrpyc 目录（含 unrpyc.py 与 decompiler/）
	Progress  func(stage string, pct float64) // 进度回调（可为 nil）
	Log       func(level, msg string)
	Ctx       context.Context
	// 资产提取过滤器：返回 false 跳过该文件；nil = 全部提取
	AssetFilter func(archivePath string) bool
}

var assetKindExts = map[string]string{
	".png": "image", ".webp": "image", ".jpg": "image", ".jpeg": "image",
	".bmp": "image", ".gif": "image", ".dds": "image", ".psd": "image",
	".tga": "image", ".iff": "image", ".svg": "image",
	".ogg": "audio", ".opus": "audio", ".mp3": "audio", ".wav": "audio",
	".flac": "audio", ".mid": "audio", ".midi": "audio", ".oga": "audio",
	".ttf": "font", ".otf": "font", ".woff": "font", ".woff2": "font",
	".webm": "video", ".mp4": "video", ".ogv": "video", ".mkv": "video",
	".rpyc": "script", ".rpy": "script", ".rpymc": "script",
	".rpa": "archive", ".py": "script", ".json": "data", ".txt": "data",
	".rpyb": "bytecode",
}

func AssetKind(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	if k, ok := assetKindExts[ext]; ok {
		return k
	}
	return "other"
}

func report(opt *Options, stage string, pct float64) {
	if opt.Progress != nil {
		opt.Progress(stage, pct)
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

// Run 执行完整解包管线，返回模型。
func Run(opt Options) (*model.GameModel, error) {
	if err := checkCancel(opt.Ctx); err != nil {
		return nil, err
	}
	report(&opt, "detect", 0.02)
	info := detect.Detect(opt.GameRoot)
	if info.Engine != "renpy" {
		return nil, fmt.Errorf("不是 Ren'Py 游戏 (检测到: %s)", info.Engine)
	}
	gameDir := filepath.Join(opt.GameRoot, "game")
	if err := os.MkdirAll(opt.OutDir, 0o755); err != nil {
		return nil, err
	}
	assetsDir := filepath.Join(opt.OutDir, "assets")
	scriptsDir := filepath.Join(opt.OutDir, "scripts")
	exportsDir := filepath.Join(opt.OutDir, "exports")
	for _, d := range []string{assetsDir, scriptsDir, exportsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}

	p := NewParser()
	p.Model.GameID = info.GameID
	p.Model.Engine = "renpy"
	p.Model.EngineVersion = info.EngineVersion
	p.Model.SourceDir = opt.GameRoot
	if p.Model.GameName == "" {
		p.Model.GameName = info.GameName
	}
	p.Model.EngineMeta = map[string]any{"detect": info.Details}

	// ---- 1) 提取 .rpa ----
	rpas, _ := filepath.Glob(filepath.Join(gameDir, "*.rpa"))
	total := len(rpas)
	for i, rpaPath := range rpas {
		if err := checkCancel(opt.Ctx); err != nil {
			return nil, err
		}
		report(&opt, "extract "+filepath.Base(rpaPath), 0.05+0.35*float64(i)/float64(max(total, 1)))
		arch, err := rpa.OpenRPA(rpaPath)
		if err != nil {
			p.Model.Notes = append(p.Model.Notes, fmt.Sprintf("打开 %s 失败: %v", filepath.Base(rpaPath), err))
			logf(&opt, "error", "打开 %s 失败: %v", filepath.Base(rpaPath), err)
			continue
		}
		names := arch.Names()
		logf(&opt, "info", "%s：%d 个条目", filepath.Base(rpaPath), len(names))
		n := 0
		for j, name := range names {
			if j%50 == 0 {
				if err := checkCancel(opt.Ctx); err != nil {
					arch.Close()
					return nil, err
				}
			}
			if opt.AssetFilter != nil && !opt.AssetFilter(name) {
				continue
			}
			rel := rpa.Sanitize(name)
			if rel == "" {
				continue // 目录占位条目
			}
			dst := filepath.Join(assetsDir, filepath.FromSlash(rel))
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				return nil, err
			}
			data, err := arch.ReadFile(name)
			if err != nil {
				p.Model.Notes = append(p.Model.Notes, fmt.Sprintf("读取 %s/%s 失败: %v", filepath.Base(rpaPath), name, err))
				continue
			}
			if err := os.WriteFile(dst, data, 0o644); err != nil {
				return nil, err
			}
			n++
		}
		arch.Close()
		p.Model.Notes = append(p.Model.Notes,
			fmt.Sprintf("从 %s 提取 %d/%d 个文件", filepath.Base(rpaPath), n, len(names)))
		logf(&opt, "info", "从 %s 提取 %d/%d 个文件", filepath.Base(rpaPath), n, len(names))
	}

	// ---- 2) 收集 .rpyc 并反编译 ----
	if err := checkCancel(opt.Ctx); err != nil {
		return nil, err
	}
	report(&opt, "decompile", 0.45)
	var rpycFiles []string
	filepath.WalkDir(gameDir, func(path string, d os.DirEntry, err error) error {
		if err == nil && d != nil && !d.IsDir() &&
			(strings.HasSuffix(d.Name(), ".rpyc") || strings.HasSuffix(d.Name(), ".rpymc")) {
			rpycFiles = append(rpycFiles, path)
		}
		return nil
	})
	filepath.WalkDir(assetsDir, func(path string, d os.DirEntry, err error) error {
		if err == nil && d != nil && !d.IsDir() &&
			(strings.HasSuffix(d.Name(), ".rpyc") || strings.HasSuffix(d.Name(), ".rpymc")) {
			rpycFiles = append(rpycFiles, path)
		}
		return nil
	})
	sort.Strings(rpycFiles)
	logf(&opt, "info", "收集到 %d 个脚本文件（.rpyc/.rpymc）", len(rpycFiles))

	py := opt.Python
	if py == "" {
		py = findPython()
	}
	decompiled := 0
	if py != "" && opt.UnrpycDir != "" && len(rpycFiles) > 0 {
		// 复制到 scripts/ 平铺目录（unrpyc 支持目录参数，递归反编译）
		workScripts := scriptsDir
		for i, f := range rpycFiles {
			dst := filepath.Join(workScripts, fmt.Sprintf("%04d_%s", i, filepath.Base(f)))
			if data, err := os.ReadFile(f); err == nil {
				_ = os.WriteFile(dst, data, 0o644)
			}
		}
		runUnrpyc := func(extra ...string) (string, error) {
			args := append([]string{filepath.Join(opt.UnrpycDir, "unrpyc.py")}, extra...)
			args = append(args, workScripts)
			var cmd *exec.Cmd
			if opt.Ctx != nil {
				cmd = exec.CommandContext(opt.Ctx, py, args...)
			} else {
				cmd = exec.Command(py, args...)
			}
			b, e := cmd.CombinedOutput()
			return string(b), e
		}
		logf(&opt, "info", "unrpyc 开始反编译 %d 个脚本文件", len(rpycFiles))
		out, err := runUnrpyc()
		rpys, _ := filepath.Glob(filepath.Join(scriptsDir, "*.rpy"))
		if err != nil || len(rpys) == 0 {
			// 商业游戏常修改 .rpyc 头部，加 --try-harder 重试
			logf(&opt, "warn", "unrpyc 首次失败，改用 --try-harder 重试")
			if out2, err2 := runUnrpyc("--try-harder"); err2 == nil {
				out, err = out2, nil
			}
		}
		rpys, _ = filepath.Glob(filepath.Join(scriptsDir, "*.rpy"))
		if err != nil || len(rpys) == 0 {
			p.Model.Notes = append(p.Model.Notes,
				fmt.Sprintf("unrpyc 反编译失败(%v): %s", err, tail(string(out), 400)))
			logf(&opt, "error", "unrpyc 反编译失败: %s", tail(string(out), 200))
		} else {
			decompiled = len(rpys)
			p.Model.Notes = append(p.Model.Notes,
				fmt.Sprintf("unrpyc 反编译 %d/%d 个脚本文件", decompiled, len(rpycFiles)))
			logf(&opt, "success", "unrpyc 反编译 %d/%d 个脚本文件", decompiled, len(rpycFiles))
		}
	} else {
		p.Model.Notes = append(p.Model.Notes,
			"未找到 python 或 unrpyc，跳过脚本反编译（只能提取资产，无法分析文案/流程）")
		logf(&opt, "warn", "未找到 python 或 unrpyc，跳过脚本反编译")
	}

	// ---- 3) 解析 .rpy ----
	if err := checkCancel(opt.Ctx); err != nil {
		return nil, err
	}
	report(&opt, "parse", 0.6)
	files := map[string]string{}
	rpys, _ := filepath.Glob(filepath.Join(scriptsDir, "*.rpy"))
	sort.Strings(rpys)
	for _, f := range rpys {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		files[filepath.Base(f)] = string(b)
	}
	p.ParseFiles(files)
	logf(&opt, "info", "解析 %d 个脚本文件：%d 句台词、%d 个选项、%d 个流程节点",
		len(files), len(p.Model.Dialogues), len(p.Model.Choices), len(p.Model.Flow.Nodes))

	// ---- 4) 资产清单 ----
	if err := checkCancel(opt.Ctx); err != nil {
		return nil, err
	}
	report(&opt, "assets", 0.8)
	assetList(p, assetsDir)
	p.LinkVoices()
	logf(&opt, "info", "资产索引：%d 个文件，语音映射 %d 句", len(p.Model.Assets), p.Model.Stats.VoiceMapped)

	// ---- 5) 导出 ----
	if err := checkCancel(opt.Ctx); err != nil {
		return nil, err
	}
	report(&opt, "export", 0.92)
	if err := writeExports(p.Model, exportsDir); err != nil {
		return nil, err
	}
	// 头文件
	hdr := GenerateHeader(p.Model)
	hdrPath := filepath.Join(exportsDir, "Model.h")
	if err := os.WriteFile(hdrPath, []byte(hdr), 0o644); err != nil {
		return nil, err
	}
	p.Model.Headers = append(p.Model.Headers, model.HeaderFile{
		Name: "Model.h", Engine: "renpy",
		Note: "Ren'Py 数据模型 C++ 头文件（store 变量 / 类字段 / 角色定义 / screen 参数），供存档编辑等修改工具做字段对照",
		Path: "exports/Model.h",
	})
	logf(&opt, "info", "导出 CSV / JSON / TXT / Mermaid 与 Model.h")

	// ---- 6) 模型落盘 ----
	if err := checkCancel(opt.Ctx); err != nil {
		return nil, err
	}
	report(&opt, "done", 1.0)
	p.Model.Normalize()
	b, _ := json.MarshalIndent(p.Model, "", "  ")
	if err := os.WriteFile(filepath.Join(opt.OutDir, "model.json"), b, 0o644); err != nil {
		return nil, err
	}
	logf(&opt, "success", "模型已写入 model.json")
	return p.Model, nil
}

// assetList 建立 assets/ 目录的资产清单并挂接脚本引用。
func assetList(p *Parser, assetsDir string) {
	filepath.WalkDir(assetsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(assetsDir, path)
		st, _ := d.Info()
		var sz int64
		if st != nil {
			sz = st.Size()
		}
		a := model.Asset{
			Path: filepath.ToSlash(rel), Kind: AssetKind(rel), Size: sz,
		}
		// 挂接脚本引用: 归一化尝试
		key := filepath.ToSlash(rel)
		candidates := []string{key}
		if strings.HasPrefix(key, "images/") {
			candidates = append(candidates, strings.TrimPrefix(key, "images/"))
		}
		candidates = append(candidates, filepath.Base(key))
		for _, c := range candidates {
			if used, ok := p.pendingUsage[c]; ok {
				a.UsedAt = used
				break
			}
		}
		// image 名
		for img, f := range p.images {
			if f == key || f == strings.TrimPrefix(key, "images/") ||
				("images/"+f) == key {
				a.ImageName = img
				break
			}
		}
		p.Model.Assets = append(p.Model.Assets, a)
		if p.Model.Stats.AssetKinds == nil {
			p.Model.Stats.AssetKinds = map[string]int{}
			p.Model.Stats.AssetBytes = map[string]int64{}
		}
		p.Model.Stats.AssetKinds[a.Kind]++
		p.Model.Stats.AssetBytes[a.Kind] += a.Size
		return nil
	})
	sort.Slice(p.Model.Assets, func(i, j int) bool {
		return p.Model.Assets[i].Path < p.Model.Assets[j].Path
	})
}

func findPython() string {
	for _, c := range []string{"python3", "python"} {
		if p, err := exec.LookPath(c); err == nil {
			return p
		}
	}
	return ""
}

func tail(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
