package main

import (
	"embed"
	"os"
	"syscall"
	"unsafe"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	_ "ServiceDesktop/services/plugins" // 触发所有插件的 init() 注册
)

//go:embed all:frontend/dist
var assets embed.FS

// Windows 命名互斥锁句柄，用于跨进程单实例保护
var singletonMutex syscall.Handle

func main() {
	// 单实例保护：Windows 命名 Mutex 是原子操作，无 TOCTOU 竞态
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

	cleanupLock()
}

// ---------- 单实例锁（Windows 命名 Mutex）----------

const mutexName = "Global\\ServiceDesktop-Singleton-{B4E9F8C2-1D3A-4E5F-8C7B-9A0D2E3F4C5B}"

func trySingleton() bool {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	createMutex := kernel32.NewProc("CreateMutexW")
	// 参数：安全属性(nil), 初始拥有者(false), 名称
	name, _ := syscall.UTF16PtrFromString(mutexName)
	ret, _, err := createMutex.Call(0, 0, uintptr(unsafe.Pointer(name)))
	h := syscall.Handle(ret)
	if h == 0 {
		return true // 保守处理
	}

	// CreateMutexW 返回 error=183(ERROR_ALREADY_EXISTS) 表示已有实例
	singletonMutex = h
	if errno, ok := err.(syscall.Errno); ok && errno == 183 {
		syscall.CloseHandle(h)
		singletonMutex = 0
		return false
	}

	return true
}

func cleanupLock() {
	if singletonMutex != 0 {
		syscall.CloseHandle(singletonMutex)
	}
}

// ---------- Windows 消息框 ----------

func msgBox(title, message string, style uintptr) {
	mod := syscall.NewLazyDLL("user32.dll")
	proc := mod.NewProc("MessageBoxW")
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(message)
	proc.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), style)
}
