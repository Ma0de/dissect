// Package api 提供 Web UI 的 HTTP 服务。
//
// 两种模式：
//   - workspace（双击 exe / dissect serve 不带目录）：BaseDir 为工作区，
//     支持上传解包、删除、结果打包下载，解包输出固定在 <BaseDir>/data。
//   - browse（dissect serve <目录>）：只读浏览已有结果，保留打包下载。
//
// API:
//
//	GET  /api/state      → {version, mode, hasModel, game, task}
//	GET  /api/model      → model.json（返回前归一化 null→[]，兼容旧数据）
//	POST /api/unpack     → multipart 上传（zip / .rpa / 整目录文件）并解包，返回 {id}
//	POST /api/pick       → 弹出系统原生选择框（kind=dir|zip），返回 {path}
//	POST /api/unpack-path→ 直接解包本地路径 {path}（文件 ▸ 打开 的主路径，不走上传）
//	GET  /api/task       → 当前任务进度 {id,stage,pct,done,ok,error,game}
//	POST /api/delete     → 删除当前解包结果（仅 workspace）
//	GET  /api/download   → 当前结果打包为 zip 下载
//	GET  /api/headers/<name> → 生成的头文件内容
//	GET  /api/export/<file>  → exports 目录交付物
//	GET  /assets/<path>      → 解包出的资产（图片/音频/字体…）
//	GET  /                   → 内嵌前端（web/out 静态导出）
package api

import (
	"archive/zip"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"dissect/internal/extract"
	"dissect/internal/model"
	"dissect/internal/webbuild"
)

//go:embed all:out
var webFS embed.FS

// Options 服务配置。
type Options struct {
	Addr          string // 监听地址，如 127.0.0.1:0
	BaseDir       string // 工作区根目录；非空 = workspace 模式
	FixedDir      string // browse 模式下浏览的输出目录
	Python        string
	UnrpycDir     string
	Il2CppDumper  string
	UnityPyWorker string
	Version       string
}

type TaskLog struct {
	Time  string `json:"time"`
	Level string `json:"level,omitempty"`
	Msg   string `json:"msg"`
}

type TaskStep struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	State string `json:"state"`
}

// TaskState 一次解包任务的状态。
type TaskState struct {
	ID      string     `json:"id"`
	Kind    string     `json:"kind,omitempty"`
	Stage   string     `json:"stage"`
	Pct     float64    `json:"pct"`
	Done    bool       `json:"done"`
	OK      bool       `json:"ok"`
	Error   string     `json:"error,omitempty"`
	Game    string     `json:"game,omitempty"`
	Engine  string     `json:"engine,omitempty"`
	Current string     `json:"current,omitempty"`
	Steps   []TaskStep `json:"steps,omitempty"`
	Logs    []TaskLog  `json:"logs,omitempty"`
}

// WebInfo 当前已生成的 Web 版信息。
type WebInfo struct {
	Ready   bool   `json:"ready"`
	Version string `json:"version,omitempty"`
	Title   string `json:"title,omitempty"`
	Bytes   int64  `json:"bytes,omitempty"`
	BuiltAt string `json:"builtAt,omitempty"`
}

// Server 交互式 Web 服务。
type Server struct {
	o        Options
	mu       sync.Mutex
	outDir   string
	task     TaskState
	running  bool
	cancel   context.CancelFunc
	stageKey string
	stepKey  string
	game      string
	web       WebInfo
	exporting bool
}

// New 创建服务并确定输出目录。
func New(o Options) *Server {
	s := &Server{o: o}
	if o.BaseDir != "" {
		s.outDir = filepath.Join(o.BaseDir, "data")
		_ = os.MkdirAll(s.outDir, 0o755)
	} else {
		s.outDir = o.FixedDir
	}
	// 启动时若上次的结果还在，读出游戏名（下次双击打开恢复现场）
	if name := readGameName(s.outDir); name != "" {
		s.game = name
	}
	s.web = readWebInfo(s.outDir)
	// Done=true 表示当前没有进行中的任务（零值会被前端误判为解包中）
	s.task = TaskState{Done: true}
	return s
}

func readWebInfo(dir string) WebInfo {
	if _, err := os.Stat(filepath.Join(dir, "web", "index.html")); err != nil {
		return WebInfo{}
	}
	b, err := os.ReadFile(filepath.Join(dir, "web", "meta.json"))
	if err != nil {
		return WebInfo{}
	}
	var meta webbuild.Meta
	if json.Unmarshal(b, &meta) != nil {
		return WebInfo{}
	}
	return WebInfo{
		Ready:   true,
		Version: meta.Version,
		Title:   meta.Title,
		Bytes:   meta.Bytes,
		BuiltAt: meta.BuiltAt,
	}
}

