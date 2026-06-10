package services

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ============================================================
// 日志行 & 流
// ============================================================

// LogStream 标识日志来源
type LogStream string

const (
	StreamStdout  LogStream = "stdout"
	StreamStderr  LogStream = "stderr"
	StreamFile    LogStream = "file"    // tail 日志文件
	StreamSystem  LogStream = "system"  // ServiceDesktop 自身提示
)

// LogLine 一行日志
type LogLine struct {
	Time    time.Time `json:"time"`
	Stream  LogStream `json:"stream"`
	Text    string    `json:"text"`
	// 解析后的结构化字段（可选，未能解析时为空）
	Level   string    `json:"level,omitempty"`   // INFO WARN ERROR DEBUG
	Logger  string    `json:"logger,omitempty"`  // 类名缩短版
	Thread  string    `json:"thread,omitempty"`
}

// logBuf 按行存储，固定上限，线程安全
type logBuf struct {
	mu    sync.RWMutex
	lines []LogLine
	cap   int
}

func newLogBuf(cap int) *logBuf {
	return &logBuf{cap: cap, lines: make([]LogLine, 0, cap)}
}

func (b *logBuf) append(line LogLine) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.lines) >= b.cap {
		// 丢弃最旧的 10%，避免频繁 copy
		drop := b.cap / 10
		b.lines = b.lines[drop:]
	}
	b.lines = append(b.lines, line)
}

// Snapshot 返回当前所有行的快照（调用方只读）
func (b *logBuf) Snapshot() []LogLine {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]LogLine, len(b.lines))
	copy(out, b.lines)
	return out
}

func (b *logBuf) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lines = b.lines[:0]
}

// ============================================================
// 日志解析
// ============================================================

// 常见 Java 日志格式（Logback / Log4j2 默认）
// 例: 2026-06-10 09:43:16.123 INFO  [main] com.example.App - hello
//      05-Jun-2026 15:01:01.067 信息 [main] org.apache.catalina.Xyz message
var javaLogRe = regexp.MustCompile(
	`^(\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}[.,]\d{3}|\d{2}-\w{3}-\d{4} \d{2}:\d{2}:\d{2}\.\d{3})` +
		`\s+(INFO|WARN|ERROR|DEBUG|TRACE|信息|警告|严重|config|fine)\s+` +
		`(?:\[([^\]]*)\]\s+)?` + // 可选 [thread]
		`(\S+)\s+[-–:]\s+(.*)$`,
)

func parseLine(raw string, stream LogStream) LogLine {
	line := LogLine{
		Time:   time.Now(),
		Stream: stream,
		Text:   raw,
	}
	m := javaLogRe.FindStringSubmatch(raw)
	if m == nil {
		return line
	}
	line.Level = normalizeLevel(m[2])
	line.Thread = m[3]
	line.Logger = shortenLogger(m[4])
	// Text 保留原始完整行，方便复制
	return line
}

func normalizeLevel(s string) string {
	switch strings.ToUpper(s) {
	case "信息", "CONFIG", "FINE":
		return "INFO"
	case "警告":
		return "WARN"
	case "严重":
		return "ERROR"
	default:
		return strings.ToUpper(s)
	}
}

