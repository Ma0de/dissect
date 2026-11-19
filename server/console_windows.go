//go:build windows

// Windows 控制台中文支持：Go 的 fmt.Print 直接写 UTF-8 字节，
// 中文 Windows 控制台默认 GBK 代码页会显示乱码。启动时把输出/输入代码页
// 切到 UTF-8（等价于 chcp 65001），双击 exe 弹出的黑窗口才能正常显示中文。
package main

import "syscall"

func init() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	_, _, _ = kernel32.NewProc("SetConsoleOutputCP").Call(65001)
	_, _, _ = kernel32.NewProc("SetConsoleCP").Call(65001)
}