func stageParts(stage string) (string, string) {
	switch {
	case stage == "准备解包":
		return "prep", "正在校验输入…"
	case stage == "接收文件":
		return "prep", "正在接收文件…"
	case stage == "zip":
		return "zip", "正在解压压缩包…"
	case stage == "detect":
		return "detect", "正在识别游戏引擎…"
	case strings.HasPrefix(stage, "extract "):
		return "extract", "正在提取 " + strings.TrimPrefix(stage, "extract ") + "…"
	case stage == "decompile":
		return "decompile", "正在反编译脚本…"
	case stage == "parse":
		return "parse", "正在解析脚本…"
	case stage == "assets":
		return "assets", "正在处理资产…"
	case stage == "export":
		return "export", "正在导出交付物…"
	case stage == "done":
		return "model", "正在写入模型…"
	case stage == "il2cppdumper":
		return "dump", "正在运行 Il2CppDumper…"
	case stage == "parse dump.cs":
		return "parseR", "正在解析 dump.cs…"
	case stage == "web-check":
		return "web-check", "正在检查 Web 兼容性…"
	case stage == "web-runtime":
		return "web-runtime", "正在准备 Web 运行时…"
	case stage == "web-sdk":
		return "web-sdk", "正在准备 Ren'Py 运行库…"
	case stage == "web-assemble":
		return "web-assemble", "正在组装 Web 版…"
	case stage == "web-done":
		return "web-done", "正在收尾…"
	}
	return "", stage
}

type stepDef struct {
	Key    string
	Label  string
	Stages []string
}

func (s *Server) stepDefs(engine string) []stepDef {
	if s.task.Kind == "web" {
		return []stepDef{
			{"wcheck", "检测", []string{"web-check"}},
			{"wruntime", "运行时", []string{"web-runtime"}},
			{"wsdk", "运行库", []string{"web-sdk"}},
			{"wbuild", "组装", []string{"web-assemble"}},
			{"wdone", "完成", []string{"web-done"}},
		}
	}
	switch engine {
	case "renpy":
		return []stepDef{
			{"unpack", "解压", []string{"prep", "zip", "detect"}},
			{"extract", "提取", []string{"extract"}},
			{"decompile", "反编译", []string{"decompile"}},
			{"parse", "解析", []string{"parse"}},
			{"export", "导出", []string{"assets", "export", "model"}},
		}
	case "unity":
		return []stepDef{
			{"unpack", "解压", []string{"prep", "zip", "detect"}},
			{"dump", "转储", []string{"dump"}},
			{"parse", "解析", []string{"parseR"}},
			{"export", "导出", []string{"assets", "export", "model"}},
		}
	default:
		return []stepDef{
			{"unpack", "解压", []string{"prep", "zip", "detect"}},
		}
	}
}

func (s *Server) stepKeyFor(stageKey string) string {
	for _, d := range s.stepDefs(s.task.Engine) {
		for _, st := range d.Stages {
			if st == stageKey {
				return d.Key
			}
		}
	}
	return ""
}

func (s *Server) buildStepsLocked(engine string) {
	defs := s.stepDefs(engine)
	steps := make([]TaskStep, 0, len(defs))
	for _, d := range defs {
		steps = append(steps, TaskStep{Key: d.Key, Label: d.Label, State: "pending"})
	}
	s.task.Steps = steps
}

func (s *Server) markStepsLocked(stageKey string) {
	if stageKey == "" {
		return
	}
	defs := s.stepDefs(s.task.Engine)
	idx := -1
	for i, d := range defs {
		for _, st := range d.Stages {
			if st == stageKey {
				idx = i
				break
			}
		}
		if idx >= 0 {
			break
		}
	}
	if idx < 0 || idx >= len(s.task.Steps) {
		return
	}
	for i := range s.task.Steps {
		st := &s.task.Steps[i]
		switch {
		case i < idx:
			switch st.State {
			case "running":
				st.State = "done"
			case "pending", "":
				st.State = "skipped"
			}
		case i == idx:
			st.State = "running"
		}
	}
}

func (s *Server) stepLabelLocked(key string) string {
	for _, st := range s.task.Steps {
		if st.Key == key {
			return st.Label
		}
	}
	return ""
}

func (s *Server) appendLogLocked(level, msg string) {
	if msg == "" {
		return
	}
	if len(s.task.Logs) >= 400 {
		s.task.Logs = s.task.Logs[len(s.task.Logs)-399:]
	}
	s.task.Logs = append(s.task.Logs, TaskLog{
		Time:  time.Now().Format("15:04:05"),
		Level: level,
		Msg:   msg,
	})
}

