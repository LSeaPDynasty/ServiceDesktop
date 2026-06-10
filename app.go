package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"ServiceDesktop/config"
	"ServiceDesktop/services"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx         context.Context
	cfg         *config.Config
	runtime     *services.Runtime
	services    []*services.Service
	i18n        map[string]string
	isQuitting  bool
	streamCancel map[string]context.CancelFunc
}

// ServiceDTO 给前端的数据传输对象
type ServiceDTO struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	InstallPath string   `json:"installPath"`
	Port        int      `json:"port"`
	Pid         int      `json:"pid"`
	Status      int      `json:"status"`
	StatusText  string   `json:"statusText"`
	Args        string   `json:"args"`
	Profiles    []string `json:"profiles"`
	Error       string   `json:"error,omitempty"`
}

func NewApp() *App {
	a := &App{
		cfg:          config.Load(),
		runtime:      services.NewRuntime(),
		streamCancel: make(map[string]context.CancelFunc),
	}
	a.loadTranslations()
	a.loadServices()
	return a
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// beforeClose 拦截关闭事件
// - 用户点 ✕ → 隐藏到托盘，不退出
// - 用户点「退出」→ 真正退出
func (a *App) beforeClose(ctx context.Context) bool {
	if a.isQuitting {
		return true // 真正退出
	}
	runtime.WindowHide(ctx)
	return false // 阻止退出，隐藏到托盘
}

// onShutdown Wails 关闭时回调
func (a *App) onShutdown(ctx context.Context) {
	// 在这里清理资源
}

// QuitApp 真正退出
func (a *App) QuitApp() {
	a.isQuitting = true
	runtime.Quit(a.ctx)
}

func (a *App) loadTranslations() {
	locale := a.cfg.Language
	if locale == "" {
		locale = "zh-Hans"
	}
	a.i18n = loadTranslationMap(locale)
	if a.i18n == nil {
		a.i18n = loadTranslationMap("en")
	}
	if a.i18n == nil {
		a.i18n = make(map[string]string)
	}
}

//go:embed i18n/*.json
var i18nFS embed.FS

func loadTranslationMap(locale string) map[string]string {
	paths := []string{
		"i18n/" + locale + ".json",
	}
	for _, path := range paths {
		data, err := i18nFS.ReadFile(path)
		if err != nil {
			continue
		}
		var m map[string]string
		if err := json.Unmarshal(data, &m); err == nil {
			return m
		}
	}
	return nil
}

func (a *App) T(key string) string {
	if v, ok := a.i18n[key]; ok {
		return v
	}
	return key
}

func (a *App) loadServices() {
	a.services = make([]*services.Service, 0)

	// 1. 加载预置插件模板
	for _, p := range services.GetAllPlugins() {
		tpl := p.Template()
		svc := tpl.Service
		// 深拷贝引用类型，避免多实例共享底层 map/slice
		svc.EnvVars = copyEnvVars(tpl.EnvVars)
		svc.Args = copyStringSlice(tpl.Args)
		if svc.InstallPath == "" || svc.InstallPath == "{install_path}" {
			if detected := services.ResolveInstallPath(tpl.DetectPaths); detected != "" {
				svc.InstallPath = detected
			}
		}
		if overridePath, ok := a.cfg.PathOverrides[svc.ID]; ok && overridePath != "" {
			svc.InstallPath = overridePath
		}
		// 应用 DiscoveredServices 中持久化的修改（仅 LogFile，StartCmd/StopCmd 由插件管理）
		for _, ds := range a.cfg.DiscoveredServices {
			if ds.ID == svc.ID {
				if ds.LogFile != "" {
					svc.LogFile = ds.LogFile
				}
				break
			}
		}
		svc.Status = a.runtime.CheckStatus(&svc)
		a.services = append(a.services, &svc)
	}

	// 2. 加载用户自定义服务
	for _, us := range a.cfg.UserServices {
		svc := services.Service{
			ID:          us.ID,
			Name:        us.Name,
			DisplayName: us.DisplayName,
			Category:    services.ServiceCategory(us.Category),
			InstallPath: us.InstallPath,
			StartCmd:    us.StartCmd,
			StopCmd:     us.StopCmd,
			Port:        us.Port,
			LogFile:     us.LogFile,
			IsTemplate:  false,
		}
		svc.Status = a.runtime.CheckStatus(&svc)
		a.services = append(a.services, &svc)
	}

	// 3. 加载已持久化的自动发现服务（确保关掉服务再重启还在列表中）
	for _, ds := range a.cfg.DiscoveredServices {
		// 跳过已在列表中的（按端口去重）
		already := false
		for _, svc := range a.services {
			if (svc.Port > 0 && svc.Port == ds.Port) || svc.ID == ds.ID {
				already = true
				break
			}
		}
		if already {
			continue
		}
		svc := services.Service{
			ID:          ds.ID,
			Name:        ds.Name,
			DisplayName: ds.DisplayName,
			Category:    services.ServiceCategory(ds.Category),
			InstallPath: ds.InstallPath,
			StartCmd:    ds.StartCmd,
			StopCmd:     ds.StopCmd,
			LogFile:     ds.LogFile,
			Port:        ds.Port,
			IsTemplate:  false,
		}
		svc.Status = a.runtime.CheckStatus(&svc)
		a.services = append(a.services, &svc)
	}

	// 4. 运行自动发现，新服务持久化到配置
	existingPorts := make(map[int]bool)
	for _, svc := range a.services {
		if svc.Port > 0 {
			existingPorts[svc.Port] = true
		}
	}
	result := services.RunDiscovery()
	for _, d := range result.Instances {
		if d.Port > 0 && existingPorts[d.Port] {
			continue
		}
		if d.Port > 0 {
			existingPorts[d.Port] = true
		}

		svc := services.Service{
			ID:          d.ID,
			Name:        d.Name,
			DisplayName: d.DisplayName,
			Category:    d.Category,
			InstallPath: d.InstallPath,
			Port:        d.Port,
			Pid:         d.Pid,
			StartCmd:    d.StartCmd,
			StopCmd:     d.StopCmd,
			LogFile:     d.LogFile,
			Status:      services.StatusRunning,
			IsTemplate:  false,
		}
		a.services = append(a.services, &svc)

		// 持久化新发现的服务
		a.cfg.DiscoveredServices = append(a.cfg.DiscoveredServices, config.DiscoveredServiceConf{
			ID:          d.ID,
			Name:        d.Name,
			DisplayName: d.DisplayName,
			Category:    string(d.Category),
			InstallPath: d.InstallPath,
			StartCmd:    d.StartCmd,
			StopCmd:     d.StopCmd,
			LogFile:     d.LogFile,
			Port:        d.Port,
		})
	}
	_ = a.cfg.Save()
}

func (a *App) GetServices() []ServiceDTO {
	a.runtime.RefreshAll(a.services)
	return a.toDTOs(a.services)
}

// StartResult 启动结果
type StartResult struct {
	Success bool      `json:"success"`
	Error   string    `json:"error,omitempty"`
	Service ServiceDTO `json:"service,omitempty"`
}

// RestartService 重启服务（停止+启动）
func (a *App) RestartService(id string) *StartResult {
	svc := a.findService(id)
	if svc == nil {
		return &StartResult{Success: false, Error: "服务未找到"}
	}
	stopErr := a.runtime.Stop(svc)
	// 轮询端口释放，最多等 15 秒，不再硬编码 sleep
	if svc.Port > 0 {
		deadline := time.Now().Add(15 * time.Second)
		for time.Now().Before(deadline) {
			if !a.runtime.IsPortOpen(svc.Port) {
				break
			}
			time.Sleep(300 * time.Millisecond)
		}
		// 如果端口仍未释放，不继续启动
		if a.runtime.IsPortOpen(svc.Port) {
			detail := "端口未能在 15 秒内释放"
			if stopErr != nil {
				detail = detail + ": " + stopErr.Error()
			}
			return &StartResult{Success: false, Error: detail, Service: a.toDTO(svc)}
		}
	}
	err := a.runtime.Start(svc)
	if err != nil {
		return &StartResult{Success: false, Error: err.Error(), Service: a.toDTO(svc)}
	}
	return &StartResult{Success: true, Service: a.toDTO(svc)}
}

// StartAllServices 并发启动所有服务（最多同时启动 4 个）
func (a *App) StartAllServices() []StartResult {
	sem := make(chan struct{}, 4) // 并发控制，最多 4 个同时启动
	var mu sync.Mutex
	var results []StartResult
	var wg sync.WaitGroup

	for _, svc := range a.services {
		if svc.Status == services.StatusRunning {
			continue
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(s *services.Service) {
			defer wg.Done()
			defer func() { <-sem }()
			err := a.runtime.Start(s)
			mu.Lock()
			if err != nil {
				results = append(results, StartResult{Success: false, Error: err.Error(), Service: a.toDTO(s)})
			} else {
				results = append(results, StartResult{Success: true, Service: a.toDTO(s)})
			}
			mu.Unlock()
		}(svc)
	}
	wg.Wait()
	return results
}

// CheckPort 检查端口占用情况
func (a *App) CheckPort(port int) services.PortCheckResult {
	return a.runtime.CheckPortAvailable(port)
}

// KillProcess 强制杀掉指定 PID 的进程
func (a *App) KillProcess(pid int) string {
	err := a.runtime.KillProcessByPid(pid)
	if err != nil {
		return "终止失败: " + err.Error()
	}
	return "ok"
}

func (a *App) StopService(id string) ServiceDTO {
	svc := a.findService(id)
	if svc == nil {
		dto := ServiceDTO{}
		dto.Error = "服务未找到"
		return dto
	}
	err := a.runtime.Stop(svc)
	dto := a.toDTO(svc)
	if err != nil {
		dto.Error = err.Error()
	}
	// 推送状态变更到前端
	a.pushStatusUpdate()
	return dto
}

func (a *App) StartService(id string) *StartResult {
	svc := a.findService(id)
	if svc == nil {
		return &StartResult{Success: false, Error: "服务未找到"}
	}
	err := a.runtime.Start(svc)
	a.pushStatusUpdate()
	if err != nil {
		return &StartResult{Success: false, Error: err.Error(), Service: a.toDTO(svc)}
	}
	return &StartResult{Success: true, Service: a.toDTO(svc)}
}

// pushStatusUpdate 向 UI 推送最新的服务状态列表
func (a *App) pushStatusUpdate() {
	if a.ctx == nil {
		return
	}
	dtos := a.toDTOs(a.services)
	data, err := json.Marshal(dtos)
	if err != nil {
		return
	}
	runtime.EventsEmit(a.ctx, "services-update", string(data))
}

func (a *App) ReadConfigFile(serviceID, fileName string) string {
	svc := a.findService(serviceID)
	if svc == nil || svc.InstallPath == "" {
		return ""
	}
	path := filepath.Join(svc.InstallPath, fileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return "// Error: " + err.Error()
	}
	return string(data)
}

func (a *App) SaveConfigFile(serviceID, fileName, content string) string {
	svc := a.findService(serviceID)
	if svc == nil || svc.InstallPath == "" {
		return "service not found"
	}
	path := filepath.Join(svc.InstallPath, fileName)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "save failed: " + err.Error()
	}
	return "ok"
}

func (a *App) GetConfigFiles(serviceID string) []string {
	svc := a.findService(serviceID)
	if svc == nil || svc.InstallPath == "" {
		return []string{}
	}

	// 优先委托给插件
	if p, ok := services.GetPlugin(svc.ID); ok {
		return p.ConfigFiles(svc.InstallPath)
	}

	// 无插件匹配的服务（自定义/发现）返回空
	return []string{}
}

// GetLogContent 读取日志内容（兼容旧模式：直接文件或第一个日志文件）
func (a *App) GetLogContent(serviceID string) string {
	svc := a.findService(serviceID)
	if svc == nil || svc.LogFile == "" {
		return "// No log file configured"
	}
	path := strings.ReplaceAll(svc.LogFile, "{install_path}", svc.InstallPath)

	// 如果是目录，读取第一个非空文件（按修改时间倒序）
	if strings.HasSuffix(path, "\\") || strings.HasSuffix(path, "/") {
		entries, err := os.ReadDir(path)
		if err != nil {
			return ""
		}
		type fileInfo struct {
			name string
			mod  time.Time
		}
		var files []fileInfo
		for _, e := range entries {
			if !e.IsDir() {
				fi, _ := e.Info()
				if fi == nil || fi.Size() == 0 {
					continue
				}
				files = append(files, fileInfo{name: e.Name(), mod: fi.ModTime()})
			}
		}
		// 按修改时间倒序（最新在前）
		sort.Slice(files, func(i, j int) bool {
			return files[i].mod.After(files[j].mod)
		})
		for _, f := range files {
			data, err := os.ReadFile(filepath.Join(path, f.name))
			if err != nil {
				continue
			}
			return string(data)
		}
		return ""
	}

	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		// 无后缀分隔符的目录
		return ""
	}

	if strings.Contains(path, "*") {
		matches, _ := filepath.Glob(path)
		if len(matches) > 0 {
			data, err := os.ReadFile(matches[0])
			if err == nil {
				return string(data)
			}
		}
		return ""
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

// GetLogFiles 列出服务的所有日志文件
func (a *App) GetLogFiles(serviceID string) []string {
	svc := a.findService(serviceID)
	if svc == nil || svc.LogFile == "" {
		return nil
	}
	resolved := strings.ReplaceAll(svc.LogFile, "{install_path}", svc.InstallPath)

	// 判断是否为目录路径
	isDir := strings.HasSuffix(resolved, "\\") || strings.HasSuffix(resolved, "/")
	if !isDir {
		if fi, err := os.Stat(resolved); err == nil && fi.IsDir() {
			isDir = true
		}
	}

	if isDir {
		entries, err := os.ReadDir(resolved)
		if err != nil {
			return nil
		}
		type fileInfo struct {
			name string
			mod  time.Time
		}
		var files []fileInfo
		for _, e := range entries {
			if !e.IsDir() {
				fi, _ := e.Info()
				if fi == nil || fi.Size() == 0 {
					continue
				}
				files = append(files, fileInfo{name: e.Name(), mod: fi.ModTime()})
			}
		}
		// 按修改时间倒序（最新在前）
		sort.Slice(files, func(i, j int) bool {
			return files[i].mod.After(files[j].mod)
		})
		names := make([]string, len(files))
		for i, f := range files {
			names[i] = f.name
		}
		return names
	}

	if strings.Contains(resolved, "*") {
		matches, err := filepath.Glob(resolved)
		if err != nil || len(matches) == 0 {
			return nil
		}
		sort.Strings(matches)
		names := make([]string, 0, len(matches))
		for _, m := range matches {
			fi, err := os.Stat(m)
			if err != nil || fi.IsDir() || fi.Size() == 0 {
				continue
			}
			names = append(names, filepath.Base(m))
		}
		return names
	}

	// 兜底：单个文件（用户手动指定的日志路径）
	if fi, err := os.Stat(resolved); err == nil && !fi.IsDir() {
		return []string{filepath.Base(resolved)}
	}

	return nil
}

// LogFileEntry 描述一个日志文件
type LogFileEntry struct {
	Type string `json:"type"` // 日志类型（catalina / localhost 等）
	Date string `json:"date"` // 日期（2026-06-05）
	Name string `json:"name"` // 完整文件名
}

// GetLogGroupedFiles 返回按类型分组的日志文件列表
func (a *App) GetLogGroupedFiles(serviceID string) map[string][]LogFileEntry {
	flat := a.GetLogFiles(serviceID)
	if flat == nil {
		return nil
	}
	groups := make(map[string][]LogFileEntry)
	for _, name := range flat {
		typ := parseLogFileType(name)
		date := parseLogFileDate(name)
		groups[typ] = append(groups[typ], LogFileEntry{
			Type: typ,
			Date: date,
			Name: name,
		})
	}
	// 每组按日期倒序
	for typ := range groups {
		sort.Slice(groups[typ], func(i, j int) bool {
			return groups[typ][i].Date > groups[typ][j].Date
		})
	}
	return groups
}

// parseLogFileType 从文件名解析日志类型: "catalina.2026-06-05.log" → "catalina"
func parseLogFileType(name string) string {
	// 去掉扩展名
	name = strings.TrimSuffix(name, ".log")
	name = strings.TrimSuffix(name, ".txt")
	name = strings.TrimSuffix(name, ".out")
	// Tomcat 格式: type.date 或 type_date
	// 尝试找日期分隔符
	if idx := strings.Index(name, "."); idx > 0 {
		// 检查后面是不是日期格式
		rest := name[idx+1:]
		if isDateLike(rest) {
			return name[:idx]
		}
	}
	if idx := strings.Index(name, "-"); idx > 0 {
		rest := name[idx+1:]
		if isTwoDigitDate(rest) {
			return name[:idx]
		}
	}
	return name
}

// parseLogFileDate 从文件名解析日期: "catalina.2026-06-05.log" → "2026-06-05"
func parseLogFileDate(name string) string {
	name = strings.TrimSuffix(name, ".log")
	name = strings.TrimSuffix(name, ".txt")
	name = strings.TrimSuffix(name, ".out")
	// 找 type.date 模式
	if idx := strings.Index(name, "."); idx > 0 {
		rest := name[idx+1:]
		if isDateLike(rest) {
			return rest
		}
	}
	if idx := strings.Index(name, "-"); idx > 0 {
		rest := name[idx+1:]
		if len(rest) >= 8 {
			// 尝试从 rest 中提取日期尾部
			return rest
		}
	}
	return ""
}

func isDateLike(s string) bool {
	// 匹配 YYYY-MM-DD
	if len(s) < 10 {
		return false
	}
	return s[4] == '-' && s[7] == '-'
}

func isTwoDigitDate(s string) bool {
	// 匹配 YY-MM-DD
	if len(s) < 8 {
		return false
	}
	return s[2] == '-' && s[5] == '-'
}

// GetLogFileContent 读取指定的日志文件内容
func (a *App) GetLogFileContent(serviceID, fileName string) string {
	svc := a.findService(serviceID)
	if svc == nil || svc.LogFile == "" {
		return ""
	}
	resolved := strings.ReplaceAll(svc.LogFile, "{install_path}", svc.InstallPath)

	var baseDir string
	if strings.HasSuffix(resolved, "\\") || strings.HasSuffix(resolved, "/") {
		baseDir = resolved
	} else if fi, err := os.Stat(resolved); err == nil && fi.IsDir() {
		baseDir = resolved
	} else if strings.Contains(resolved, "*") {
		// 去除通配符部分，找到真实目录
		baseDir = resolved
		for strings.ContainsAny(baseDir, "*?[") {
			baseDir = filepath.Dir(baseDir)
		}
	} else {
		baseDir = filepath.Dir(resolved)
	}
	// 去掉末尾分隔符保证 Join 正确
	baseDir = strings.TrimRight(baseDir, "\\/")
	path := filepath.Join(baseDir, fileName)

	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) > 500 {
		lines = lines[len(lines)-500:]
	}
	return strings.Join(lines, "\n")
}

func (a *App) AddCustomService(name, displayName, category, installPath, startCmd, stopCmd, logFile, args, envVars string, port int) {
	us := config.UserServiceConf{
		ID:          "custom-" + name + "-" + fmt.Sprint(time.Now().UnixMilli()),
		Name:        name,
		DisplayName: displayName,
		Category:    category,
		InstallPath: installPath,
		StartCmd:    startCmd,
		StopCmd:     stopCmd,
		Port:        port,
		LogFile:     logFile,
		Args:        args,
		EnvVars:     envVars,
	}
	a.cfg.UserServices = append(a.cfg.UserServices, us)
	_ = a.cfg.Save()
	a.loadServices()
}

// SetInstallPath 用户手动设置服务的安装路径
func (a *App) SetInstallPath(serviceID, path string) string {
	svc := a.findService(serviceID)
	if svc == nil {
		return "服务未找到"
	}
	svc.InstallPath = path

	// 如果是自定义服务，更新持久化配置
	for i, us := range a.cfg.UserServices {
		if us.ID == serviceID {
			a.cfg.UserServices[i].InstallPath = path
			_ = a.cfg.Save()
			return "ok"
		}
	}

	// 如果是模板服务，存为路径覆盖
	if a.cfg.PathOverrides == nil {
		a.cfg.PathOverrides = make(map[string]string)
	}
	a.cfg.PathOverrides[serviceID] = path
	_ = a.cfg.Save()
	return "ok"
}

// OpenFolder 在资源管理器中打开安装路径
func (a *App) OpenFolder(serviceID string) string {
	svc := a.findService(serviceID)
	if svc == nil || svc.InstallPath == "" {
		return "路径未设置"
	}
	cmd := services.HiddenCmd("explorer", svc.InstallPath)
	_ = cmd.Start()
	return "ok"
}

// BrowseFolder 打开原生文件夹选择对话框，返回选中的路径
func (a *App) BrowseFolder() string {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择服务安装目录",
	})
	if err != nil {
		return ""
	}
	return dir
}

