// Package extract: extract.go —— 顶层入口：zip/目录解包、引擎分发到具体管线。
package extract

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"dissect/internal/detect"
	"dissect/internal/extract/renpy"
	"dissect/internal/extract/unity"
	"dissect/internal/model"
	"dissect/internal/rpa"
)

// Opts 解包全局选项。
type Opts struct {
	WorkDir       string
	Python        string
	UnrpycDir     string
	Il2CppDumper  string
	UnityPyWorker string
	Progress      func(string, float64)
	Log           func(level, msg string)
	OnEngine      func(engine, version string)
	Ctx           context.Context
}

func logf(o Opts, level, format string, args ...any) {
	if o.Log != nil {
		o.Log(level, fmt.Sprintf(format, args...))
	}
}

func ctxErr(ctx context.Context) error {
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

// Unpack 对输入（zip 或已解包目录）执行解包分析，输出到 outDir。
// 输入是 zip 时先解到临时目录（传入 Opts.WorkDir 为空则用 outDir/.work_src）。
func Unpack(src, outDir string, o Opts) (*model.GameModel, error) {
	workDir, python, unrpycDir, progress := o.WorkDir, o.Python, o.UnrpycDir, o.Progress
	if err := ctxErr(o.Ctx); err != nil {
		return nil, err
	}
	root := src
	st, err := os.Stat(src)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() && strings.EqualFold(filepath.Ext(src), ".zip") {
		if workDir == "" {
			workDir = filepath.Join(outDir, ".work_src")
		}
		if err := os.MkdirAll(workDir, 0o755); err != nil {
			return nil, err
		}
		if progress != nil {
			progress("zip", 0.005)
		}
		logf(o, "info", "解压压缩包：%s", filepath.Base(src))
		if err := extractZip(src, workDir, o.Ctx, func(done, total int) {
			if progress != nil && total > 0 {
				progress("zip", 0.005+0.015*float64(done)/float64(total))
			}
		}); err != nil {
			return nil, fmt.Errorf("解压 zip: %w", err)
		}
		logf(o, "info", "压缩包解压完成")
		root = FindGameRoot(workDir)
	} else if st.IsDir() {
		// 目录输入也先定位游戏根（兼容把上级目录/散文件拖进来的情况）
		root = FindGameRoot(src)
	} else {
		return nil, fmt.Errorf("输入必须是目录或 zip: %s", src)
	}
	if err := ctxErr(o.Ctx); err != nil {
		return nil, err
	}
	logf(o, "info", "定位游戏根目录：%s", root)

	info := detect.Detect(root)
	if progress != nil {
		progress("detect", 0.02)
	}
	if o.OnEngine != nil {
		o.OnEngine(info.Engine, info.EngineVersion)
	}
	switch info.Engine {
	case "renpy":
		opt := renpy.Options{
			GameRoot: root, OutDir: outDir, Python: python, UnrpycDir: unrpycDir,
			Progress: progress, Log: o.Log, Ctx: o.Ctx,
		}
		return renpy.Run(opt)
	case "unity":
		uopt := unity.Options{
			GameRoot: root, OutDir: outDir, Il2CppDumper: o.Il2CppDumper,
			Python: python, UnityPyWorker: o.UnityPyWorker, Progress: progress,
			Log: o.Log, Ctx: o.Ctx,
		}
		return unity.Run(uopt)
	default:
		return nil, fmt.Errorf(
			"无法识别引擎（当前支持 Ren'Py / Unity IL2CPP）。\n扫描目录: %s\n"+
				"提示: 请选择游戏根目录——Ren'Py 选含 game/ 和 lib/ 的那一层，Unity 选含 GameAssembly.dll 的那一层", root)
	}
}

// FindGameRoot 在目录里定位真正的游戏根目录（含 game/ 或 GameAssembly 等）。
// 逐层向下最多 4 层，返回第一个能识别出引擎的目录；都识别不出时返回 base。
func FindGameRoot(base string) string {
	queue := []string{base}
	for depth := 0; depth < 4 && len(queue) > 0; depth++ {
		var next []string
		for _, dir := range queue {
			if detect.Detect(dir).Engine != "unknown" {
				return dir
			}
			ents, err := os.ReadDir(dir)
			if err != nil {
				continue
			}
			for _, e := range ents {
				if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
					next = append(next, filepath.Join(dir, e.Name()))
				}
			}
		}
		queue = next
	}
	return base
}

// ExtractZip 解压 zip（带 GBK 文件名容错提示；目录穿越防护）。
func ExtractZip(zipPath, dest string) error {
	return extractZip(zipPath, dest, nil, nil)
}

func extractZip(zipPath, dest string, ctx context.Context, onCount func(done, total int)) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	total := len(r.File)
	for i, f := range r.File {
		if err := ctxErr(ctx); err != nil {
			return err
		}
		name := f.Name
		if f.NonUTF8 && !utf8.ValidString(name) {
			name = decodeGBKName(name)
		}
		dst := filepath.Join(dest, filepath.FromSlash(rpa.Sanitize(name)))
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dst, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := copyZipEntry(f, dst); err != nil {
			return err
		}
		if onCount != nil && (i%20 == 0 || i == total-1) {
			onCount(i+1, total)
		}
	}
	return nil
}

func copyZipEntry(f *zip.File, dst string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

// decodeGBKName 简易处理：非 UTF-8 名按原样返回并替换非法字节，
// 完整 GBK 支持通过注释说明（本工具的目标 zip 均为 UTF-8 命名）。
func decodeGBKName(s string) string {
	if !utf8.ValidString(s) {
		b := []byte(s)
		for i := range b {
			if b[i] >= 0x80 {
				b[i] = '_'
			}
		}
		return string(b)
	}
	return s
}