func (s *Server) logf(level, format string, args ...any) {
	s.mu.Lock()
	s.appendLogLocked(level, fmt.Sprintf(format, args...))
	s.mu.Unlock()
}

func (s *Server) beginTask(kind, stage string) (string, context.Context, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running {
		return "", nil, false
	}
	ctx, cancel := context.WithCancel(context.Background())
	id := fmt.Sprint(time.Now().UnixNano())
	s.running = true
	s.cancel = cancel
	s.stageKey = ""
	s.stepKey = ""
	s.task = TaskState{ID: id, Kind: kind, Stage: stage, Pct: 0}
	key, current := stageParts(stage)
	s.task.Current = current
	s.buildStepsLocked("")
	s.markStepsLocked(key)
	s.stageKey = key
	s.stepKey = ""
	if sk := s.stepKeyFor(key); sk != "" {
		s.stepKey = sk
		if label := s.stepLabelLocked(sk); label != "" {
			s.appendLogLocked("info", "开始："+label)
		}
	}
	return id, ctx, true
}

func (s *Server) progress(stage string, pct float64) {
	key, current := stageParts(stage)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.task.Stage = stage
	s.task.Pct = pct
	if current != "" {
		s.task.Current = current
	}
	if key == "" {
		return
	}
	s.stageKey = key
	s.markStepsLocked(key)
	sk := s.stepKeyFor(key)
	if sk == "" || sk == s.stepKey {
		return
	}
	s.stepKey = sk
	if label := s.stepLabelLocked(sk); label != "" {
		s.appendLogLocked("info", "开始："+label)
	}
}

func (s *Server) onEngine(engine, version string) {
	if engine == "" || engine == "unknown" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if engine == s.task.Engine {
		return
	}
	s.task.Engine = engine
	s.buildStepsLocked(engine)
	s.markStepsLocked(s.stageKey)
	name := engine
	if version != "" {
		name += " " + version
	}
	s.appendLogLocked("info", "识别引擎："+name)
}

// Handler 组装全部路由。
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/state", s.handleState)
	mux.HandleFunc("GET /api/model", s.handleModel)
	mux.HandleFunc("POST /api/unpack", s.handleUnpack)
	mux.HandleFunc("POST /api/pick", s.handlePick)
	mux.HandleFunc("POST /api/unpack-path", s.handleUnpackPath)
	mux.HandleFunc("POST /api/reveal", s.handleReveal)
	mux.HandleFunc("GET /api/task", s.handleTask)
	mux.HandleFunc("POST /api/task/cancel", s.handleCancel)
	mux.HandleFunc("POST /api/web/check", s.handleWebCheck)
	mux.HandleFunc("POST /api/web/build", s.handleWebBuild)
	mux.HandleFunc("GET /api/web", s.handleWebInfo)
	mux.HandleFunc("POST /api/web/export", s.handleWebExport)
	mux.HandleFunc("/play/", s.handlePlay)
	mux.HandleFunc("/play", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/play/", http.StatusFound)
	})
	mux.HandleFunc("POST /api/delete", s.handleDelete)
	mux.HandleFunc("GET /api/download", s.handleDownload)
	mux.HandleFunc("GET /api/headers/", s.handleHeaders)
	mux.HandleFunc("GET /api/scripts", s.handleScripts)
	mux.HandleFunc("GET /api/script", s.handleScript)
	mux.HandleFunc("/api/export/", s.handleExport)
	mux.Handle("/assets/", http.StripPrefix("/assets/",
		http.FileServer(http.Dir(filepath.Join(s.outDir, "assets")))))

	// 前端静态资源（内嵌）
	sub, err := fs.Sub(webFS, "out")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		p := strings.Trim(r.URL.Path, "/")
		if p == "" {
			fileServer.ServeHTTP(w, r) // FileServer 自带 index.html
			return
		}
		_, err1 := fs.Stat(sub, p)
		_, err2 := fs.Stat(sub, p+"/index.html")
		if err1 != nil && err2 != nil {
			// SPA fallback：未知的无扩展名路径回退到首页
			r.URL.Path = "/"
		}
		fileServer.ServeHTTP(w, r)
	})
	return mux
}

// ListenAndServe 阻塞运行。
func (s *Server) ListenAndServe() error { return http.ListenAndServe(s.o.Addr, s.Handler()) }

// ---- handlers ----

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	t, game, web := s.task, s.game, s.web
	s.mu.Unlock()
	mode := "browse"
	if s.o.BaseDir != "" {
		mode = "workspace"
	}
	writeJSON(w, map[string]any{
		"version":  s.o.Version,
		"mode":     mode,
		"hasModel": s.hasModel(),
		"game":     game,
		"dataDir":  s.outDir,
		"task":     t,
		"web":      web,
	})
}