// shortenLogger 只保留类名最后两段，减少视觉噪音
// org.apache.catalina.startup.VersionLoggerListener → startup.VersionLoggerListener
func shortenLogger(full string) string {
	parts := strings.Split(full, ".")
	if len(parts) <= 2 {
		return full
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

// ============================================================
// LogCollector：单个服务的日志收集器
// ============================================================

// LogCollector 管理一个服务的全部日志来源
type LogCollector struct {
	buf      *logBuf
	subs     []chan LogLine // 订阅者（UI 连接）
	subsMu   sync.Mutex
	cancelFn context.CancelFunc
	wg       sync.WaitGroup
}

func newLogCollector() *LogCollector {
	return &LogCollector{
		buf: newLogBuf(2000), // 最多保留 2000 行
	}
}

// Subscribe 注册一个订阅 channel，实时接收新日志行
// 调用方负责消费，否则会丢行（非阻塞发送）
func (c *LogCollector) Subscribe() <-chan LogLine {
	ch := make(chan LogLine, 256)
	c.subsMu.Lock()
	c.subs = append(c.subs, ch)
	c.subsMu.Unlock()
	return ch
}

// Unsubscribe 取消订阅
func (c *LogCollector) Unsubscribe(ch <-chan LogLine) {
	c.subsMu.Lock()
	defer c.subsMu.Unlock()
	for i, s := range c.subs {
		if s == ch {
			c.subs = append(c.subs[:i], c.subs[i+1:]...)
			func() {
				defer func() { recover() }()
				close(s)
			}()
			return
		}
	}
}

func (c *LogCollector) publish(line LogLine) {
	c.buf.append(line)
	c.subsMu.Lock()
	defer c.subsMu.Unlock()
	for _, ch := range c.subs {
		select {
		case ch <- line:
		default:
			// 订阅方消费太慢，丢弃，不阻塞
		}
	}
}

func (c *LogCollector) systemLine(text string) {
	c.publish(LogLine{
		Time:   time.Now(),
		Stream: StreamSystem,
		Level:  "INFO",
		Text:   text,
	})
}

// Snapshot 返回历史日志快照（UI 初次加载用）
func (c *LogCollector) Snapshot() []LogLine {
	return c.buf.Snapshot()
}

// StopTail 停止当前所有的日志采集 goroutine（tail / pipe），保留缓冲区和订阅
func (c *LogCollector) StopTail() {
	if c.cancelFn != nil {
		c.cancelFn()
		c.cancelFn = nil
	}
	c.wg.Wait()
}

func (c *LogCollector) Clear() {
	c.buf.Clear()
}

// StopTail 停止当前所有的日志采集（tail / pipe），保留缓冲区
func (c *LogCollector) stop() {
	if c.cancelFn != nil {
		c.cancelFn()
	}
	c.wg.Wait()
	// 关闭所有订阅 channel
	c.subsMu.Lock()
	defer c.subsMu.Unlock()
	for _, ch := range c.subs {
		func() {
			defer func() { recover() }()
			close(ch)
		}()
	}
	c.subs = nil
}

// ============================================================
// 进程输出采集（用于 ServiceDesktop 自己启动的服务）
// ============================================================

// attachProcess 接管 cmd 的 stdout / stderr，写入 collector
// 必须在 cmd.Start() 之前调用（因为要设置 Pipe）
func (c *LogCollector) attachProcess(cmd *exec.Cmd) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.cancelFn = cancel

	readPipe := func(r io.Reader, stream LogStream) {
		defer c.wg.Done()
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 256*1024), 1024*1024)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			default:
			}
			c.publish(parseLine(scanner.Text(), stream))
		}
	}

	c.wg.Add(2)
	go readPipe(stdout, StreamStdout)
	go readPipe(stderr, StreamStderr)

	return nil
}

// ============================================================
// 文件 tail（用于 SmartTomcat / IDEA 启动的服务）
// ============================================================

// TailFile 开始 tail 一个日志文件，新行实时推送到 collector
// 可以多次调用以 tail 多个文件（如 app.log + error.log）
func (c *LogCollector) TailFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("打开日志文件失败 %s: %w", path, err)
	}

	// 跳到文件末尾，只看新增内容
	if _, err := f.Seek(0, io.SeekEnd); err != nil {
		f.Close()
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	// 如果已有 cancel（比如之前 attachProcess 设过），链式取消
	if c.cancelFn != nil {
		prevCancel := c.cancelFn
		c.cancelFn = func() { prevCancel(); cancel() }
	} else {
		c.cancelFn = cancel
	}

	c.systemLine(fmt.Sprintf("► 开始监听日志文件: %s", filepath.Base(path)))

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer f.Close()

		reader := bufio.NewReader(f)
		var pending strings.Builder // 缓存不完整的行

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					// 检查文件是否被 rotate（文件大小减小）
					if info, statErr := f.Stat(); statErr == nil {
						pos, _ := f.Seek(0, io.SeekCurrent)
						if info.Size() < pos {
							// 文件被截断/rotate，重新从头读
							f.Seek(0, io.SeekStart)
							reader.Reset(f)
							c.systemLine("► 日志文件已轮转，重新读取")
							pending.Reset()
							continue
						}
					}
					time.Sleep(200 * time.Millisecond)
					continue
				}
				// 真实读取错误
				c.systemLine(fmt.Sprintf("► 读取日志文件出错: %v", err))
				return
			}

			// 有完整行
			pending.WriteString(line)
			fullLine := strings.TrimRight(pending.String(), "\r\n")
			pending.Reset()

			if fullLine != "" {
				c.publish(parseLine(fullLine, StreamFile))
			}
		}
	}()

	return nil
}

