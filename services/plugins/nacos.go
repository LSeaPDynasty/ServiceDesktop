package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"ServiceDesktop/services"
)

type nacosPlugin struct{}

func init() { services.Register(&nacosPlugin{}) }

func (p *nacosPlugin) ID() string { return "nacos" }

func (p *nacosPlugin) Template() services.ServiceTemplate {
	return services.ServiceTemplate{
		Service: services.Service{
			ID:          "nacos",
			Name:        "Nacos",
			DisplayName: "Nacos",
			Category:    services.CategoryMiddleware,
			StartCmd:    `{install_path}\bin\startup.cmd`,
			StopCmd:     `{install_path}\bin\shutdown.cmd`,
			Port:        8848,
			LogFile:     `{install_path}\logs\`,
			// startup.cmd 解析 -m 参数（第 34-43 行），standalone 模式必须传 -m standalone
			Args: []string{"-m", "standalone"},
			EnvVars: map[string]string{
				// JDK 内存：standalone 模式建议 512m，避免占用过大
				"CUSTOM_NACOS_MEMORY": "-Xms256m -Xmx512m -Xmn128m",
			},
			IsTemplate: true,
		},
		Description: "阿里云开源动态服务发现、配置和服务管理平台",
		HomeVar:     "NACOS_HOME",
		DefaultPort: 8848,
		DetectPaths: []string{
			`C:\tools\nacos*`,
			`D:\tools\nacos*`,
		},
	}
}

func (p *nacosPlugin) ConfigSchema() json.RawMessage {
	return json.RawMessage(`{"fields":[{"key":"mode","label":"运行模式","type":"select","options":["standalone","cluster"]},{"key":"dbUrl","label":"数据库地址","type":"string","placeholder":"仅 cluster 模式需要"}]}`)
}

func (p *nacosPlugin) ConfigFiles(installPath string) []string {
	files := []string{`conf\application.properties`}
	var found []string
	for _, rel := range files {
		if _, err := os.Stat(filepath.Join(installPath, rel)); err == nil {
			found = append(found, rel)
		}
	}
	return found
}

func (p *nacosPlugin) BeforeStart(svc *services.Service) error {
	if svc.InstallPath == "" {
		return nil
	}
	_ = os.MkdirAll(filepath.Join(svc.InstallPath, "logs"), 0755)

	// 自动修复 application.properties：standalone 模式禁用远程 address server
	// 否则会尝试连接 jmenv.tbsite.net 导致 UnknownHostException 启动失败
	confFile := filepath.Join(svc.InstallPath, "conf", "application.properties")
	if data, err := os.ReadFile(confFile); err == nil {
		content := string(data)
		changed := false

		// 确保使用本地文件成员发现，不连阿里云 address server
		if !strings.Contains(content, "nacos.core.member.lookup.type") {
			content += "\r\n# standalone 模式：使用本地文件发现成员，不依赖远程地址服务\r\nnacos.core.member.lookup.type=file\r\n"
			changed = true
		} else if !strings.Contains(content, "nacos.core.member.lookup.type=file") {
			content = strings.ReplaceAll(content, "nacos.core.member.lookup.type=address", "nacos.core.member.lookup.type=file")
			changed = true
		}

		if changed {
			_ = os.WriteFile(confFile, []byte(content), 0644)
		}
	}
	// 文件不存在则不处理（Nacos 首次启动会自动生成，下次启动时生效）

	return nil
}

func (p *nacosPlugin) AfterStop(svc *services.Service) error { return nil }
