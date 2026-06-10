package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"

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
			// startup.cmd 不接收命令行参数，模式由环境变量 MODE 控制
			StartCmd: `{install_path}\bin\startup.cmd`,
			StopCmd:  `{install_path}\bin\shutdown.cmd`,
			Port:     8848,
			LogFile:  `{install_path}\logs\`,
			Args:     []string{}, // 不传参数，脚本内部读取 MODE 环境变量
			EnvVars: map[string]string{
				"MODE": "standalone",
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
	if svc.InstallPath != "" {
		_ = os.MkdirAll(filepath.Join(svc.InstallPath, "logs"), 0755)
	}
	return nil
}

func (p *nacosPlugin) AfterStop(svc *services.Service) error { return nil }
