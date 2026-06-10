package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	gopsutilNet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"
)

// ============================================================
// 类型定义
// ============================================================

// Confidence 发现结果的置信度
type Confidence int

const (
	ConfLow    Confidence = iota // 仅端口有响应
	ConfMedium                   // 端口 + 进程匹配
	ConfHigh                     // 端口 + 进程 + 协议验证
)

// ServiceInstance 发现的服务实例
type ServiceInstance struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	DisplayName string          `json:"displayName"`
	Category    ServiceCategory `json:"category"`
	Source      string          `json:"source"` // port/process/docker/protocol/idea
	Confidence  Confidence      `json:"confidence"`
	Port        int             `json:"port"`
	Pid         int             `json:"pid"`
	InstallPath string          `json:"installPath"`
	StartCmd    string          `json:"startCmd"`
	StopCmd     string          `json:"stopCmd"`
	LogFile     string          `json:"logFile"`
}

// ScanResult 合并后的扫描结果
type ScanResult struct {
	Instances []ServiceInstance `json:"instances"`
}

// ============================================================
// 已知服务配置
// ============================================================

type knownServiceDef struct {
	Name     string
	Category ServiceCategory
	Ports    []int
	CmdHints []string
	StartCmd string // 默认启动命令
	StopCmd  string // 默认停止命令
	LogFile  string // 默认日志路径
}

var knownServices = []knownServiceDef{
	{"Tomcat", CategoryMiddleware, []int{8080, 8081, 8082, 8083, 8084, 8085, 8086, 8087, 8088, 8089, 9090},
		[]string{"catalina", "tomcat", "bootstrap.jar"},
		"{install_path}\\bin\\startup.bat", "{install_path}\\bin\\shutdown.bat", "{install_path}\\logs\\catalina.out"},
	{"Redis", CategoryDatabase, []int{6379}, []string{"redis"},
		"{install_path}\\redis-server.exe", "", ""},
	{"Kafka", CategoryMiddleware, []int{9092, 9093, 9094}, []string{"kafka"},
		"{install_path}\\bin\\windows\\kafka-server-start.bat", "", ""},
	{"Nacos", CategoryMiddleware, []int{8848, 9848}, []string{"nacos"},
		"{install_path}\\bin\\startup.cmd", "{install_path}\\bin\\shutdown.cmd", "{install_path}\\logs\\nacos.log"},
	{"Nginx", CategoryMiddleware, []int{80, 443, 8080}, []string{"nginx"},
		"{install_path}\\nginx.exe", "{install_path}\\nginx.exe -s stop", ""},
	{"MySQL", CategoryDatabase, []int{3306, 3307}, []string{"mysql", "mysqld"},
		"{install_path}\\bin\\mysqld.exe", "", ""},
	{"PostgreSQL", CategoryDatabase, []int{5432, 5433}, []string{"postgres", "pgsql"},
		"{install_path}\\bin\\pg_ctl.exe", "{install_path}\\bin\\pg_ctl.exe stop -D {install_path}\\data -m fast", ""},
	{"MongoDB", CategoryDatabase, []int{27017, 27018}, []string{"mongod", "mongo"},
		"{install_path}\\bin\\mongod.exe", "", ""},
}

func buildPortMap() map[int]knownServiceDef {
	m := make(map[int]knownServiceDef)
	for _, ks := range knownServices {
		for _, p := range ks.Ports {
			m[p] = ks
		}
	}
	return m
}

// containsPort 精确判断端口是否在列表中
func containsPort(ports []int, port int) bool {
	for _, p := range ports {
		if p == port {
			return true
		}
	}
	return false
}

// ============================================================
// 主扫描器 - 分层并发扫描
// ============================================================

// RunDiscovery 执行完整服务发现流程
// 端口扫描 + Docker 扫描并发 → 进程补充 → 协议验证
func RunDiscovery() *ScanResult {
	result := &ScanResult{}
	seen := make(map[int]*ServiceInstance)
	var mu sync.Mutex
	var wg sync.WaitGroup

	portMap := buildPortMap()

	// 并发执行：端口扫描 和 Docker 扫描
	wg.Add(2)

	go func() {
		defer wg.Done()
		insts := portScan(portMap)
		mu.Lock()
		for i := range insts {
			seen[insts[i].Port] = &insts[i]
		}
		mu.Unlock()
	}()

	go func() {
		defer wg.Done()
		insts := dockerScan()
		mu.Lock()
		for i := range insts {
			if existing, ok := seen[insts[i].Port]; ok {
				insts[i].Confidence = existing.Confidence
				if insts[i].Confidence < ConfHigh {
					insts[i].Confidence = ConfHigh
				}
				seen[insts[i].Port] = &insts[i]
			} else {
				seen[insts[i].Port] = &insts[i]
			}
		}
		mu.Unlock()
	}()

	wg.Wait()

	// 进程补充（必须在端口扫描之后）
	processEnrich(seen)

	// 协议验证（并发）
	protocolVerify(seen)

	for _, inst := range seen {
		result.Instances = append(result.Instances, *inst)
	}
	return result
}