// DiscoverAppLogs 扫描 baseDir/logs 目录，返回应用日志文件列表
// 过滤掉 Tomcat 容器自身日志（catalina.* / localhost.* / manager.* / host-manager.*）
func DiscoverAppLogs(baseDir string) []string {
	logsDir := filepath.Join(baseDir, "logs")
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		return nil
	}

	// Tomcat 容器自带日志的前缀，排除
	tomcatPrefixes := []string{
		"catalina.", "localhost.", "manager.", "host-manager.",
		"localhost_access_log.",
	}

	var result []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".log") && !strings.HasSuffix(name, ".txt") {
			continue
		}
		isTomcat := false
		for _, prefix := range tomcatPrefixes {
			if strings.HasPrefix(name, prefix) {
				isTomcat = true
				break
			}
		}
		if !isTomcat {
			result = append(result, filepath.Join(logsDir, name))
		}
	}
	return result
}

// ============================================================
// Runtime：服务启停 + 日志收集统一入口
// ============================================================

// PortCheckResult 端口检查结果
type PortCheckResult struct {
	Available   bool   `json:"available"`
	Port        int    `json:"port"`
	Pid         int    `json:"pid"`
	ProcessName string `json:"processName"`
	Message     string `json:"message"`
}

// Runtime 管理所有服务的生命周期
type Runtime struct {
	mu         sync.Mutex
	processes  map[string]*exec.Cmd
	collectors map[string]*LogCollector
}

// NewRuntime 创建运行时管理器
func NewRuntime() *Runtime {
	return &Runtime{
		processes:  make(map[string]*exec.Cmd),
		collectors: make(map[string]*LogCollector),
	}
}

func (r *Runtime) getOrCreateCollector(id string) *LogCollector {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.collectors[id]; ok {
		return c
	}
	c := newLogCollector()
	r.collectors[id] = c
	return c
}

// Collector 返回指定服务的日志收集器（供 UI 订阅）
func (r *Runtime) Collector(id string) *LogCollector {
	return r.getOrCreateCollector(id)
}

// resolvePath 替换 {install_path} 占位符
func (r *Runtime) resolvePath(svc *Service, raw string) string {
	return strings.ReplaceAll(raw, `{install_path}`, svc.InstallPath)
}

