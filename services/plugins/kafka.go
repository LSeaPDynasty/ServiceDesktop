package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"

	"ServiceDesktop/services"
)

type kafkaPlugin struct{}

func init() { services.Register(&kafkaPlugin{}) }

func (p *kafkaPlugin) ID() string { return "kafka" }

func (p *kafkaPlugin) Template() services.ServiceTemplate {
	return services.ServiceTemplate{
		Service: services.Service{
			ID:          "kafka",
			Name:        "Kafka",
			DisplayName: "Apache Kafka",
			Category:    services.CategoryMiddleware,
			StartCmd:    `{install_path}\bin\windows\kafka-server-start.bat`,
			// Windows 没有 kafka-server-stop.bat，由 Runtime.Stop 强制终止
			StopCmd:    ``,
			Port:        9092,
			LogFile:     `{install_path}\logs\`,
			Args:        []string{`{install_path}\config\server.properties`},
			EnvVars: map[string]string{
				"KAFKA_HEAP_OPTS": "-Xmx1G -Xms1G",
			},
			IsTemplate: true,
		},
		Description: "分布式消息队列，需先启动 ZooKeeper（端口 2181）",
		HomeVar:     "KAFKA_HOME",
		DefaultPort: 9092,
		DetectPaths: []string{
			`C:\tools\kafka*`,
			`D:\tools\kafka*`,
		},
	}
}

func (p *kafkaPlugin) ConfigSchema() json.RawMessage {
	return json.RawMessage(`{"fields":[{"key":"heap","label":"堆内存","type":"string","placeholder":"如 -Xmx1G -Xms1G"},{"key":"zkConnect","label":"ZK 地址","type":"string","placeholder":"localhost:2181"}]}`)
}

func (p *kafkaPlugin) ConfigFiles(installPath string) []string {
	files := []string{`config\server.properties`}
	var found []string
	for _, rel := range files {
		if _, err := os.Stat(filepath.Join(installPath, rel)); err == nil {
			found = append(found, rel)
		}
	}
	return found
}

func (p *kafkaPlugin) BeforeStart(svc *services.Service) error {
	if svc.InstallPath != "" {
		_ = os.MkdirAll(filepath.Join(svc.InstallPath, "logs"), 0755)
	}
	return nil
}

func (p *kafkaPlugin) AfterStop(svc *services.Service) error { return nil }