// ============================================================
// 第1层: 端口扫描（跨平台，用 gopsutil）
// ============================================================

func portScan(portMap map[int]knownServiceDef) []ServiceInstance {
	var results []ServiceInstance

	conns, err := gopsutilNet.Connections("tcp")
	if err != nil {
		return results
	}

	seenPort := make(map[int]bool)
	for _, c := range conns {
		if c.Status != "LISTEN" {
			continue
		}
		port := int(c.Laddr.Port)
		if port <= 0 || seenPort[port] {
			continue
		}
		ks, ok := portMap[port]
		if !ok {
			continue
		}
		seenPort[port] = true

		results = append(results, ServiceInstance{
			ID:          fmt.Sprintf("discovered-%s-%d", strings.ToLower(ks.Name), port),
			Name:        fmt.Sprintf("%s(%d)", ks.Name, port),
			DisplayName: fmt.Sprintf("%s :%d", ks.Name, port),
			Category:    ks.Category,
			Source:      "port",
			Confidence:  ConfLow,
			Port:        port,
			Pid:         int(c.Pid),
			StartCmd:    ks.StartCmd,
			StopCmd:     ks.StopCmd,
			LogFile:     ks.LogFile,
		})
	}
	return results
}

// ============================================================
// 第2层: 进程扫描（gopsutil）
// ============================================================

func processEnrich(instances map[int]*ServiceInstance) {
	for _, inst := range instances {
		if inst.Pid <= 0 {
			continue
		}
		p, err := process.NewProcess(int32(inst.Pid))
		if err != nil {
			continue
		}

		name, _ := p.Name()
		cmdline, _ := p.Cmdline()
		cmdlineLower := strings.ToLower(cmdline)

		// 精确查找匹配的已知服务定义
		for _, ks := range knownServices {
			if !containsPort(ks.Ports, inst.Port) {
				continue
			}
			for _, hint := range ks.CmdHints {
				if strings.Contains(cmdlineLower, hint) {
					inst.Name = ks.Name
					inst.DisplayName = ks.Name
					inst.Confidence = ConfMedium
					inst.Source = "process"

					// 提取安装路径（修复：- 不是截断符）
					if strings.Contains(cmdlineLower, "catalina") {
						inst.InstallPath = extractCatalinaPath(cmdline)
					}

					// SmartTomcat 特征：路径包含 .SmartTomcat（必须在 IDEA 检测之前）
					if strings.Contains(cmdlineLower, ".smarttomcat") {
						inst.Source = "smarttomcat"
						inst.DisplayName = inst.Name + " (SmartTomcat)"
					} else if strings.Contains(cmdlineLower, "idea_rt") || strings.Contains(cmdlineLower, "jetbrains") || strings.Contains(cmdlineLower, "intellij") {
						inst.Source = "idea"
						inst.DisplayName = inst.Name + " (IDEA)"
					}

					// Tomcat 特有：推演日志路径和启停命令
					if inst.Name == "Tomcat" {
						// 优先用正确提取的路径（非小写化版本）
						if p := extractCatalinaPath(cmdline); p != "" {
							inst.InstallPath = p
						}
						inferTomcatPaths(inst)
						inferTomcatCommands(inst)
					}
					goto nextInst
				}
			}
		}

		if name == "com.docker.backend" || name == "dockerd" || name == "wslrelay" {
			inst.Name = inst.Name + "(Docker)"
			inst.DisplayName = inst.DisplayName + " (Docker)"
		}

	nextInst:
	}
}

// extractCatalinaPath 从命令行中提取 catalina.base/home 路径（修复 - 截断 bug）
func extractCatalinaPath(cmdline string) string {
	for _, prefix := range []string{"-Dcatalina.base=", "-Dcatalina.home="} {
		idx := strings.Index(cmdline, prefix)
		if idx < 0 {
			continue
		}
		val := cmdline[idx+len(prefix):]

		var path string
		if strings.HasPrefix(val, `"`) {
			end := strings.Index(val[1:], `"`)
			if end >= 0 {
				path = val[1 : end+1]
			}
		} else {
			end := strings.Index(val, " ")
			if end >= 0 {
				path = val[:end]
			} else {
				path = val
			}
		}

		if path != "" {
			return filepath.Clean(path)
		}
	}
	return ""
}

// inferTomcatPaths 自动推演 Tomcat 日志文件路径
func inferTomcatPaths(inst *ServiceInstance) {
	if inst.InstallPath == "" {
		return
	}
	// 用 logs 目录作为日志路径
	inst.LogFile = filepath.Join(inst.InstallPath, "logs") + "\\"
}

// inferTomcatCommands 根据来源推断启停命令
func inferTomcatCommands(inst *ServiceInstance) {
	if inst.InstallPath == "" {
		return
	}
	switch inst.Source {
	case "idea", "smarttomcat":
		// IDEA/SmartTomcat 管理的，不生成启停命令，提示用户在 IDE 内操作
		inst.StartCmd = ""
		inst.StopCmd = ""
	default:
		// 原生 Tomcat
		bin := filepath.Join(inst.InstallPath, "bin")
		inst.StartCmd = filepath.Join(bin, "startup.bat")
		inst.StopCmd = filepath.Join(bin, "shutdown.bat")
	}
}

