package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"ServiceDesktop/services"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// 单实例保护
	if !trySingleton() {
		msgBox("ServiceDesktop", "ServiceDesktop 已在运行中", 0)
		os.Exit(0)
	}

	app := NewApp()

	err := wails.Run(&options.App{
		Title:     "ServiceDesktop",
		Width:     1100,
		Height:    740,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 240, G: 239, B: 237, A: 255},
		OnStartup:        app.startup,
		OnBeforeClose:    app.beforeClose,
		OnShutdown:       app.onShutdown,
		Windows: &windows.Options{
			WebviewIsTransparent: false,
		},
		Linux: &linux.Options{},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}

	// 清理锁文件
	cleanupLock()
}

// ---------- 单实例锁 ----------

func lockFilePath() string {
	return filepath.Join(os.TempDir(), "servicedesktop.lock")
}

func trySingleton() bool {
	lockFile := lockFilePath()

	// 1. 尝试创建锁文件（独占模式）
	f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err == nil {
		fmt.Fprintf(f, "%d\n", os.Getpid())
		f.Close()
		return true
	}

	// 2. 文件已存在，检查是否是有效进程
	if os.IsExist(err) {
		if !isProcessAliveFromLock(lockFile) {
			// 旧进程已死，删除锁文件重试
			os.Remove(lockFile)
			f, err := os.OpenFile(lockFile, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
			if err == nil {
				fmt.Fprintf(f, "%d\n", os.Getpid())
				f.Close()
				return true
			}
		}
		return false
	}

	return true
}

// isProcessAliveFromLock 读取锁文件中的 PID 检查进程是否存活
func isProcessAliveFromLock(lockFile string) bool {
	data, err := os.ReadFile(lockFile)
	if err != nil {
		return false
	}
	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil || pid <= 0 {
		return false
	}
	// Windows 上用 tasklist 查进程（Signal 在 Windows 不可靠）
	cmd := services.HiddenCmd("tasklist", "/fi", fmt.Sprintf("pid eq %d", pid), "/nh")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(output), strconv.Itoa(pid))
}

func cleanupLock() {
	os.Remove(lockFilePath())
}

// ---------- Windows 消息框 ----------

func msgBox(title, message string, style uintptr) {
	mod := syscall.NewLazyDLL("user32.dll")
	proc := mod.NewProc("MessageBoxW")
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(message)
	proc.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), style)
}
