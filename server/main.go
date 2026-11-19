// dissect —— 通用游戏解包与逆向分析工具。
// 双击 exe（无参数）直接进入 WebUI 工作区模式；CLI: unpack / unity / detect / serve。详细用法见 README。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"dissect/internal/api"
	"dissect/internal/detect"
	"dissect/internal/extract"
	"dissect/internal/model"
)

var version = "0.4.0"

func main() {
	if len(os.Args) < 2 {
		// 无参数（双击 exe / 直接运行）：进入 WebUI 工作区模式，自动选端口并打开浏览器
		runWeb("", true)
		return
	}
	switch os.Args[1] {
	case "unpack":
		fs := flag.NewFlagSet("unpack", flag.ExitOnError)
		out := fs.String("o", "./out", "输出目录")
		python := fs.String("python", "", "python 解释器路径（默认自动探测）")
		unrpycDir := fs.String("unrpyc", defaultUnrpyc(), "unrpyc 目录")
		work := fs.String("work", "", "zip 解压临时目录（默认 <out>/.work_src）")
		pos := parseFlexible(fs, os.Args[2:])
		if len(pos) < 1 {
			fmt.Println("用法: dissect unpack <游戏zip或目录> [-o 输出目录]")
			os.Exit(2)
		}
		progress := func(stage string, pct float64) {
			fmt.Printf("[%5.1f%%] %s\n", pct*100, stage)
		}
		m, err := extract.Unpack(pos[0], *out, extract.Opts{
			WorkDir: *work, Python: *python, UnrpycDir: *unrpycDir, Progress: progress,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "错误:", err)
			os.Exit(1)
		}
		printSummary(m, *out)

	case "unity":
		fs := flag.NewFlagSet("unity", flag.ExitOnError)
		out := fs.String("o", "./out", "输出目录")
		python := fs.String("python", "", "python 解释器路径（默认自动探测）")
		il2cppdumper := fs.String("il2cppdumper", "", "Il2CppDumper 可执行文件路径")
		fs.Parse(os.Args[2:])
		pos := parseFlexible(fs, os.Args[2:])
		if len(pos) < 1 {
			fmt.Println("用法: dissect unity <游戏目录> [-o 输出目录] [--il2cppdumper 路径]")
			os.Exit(2)
		}
		progress := func(stage string, pct float64) {
			fmt.Printf("[%5.1f%%] %s\n", pct*100, stage)
		}
		m, err := extract.Unpack(pos[0], *out, extract.Opts{
			Python: *python, Il2CppDumper: *il2cppdumper,
			UnityPyWorker: defaultUnityPy(), Progress: progress,
		})
		if err != nil {
			fmt.Fprintln(os.Stderr, "错误:", err)
			os.Exit(1)
		}
		printSummary(m, *out)

	case "detect":
		fs := flag.NewFlagSet("detect", flag.ExitOnError)
		pos := parseFlexible(fs, os.Args[2:])
		if len(pos) < 1 {
			fmt.Println("用法: dissect detect <目录>")
			os.Exit(2)
		}
		info := detect.Detect(pos[0])
		b, _ := json.MarshalIndent(info, "", "  ")
		fmt.Println(string(b))

	case "serve":
		fs := flag.NewFlagSet("serve", flag.ExitOnError)
		addr := fs.String("addr", "", "监听地址（默认 127.0.0.1 自动选端口）")
		pos := parseFlexible(fs, os.Args[2:])
		if len(pos) < 1 {
			// serve 不带目录：工作区模式（可上传/删除）
			runWeb(*addr, *addr == "")
			return
		}
		// serve <目录>：只读浏览已有结果
		if *addr == "" {
			*addr = "127.0.0.1:0"
		}
		ln, err := net.Listen("tcp", *addr)
		if err != nil {
			fmt.Fprintln(os.Stderr, "错误:", err)
			os.Exit(1)
		}
		url := fmt.Sprintf("http://%s", ln.Addr().String())
		if h, p, _ := net.SplitHostPort(ln.Addr().String()); h == "::" || h == "0.0.0.0" || h == "[::]" {
			url = fmt.Sprintf("http://localhost:%s", p)
		}
		fmt.Printf("Web UI: %s  (Ctrl+C 退出)\n", url)
		srv := api.New(api.Options{
			Addr: *addr, FixedDir: pos[0], Version: version,
			UnrpycDir: defaultUnrpyc(), UnityPyWorker: defaultUnityPy(),
		})
		if err := http.Serve(ln, srv.Handler()); err != nil {
			fmt.Fprintln(os.Stderr, "错误:", err)
			os.Exit(1)
		}

	case "version":
		fmt.Println("dissect v" + version)

	default:
		usage()
		os.Exit(2)
	}
}

