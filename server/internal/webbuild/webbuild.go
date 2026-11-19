package webbuild

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Options struct {
	OutDir    string
	CacheDir  string
	Version   string
	Title     string
	SourceDir string
	Live2D    bool
	Stubs     []string
	Ctx       context.Context
	Progress  func(stage string, pct float64)
	Log       func(level, msg string)
}

type Result struct {
	Dir   string
	Bytes int64
}

type Meta struct {
	Version string `json:"version"`
	Title   string `json:"title"`
	Bytes   int64  `json:"bytes"`
	BuiltAt string `json:"builtAt"`
	Live2D  bool   `json:"live2d"`
}

func (o *Options) logf(level, format string, args ...any) {
	if o.Log != nil {
		o.Log(level, fmt.Sprintf(format, args...))
	}
}

func (o *Options) report(stage string, pct float64) {
	if o.Progress != nil {
		o.Progress(stage, pct)
	}
}

func (o *Options) ctx() context.Context {
	if o.Ctx == nil {
		return context.Background()
	}
	return o.Ctx
}

func Build(o Options) (*Result, error) {
	if o.OutDir == "" {
		return nil, errors.New("缺少输出目录")
	}
	ver := strings.TrimSpace(o.Version)
	if ver == "" {
		return nil, errors.New("未能识别 Ren'Py 版本，无法生成 Web 版")
	}
	major, err := strconv.Atoi(strings.SplitN(ver, ".", 2)[0])
	if err != nil || major != 8 {
		return nil, fmt.Errorf("只支持 Ren'Py 8.x，当前版本为 %s", ver)
	}
	if o.CacheDir == "" {
		return nil, errors.New("缺少运行时缓存目录")
	}

	ctx := o.ctx()
	o.report("web-check", 0.03)

	runtimeDir, err := ensureRuntime(ctx, o)
	if err != nil {
		return nil, err
	}
	sdkDir, err := ensureSDK(ctx, o)
	if err != nil {
		return nil, err
	}

	langCompat := needsLangCompat(o.OutDir) && !sdkHasLangCompat(sdkDir)
	if langCompat {
		o.logf("info", "游戏使用分语言扩展（text/image/voice language），注入运行库兼容层")
	}

	o.report("web-assemble", 0.76)
	webDir := filepath.Join(o.OutDir, "web")
	tmpDir := webDir + ".tmp"
	_ = os.RemoveAll(tmpDir)
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return nil, err
	}
	if err := copyTree(runtimeDir, tmpDir); err != nil {
		return nil, err
	}
	prepareIcons(tmpDir)
	if err := writeIndexHTML(tmpDir, o.Title); err != nil {
		return nil, err
	}

	games, err := collectGameFiles(o.OutDir, o.SourceDir)
	if err != nil {
		return nil, err
	}
	if len(games) == 0 {
		return nil, errors.New("没有找到可打包的游戏文件")
	}
	o.logf("info", "游戏文件：%d 个", len(games))

	zipPath := filepath.Join(tmpDir, "game.zip")
	if err := buildGameZip(ctx, zipPath, sdkDir, games, o.Stubs, langCompat, o.logf); err != nil {
		return nil, err
	}
	st, err := os.Stat(zipPath)
	if err != nil {
		return nil, err
	}

	total := st.Size()
	_ = filepath.WalkDir(tmpDir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && p != zipPath {
			if fi, e := d.Info(); e == nil {
				total += fi.Size()
			}
		}
		return nil
	})

	meta := Meta{
		Version: ver,
		Title:   o.Title,
		Bytes:   total,
		BuiltAt: time.Now().Format(time.RFC3339),
		Live2D:  o.Live2D,
	}
	if b, err := json.MarshalIndent(meta, "", "  "); err == nil {
		_ = os.WriteFile(filepath.Join(tmpDir, "meta.json"), b, 0o644)
	}

	o.report("web-done", 0.97)
	_ = os.RemoveAll(webDir)
	if err := os.Rename(tmpDir, webDir); err != nil {
		return nil, err
	}
	o.report("web-done", 1)
	o.logf("success", "Web 版已生成：%s（%.1f MB）", webDir, float64(total)/1048576)
	return &Result{Dir: webDir, Bytes: total}, nil
}