// ============================================================
// 第3层: Docker 扫描（docker ps 命令行）
// ============================================================

func dockerScan() []ServiceInstance {
	cmd := HiddenCmd("docker", "ps", "--format", `{"id":"{{.ID}}","image":"{{.Image}}","names":"{{.Names}}","ports":"{{.Ports}}","status":"{{.Status}}"}`)
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var results []ServiceInstance
	seenPort := make(map[int]bool)

	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") || !strings.HasSuffix(line, "}") {
			continue
		}

		var container struct {
			ID     string `json:"id"`
			Image  string `json:"image"`
			Names  string `json:"names"`
			Ports  string `json:"ports"`
			Status string `json:"status"`
		}
		if err := json.Unmarshal([]byte(line), &container); err != nil {
			continue
		}
		if !strings.HasPrefix(container.Status, "Up") {
			continue
		}

		image := strings.ToLower(container.Image)
		svcDef := matchDockerImage(image)
		if svcDef == nil {
			continue
		}

		port := extractDockerPort(container.Ports, svcDef.Ports[0])
		if port <= 0 || seenPort[port] {
			continue
		}
		seenPort[port] = true

		results = append(results, ServiceInstance{
			ID:          fmt.Sprintf("docker-%s-%d", svcDef.Name, port),
			Name:        fmt.Sprintf("%s(Docker)", svcDef.Name),
			DisplayName: fmt.Sprintf("%s(Docker) :%d", svcDef.Name, port),
			Category:    svcDef.Category,
			Source:      "docker",
			Confidence:  ConfHigh,
			Port:        port,
			InstallPath: "docker:" + container.Image,
			StartCmd:    svcDef.StartCmd,
			StopCmd:     svcDef.StopCmd,
			LogFile:     svcDef.LogFile,
		})
	}
	return results
}

func extractDockerPort(portsStr string, defaultPort int) int {
	if strings.Contains(portsStr, "->") {
		parts := strings.Split(portsStr, "->")
		hostPart := parts[0]
		if idx := strings.LastIndex(hostPart, ":"); idx >= 0 {
			port, err := strconv.Atoi(hostPart[idx+1:])
			if err == nil && port > 0 {
				return port
			}
		}
	}
	return defaultPort
}

func matchDockerImage(image string) *knownServiceDef {
	for _, ks := range knownServices {
		if strings.Contains(image, strings.ToLower(ks.Name)) {
			return &ks
		}
	}
	return nil
}

// ============================================================
// 第4层: 协议探针（并发）
// ============================================================

func protocolVerify(instances map[int]*ServiceInstance) {
	var wg sync.WaitGroup
	for port, inst := range instances {
		if inst.Confidence >= ConfHigh {
			continue
		}
		wg.Add(1)
		go func(port int, inst *ServiceInstance) {
			defer wg.Done()
			switch {
			case strings.Contains(inst.Name, "Redis") || port == 6379:
				if pingRedis(port) {
					inst.Confidence = ConfHigh
					inst.Name = "Redis"
					inst.DisplayName = "Redis"
				}
			case strings.Contains(inst.Name, "MySQL") || port == 3306:
				if probeMySQL(port) {
					inst.Confidence = ConfHigh
					inst.Name = "MySQL"
					inst.DisplayName = "MySQL"
				}
			case strings.Contains(inst.Name, "Tomcat") || (port >= 8080 && port <= 8090):
				if probeTomcat(port) {
					inst.Confidence = ConfHigh
					inst.Name = "Tomcat"
					inst.DisplayName = fmt.Sprintf("Apache Tomcat :%d", port)
				}
			}
		}(port, inst)
	}
	wg.Wait()
}

// pingRedis 发 PING 命令验证 Redis
func pingRedis(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	_, _ = conn.Write([]byte("*1\r\n$4\r\nPING\r\n"))
	buf := make([]byte, 32)
	n, _ := conn.Read(buf)
	return strings.Contains(string(buf[:n]), "+PONG")
}

// probeMySQL 读 MySQL 握手包确认
func probeMySQL(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))

	buf := make([]byte, 64)
	n, err := io.ReadAtLeast(conn, buf, 5)
	if err != nil || n < 5 {
		return false
	}
	// 第 5 字节（buf[4]）是 MySQL 协议版本号
	// MySQL 5.x / 8.x 协议版本号 = 10 (0x0a)
	return buf[4] == 0x0a
}

// probeTomcat 发 HTTP GET 验证 Tomcat
func probeTomcat(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 2*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(3 * time.Second))
	_, _ = conn.Write([]byte("GET / HTTP/1.0\r\nHost: localhost\r\n\r\n"))
	buf := make([]byte, 512)
	n, _ := conn.Read(buf)
	response := strings.ToLower(string(buf[:n]))
	// Tomcat 特有的是 coyote 或 apache tomcat
	// 避免误判 Apache httpd（只含 apache 不含 coyote/tomcat）
	return strings.Contains(response, "coyote") || strings.Contains(response, "apache tomcat")
}
