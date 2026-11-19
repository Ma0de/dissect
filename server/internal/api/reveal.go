// Package api: reveal.go —— 在系统文件管理器中定位输出目录（资源管理器快速定位）。
//   Windows → explorer /select,"<目录>"
//   macOS   → open -R <目录>
//   Linux   → xdg-open <目录>
package api

import (
	"fmt"
	"os/exec"
	"runtime"
)

// revealPath 在系统文件管理器中显示 dir（Windows 下选中该目录）。
func revealPath(dir string) error {
	if dir == "" {
		return fmt.Errorf("没有可定位的输出目录")
	}
	switch runtime.GOOS {
	case "windows":
		// explorer /select, "path" —— 参数拆开传，Go 会对含空格路径自动加引号
		cmd := exec.Command("explorer", "/select,", dir)
		return cmd.Start() // explorer 立即返回，不等窗口关闭
	case "darwin":
		return exec.Command("open", "-R", dir).Start()
	default:
		return exec.Command("xdg-open", dir).Start()
	}
}