func ensureRuntime(ctx context.Context, o Options) (string, error) {
	o.report("web-runtime", 0.06)
	dir := filepath.Join(o.CacheDir, "renpy-web", o.Version)
	base := filepath.Join(dir, "index.html")
	if _, err := os.Stat(base); err == nil {
		o.logf("info", "复用 Web 运行时 %s", o.Version)
		return dir, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	url := fmt.Sprintf("https://www.renpy.org/dl/%s/renpy-%s-web.zip", o.Version, o.Version)
	zipPath := filepath.Join(o.CacheDir, "downloads", "renpy-"+o.Version+"-web.zip")
	o.logf("info", "下载 Ren'Py %s Web 运行时…", o.Version)
	o.report("web-runtime", 0.06)
	if err := download(ctx, url, zipPath, func(done, total int64) {
		if total > 0 {
			o.report("web-runtime", 0.06+0.22*float64(done)/float64(total))
		}
	}); err != nil {
		return "", fmt.Errorf("下载 Web 运行时失败：%w", err)
	}
	if err := extractPrefix(zipPath, "web/", dir); err != nil {
		return "", fmt.Errorf("解出 Web 运行时失败：%w", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err != nil {
		return "", errors.New("Web 运行时内容不完整")
	}
	o.logf("success", "Web 运行时就绪（%s）", o.Version)
	return dir, nil
}

func ensureSDK(ctx context.Context, o Options) (string, error) {
	o.report("web-sdk", 0.3)
	dir := filepath.Join(o.CacheDir, "renpy-sdk", o.Version)
	if _, err := os.Stat(filepath.Join(dir, "renpy", "__init__.py")); err == nil {
		o.logf("info", "复用 Ren'Py %s 运行库", o.Version)
		return dir, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	url := fmt.Sprintf("https://www.renpy.org/dl/%s/renpy-%s-sdk.zip", o.Version, o.Version)
	prefix := "renpy-" + o.Version + "-sdk/"
	o.logf("info", "准备 Ren'Py %s 运行库（只取 renpy 子树）…", o.Version)
	o.report("web-sdk", 0.3)

	size, sizeErr := remoteSize(ctx, url)
	if sizeErr == nil {
		err := extractRemoteSDK(ctx, url, size, prefix, dir, func(done, total int64) {
			if total > 0 {
				o.report("web-sdk", 0.3+0.42*float64(done)/float64(total))
			}
		})
		if err == nil {
			if _, statErr := os.Stat(filepath.Join(dir, "renpy", "__init__.py")); statErr == nil {
				o.logf("success", "Ren'Py 运行库就绪（%s）", o.Version)
				return dir, nil
			}
		}
		o.logf("warn", "按子树读取失败，改为整包下载：%v", err)
		_ = os.RemoveAll(dir)
		if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
			return "", mkErr
		}
	}

	zipPath := filepath.Join(o.CacheDir, "downloads", "renpy-"+o.Version+"-sdk.zip")
	o.logf("info", "下载 Ren'Py %s SDK（约 160 MB，仅首次）…", o.Version)
	if err := download(ctx, url, zipPath, func(done, total int64) {
		if total > 0 {
			o.report("web-sdk", 0.3+0.42*float64(done)/float64(total))
		}
	}); err != nil {
		return "", fmt.Errorf("下载 Ren'Py SDK 失败：%w", err)
	}
	if err := extractSDKTree(zipPath, prefix, dir); err != nil {
		return "", fmt.Errorf("解出 Ren'Py 运行库失败：%w", err)
	}
	o.logf("success", "Ren'Py 运行库就绪（%s）", o.Version)
	return dir, nil
}

func download(ctx context.Context, url, dst string, onBytes func(done, total int64)) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	total, err := remoteSize(ctx, url)
	if err != nil {
		return err
	}
	if total <= 0 {
		return downloadSingle(ctx, url, dst, onBytes)
	}
	if fi, err := os.Stat(dst); err == nil && fi.Size() == total {
		return nil
	}

	const workers = 8
	chunk := (total + workers - 1) / workers
	parts := make([]string, workers)
	var done int64
	var mu sync.Mutex
	var firstErr error
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		start := int64(i) * chunk
		end := start + chunk - 1
		if end >= total {
			end = total - 1
		}
		parts[i] = fmt.Sprintf("%s.part%d", dst, i)
		wg.Add(1)
		go func(i int, start, end int64) {
			defer wg.Done()
			for attempt := 0; attempt < 4; attempt++ {
				if ctx.Err() != nil {
					return
				}
				n, err := fetchRange(ctx, url, start, end, parts[i])
				if err == nil {
					atomic.AddInt64(&done, n)
					mu.Lock()
					if onBytes != nil {
						onBytes(atomic.LoadInt64(&done), total)
					}
					mu.Unlock()
					return
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(time.Duration(attempt+1) * 2 * time.Second):
				}
			}
			mu.Lock()
			if firstErr == nil {
				firstErr = fmt.Errorf("下载分片 %d 失败", i)
			}
			mu.Unlock()
		}(i, start, end)
	}
	wg.Wait()
	if ctx.Err() != nil {
		for _, p := range parts {
			_ = os.Remove(p)
		}
		return ctx.Err()
	}
	if firstErr != nil {
		for _, p := range parts {
			_ = os.Remove(p)
		}
		return firstErr
	}

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	for _, p := range parts {
		f, err := os.Open(p)
		if err != nil {
			out.Close()
			return err
		}
		_, err = io.Copy(out, f)
		f.Close()
		if err != nil {
			out.Close()
			return err
		}
		_ = os.Remove(p)
	}
	if err := out.Close(); err != nil {
		return err
	}
	if fi, err := os.Stat(dst); err != nil || fi.Size() != total {
		return errors.New("下载文件大小不一致")
	}
	return nil
}

func remoteSize(ctx context.Context, url string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Range", "bytes=0-0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode == http.StatusPartialContent {
		cr := resp.Header.Get("Content-Range")
		if i := strings.LastIndex(cr, "/"); i >= 0 {
			if n, err := strconv.ParseInt(cr[i+1:], 10, 64); err == nil && n > 0 {
				return n, nil
			}
		}
	}
	if resp.StatusCode == http.StatusOK {
		if n := resp.ContentLength; n > 0 {
			return n, nil
		}
	}
	return 0, fmt.Errorf("无法获取文件大小（HTTP %d）", resp.StatusCode)
}

func fetchRange(ctx context.Context, url string, start, end int64, dst string) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(f, resp.Body)
	cerr := f.Close()
	if err != nil {
		return 0, err
	}
	if cerr != nil {
		return 0, cerr
	}
	return n, nil
}

