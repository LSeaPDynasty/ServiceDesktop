package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
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

	// 自动修复 application.properties：禁用远程 address server
	// Nacos 默认用 address-server 模式连接 jmenv.tbsite.net，离线环境必崩
	// 写入 nacos.core.member.lookup.type=file 改成本地文件发现
	confFile := filepath.Join(svc.InstallPath, "conf", "application.properties")
	data, err := os.ReadFile(confFile)
	if err != nil {
		return nil // 文件不存在，Nacos 首次启动会生成，下次生效
	}
	content := string(data)

	// 移除所有 member.lookup.type 行（注释或非注释）
	re := regexp.MustCompile(`(?m)^[#\s]*nacos\.core\.member\.lookup\.type[= ].*$`)
	if re.MatchString(content) {
		content = re.ReplaceAllString(content, "")
	}
	// 追加有效配置
	content = strings.TrimRight(content, "\r\n") + "\r\n\r\n# standalone 模式：本地文件成员发现\r\nnacos.core.member.lookup.type=file\r\n"
	_ = os.WriteFile(confFile, []byte(content), 0644)

	return nil
}

func (p *nacosPlugin) AfterStop(svc *services.Service) error { return nil }
