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
	SrcDir   string
	Dest     string
	Title    string
	Version  string
	Zip      bool
	Progress func(done, total int64)
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
	if o.Progress != nil {
		o.Progress(0, total)
	}
	if o.Zip {
		return zipSite(src, dest, name, files, total, o.Progress)
	}
	return copySite(src, dest, name, files, total, o.Progress)
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

func copySite(src, dest, name string, files []string, total int64, progress func(int64, int64)) (*ExportResult, error) {
	target := filepath.Join(dest, name)
	if _, err := os.Stat(target); err == nil {
		return nil, fmt.Errorf("目标已存在：%s", target)
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return nil, fmt.Errorf("无法创建目录 %s：%w", target, err)
	}
	pw := &byteProgress{total: total, cb: progress}
	for _, rel := range files {
		if err := copyFileTo(filepath.Join(src, filepath.FromSlash(rel)), filepath.Join(target, filepath.FromSlash(rel)), pw); err != nil {
			return nil, fmt.Errorf("复制 %s 失败：%w", rel, err)
		}
	}
	if progress != nil {
		progress(total, total)
	}
	return &ExportResult{Path: target, Bytes: total, Files: len(files)}, nil
}

func zipSite(src, dest, name string, files []string, total int64, progress func(int64, int64)) (*ExportResult, error) {
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
	pw := &byteProgress{total: total, cb: progress}
	for _, rel := range files {
		if err := addFileToZipProgress(zw, filepath.Join(src, filepath.FromSlash(rel)), rel, pw); err != nil {
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
	if progress != nil {
		progress(total, total)
	}
	return &ExportResult{Path: zipPath, Bytes: total, Files: len(files)}, nil
}

type byteProgress struct {
	w     io.Writer
	done  int64
	total int64
	last  int64
	cb    func(int64, int64)
}

func (p *byteProgress) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.done += int64(n)
	if p.cb != nil && p.done-p.last >= 8<<20 {
		p.last = p.done
		p.cb(p.done, p.total)
	}
	return n, err
}

func copyFileTo(src, dst string, pw *byteProgress) error {
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
	pw.w = out
	if _, err := io.Copy(pw, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func addFileToZipProgress(zw *zip.Writer, src, name string, pw *byteProgress) error {
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
	pw.w = w
	_, err = io.Copy(pw, f)
	return err
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