// SetStartArgs 设置服务的启动参数
func (a *App) SetStartArgs(serviceID, args string) string {
	svc := a.findService(serviceID)
	if svc == nil {
		return "服务未找到"
	}
	svc.Args = splitArgs(args)

	// 如果是自定义服务，持久化
	for i, us := range a.cfg.UserServices {
		if us.ID == serviceID {
			a.cfg.UserServices[i].Args = args
			_ = a.cfg.Save()
			return "ok"
		}
	}
	// 模板服务：存为默认 profile
	if a.cfg.StartProfiles == nil {
		a.cfg.StartProfiles = make(map[string]map[string]string)
	}
	if a.cfg.StartProfiles[serviceID] == nil {
		a.cfg.StartProfiles[serviceID] = make(map[string]string)
	}
	a.cfg.StartProfiles[serviceID]["_default"] = args
	_ = a.cfg.Save()
	return "ok"
}

// SaveStartProfile 保存启动参数配置（Profile）
func (a *App) SaveStartProfile(serviceID, profileName, args string) string {
	svc := a.findService(serviceID)
	if svc == nil {
		return "服务未找到"
	}
	if profileName == "" {
		return "配置名称不能为空"
	}
	if a.cfg.StartProfiles == nil {
		a.cfg.StartProfiles = make(map[string]map[string]string)
	}
	if a.cfg.StartProfiles[serviceID] == nil {
		a.cfg.StartProfiles[serviceID] = make(map[string]string)
	}
	a.cfg.StartProfiles[serviceID][profileName] = args
	_ = a.cfg.Save()
	return "ok"
}

