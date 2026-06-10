package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"

	"ServiceDesktop/services"
)

type redisPlugin struct{}

func init() { services.Register(&redisPlugin{}) }

func (p *redisPlugin) ID() string { return "redis" }

func (p *redisPlugin) Template() services.ServiceTemplate {
	return services.ServiceTemplate{
		Service: services.Service{
			ID:          "redis",
			Name:        "Redis",
			DisplayName: "Redis",
			Category:    services.CategoryDatabase,
			StartCmd:    `{install_path}\redis-server.exe`,
			// StopCmd 留空：windows redis 无 cli，由 Runtime.Stop 强制终止
			StopCmd:    ``,
			Port:        6379,
			LogFile:     `{install_path}\logs\`,
			Args:        []string{`{install_path}\redis.windows.conf`},
			EnvVars:     map[string]string{},
			IsTemplate:  true,
		},
		Description: "内存键值数据库，常用于缓存",
		HomeVar:     "REDIS_HOME",
		DefaultPort: 6379,
		DetectPaths: []string{
			`C:\tools\redis*`,
			`D:\tools\redis*`,
		},
	}
}

func (p *redisPlugin) ConfigSchema() json.RawMessage {
	return json.RawMessage(`{"fields":[{"key":"password","label":"requirepass","type":"string","placeholder":"留空则无密码"},{"key":"maxmemory","label":"最大内存","type":"string","placeholder":"如 256mb"},{"key":"persist","label":"持久化","type":"select","options":["RDB","AOF","RDB+AOF"]}]}`)
}

func (p *redisPlugin) ConfigFiles(installPath string) []string {
	candidates := []string{`redis.windows.conf`, `redis.conf`}
	var found []string
	for _, rel := range candidates {
		if _, err := os.Stat(filepath.Join(installPath, rel)); err == nil {
			found = append(found, rel)
		}
	}
	return found
}

func (p *redisPlugin) BeforeStart(svc *services.Service) error {
	if svc.InstallPath != "" {
		_ = os.MkdirAll(filepath.Join(svc.InstallPath, "logs"), 0755)
	}
	return nil
}

func (p *redisPlugin) AfterStop(svc *services.Service) error { return nil }