// runWeb 启动 WebUI 工作区模式。addr 为空时监听 127.0.0.1 自动选端口；open 为 true 时打开浏览器。
func runWeb(addr string, open bool) {
	base, err := defaultWorkspace()
	if err != nil {
		base = "./dissect-work"
	}
	_ = os.MkdirAll(base, 0o755)
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误: 无法监听", addr, ":", err)
		os.Exit(1)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d", ln.Addr().(*net.TCPAddr).Port)

	srv := api.New(api.Options{
		Addr: addr, BaseDir: base, Version: version,
		UnrpycDir: defaultUnrpyc(), UnityPyWorker: defaultUnityPy(),
	})
	fmt.Printf("dissect v%s\nWeb UI: %s\n工作区: %s\n把游戏 zip 或文件夹拖进浏览器窗口即可解包；关闭此窗口即退出。\n",
		version, url, base)
	if open {
		go func() {
			time.Sleep(200 * time.Millisecond)
			openBrowser(url)
		}()
	}
	if err := http.Serve(ln, srv.Handler()); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

// defaultWorkspace 交互模式的工作区目录（上传暂存与解包结果都在这里）。
func defaultWorkspace() (string, error) {
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "dissect"), nil
	}
	return filepath.Join(".", "dissect-work"), nil
}

// parseFlexible 允许标志与位置参数混排（flag 包默认遇到位置参数即停）。
func parseFlexible(fs *flag.FlagSet, args []string) []string {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") && a != "-" && a != "--" {
			flags = append(flags, a)
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flags = append(flags, args[i+1])
				i++
			}
		} else {
			pos = append(pos, a)
		}
	}
	// 标志可能带值（-o out），需要保序传给 fs.Parse
	fs.Parse(flags)
	return pos
}

func defaultUnrpyc() string {
	if _, err := os.Stat("./vendor/unrpyc/unrpyc.py"); err == nil {
		return "./vendor/unrpyc"
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "vendor", "unrpyc")
		if _, err := os.Stat(filepath.Join(p, "unrpyc.py")); err == nil {
			return p
		}
	}
	return ""
}

func defaultUnityPy() string {
	if _, err := os.Stat("./vendor/unitypy_worker.py"); err == nil {
		return "./vendor/unitypy_worker.py"
	}
	if exe, err := os.Executable(); err == nil {
		p := filepath.Join(filepath.Dir(exe), "vendor", "unitypy_worker.py")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func printSummary(m *model.GameModel, out string) {
	b, _ := json.MarshalIndent(map[string]any{
		"game":         m.GameName,
		"engine":       m.Engine + " " + m.EngineVersion,
		"dialogues":    len(m.Dialogues),
		"choices":      len(m.Choices),
		"fields":       len(m.Fields),
		"characters":   len(m.Characters),
		"flowNodes":    len(m.Flow.Nodes),
		"flowEdges":    len(m.Flow.Edges),
		"assets":       len(m.Assets),
		"scripts":      len(m.ScriptFiles),
	}, "", "  ")
	fmt.Println(string(b))
	fmt.Println("输出目录:", out)
	for _, n := range m.Notes {
		fmt.Println("备注:", n)
	}
}

func usage() {
	fmt.Printf(`dissect v%s — 通用游戏解包与逆向分析工具 (FormGLM)

双击 exe（或无参数运行）直接启动 WebUI：自动选端口并打开浏览器，
把游戏 zip / .rpa / 文件夹拖进窗口即可解包分析。

用法:
  dissect unpack <游戏zip或目录> [-o 输出目录] [--python 路径] [--unrpyc 目录]
      解包并分析游戏: 资产提取 + 脚本反编译 + 文案/选项/流程/字段提取 + Model.h 生成
  dissect unity <游戏目录> [-o 输出目录] [--il2cppdumper 路径]
      Unity IL2CPP 游戏: 调用 Il2CppDumper 生成 dump.cs/il2cpp.h 并解析字段
  dissect detect <游戏目录>
      识别引擎
  dissect serve [-addr :8080]
      WebUI 工作区模式（可上传解包）
  dissect serve <输出目录> [-addr :8080]
      只读浏览一次解包的结果
  dissect version`, version)
}