func downloadSingle(ctx context.Context, url, dst string, onBytes func(done, total int64)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	var done int64
	buf := make([]byte, 256*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			done += int64(n)
			if onBytes != nil {
				onBytes(done, resp.ContentLength)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return nil
}

func extractPrefix(zipPath, prefix, dst string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if !strings.HasPrefix(f.Name, prefix) || f.FileInfo().IsDir() {
			continue
		}
		rel := strings.TrimPrefix(f.Name, prefix)
		if rel == "" {
			continue
		}
		if err := writeZipEntry(f, filepath.Join(dst, filepath.FromSlash(rel))); err != nil {
			return err
		}
	}
	return nil
}

func extractSDKTree(zipPath, prefix, dst string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		if f.FileInfo().IsDir() || strings.HasSuffix(f.Name, "/") {
			continue
		}
		rel := ""
		switch {
		case f.Name == prefix+"renpy.py":
			rel = "renpy.py"
		case strings.HasPrefix(f.Name, prefix+"renpy/"):
			rel = strings.TrimPrefix(f.Name, prefix)
		default:
			continue
		}
		if err := writeZipEntry(f, filepath.Join(dst, filepath.FromSlash(rel))); err != nil {
			return err
		}
	}
	return nil
}

func writeZipEntry(f *zip.File, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}

func prepareIcons(dir string) {
	icon, err := os.ReadFile(filepath.Join(dir, "web-icon.png"))
	if err != nil {
		return
	}
	b, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return
	}
	var man struct {
		Icons []struct {
			Src string `json:"src"`
		} `json:"icons"`
	}
	if json.Unmarshal(b, &man) != nil {
		return
	}
	for _, ic := range man.Icons {
		src := strings.TrimPrefix(strings.TrimPrefix(ic.Src, "./"), "/")
		if src == "" {
			continue
		}
		dst := filepath.Join(dir, filepath.FromSlash(src))
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			continue
		}
		_ = os.WriteFile(dst, icon, 0o644)
	}
}

func writeIndexHTML(dir, title string) error {
	p := filepath.Join(dir, "index.html")
	b, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	if title == "" {
		title = "dissect"
	}
	title = strings.NewReplacer("\r", " ", "\n", " ").Replace(title)
	html := strings.ReplaceAll(string(b), "Ren'Py Web Game", title)
	return os.WriteFile(p, []byte(html), 0o644)
}

type gameEntry struct {
	name string
	path string
}

func collectGameFiles(outDir, sourceDir string) ([]gameEntry, error) {
	files := map[string]string{}
	add := func(name, p string) {
		name = strings.TrimPrefix(filepath.ToSlash(name), "/")
		if name == "" || strings.HasSuffix(strings.ToLower(name), ".rpa") {
			return
		}
		if strings.Contains(name, "/saves/") || strings.HasPrefix(name, "game/saves/") {
			return
		}
		files[name] = p
	}
	assets := filepath.Join(outDir, "assets")
	_ = filepath.WalkDir(assets, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(assets, p)
		if e != nil {
			return nil
		}
		add("game/"+filepath.ToSlash(rel), p)
		return nil
	})
	if sourceDir != "" {
		gdir := filepath.Join(sourceDir, "game")
		if st, err := os.Stat(gdir); err == nil && st.IsDir() {
			_ = filepath.WalkDir(gdir, func(p string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				rel, e := filepath.Rel(gdir, p)
				if e != nil {
					return nil
				}
				add("game/"+filepath.ToSlash(rel), p)
				return nil
			})
		}
	}
	out := make([]gameEntry, 0, len(files))
	for name, p := range files {
		out = append(out, gameEntry{name: name, path: p})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out, nil
}