// DeleteStartProfile 删除启动参数配置
func (a *App) DeleteStartProfile(serviceID, profileName string) string {
	if a.cfg.StartProfiles == nil || a.cfg.StartProfiles[serviceID] == nil {
		return "未找到"
	}
	delete(a.cfg.StartProfiles[serviceID], profileName)
	_ = a.cfg.Save()
	return "ok"
}

// GetStartProfiles 获取服务的所有启动参数配置
func (a *App) GetStartProfiles(serviceID string) map[string]string {
	if a.cfg.StartProfiles == nil || a.cfg.StartProfiles[serviceID] == nil {
		return map[string]string{}
	}
	return a.cfg.StartProfiles[serviceID]
}

// ========== 设置 ==========

// GetAppConfig 返回应用配置给前端
type AppConfigDTO struct {
	Language  string `json:"language"`
	AutoStart bool   `json:"autoStart"`
}

func (a *App) GetAppConfig() AppConfigDTO {
	return AppConfigDTO{
		Language:  a.cfg.Language,
		AutoStart: a.cfg.AutoStart,
	}
}

// SetAppConfig 保存应用配置
func (a *App) SetAppConfig(language string, autoStart bool) string {
	if language != "" {
		a.cfg.Language = language
		a.loadTranslations()
	}
	a.cfg.AutoStart = autoStart
	_ = a.cfg.Save()

	// 处理开机自启（Windows 注册表）
	if autoStart {
		setAutoStart(true)
	} else {
		setAutoStart(false)
	}
	return "ok"
}

