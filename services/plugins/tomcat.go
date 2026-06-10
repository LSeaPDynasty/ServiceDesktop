package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"

	"ServiceDesktop/services"
)

type tomcatPlugin struct{}

func init() { services.Register(&tomcatPlugin{}) }

func (p *tomcatPlugin) ID() string { return "tomcat" }

func (p *tomcatPlugin) Template() services.ServiceTemplate {
	return services.ServiceTemplate{
		Service: services.Service{
			ID:          "tomcat",
			Name:        "Tomcat",
			DisplayName: "Apache Tomcat",
			Category:    services.CategoryMiddleware,
			StartCmd:    `{install_path}\bin\startup.bat`,
			StopCmd:     `{install_path}\bin\shutdown.bat`,
			Port:        8080,
			LogFile:     `{install_path}\logs\`,
			Args:        []string{},
			EnvVars:     map[string]string{}, // JAVA_HOME/CATALINA_HOME 由系统环境继承
			IsTemplate:  true,
		},
		Description: "Java Servlet 容器，支持部署多个 WAR/JAR 应用",
		HomeVar:     "CATALINA_HOME",
		DefaultPort: 8080,
		DetectPaths: []string{
			`C:\Program Files\Apache Software Foundation\Tomcat *`,
			`C:\tools\tomcat*`,
			`D:\tools\tomcat*`,
		},
	}
}

func (p *tomcatPlugin) ConfigSchema() json.RawMessage { return nil }

func (p *tomcatPlugin) ConfigFiles(installPath string) []string {
	files := []string{`conf\server.xml`, `conf\catalina.properties`, `conf\web.xml`}
	var found []string
	for _, rel := range files {
		if _, err := os.Stat(filepath.Join(installPath, rel)); err == nil {
			found = append(found, rel)
		}
	}
	return found
}

func (p *tomcatPlugin) BeforeStart(svc *services.Service) error {
	if svc.InstallPath == "" {
		return nil
	}
	// 确保 logs 目录存在
	logsDir := filepath.Join(svc.InstallPath, "logs")
	_ = os.MkdirAll(logsDir, 0755)
	return nil
}

func (p *tomcatPlugin) AfterStop(svc *services.Service) error { return nil }