// handleReveal 在系统文件管理器中显示目录（默认输出目录，可用 ?path= 指定导出目录）。
func (s *Server) handleReveal(w http.ResponseWriter, r *http.Request) {
	target := strings.TrimSpace(r.URL.Query().Get("path"))
	if target == "" {
		target = s.outDir
	} else if !filepath.IsAbs(target) {
		httpErrorJSON(w, http.StatusBadRequest, "路径必须是绝对路径")
		return
	}
	if _, err := os.Stat(target); err != nil {
		httpErrorJSON(w, http.StatusNotFound, "目录还不存在："+target)
		return
	}
	if err := revealPath(target); err != nil {
		httpErrorJSON(w, http.StatusInternalServerError, "打开文件管理器失败: "+err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleModel(w http.ResponseWriter, r *http.Request) {
	p := filepath.Join(s.outDir, "model.json")
	b, err := os.ReadFile(p)
	if err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(map[string]any{
			"error": "尚未解包：把游戏 zip 或文件夹拖进窗口，或使用 文件 ▸ 打开",
		})
		return
	}
	// 兼容旧数据：把 null 数组归一化为 []，避免前端 null.length 崩溃
	var m model.GameModel
	if json.Unmarshal(b, &m) == nil {
		m.Normalize()
		if b2, e := json.Marshal(&m); e == nil {
			b = b2
		}
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(b)
}

func (s *Server) handleUnpack(w http.ResponseWriter, r *http.Request) {
	if s.o.BaseDir == "" {
		httpErrorJSON(w, http.StatusForbidden,
			"浏览模式（serve <目录>）不支持上传；直接双击 dissect 启动即可拖拽解包")
		return
	}
	id, ctx, ok := s.beginTask("unpack", "接收文件")
	if !ok {
		httpErrorJSON(w, http.StatusConflict, "已有解包任务进行中，请等待完成")
		return
	}
	s.logf("info", "接收上传文件…")

	incoming := filepath.Join(s.o.BaseDir, "incoming", id)
	if err := os.MkdirAll(incoming, 0o755); err != nil {
		s.failTask(id, "创建暂存目录失败: "+err.Error())
		httpErrorJSON(w, http.StatusInternalServerError, "创建暂存目录失败: "+err.Error())
		return
	}
	n, err := saveMultipart(r, incoming)
	if err != nil || n == 0 {
		os.RemoveAll(incoming)
		msg := "上传内容为空"
		if err != nil {
			msg = "接收上传失败: " + err.Error()
		}
		s.failTask(id, msg)
		httpErrorJSON(w, http.StatusBadRequest, msg)
		return
	}
	s.logf("info", "接收完成：%d 个文件", n)

	src, err := pickSource(incoming)
	if err != nil {
		os.RemoveAll(incoming)
		s.failTask(id, err.Error())
		httpErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	go s.runUnpack(ctx, id, src, incoming)
	writeJSON(w, map[string]any{"id": id})
}

// handlePick 弹出系统原生选择框（不读取文件内容，只返回路径）。
func (s *Server) handlePick(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	if kind != "zip" && kind != "export-dir" {
		kind = "dir"
	}
	if s.o.BaseDir == "" && kind != "export-dir" {
		httpErrorJSON(w, http.StatusForbidden, "浏览模式不需要打开：当前展示的就是指定目录")
		return
	}
	path, err := pickDialog(kind)
	if err != nil {
		writeJSON(w, map[string]any{"path": "", "error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"path": path})
}

// handleUnpackPath 直接解包本地路径（本地 WebUI：服务端与浏览器同机，无需上传）。
func (s *Server) handleUnpackPath(w http.ResponseWriter, r *http.Request) {
	if s.o.BaseDir == "" {
		httpErrorJSON(w, http.StatusForbidden, "浏览模式不支持打开新目录")
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		httpErrorJSON(w, http.StatusBadRequest, "缺少 path 参数")
		return
	}
	st, err := statPath(req.Path)
	if err != nil {
		httpErrorJSON(w, http.StatusBadRequest, "路径不存在或不可访问: "+req.Path)
		return
	}
	id, ctx, ok := s.beginTask("unpack", "准备解包")
	if !ok {
		httpErrorJSON(w, http.StatusConflict, "已有解包任务进行中，请等待完成")
		return
	}
	s.logf("info", "打开：%s", req.Path)

	// 目录 / zip 直接解；裸 .rpa 用硬链接（失败则复制）构造 game/ 结构供识别
	src, incoming := req.Path, ""
	if !st.IsDir() {
		ext := strings.ToLower(filepath.Ext(req.Path))
		switch ext {
		case ".zip":
			// extract.Unpack 原生支持 zip
		case ".rpa":
			incoming = filepath.Join(s.o.BaseDir, "incoming", id)
			g := filepath.Join(incoming, "game")
			if err := os.MkdirAll(g, 0o755); err != nil {
				s.failTask(id, "创建暂存目录失败: "+err.Error())
				httpErrorJSON(w, http.StatusInternalServerError, "创建暂存目录失败: "+err.Error())
				return
			}
			dst := filepath.Join(g, filepath.Base(req.Path))
			if os.Link(req.Path, dst) != nil {
				if err := copyFile(req.Path, dst); err != nil {
					os.RemoveAll(incoming)
					s.failTask(id, "暂存 .rpa 失败: "+err.Error())
					httpErrorJSON(w, http.StatusInternalServerError, "暂存 .rpa 失败: "+err.Error())
					return
				}
			}
			src = incoming
		default:
			s.failTask(id, "仅支持游戏目录、zip 或 .rpa："+filepath.Base(req.Path))
			httpErrorJSON(w, http.StatusBadRequest,
				"仅支持游戏目录、zip 或 .rpa："+filepath.Base(req.Path))
			return
		}
	}
	go s.runUnpack(ctx, id, src, incoming)
	writeJSON(w, map[string]any{"id": id})
}

func (s *Server) handleCancel(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if !s.running || s.task.Done {
		s.mu.Unlock()
		httpErrorJSON(w, http.StatusConflict, "没有进行中的任务")
		return
	}
	cancel := s.cancel
	s.task.Current = "正在停止任务…"
	s.appendLogLocked("warn", "收到取消请求，正在停止…")
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleTask(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	t := s.task
	s.mu.Unlock()
	writeJSON(w, t)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if s.o.BaseDir == "" {
		httpErrorJSON(w, http.StatusForbidden, "浏览模式不可删除外部目录")
		return
	}
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		httpErrorJSON(w, http.StatusConflict, "解包任务进行中，稍后再试")
		return
	}
	s.running = true
	s.mu.Unlock()

	err := os.RemoveAll(s.outDir)
	_ = os.MkdirAll(s.outDir, 0o755)
	s.mu.Lock()
	s.task = TaskState{Done: true}
	s.game = ""
	s.web = WebInfo{}
	s.stageKey = ""
	s.stepKey = ""
	s.running = false
	s.mu.Unlock()
	if err != nil {
		httpErrorJSON(w, http.StatusInternalServerError, "删除失败: "+err.Error())
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	if !s.hasModel() {
		httpErrorJSON(w, http.StatusNotFound, "没有可保存的解包结果")
		return
	}
	name := s.currentGame()
	if name == "" {
		name = "dissect-result"
	}
	name += ".zip"
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(name))

	zw := zip.NewWriter(w)
	dir := s.outDir
	filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d == nil || d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if rel == ".work_src" || strings.HasPrefix(rel, ".work_src/") {
			return nil // zip 解压临时目录不打包
		}
		h := &zip.FileHeader{Name: rel, Method: zip.Deflate}
		h.SetMode(0o644)
		fw, err := zw.CreateHeader(h)
		if err != nil {
			return nil
		}
		src, err := os.Open(p)
		if err != nil {
			return nil
		}
		defer src.Close()
		_, _ = io.Copy(fw, src)
		return nil
	})
	zw.Close()
}

func (s *Server) handleHeaders(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/headers/")
	if strings.ContainsAny(name, "/\\.") &&
		!strings.HasSuffix(name, ".h") && !strings.HasSuffix(name, ".cs") {
		http.Error(w, "bad name", http.StatusBadRequest)
		return
	}
	p := filepath.Join(s.outDir, "exports", filepath.Base(name))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	http.ServeFile(w, r, p)
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/export/")
	p := filepath.Join(s.outDir, "exports", filepath.Base(name))
	if _, err := os.Stat(p); err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, p)
}

// ---- 脚本查看 ----

type scriptEntry struct {
	Name  string `json:"name"`
	Lines int    `json:"lines"`
	Size  int64  `json:"size"`
}

func (s *Server) scriptsDir() string { return filepath.Join(s.outDir, "scripts") }

func (s *Server) handleScripts(w http.ResponseWriter, r *http.Request) {
	dir := s.scriptsDir()
	ents, err := os.ReadDir(dir)
	if err != nil {
		writeJSON(w, []scriptEntry{})
		return
	}
	out := make([]scriptEntry, 0, len(ents))
	for _, e := range ents {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".rpy") {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		lines := 0
		if b, err := os.ReadFile(filepath.Join(dir, e.Name())); err == nil {
			lines = bytes.Count(b, []byte{'\n'}) + 1
		}
		out = append(out, scriptEntry{Name: e.Name(), Lines: lines, Size: fi.Size()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	writeJSON(w, out)
}

func (s *Server) handleScript(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" || filepath.Base(name) != name || !strings.HasSuffix(strings.ToLower(name), ".rpy") {
		httpErrorJSON(w, http.StatusBadRequest, "脚本名不正确")
		return
	}
	b, err := os.ReadFile(filepath.Join(s.scriptsDir(), name))
	if err != nil {
		httpErrorJSON(w, http.StatusNotFound, "找不到脚本："+name)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(b)
}

// ---- Web 版 ----

func (s *Server) loadModel() (*model.GameModel, error) {
	b, err := os.ReadFile(filepath.Join(s.outDir, "model.json"))
	if err != nil {
		return nil, errors.New("请先解包一个游戏")
	}
	var m model.GameModel
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	m.Normalize()
	return &m, nil
}

func (s *Server) handleWebCheck(w http.ResponseWriter, r *http.Request) {
	m, err := s.loadModel()
	if err != nil {
		httpErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, webbuild.Inspect(s.outDir, m))
}

func (s *Server) handleWebInfo(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	web := s.web
	s.mu.Unlock()
	writeJSON(w, web)
}

func (s *Server) handleWebBuild(w http.ResponseWriter, r *http.Request) {
	if s.o.BaseDir == "" {
		httpErrorJSON(w, http.StatusForbidden, "浏览模式不支持生成 Web 版")
		return
	}
	m, err := s.loadModel()
	if err != nil {
		httpErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	var req struct {
		AllowWindows bool `json:"allowWindows"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	c := webbuild.Inspect(s.outDir, m)
	if !c.Supported {
		httpErrorJSON(w, http.StatusBadRequest, c.Reason)
		return
	}
	if len(c.WindowsImports) > 0 && !req.AllowWindows {
		httpErrorJSON(w, http.StatusConflict,
			"检测到仅 Windows 可用的导入："+strings.Join(c.WindowsImports, "、")+"；如继续，相关功能会失效")
		return
	}
	s.mu.Lock()
	busyExport := s.exporting
	s.mu.Unlock()
	if busyExport {
		httpErrorJSON(w, http.StatusConflict, "正在导出静态站点，请等待完成")
		return
	}
	id, ctx, ok := s.beginTask("web", "web-check")
	if !ok {
		httpErrorJSON(w, http.StatusConflict, "已有任务进行中，请等待完成")
		return
	}
	go s.runWebBuild(ctx, id, c, m, req.AllowWindows)
	writeJSON(w, map[string]any{"id": id})
}

func (s *Server) runWebBuild(ctx context.Context, id string, c webbuild.Check, m *model.GameModel, allowWindows bool) {
	defer s.resetRunning()
	stubs := []string{}
	if allowWindows {
		stubs = c.WindowsImports
		if len(stubs) > 0 {
			s.logf("warn", "已注入兼容桩：%s（相关系统功能不可用）", strings.Join(stubs, "、"))
		}
	}
	_, err := webbuild.Build(webbuild.Options{
		OutDir:    s.outDir,
		CacheDir:  filepath.Join(s.o.BaseDir, "runtimes"),
		Version:   c.Version,
		Title:     m.GameName,
		SourceDir: m.SourceDir,
		Live2D:    c.Live2D,
		Stubs:     stubs,
		Ctx:       ctx,
		Progress:  s.progress,
		Log:       func(level, msg string) { s.logf(level, "%s", msg) },
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			s.finishCancelled(id)
			return
		}
		s.finish(id, false, err.Error(), "")
		return
	}
	s.finish(id, true, "", m.GameName)
}

func (s *Server) handleWebExport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Dest   string `json:"dest"`
		Format string `json:"format"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpErrorJSON(w, http.StatusBadRequest, "请求格式不正确")
		return
	}
	webDir := filepath.Join(s.outDir, "web")
	if _, err := os.Stat(filepath.Join(webDir, "index.html")); err != nil {
		httpErrorJSON(w, http.StatusBadRequest, "还没有生成 Web 版，请先生成")
		return
	}
	s.mu.Lock()
	if s.exporting || s.running {
		s.mu.Unlock()
		httpErrorJSON(w, http.StatusConflict, "正在导出或生成，请等待当前操作完成")
		return
	}
	s.exporting = true
	title := s.game
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.exporting = false
		s.mu.Unlock()
	}()

	var meta webbuild.Meta
	if b, err := os.ReadFile(filepath.Join(webDir, "meta.json")); err == nil {
		_ = json.Unmarshal(b, &meta)
	}
	if title == "" {
		title = meta.Title
	}
	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	flusher, _ := w.(http.Flusher)
	emit := func(v map[string]any) {
		b, err := json.Marshal(v)
		if err != nil {
			return
		}
		if _, err := w.Write(append(b, '\n')); err == nil && flusher != nil {
			flusher.Flush()
		}
	}

	var lastReport time.Time
	res, err := webbuild.ExportStatic(webbuild.ExportOptions{
		SrcDir:  webDir,
		Dest:    req.Dest,
		Title:   title,
		Version: meta.Version,
		Zip:     strings.EqualFold(req.Format, "zip"),
		Progress: func(done, total int64) {
			if done < total && time.Since(lastReport) < 120*time.Millisecond {
				return
			}
			lastReport = time.Now()
			emit(map[string]any{"stage": "progress", "done": done, "total": total})
		},
	})
	if err != nil {
		emit(map[string]any{"stage": "error", "error": err.Error()})
		return
	}
	s.logf("success", "静态站点已导出：%s（%.1f MB）", res.Path, float64(res.Bytes)/1048576)
	emit(map[string]any{
		"stage": "done",
		"path":  res.Path,
		"bytes": res.Bytes,
		"files": res.Files,
	})
}

func (s *Server) handlePlay(w http.ResponseWriter, r *http.Request) {
	rel := strings.TrimPrefix(r.URL.Path, "/play/")
	rel = strings.TrimPrefix(path.Clean("/"+rel), "/")
	if rel == "" || strings.HasSuffix(r.URL.Path, "/") {
		rel = path.Join(rel, "index.html")
	}
	root := filepath.Join(s.outDir, "web")
	full := filepath.Join(root, filepath.FromSlash(rel))
	if !strings.HasPrefix(full, root+string(os.PathSeparator)) {
		http.NotFound(w, r)
		return
	}
	if _, err := os.Stat(full); err != nil {
		http.NotFound(w, r)
		return
	}
	switch strings.ToLower(filepath.Ext(full)) {
	case ".wasm":
		w.Header().Set("Content-Type", "application/wasm")
	case ".data", ".zip":
		w.Header().Set("Content-Type", "application/octet-stream")
	case ".js":
		w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
	w.Header().Set("Cross-Origin-Embedder-Policy", "require-corp")
	http.ServeFile(w, r, full)
}

// ---- 解包任务 ----

// runUnpack 在后台执行解包，进度写入 task。
// incoming 仅在走暂存目录（上传 / 裸 .rpa 硬链接）时非空且位于 BaseDir/incoming 下，
// 结束后清理；直接解包用户本地目录/zip 时必须为空，绝不触碰用户数据。
func (s *Server) runUnpack(ctx context.Context, id, src, incoming string) {
	defer func() {
		if incoming != "" && strings.HasPrefix(incoming, filepath.Join(s.o.BaseDir, "incoming")) {
			os.RemoveAll(incoming)
		}
		s.resetRunning()
	}()
	if err := os.RemoveAll(s.outDir); err != nil {
		s.finish(id, false, "清空输出目录失败: "+err.Error(), "")
		return
	}
	s.mu.Lock()
	s.web = WebInfo{}
	s.mu.Unlock()
	m, err := extract.Unpack(src, s.outDir, extract.Opts{
		Python:        s.o.Python,
		UnrpycDir:     s.o.UnrpycDir,
		Il2CppDumper:  s.o.Il2CppDumper,
		UnityPyWorker: s.o.UnityPyWorker,
		Ctx:           ctx,
		Progress:      s.progress,
		Log:           func(level, msg string) { s.logf(level, "%s", msg) },
		OnEngine:      s.onEngine,
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			s.finishCancelled(id)
			return
		}
		s.finish(id, false, err.Error(), "")
		return
	}
	s.finish(id, true, "", m.GameName)
}

func (s *Server) finish(id string, ok bool, errMsg, game string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.task.ID = id
	s.task.Stage = "完成"
	s.task.Done = true
	s.task.OK = ok
	s.task.Error = errMsg
	s.task.Game = game
	if ok {
		s.task.Pct = 1
		if s.task.Kind == "web" {
			s.task.Current = "Web 版已生成"
			for i := range s.task.Steps {
				switch s.task.Steps[i].State {
				case "running", "pending", "":
					s.task.Steps[i].State = "done"
				}
			}
			s.appendLogLocked("success", "Web 版生成完成")
			s.web = readWebInfo(s.outDir)
			return
		}
		s.task.Current = "任务完成"
		for i := range s.task.Steps {
			switch s.task.Steps[i].State {
			case "running":
				s.task.Steps[i].State = "done"
			case "pending", "":
				s.task.Steps[i].State = "skipped"
			}
		}
		s.appendLogLocked("success", "解包完成："+game)
		if game != "" {
			s.game = game
		}
		return
	}
	s.task.Current = "任务失败"
	for i := range s.task.Steps {
		if s.task.Steps[i].State == "running" {
			s.task.Steps[i].State = "error"
		}
	}
	s.appendLogLocked("error", errMsg)
}

func (s *Server) finishCancelled(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.task.ID = id
	s.task.Stage = "完成"
	s.task.Done = true
	s.task.OK = false
	s.task.Error = "任务已取消"
	s.task.Current = "任务已取消"
	for i := range s.task.Steps {
		st := &s.task.Steps[i]
		if st.State == "running" || st.State == "pending" {
			st.State = "skipped"
		}
	}
	s.appendLogLocked("warn", "任务已取消")
}

func (s *Server) failTask(id, msg string) {
	s.mu.Lock()
	s.task.ID = id
	s.task.Stage = "完成"
	s.task.Done = true
	s.task.OK = false
	s.task.Error = msg
	s.task.Current = "任务失败"
	for i := range s.task.Steps {
		if s.task.Steps[i].State == "running" {
			s.task.Steps[i].State = "error"
		}
	}
	s.appendLogLocked("error", msg)
	s.mu.Unlock()
	s.resetRunning()
}

func (s *Server) resetRunning() {
	s.mu.Lock()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	s.running = false
	s.mu.Unlock()
}

// pickSource 根据上传内容决定解包输入：
// 单个 zip → 直接解；单个 .rpa → 构造 game/ 目录；其余（散文件/目录）原样交给 findGameRoot。
func pickSource(incoming string) (string, error) {
	ents, err := os.ReadDir(incoming)
	if err != nil {
		return "", err
	}
	var files []string
	for _, e := range ents {
		if !e.IsDir() {
			files = append(files, filepath.Join(incoming, e.Name()))
		}
	}
	if len(files) == 1 {
		name := filepath.Base(files[0])
		switch {
		case strings.EqualFold(filepath.Ext(name), ".zip"):
			return files[0], nil
		case strings.EqualFold(filepath.Ext(name), ".rpa"):
			g := filepath.Join(incoming, "game")
			if err := os.MkdirAll(g, 0o755); err != nil {
				return "", err
			}
			if err := os.Rename(files[0], filepath.Join(g, name)); err != nil {
				return "", err
			}
			return incoming, nil
		}
	}
	return incoming, nil
}

// saveMultipart 把 multipart 里的文件部分按相对路径流式落盘。
func saveMultipart(r *http.Request, dstRoot string) (int, error) {
	mr, err := r.MultipartReader()
	if err != nil {
		return 0, err
	}
	n := 0
	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return n, err
		}
		if p.FileName() == "" {
			continue // 普通表单字段
		}
		rel := sanitizeRelName(p.FileName())
		if rel == "" {
			continue
		}
		dst := filepath.Join(dstRoot, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return n, err
		}
		f, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return n, err
		}
		_, err = io.Copy(f, p)
		f.Close()
		if err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

// sanitizeRelName 清理上传相对路径：反斜杠归一、去根符号、防目录穿越。
func sanitizeRelName(name string) string {
	name = strings.ReplaceAll(name, "\\", "/")
	name = strings.TrimPrefix(name, "/")
	if name == "" {
		return ""
	}
	name = filepath.ToSlash(filepath.Clean("/" + name)) // 永远落在根内
	name = strings.TrimPrefix(name, "/")
	if name == "" || name == "." {
		return ""
	}
	return name
}

// ---- 小工具 ----

func (s *Server) hasModel() bool {
	_, err := os.Stat(filepath.Join(s.outDir, "model.json"))
	return err == nil
}

func (s *Server) currentGame() string {
	s.mu.Lock()
	g := s.game
	s.mu.Unlock()
	if g == "" {
		g = readGameName(s.outDir)
	}
	return g
}

func readGameName(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, "model.json"))
	if err != nil {
		return ""
	}
	var m struct {
		GameName string `json:"gameName"`
	}
	if json.Unmarshal(b, &m) != nil {
		return ""
	}
	return m.GameName
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(v)
}

func httpErrorJSON(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{"error": msg})
}

// copyFile 兜底复制（os.Link 跨卷失败时暂存 .rpa 用）。
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