// Start 启动服务
func (r *Runtime) Start(svc *Service) error {
	if svc.Status == StatusRunning {
		return fmt.Errorf("%s 已在运行中", svc.DisplayName)
	}

	// 端口预检
	if svc.Port > 0 {
		check := r.CheckPortAvailable(svc.Port)
		if !check.Available {
			hint := ""
			if check.Pid > 0 {
				hint = fmt.Sprintf("，可调用 KillProcessByPid(%d) 强制释放", check.Pid)
			}
			return fmt.Errorf("启动失败：%s%s", check.Message, hint)
		}
	}

	// 路径预检
	if svc.InstallPath == "" {
		return fmt.Errorf("启动失败：%s 安装路径未配置", svc.DisplayName)
	}
	if _, err := os.Stat(svc.InstallPath); os.IsNotExist(err) {
		return fmt.Errorf("启动失败：安装路径不存在 (%s)", svc.InstallPath)
	}

	svc.Status = StatusStarting

	// 调用插件预检钩子
	if p, ok := GetPlugin(svc.ID); ok {
		if err := p.BeforeStart(svc); err != nil {
			svc.Status = StatusError
			return fmt.Errorf("启动 %s 失败: %w", svc.DisplayName, err)
		}
	}

	collector := r.getOrCreateCollector(svc.ID)
	collector.Clear()
	collector.systemLine(fmt.Sprintf("► 正在启动 %s ...", svc.DisplayName))

	startCmd := r.resolvePath(svc, svc.StartCmd)
	if startCmd == "" {
		svc.Status = StatusError
		return fmt.Errorf("启动失败：%s 未配置启动命令（该服务可能由 IDE 管理）", svc.DisplayName)
	}

	// 兼容旧配置：如果 startCmd 包含内嵌参数（如 "startup.cmd -m standalone"），拆分
	inlineArgs := splitInlineArgs(startCmd)
	startCmd = inlineArgs[0]

	var args []string
	if len(inlineArgs) > 1 {
		args = append(args, inlineArgs[1:]...)
	}
	for _, a := range svc.Args {
		args = append(args, r.resolvePath(svc, a))
	}

	var cmd *exec.Cmd
	if isBatchFile(startCmd) {
		// cmd /c 需要命令+参数合为一个字符串，否则 %* 在 batch 脚本中无法正确展开
		all := append([]string{startCmd}, args...)
		var sb strings.Builder
		for i, a := range all {
			if i > 0 {
				sb.WriteByte(' ')
			}
			if strings.Contains(a, " ") {
				fmt.Fprintf(&sb, `"%s"`, a)
			} else {
				sb.WriteString(a)
			}
		}
		cmd = hiddenCmd("cmd", "/c", sb.String())
	} else {
		cmd = hiddenCmd(startCmd, args...)
	}
	cmd.Dir = svc.InstallPath
	cmd.Env = buildEnv(svc, r)

	// 判断来源：SmartTomcat / IDEA 启动的进程，stdout 不归我们管
	ownedByIDE := svc.Source == "smarttomcat" || svc.Source == "idea"

	if !ownedByIDE {
		// 接管 stdout / stderr
		if err := collector.attachProcess(cmd); err != nil {
			// attach 失败不影响启动，降级为无日志模式
			collector.systemLine(fmt.Sprintf("► 无法捕获进程输出: %v", err))
		}
	}

	if err := cmd.Start(); err != nil {
		svc.Status = StatusError
		return fmt.Errorf("启动 %s 失败: %w", svc.DisplayName, err)
	}

	r.mu.Lock()
	r.processes[svc.ID] = cmd
	r.mu.Unlock()
	svc.Pid = cmd.Process.Pid

	// 监听进程退出，自动更新状态
	go func() {
		_ = cmd.Wait()
		if svc.Status == StatusRunning {
			svc.Status = StatusError
			collector.systemLine(fmt.Sprintf("► %s 进程意外退出 (退出码: %d)", svc.DisplayName, cmd.ProcessState.ExitCode()))
		}
	}()

	// 对 IDE 管理的服务，尝试 tail 应用日志文件
	if ownedByIDE && svc.InstallPath != "" {
		appLogs := DiscoverAppLogs(svc.InstallPath)
		if len(appLogs) > 0 {
			for _, logPath := range appLogs {
				_ = collector.TailFile(logPath)
			}
		} else {
			collector.systemLine("► 未发现应用日志文件，请在服务设置中手动指定日志路径")
		}
	}

	// 等待端口就绪（最多 30 秒）
	if svc.Port > 0 {
		if err := r.waitForPort(svc.Port, 30*time.Second); err != nil {
			svc.Status = StatusError
			return fmt.Errorf("%s 端口 %d 未在 30s 内就绪，请查看日志", svc.DisplayName, svc.Port)
		}
	}

	svc.Status = StatusRunning
	collector.systemLine(fmt.Sprintf("► %s 已启动 (PID %d, 端口 %d)", svc.DisplayName, svc.Pid, svc.Port))
	return nil
}

