package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"

	"ServiceDesktop/services"
)

type mysqlPlugin struct{}

func init() { services.Register(&mysqlPlugin{}) }

func (p *mysqlPlugin) ID() string { return "mysql" }

func (p *mysqlPlugin) Template() services.ServiceTemplate {
	return services.ServiceTemplate{
		Service: services.Service{
			ID:          "mysql",
			Name:        "MySQL",
			DisplayName: "MySQL",
			Category:    services.CategoryDatabase,
			// mysqld 需要 --console 才能在前台运行，否则会作为 Windows 服务启动
			StartCmd: `{install_path}\bin\mysqld.exe`,
			// mysqladmin 几乎总需要密码，停止由 Runtime.Stop 强制终止
			StopCmd:    ``,
			Port:        3306,
			LogFile:     `{install_path}\data\`,
			Args:        []string{"--console", "--defaults-file={install_path}\\my.ini"},
			EnvVars:     map[string]string{},
			IsTemplate:  true,
		},
		Description: "最流行的开源关系型数据库（首次使用需初始化 data 目录）",
		HomeVar:     "MYSQL_HOME",
		DefaultPort: 3306,
		DetectPaths: []string{
			`C:\Program Files\MySQL\MySQL Server *`,
			`C:\tools\mysql*`,
			`D:\tools\mysql*`,
		},
	}
}

func (p *mysqlPlugin) ConfigSchema() json.RawMessage {
	return json.RawMessage(`{"fields":[{"key":"rootPwd","label":"root 密码","type":"string","placeholder":"连接 MySQL 的 root 密码"},{"key":"dataDir","label":"数据目录","type":"string","placeholder":"{install_path}\\data"},{"key":"charset","label":"字符集","type":"select","options":["utf8mb4","utf8","gbk","latin1"]}]}`)
}

func (p *mysqlPlugin) ConfigFiles(installPath string) []string {
	candidates := []string{`my.ini`}
	var found []string
	for _, rel := range candidates {
		if _, err := os.Stat(filepath.Join(installPath, rel)); err == nil {
			found = append(found, rel)
		}
	}
	return found
}

func (p *mysqlPlugin) BeforeStart(svc *services.Service) error {
	if svc.InstallPath == "" {
		return nil
	}
	// 确保 data 目录存在
	dataDir := filepath.Join(svc.InstallPath, "data")
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		_ = os.MkdirAll(dataDir, 0755)
		// 提示用户需要初始化
		// 注意：mysqld --initialize 只需要运行一次，这里不自动执行以避免破坏已有数据
	}
	return nil
}

func (p *mysqlPlugin) AfterStop(svc *services.Service) error { return nil }