// setAutoStart 设置/取消开机自启（Windows）
func setAutoStart(enable bool) {
	cmd := services.HiddenCmd("reg", "add",
		"HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run",
		"/v", "ServiceDesktop",
		"/t", "REG_SZ")
	if enable {
		exe, _ := os.Executable()
		cmd.Args = append(cmd.Args, "/d", "\""+exe+"\"")
	} else {
		cmd = services.HiddenCmd("reg", "delete",
			"HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run",
			"/v", "ServiceDesktop", "/f")
	}
	_ = cmd.Run()
}

// ========== 编辑/删除服务 ==========

// EditCustomService 编辑自定义服务
func (a *App) EditCustomService(id, name, displayName, category, installPath, startCmd, stopCmd, logFile, args, envVars string, port int) string {
	// 更新内存中的服务
	for _, svc := range a.services {
		if svc.ID == id {
			svc.Name = name
			svc.DisplayName = displayName
			svc.Category = services.ServiceCategory(category)
			svc.InstallPath = installPath
			svc.StartCmd = startCmd
			svc.StopCmd = stopCmd
			svc.Port = port
			svc.LogFile = logFile
			svc.Args = splitArgs(args)
			svc.EnvVars = parseEnvVars(envVars)
			break
		}
	}
	// 更新持久化 — 先在 UserServices 中找
	for i, us := range a.cfg.UserServices {
		if us.ID == id {
			a.cfg.UserServices[i].Name = name
			a.cfg.UserServices[i].DisplayName = displayName
			a.cfg.UserServices[i].Category = category
			a.cfg.UserServices[i].InstallPath = installPath
			a.cfg.UserServices[i].StartCmd = startCmd
			a.cfg.UserServices[i].StopCmd = stopCmd
			a.cfg.UserServices[i].Port = port
			a.cfg.UserServices[i].LogFile = logFile
			a.cfg.UserServices[i].Args = args
			a.cfg.UserServices[i].EnvVars = envVars
			_ = a.cfg.Save()
			return "ok"
		}
	}
	// 再在 DiscoveredServices 中找（自动发现已持久化的服务）
	for i, ds := range a.cfg.DiscoveredServices {
		if ds.ID == id {
			a.cfg.DiscoveredServices[i].Name = name
			a.cfg.DiscoveredServices[i].DisplayName = displayName
			a.cfg.DiscoveredServices[i].Category = category
			a.cfg.DiscoveredServices[i].InstallPath = installPath
			a.cfg.DiscoveredServices[i].StartCmd = startCmd
			a.cfg.DiscoveredServices[i].StopCmd = stopCmd
			a.cfg.DiscoveredServices[i].Port = port
			a.cfg.DiscoveredServices[i].LogFile = logFile
			_ = a.cfg.Save()
			return "ok"
		}
	}
	// 内置模板服务且未持久化过 → 新加一条到 DiscoveredServices
	a.cfg.DiscoveredServices = append(a.cfg.DiscoveredServices, config.DiscoveredServiceConf{
		ID:          id,
		Name:        name,
		DisplayName: displayName,
		Category:    category,
		InstallPath: installPath,
		StartCmd:    startCmd,
		StopCmd:     stopCmd,
		Port:        port,
		LogFile:     logFile,
	})
	_ = a.cfg.Save()
	return "ok"
}