// Stop 停止服务
func (r *Runtime) Stop(svc *Service) error {
	if svc.Status == StatusStopped {
		return nil
	}

	collector := r.getOrCreateCollector(svc.ID)
	collector.systemLine(fmt.Sprintf("► 正在停止 %s ...", svc.DisplayName))
	svc.Status = StatusStopping
	var errs []string

	// 1. 先尝试 StopCmd
	if svc.StopCmd != "" {
		stopCmd := r.resolvePath(svc, svc.StopCmd)
		cmd := hiddenCmd("cmd", "/c", stopCmd)
		cmd.Dir = svc.InstallPath
		cmd.Env = buildEnv(svc, r)
		if err := cmd.Run(); err != nil {
			errs = append(errs, fmt.Sprintf("停止命令失败: %v", err))
		}
	} else if svc.Source != "smarttomcat" && svc.Source != "idea" {
		errs = append(errs, "未配置停止命令")
	}

	// 2. 等待进程退出 + 端口释放（最多 15 秒）
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		stillAlive := false
		if svc.Port > 0 && r.isPortOpen(svc.Port) {
			stillAlive = true
		} else if svc.Pid > 0 && isProcessAlive(svc.Pid) {
			stillAlive = true
		}
		if !stillAlive {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	// 3. 若进程仍在运行，强制杀掉
	forceKilled := false
	if svc.Port > 0 && r.isPortOpen(svc.Port) {
		if pid := r.findPidByPort(svc.Port); pid > 0 {
			if killProcess(pid) == nil {
				forceKilled = true
			}
		}
	} else if svc.Pid > 0 && isProcessAlive(svc.Pid) {
		if killProcess(svc.Pid) == nil {
			forceKilled = true
		}
	}

	// 等待端口释放（最多 5 秒）
	if svc.Port > 0 {
		_ = r.waitForPortClosed(svc.Port, 5*time.Second)
	}

	// 4. 最后停止日志收集器
	collector.systemLine(fmt.Sprintf("► %s 已停止%s", svc.DisplayName,
		map[bool]string{true: "（强制终止）", false: ""}[forceKilled]))

	r.mu.Lock()
	if c, ok := r.collectors[svc.ID]; ok {
		c.stop()
		delete(r.collectors, svc.ID)
	}
	delete(r.processes, svc.ID)
	r.mu.Unlock()

	svc.Status = StatusStopped
	svc.Pid = 0

	// 调用插件清理钩子
	if p, ok := GetPlugin(svc.ID); ok {
		_ = p.AfterStop(svc)
	}

	if len(errs) > 0 {
		return fmt.Errorf("%s", strings.Join(errs, "; "))
	}
	return nil
}

// CheckStatus 检测服务当前状态
func (r *Runtime) CheckStatus(svc *Service) ServiceStatus {
	if svc.Port > 0 {
		if r.isPortOpen(svc.Port) {
			if pid := r.findPidByPort(svc.Port); pid > 0 {
				svc.Pid = pid
			}
			return StatusRunning
		}
		return StatusStopped
	}
	if svc.Pid > 0 {
		if isProcessAlive(svc.Pid) {
			return StatusRunning
		}
		svc.Pid = 0
	}
	return StatusStopped
}

// RefreshAll 刷新所有服务状态
func (r *Runtime) RefreshAll(services []*Service) {
	var wg sync.WaitGroup
	for _, svc := range services {
		svc := svc
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc.Status = r.CheckStatus(svc)
		}()
	}
	wg.Wait()
}

// CheckPortAvailable 检查端口是否可用
func (r *Runtime) CheckPortAvailable(port int) PortCheckResult {
	if !r.isPortOpen(port) {
		return PortCheckResult{Available: true, Port: port,
			Message: fmt.Sprintf("端口 %d 可用", port)}
	}
	pid := r.findPidByPort(port)
	name := ""
	if pid > 0 {
		name = getProcessName(pid)
	}
	return PortCheckResult{
		Available:   false,
		Port:        port,
		Pid:         pid,
		ProcessName: name,
		Message:     fmt.Sprintf("端口 %d 已被 %s (PID %d) 占用", port, name, pid),
	}
}

// KillProcessByPid 强制终止进程（供 UI 调用）
func (r *Runtime) KillProcessByPid(pid int) error {
	return killProcess(pid)
}

// ============================================================
// 平台相关辅助（跨平台）
// ============================================================

func hiddenCmd(command string, args ...string) *exec.Cmd {
	cmd := exec.Command(command, args...)
	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	}
	return cmd
}

// HiddenCmd 导出版本，供其他包使用
func HiddenCmd(command string, args ...string) *exec.Cmd {
	return hiddenCmd(command, args...)
}

func isBatchFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".bat") || strings.HasSuffix(lower, ".cmd")
}

