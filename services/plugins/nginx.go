package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"

	"ServiceDesktop/services"
)

type nginxPlugin struct{}

func init() { services.Register(&nginxPlugin{}) }

func (p *nginxPlugin) ID() string { return "nginx" }

func (p *nginxPlugin) Template() services.ServiceTemplate {
	return services.ServiceTemplate{
		Service: services.Service{
			ID:          "nginx",
			Name:        "Nginx",
			DisplayName: "Nginx",
			Category:    services.CategoryMiddleware,
			StartCmd:    `{install_path}\nginx.exe`,
			StopCmd:     `{install_path}\nginx.exe -s stop`,
			// 默认端口改为 8080，避免 Windows 非管理员 80 端口权限问题
			Port:        8080,
			LogFile:     `{install_path}\logs\`,
			Args:        []string{},
			EnvVars:     map[string]string{},
			IsTemplate:  true,
		},
		Description: "高性能 HTTP 服务器和反向代理（默认 8080 端口）",
		HomeVar:     "NGINX_HOME",
		DefaultPort: 80,
		DetectPaths: []string{
			`C:\tools\nginx*`,
			`C:\nginx*`,
			`D:\tools\nginx*`,
		},
	}
}

func (p *nginxPlugin) ConfigSchema() json.RawMessage {
	return json.RawMessage(`{"fields":[{"key":"listenPort","label":"监听端口","type":"number","placeholder":"8080"},{"key":"confFile","label":"配置文件","type":"string","placeholder":"conf/nginx.conf"}]}`)
}

func (p *nginxPlugin) ConfigFiles(installPath string) []string {
	files := []string{`conf\nginx.conf`}
	var found []string
	for _, rel := range files {
		if _, err := os.Stat(filepath.Join(installPath, rel)); err == nil {
			found = append(found, rel)
		}
	}
	return found
}

func (p *nginxPlugin) BeforeStart(svc *services.Service) error {
	if svc.InstallPath != "" {
		_ = os.MkdirAll(filepath.Join(svc.InstallPath, "logs"), 0755)
	}
	return nil
}

func (p *nginxPlugin) AfterStop(svc *services.Service) error { return nil }
