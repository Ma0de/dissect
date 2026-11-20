// Package api: picker.go —— 调用系统原生文件/目录选择框。
// 本地 WebUI 工具：文件 ▸ 打开 应该弹的是系统对话框，服务端直接读本地路径，
// 不经过浏览器上传。实现保持零第三方依赖：
//   Windows  → PowerShell + WinForms OpenFileDialog（现代 IFileDialog：
//              有地址栏可直接粘贴路径、左侧驱动器栏，弃用树状 FolderBrowserDialog）
//   macOS    → osascript (choose folder / choose file)
//   Linux    → zenity / kdialog（无桌面环境时返回可读错误，前端回退到粘贴路径 / 拖拽上传）
//
// 中文路径修复：PowerShell 管道输出默认用系统 ANSI 代码页（中文系统 = GBK），
// Go 侧按 UTF-8 解码会得到乱码路径。所有脚本先强制
// [Console]::OutputEncoding = UTF8，保证 Write-Output 出来的就是 UTF-8 字节。
package api

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// winDirPlaceholder 目录选框的占位文件名：进入目标目录后点“打开”，
// 对话框返回 "<目录>\占位名"，Go 侧剥掉它得到真实目录。
const winDirPlaceholder = "选择此目录"

// pickDialog 弹出系统选择框。kind: "dir"（目录）或 "zip"（zip/.rpa 文件）。
// 返回所选路径；用户取消返回 ("", nil)；环境不支持返回错误。
func pickDialog(kind string) (string, error) {
	// 强制 PowerShell 输出 UTF-8（中文路径不再乱码的关键）
	const utf8pre = "[Console]::OutputEncoding=[System.Text.Encoding]::UTF8; "
	switch runtime.GOOS {
	case "windows":
		if kind == "zip" {
			script := utf8pre + `
Add-Type -AssemblyName System.Windows.Forms | Out-Null
$d = New-Object System.Windows.Forms.OpenFileDialog
$d.Title = '选择游戏 zip 或 .rpa 档案'
$d.Filter = '游戏档案 (*.zip;*.rpa)|*.zip;*.rpa|所有文件 (*.*)|*.*'
if ($d.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { Write-Output $d.FileName }`
			return runPicker("powershell", "-STA", "-NoProfile", "-Command", script)
		}
		title := "选择要解包的游戏目录（含 game/ 或 GameAssembly.dll 的那一层）"
		if kind == "export-dir" {
			title = "选择静态站点的导出位置"
		}
		script := utf8pre + `
Add-Type -AssemblyName System.Windows.Forms | Out-Null
$d = New-Object System.Windows.Forms.OpenFileDialog
$d.Title = '` + title + `'
$d.Filter = 'folders|/'
$d.ValidateNames = $false
$d.CheckFileExists = $false
$d.CheckPathExists = $true
$d.FileName = '` + winDirPlaceholder + `'
if ($d.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { Write-Output $d.FileName }`
		path, err := runPicker("powershell", "-STA", "-NoProfile", "-Command", script)
		return normalizeWinDirPick(path), err

	case "darwin":
		if kind == "zip" {
			return runPicker("osascript", "-e",
				`POSIX path of (choose file with prompt "选择游戏 zip 或 .rpa")`)
		}
		title := "选择要解包的游戏目录"
		if kind == "export-dir" {
			title = "选择静态站点的导出位置"
		}
		return runPicker("osascript", "-e",
			`POSIX path of (choose folder with prompt "`+title+`")`)

	default: // linux 及其他
		title := "选择要解包的游戏目录"
		if kind == "zip" {
			title = "选择游戏 zip 或 .rpa 档案"
		}
		if kind == "export-dir" {
			title = "选择静态站点的导出位置"
		}
		if _, err := exec.LookPath("zenity"); err == nil {
			if kind == "zip" {
				return runPicker("zenity", "--file-selection", "--title", title,
					"--file-filter", "游戏档案 | *.zip *.rpa", "--file-filter", "所有文件 | *")
			}
			return runPicker("zenity", "--file-selection", "--directory", "--title", title)
		}
		if _, err := exec.LookPath("kdialog"); err == nil {
			if kind == "zip" {
				return runPicker("kdialog", "--getopenfilename", ".", "*.zip *.rpa|游戏档案", "--title", title)
			}
			return runPicker("kdialog", "--getexistingdirectory", ".", "--title", title)
		}
		return "", fmt.Errorf("当前环境没有系统选择框（未找到 zenity/kdialog）。" +
			"本地使用请安装 zenity，或远程使用时直接粘贴路径 / 把文件拖进窗口上传")
	}
}

// normalizeWinDirPick 兼容现代对话框两种返回形态：
//   "<目录>"（部分实现直接返回目录）
//   "<目录>\<占位名>"（标准行为：占位名被当作文件名拼在末尾）
// 校验路径存在；不存在时回退到父目录。
func normalizeWinDirPick(p string) string {
	p = strings.TrimSpace(p)
	p = strings.Trim(p, "\"")
	if p == "" {
		return ""
	}
	if info, err := os.Stat(p); err == nil && info.IsDir() {
		return p
	}
	// 去掉占位文件名
	if base := filepath.Base(p); base == winDirPlaceholder {
		dir := filepath.Dir(p)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return p
}

// runPicker 执行选择框命令。约定：
//   - 有输出 → 所选路径
//   - 无输出 → 用户取消（返回 "", nil）
//   - osascript 取消码 -128 → 视为取消
//   - 其他错误 → 原样返回
func runPicker(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	path := strings.TrimSpace(string(out))
	if err != nil {
		if path == "" && (strings.Contains(err.Error(), "-128") || strings.Contains(err.Error(), "用户已取消") || strings.Contains(err.Error(), "User canceled")) {
			return "", nil
		}
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 && path == "" {
			// zenity/kdialog 取消
			return "", nil
		}
		if path != "" {
			return path, nil // 有的实现把取消信息写 stdout
		}
		return "", fmt.Errorf("%s: %v", name, err)
	}
	if path == "" {
		return "", nil // 取消
	}
	return path, nil
}

// statPath 校验路径存在并返回信息。
func statPath(p string) (os.FileInfo, error) {
	if p == "" {
		return nil, os.ErrInvalid
	}
	return os.Stat(p)
}
