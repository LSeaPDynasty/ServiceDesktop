package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"

	"ServiceDesktop/services"
)

type mongoPlugin struct{}

func init() { services.Register(&mongoPlugin{}) }

func (p *mongoPlugin) ID() string { return "mongodb" }

func (p *mongoPlugin) Template() services.ServiceTemplate {
	return services.ServiceTemplate{
		Service: services.Service{
			ID:          "mongodb",
			Name:        "MongoDB",
			DisplayName: "MongoDB",
			Category:    services.CategoryDatabase,
			StartCmd:    `{install_path}\bin\mongod.exe`,
			// --shutdown 在较新版本不可靠，由 Runtime.Stop 强制终止兜底
			StopCmd: ``,
			Port:    27017,
			LogFile: `{install_path}\data\log\`,
			Args: []string{
				"--dbpath", `{install_path}\data\db`,
				"--logpath", `{install_path}\data\log\mongod.log`,
			},
			EnvVars:    map[string]string{},
			IsTemplate: true,
		},
		Description: "文档型 NoSQL 数据库，适合快速原型开发",
		HomeVar:     "MONGODB_HOME",
		DefaultPort: 27017,
		DetectPaths: []string{
			`C:\Program Files\MongoDB\Server\*`,
			`C:\tools\mongodb*`,
			`D:\tools\mongodb*`,
		},
	}
}

func (p *mongoPlugin) ConfigSchema() json.RawMessage {
	return json.RawMessage(`{"fields":[{"key":"dbpath","label":"数据目录","type":"string","placeholder":"{install_path}\\data\\db"},{"key":"logpath","label":"日志路径","type":"string","placeholder":"{install_path}\\data\\log\\mongod.log"},{"key":"replSet","label":"副本集名称","type":"string","placeholder":"留空则单节点"}]}`)
}

func (p *mongoPlugin) ConfigFiles(installPath string) []string {
	candidates := []string{`mongod.cfg`}
	var found []string
	for _, rel := range candidates {
		if _, err := os.Stat(filepath.Join(installPath, rel)); err == nil {
			found = append(found, rel)
		}
	}
	return found
}

func (p *mongoPlugin) BeforeStart(svc *services.Service) error {
	if svc.InstallPath == "" {
		return nil
	}
	_ = os.MkdirAll(filepath.Join(svc.InstallPath, "data", "db"), 0755)
	_ = os.MkdirAll(filepath.Join(svc.InstallPath, "data", "log"), 0755)
	return nil
}

func (p *mongoPlugin) AfterStop(svc *services.Service) error { return nil }