// splitInlineArgs 拆分 StartCmd 中内嵌的参数
// 如 "F:\nacos\bin\startup.cmd -m standalone" → ["F:\nacos\bin\startup.cmd", "-m", "standalone"]
// 如 "F:\nginx\nginx.exe" → ["F:\nginx\nginx.exe"]
func splitInlineArgs(cmd string) []string {
	// 先检查完整路径是否是一个存在的文件
	if _, err := os.Stat(cmd); err == nil {
		return []string{cmd}
	}
	// 在空格处拆分，尝试第一个 token 作为文件
	idx := strings.Index(cmd, " ")
	if idx < 0 {
		return []string{cmd}
	}
	exe := cmd[:idx]
	rest := strings.TrimSpace(cmd[idx+1:])
	if _, err := os.Stat(exe); err != nil {
		// 第一个 token 也不是文件，返回原始值（让 exec 报出清晰错误）
		return []string{cmd}
	}
	// 把 rest 按空格拆开（引号暂不处理，旧配置不会有引号）
	return append([]string{exe}, strings.Fields(rest)...)
}

func buildEnv(svc *Service, r *Runtime) []string {
	env := os.Environ()
	for k, v := range svc.EnvVars {
		if v != "" {
			env = append(env, fmt.Sprintf("%s=%s", k, r.resolvePath(svc, v)))
		}
	}
	return env
}

func killProcess(pid int) error {
	switch runtime.GOOS {
	case "windows":
		return hiddenCmd("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid)).Run()
	default:
		// macOS / Linux: 先 SIGTERM，2 秒后 SIGKILL
		p, err := os.FindProcess(pid)
		if err != nil {
			return err
		}
		if err := p.Signal(syscall.SIGTERM); err != nil {
			return p.Signal(syscall.SIGKILL)
		}
		time.Sleep(2 * time.Second)
		_ = p.Signal(syscall.SIGKILL)
		return nil
	}
}

func isProcessAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	switch runtime.GOOS {
	case "windows":
		cmd := hiddenCmd("tasklist", "/fi", fmt.Sprintf("pid eq %d", pid), "/fo", "csv", "/nh")
		out, err := cmd.Output()
		return err == nil && strings.Contains(string(out), strconv.Itoa(pid))
	default:
		return p.Signal(syscall.Signal(0)) == nil
	}
}

func (r *Runtime) isPortOpen(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// IsPortOpen 公开版本，供 app.go 使用
func (r *Runtime) IsPortOpen(port int) bool {
	return r.isPortOpen(port)
}

func (r *Runtime) findPidByPort(port int) int {
	switch runtime.GOOS {
	case "windows":
		return findPidByPortWindows(port)
	default:
		return findPidByPortUnix(port)
	}
}

func findPidByPortWindows(port int) int {
	cmd := hiddenCmd("netstat", "-ano")
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	re := regexp.MustCompile(fmt.Sprintf(`:%d\s+\S+\s+LISTENING\s+(\d+)`, port))
	m := re.FindStringSubmatch(string(out))
	if len(m) > 1 {
		pid, _ := strconv.Atoi(m[1])
		return pid
	}
	return 0
}

func findPidByPortUnix(port int) int {
	cmd := exec.Command("lsof", "-ti", fmt.Sprintf(":%d", port))
	out, err := cmd.Output()
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return pid
}

func getProcessName(pid int) string {
	switch runtime.GOOS {
	case "windows":
		cmd := hiddenCmd("tasklist", "/fi", fmt.Sprintf("pid eq %d", pid), "/fo", "csv", "/nh")
		out, err := cmd.Output()
		if err != nil {
			return ""
		}
		parts := strings.SplitN(string(out), ",", 2)
		if len(parts) > 0 {
			return strings.Trim(parts[0], "\" \r\n")
		}
	default:
		cmd := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "comm=")
		out, err := cmd.Output()
		if err == nil {
			return strings.TrimSpace(string(out))
		}
	}
	return ""
}

func (r *Runtime) waitForPort(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if r.isPortOpen(port) {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("等待端口 %d 超时", port)
}

func (r *Runtime) waitForPortClosed(port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !r.isPortOpen(port) {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}

// ResolveInstallPath 遍历候选路径，返回第一个存在的
func ResolveInstallPath(candidates []string) string {
	for _, pattern := range candidates {
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			return matches[0]
		}
	}
	return ""
}