func buildGameZip(ctx context.Context, dst, sdkDir string, games []gameEntry, stubs []string, langCompat bool, logf func(level, format string, args ...any)) error {
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	fail := func(e error) error {
		zw.Close()
		f.Close()
		return e
	}

	mainPy := filepath.Join(sdkDir, "renpy.py")
	if err := addFileToZip(zw, mainPy, "main.py"); err != nil {
		return fail(err)
	}

	renpyDir := filepath.Join(sdkDir, "renpy")
	patchedPyc := map[string]bool{}
	pyc := map[string]map[string]string{}
	_ = filepath.WalkDir(renpyDir, func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() || d.Name() != "__pycache__" {
			return nil
		}
		rel, e := filepath.Rel(renpyDir, filepath.Dir(p))
		if e != nil {
			return nil
		}
		m := map[string]string{}
		ents, e := os.ReadDir(p)
		if e != nil {
			return nil
		}
		for _, ent := range ents {
			if !ent.IsDir() && strings.HasSuffix(ent.Name(), ".pyc") {
				m[strings.SplitN(ent.Name(), ".", 2)[0]] = filepath.Join(p, ent.Name())
			}
		}
		pyc[rel] = m
		return nil
	})

	err = filepath.WalkDir(renpyDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			if d.Name() == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".pyx" || ext == ".pxd" || ext == ".pxi" || ext == ".pyo" {
			return nil
		}
		rel, e := filepath.Rel(renpyDir, p)
		if e != nil {
			return e
		}
		dir := filepath.Dir(rel)
		relSlash := filepath.ToSlash(rel)
		if langCompat {
			if lc := findLangCompatPatch("renpy/" + relSlash); lc != nil {
				data, err := os.ReadFile(p)
				if err != nil {
					return err
				}
				patched, err := lc.apply(data)
				if err != nil {
					return fmt.Errorf("注入分语言兼容层失败（%s）：%w", relSlash, err)
				}
				patchedPyc[dir+"\x00"+strings.TrimSuffix(name, ".py")] = true
				if logf != nil {
					logf("info", "兼容层已注入：%s", relSlash)
				}
				return addBytesToZip(zw, "renpy/"+relSlash, patched)
			}
		}
		if ext == ".py" {
			if m, ok := pyc[dir]; ok {
				if _, has := m[strings.TrimSuffix(name, ".py")]; has {
					return nil
				}
			}
		}
		return addFileToZip(zw, p, "renpy/"+relSlash)
	})
	if err != nil {
		return fail(err)
	}

	for rel, m := range pyc {
		for stem, p := range m {
			if patchedPyc[rel+"\x00"+stem] {
				continue
			}
			name := "renpy/" + stem + ".pyc"
			if rel != "." {
				name = "renpy/" + filepath.ToSlash(rel) + "/" + stem + ".pyc"
			}
			if err := addFileToZip(zw, p, name); err != nil {
				return fail(err)
			}
		}
	}

	for _, g := range games {
		if ctx.Err() != nil {
			return fail(ctx.Err())
		}
		if err := addFileToZip(zw, g.path, g.name); err != nil {
			return fail(err)
		}
	}

	stubData := stubEntries(stubs)
	for _, name := range sortedStubNames(stubData) {
		if err := addBytesToZip(zw, name, stubData[name]); err != nil {
			return fail(err)
		}
	}

	if err := zw.Close(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

var zipEpoch = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

func addBytesToZip(zw *zip.Writer, name string, data []byte) error {
	hdr := &zip.FileHeader{Name: name, Method: zip.Store}
	hdr.SetModTime(zipEpoch)
	hdr.SetMode(0o644)
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func addFileToZip(zw *zip.Writer, src, name string) error {
	st, err := os.Stat(src)
	if err != nil {
		return err
	}
	hdr, err := zip.FileInfoHeader(st)
	if err != nil {
		return err
	}
	hdr.Name = name
	hdr.Method = zip.Store
	if hdr.Modified.Before(zipEpoch) {
		hdr.SetModTime(zipEpoch)
	}
	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}