// GetServiceDetail 获取服务的完整配置（用于编辑）
func (a *App) GetServiceDetail(id string) *ServiceDetailDTO {
	svc := a.findService(id)
	if svc == nil {
		return nil
	}
	envStr := ""
	for k, v := range svc.EnvVars {
		if envStr != "" {
			envStr += ";"
		}
		envStr += k + "=" + v
	}
	return &ServiceDetailDTO{
		ID:          svc.ID,
		Name:        svc.Name,
		DisplayName: svc.DisplayName,
		Category:    string(svc.Category),
		InstallPath: svc.InstallPath,
		StartCmd:    svc.StartCmd,
		StopCmd:     svc.StopCmd,
		Port:        svc.Port,
		LogFile:     svc.LogFile,
		Args:        strings.Join(svc.Args, " "),
		EnvVars:     envStr,
		IsTemplate:  svc.IsTemplate,
	}
}

// ServiceDetailDTO 服务完整配置
type ServiceDetailDTO struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Category    string `json:"category"`
	InstallPath string `json:"installPath"`
	StartCmd    string `json:"startCmd"`
	StopCmd     string `json:"stopCmd"`
	Port        int    `json:"port"`
	LogFile     string `json:"logFile"`
	Args        string `json:"args"`
	EnvVars     string `json:"envVars"`
	IsTemplate  bool   `json:"isTemplate"`
}

