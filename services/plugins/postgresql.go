package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"

	"ServiceDesktop/services"
)

type postgresPlugin struct{}

func init() { services.Register(&postgresPlugin{}) }

func (p *postgresPlugin) ID() string { return "postgresql" }

func (p *postgresPlugin) Template() services.ServiceTemplate {
	return services.ServiceTemplate{
		Service: services.Service{
			ID:          "postgresql",
			Name:        "PostgreSQL",
			DisplayName: "PostgreSQL",
			Category:    services.CategoryDatabase,
			// pg_ctl.exe 作为命令，参数分离到 Args（避免 Runtime.Start 把整串当程序名）
			StartCmd: `{install_path}\bin\pg_ctl.exe`,
			StopCmd:  `{install_path}\bin\pg_ctl.exe stop -D {install_path}\data -m fast`,
			Port:     5432,
			LogFile:  `{install_path}\data\log\`,
			Args: []string{
				"start",
				"-D", `{install_path}\data`,
				"-l", `{install_path}\data\log\pg.log`,
			},
			EnvVars: map[string]string{
				"PGDATA": `{install_path}\data`,
			},
			IsTemplate: true,
		},
		Description: "功能强大的开源关系型数据库",
		HomeVar:     "PG_HOME",
		DefaultPort: 5432,
		DetectPaths: []string{
			`C:\Program Files\PostgreSQL\*`,
			`C:\tools\pgsql*`,
			`D:\tools\pgsql*`,
		},
	}
}

func (p *postgresPlugin) ConfigSchema() json.RawMessage {
	return json.RawMessage(`{"fields":[{"key":"dataDir","label":"数据目录","type":"string","placeholder":"{install_path}\\data"},{"key":"listenPort","label":"监听端口","type":"number","placeholder":"5432"}]}`)
}

func (p *postgresPlugin) ConfigFiles(installPath string) []string {
	candidates := []string{`data\postgresql.conf`}
	var found []string
	for _, rel := range candidates {
		if _, err := os.Stat(filepath.Join(installPath, rel)); err == nil {
			found = append(found, rel)
		}
	}
	return found
}

func (p *postgresPlugin) BeforeStart(svc *services.Service) error {
	if svc.InstallPath == "" {
		return nil
	}
	_ = os.MkdirAll(filepath.Join(svc.InstallPath, "data", "log"), 0755)
	return nil
}

func (p *postgresPlugin) AfterStop(svc *services.Service) error { return nil }
