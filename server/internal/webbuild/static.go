package webbuild

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type ExportOptions struct {
	SrcDir  string
	Dest    string
	Title   string
	Version string
	Zip     bool
}

type ExportResult struct {
	Path  string
	Bytes int64
	Files int
}

func ExportStatic(o ExportOptions) (*ExportResult, error) {
	src := o.SrcDir
	st, err := os.Stat(filepath.Join(src, "index.html"))
	if err != nil || st.IsDir() {
		return nil, errors.New("还没有生成 Web 版，请先生成后再导出")
	}
	dest := strings.TrimSpace(o.Dest)
	if dest == "" || !filepath.IsAbs(dest) {
		return nil, errors.New("请选择一个有效的导出位置")
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return nil, fmt.Errorf("无法创建导出位置：%w", err)
	}
	files, total, err := scanSite(src)
	if err != nil {
		return nil, err
	}
	title := strings.TrimSpace(o.Title)
	if title == "" {
		title = "Web 游戏"
	}
	name := exportName(title)
	readme := []byte(readmeText(title, strings.TrimSpace(o.Version), total, len(files)+1))
	if o.Zip {
		return zipSite(src, dest, name, files, total, readme)
	}
	return copySite(src, dest, name, files, total, readme)
}

func scanSite(src string) ([]string, int64, error) {
	var files []string
	var total int64
	err := filepath.WalkDir(src, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, e := filepath.Rel(src, p)
		if e != nil {
			return e
		}
		fi, e := d.Info()
		if e != nil {
			return e
		}
		files = append(files, filepath.ToSlash(rel))
		total += fi.Size()
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	sort.Strings(files)
	return files, total, nil
}

func copySite(src, dest, name string, files []string, total int64, readme []byte) (*ExportResult, error) {
	target := filepath.Join(dest, name)
	if _, err := os.Stat(target); err == nil {
		return nil, fmt.Errorf("目标已存在：%s", target)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return nil, fmt.Errorf("无法创建目录 %s：%w", target, err)
	}
	for _, rel := range files {
		if err := copyFile(filepath.Join(src, filepath.FromSlash(rel)), filepath.Join(target, filepath.FromSlash(rel))); err != nil {
			return nil, fmt.Errorf("复制 %s 失败：%w", rel, err)
		}
	}
	if err := os.WriteFile(filepath.Join(target, "README.txt"), readme, 0o644); err != nil {
		return nil, fmt.Errorf("写入说明文件失败：%w", err)
	}
	return &ExportResult{Path: target, Bytes: total + int64(len(readme)), Files: len(files) + 1}, nil
}

func zipSite(src, dest, name string, files []string, total int64, readme []byte) (*ExportResult, error) {
	zipPath := filepath.Join(dest, name+".zip")
	if _, err := os.Stat(zipPath); err == nil {
		return nil, fmt.Errorf("目标已存在：%s", zipPath)
	}
	f, err := os.Create(zipPath)
	if err != nil {
		return nil, fmt.Errorf("无法创建 %s：%w", zipPath, err)
	}
	zw := zip.NewWriter(f)
	fail := func(e error) (*ExportResult, error) {
		zw.Close()
		f.Close()
		os.Remove(zipPath)
		return nil, e
	}
	if err := addBytesToZip(zw, "README.txt", readme); err != nil {
		return fail(err)
	}
	for _, rel := range files {
		if err := addFileToZip(zw, filepath.Join(src, filepath.FromSlash(rel)), rel); err != nil {
			return fail(fmt.Errorf("打包 %s 失败：%w", rel, err))
		}
	}
	if err := zw.Close(); err != nil {
		f.Close()
		os.Remove(zipPath)
		return nil, err
	}
	if err := f.Close(); err != nil {
		os.Remove(zipPath)
		return nil, err
	}
	return &ExportResult{Path: zipPath, Bytes: total + int64(len(readme)), Files: len(files) + 1}, nil
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func exportName(title string) string {
	name := safeName(title)
	if name == "" {
		name = "Web 游戏"
	}
	return name + " Web"
}

func safeName(s string) string {
	s = strings.Map(func(r rune) rune {
		switch r {
		case '<', '>', ':', '"', '/', '\\', '|', '?', '*':
			return '_'
		}
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, strings.TrimSpace(s))
	s = strings.TrimRight(s, " .")
	if runes := []rune(s); len(runes) > 60 {
		s = strings.TrimRight(string(runes[:60]), " .")
	}
	return s
}

func readmeText(title, version string, total int64, files int) string {
	pyver := ""
	if version != "" {
		pyver = "（Ren'Py " + version + "）"
	}
	return fmt.Sprintf(`%s
Web 版静态站点%s

这是一个完整的静态站点，不依赖 dissect，不需要任何后端服务。

【本地预览】
在本目录下启动任意静态服务器，例如：
    python -m http.server 8000
    php -S 127.0.0.1:8000
    npx serve .
然后用浏览器打开 http://127.0.0.1:8000/ 。

不要直接双击 index.html（file:// 协议）打开：浏览器会因跨域限制读不到 game.zip。

【部署】
把本目录里的全部文件上传到任意静态 HTTP 服务器或静态托管平台即可，
例如 Nginx / Apache / Caddy / GitHub Pages / Cloudflare Pages / Netlify /
Vercel / 对象存储 + CDN 等。入口是 index.html，页面内全部使用相对路径，
放在根目录或任意子路径都能正常工作。

【服务器要求】
1. .wasm 需要以 application/wasm 返回。Nginx 和较新的 Apache 默认已正确；
   老版本 Apache 可在 .htaccess 中加一行：AddType application/wasm .wasm
2. 不需要 COOP / COEP 等特殊响应头，也不需要任何服务端程序。
3. 建议使用 HTTPS：离线缓存（Service Worker）只在 HTTPS 或 localhost 下生效。
4. 注意托管平台的单文件大小限制（例如 Cloudflare Pages 25 MB、GitHub Pages
   100 MB），game.zip 可能超过限制，大体积游戏请使用自建服务器或对象存储。

【体积】
共 %d 个文件，约 %s。游玩时 game.zip 会被完整下载并载入浏览器内存，
建议设备预留 2 GB 以上可用内存。

【存档】
存档保存在浏览器本地（IndexedDB / localStorage），与访问域名绑定；
更换域名、更换浏览器或清除站点数据都会丢失。
`, title, pyver, files, formatBytes(total))
}

func formatBytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.2f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}