// DeleteCustomService 删除自定义服务
func (a *App) DeleteCustomService(id string) string {
	for i, svc := range a.services {
		if svc.ID == id && !svc.IsTemplate {
			a.services = append(a.services[:i], a.services[i+1:]...)
			break
		}
	}
	// 删除 UserServices
	for i, us := range a.cfg.UserServices {
		if us.ID == id {
			a.cfg.UserServices = append(a.cfg.UserServices[:i], a.cfg.UserServices[i+1:]...)
			_ = a.cfg.Save()
			return "ok"
		}
	}
	// 删除 DiscoveredServices（自动发现持久化的）
	for i, ds := range a.cfg.DiscoveredServices {
		if ds.ID == id {
			a.cfg.DiscoveredServices = append(a.cfg.DiscoveredServices[:i], a.cfg.DiscoveredServices[i+1:]...)
			_ = a.cfg.Save()
			return "ok"
		}
	}
	return "未找到或不可删除"
}

// splitArgs 将空格分隔的参数字符串拆分为切片（支持引号）
func splitArgs(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	current := ""
	inQuote := false
	for _, c := range s {
		switch {
		case c == '"':
			inQuote = !inQuote
		case c == ' ' && !inQuote:
			if current != "" {
				result = append(result, current)
				current = ""
			}
		default:
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

// parseEnvVars 将 "KEY=VAL;KEY2=VAL2" 格式的环境变量字符串转为 map
func parseEnvVars(s string) map[string]string {
	m := make(map[string]string)
	if s == "" {
		return m
	}
	for _, pair := range strings.Split(s, ";") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			m[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return m
}

// copyEnvVars 深拷贝环境变量 map
func copyEnvVars(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// copyStringSlice 深拷贝字符串切片
func copyStringSlice(src []string) []string {
	if src == nil {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

// StreamLog 开始实时推送日志到前端（通过 Wails Events），使用订阅模式
// 每次调用会取消该服务上一次的订阅，防止 goroutine 泄漏
func (a *App) StreamLog(serviceID string) string {
	// 取消旧订阅
	if cancel, ok := a.streamCancel[serviceID]; ok {
		cancel()
	}

	collector := a.runtime.Collector(serviceID)
	ch := collector.Subscribe()

	ctx, cancel := context.WithCancel(context.Background())
	a.streamCancel[serviceID] = cancel

	go func() {
		defer collector.Unsubscribe(ch)
		var buf []services.LogLine
		lastSent := 0
		ticker := time.NewTicker(800 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case line, ok := <-ch:
				if !ok {
					return
				}
				buf = append(buf, line)
			case <-ticker.C:
				if len(buf) == lastSent {
					continue
				}
				// 推送最近新增的行
				newLines := buf[lastSent:]
				lastSent = len(buf)

				data, err := json.Marshal(newLines)
				if err != nil {
					continue
				}
				runtime.EventsEmit(a.ctx, "log-update", map[string]interface{}{
					"id":    serviceID,
					"lines": string(data),
				})
			}
		}
	}()
	return "ok"
}

// GetConsoleLog 获取该服务的控制台历史日志（JSON 数组）
func (a *App) GetConsoleLog(serviceID string) string {
	collector := a.runtime.Collector(serviceID)
	snapshot := collector.Snapshot()
	data, err := json.Marshal(snapshot)
	if err != nil {
		return "[]"
	}
	return string(data)
}

// ClearConsoleLog 清空该服务的控制台缓冲区
func (a *App) ClearConsoleLog(serviceID string) string {
	collector := a.runtime.Collector(serviceID)
	collector.Clear()
	return "ok"
}

// SetWatchLogFile 设置该服务要实时 tail 的日志文件路径
// 会同时保存到服务持久化配置的 LogFile
func (a *App) SetWatchLogFile(serviceID, filePath string) string {
	collector := a.runtime.Collector(serviceID)

	if filePath == "" {
		return "ok"
	}

	// 验证文件存在
	if fi, err := os.Stat(filePath); os.IsNotExist(err) {
		return "文件不存在: " + filePath
	} else if fi.IsDir() {
		return "路径是一个目录，请选择具体日志文件"
	}

	// 保存到服务持久化配置
	for i, us := range a.cfg.UserServices {
		if us.ID == serviceID {
			a.cfg.UserServices[i].LogFile = filePath
			_ = a.cfg.Save()
			break
		}
	}
	for i, ds := range a.cfg.DiscoveredServices {
		if ds.ID == serviceID {
			a.cfg.DiscoveredServices[i].LogFile = filePath
			_ = a.cfg.Save()
			break
		}
	}
	// 也更新内存中的服务
	for _, svc := range a.services {
		if svc.ID == serviceID {
			svc.LogFile = filePath
			break
		}
	}

	// 开始 tail（collector 允许同时 tail 多个文件）
	if err := collector.TailFile(filePath); err != nil {
		return "tail 失败: " + err.Error()
	}
	return "ok"
}

// GetWatchLogFile 返回该服务的 LogFile 配置
func (a *App) GetWatchLogFile(serviceID string) string {
	svc := a.findService(serviceID)
	if svc == nil {
		return ""
	}
	return svc.LogFile
}

func (a *App) findService(id string) *services.Service {
	for _, svc := range a.services {
		if svc.ID == id {
			return svc
		}
	}
	return nil
}

// GetServiceLogSources 返回该服务的日志来源列表
// type: "process" | "file"
func (a *App) GetServiceLogSources(serviceID string) []map[string]string {
	svc := a.findService(serviceID)
	if svc == nil {
		return nil
	}

	// 判断进程输出是否可用
	processAvailable := true
	if svc.Source == "smarttomcat" || svc.Source == "idea" {
		processAvailable = false
	}

	sources := make([]map[string]string, 0)

	if processAvailable {
		sources = append(sources, map[string]string{
			"id":   "__process__",
			"name": "● 实时输出",
			"type": "process",
			"note": "",
		})
	} else {
		sources = append(sources, map[string]string{
			"id":   "__process__",
			"name": "● 实时输出",
			"type": "process",
			"note": "由 IDE 管理，无法捕获进程输出",
		})
	}

	// 扫描应用日志文件
	if svc.InstallPath != "" {
		files := services.DiscoverAppLogs(svc.InstallPath)
		if len(files) > 0 {
			for _, f := range files {
				sources = append(sources, map[string]string{
					"id":   "file:" + filepath.Base(f),
					"name": filepath.Base(f),
					"type": "file",
					"path": f,
					"note": "",
				})
			}
		} else {
			sources = append(sources, map[string]string{
				"id":   "__no_files__",
				"name": "未发现应用日志文件",
				"type": "none",
				"note": "服务启动后自动扫描，或手动设置路径",
			})
		}
	}
	return sources
}

func (a *App) toDTO(svc *services.Service) ServiceDTO {
	profileNames := a.cfg.GetProfileNames(svc.ID)
	return ServiceDTO{
		ID:          svc.ID,
		Name:        svc.DisplayName,
		Category:    string(svc.Category),
		InstallPath: svc.InstallPath,
		Port:        svc.Port,
		Pid:         svc.Pid,
		Status:      int(svc.Status),
		StatusText:  svc.Status.String(),
		Args:        strings.Join(svc.Args, " "),
		Profiles:    profileNames,
	}
}

func (a *App) toDTOs(svcs []*services.Service) []ServiceDTO {
	result := make([]ServiceDTO, len(svcs))
	for i, svc := range svcs {
		result[i] = a.toDTO(svc)
	}
	return result
}
