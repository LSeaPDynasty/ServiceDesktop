package services

// ServiceStatus 表示服务的运行状态
type ServiceStatus int

const (
	StatusStopped ServiceStatus = iota
	StatusRunning
	StatusError
	StatusStarting
	StatusStopping
)

func (s ServiceStatus) String() string {
	switch s {
	case StatusRunning:
		return "running"
	case StatusStopped:
		return "stopped"
	case StatusError:
		return "error"
	case StatusStarting:
		return "starting"
	case StatusStopping:
		return "stopping"
	default:
		return "unknown"
	}
}

// ServiceCategory 服务分类
type ServiceCategory string

const (
	CategoryMiddleware ServiceCategory = "Middleware"   // 中间件 (Tomcat, Kafka, Nginx, Nacos)
	CategoryDatabase   ServiceCategory = "Database"     // 数据库 (MySQL, PostgreSQL, MongoDB, Redis)
	CategoryCustom     ServiceCategory = "Custom"       // 用户自定义
	CategoryLanguage   ServiceCategory = "Language"     // 语言运行时 (Java, Python)
)

// Service 表示一个可管理的服务
type Service struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	DisplayName string            `json:"display_name"`
	Category    ServiceCategory   `json:"category"`
	InstallPath string            `json:"install_path"`
	StartCmd    string            `json:"start_cmd"`
	StopCmd     string            `json:"stop_cmd"`
	Port        int               `json:"port"`
	LogFile     string            `json:"log_file"`
	Args        []string          `json:"args"`
	EnvVars     map[string]string `json:"env_vars"`
	IsTemplate  bool              `json:"is_template"`
	Source      string            `json:"source,omitempty"` // 来源: port/process/smarttomcat/idea/docker

	// 运行时状态（不持久化）
	Status ServiceStatus `json:"-"`
	Pid    int           `json:"-"`
}

// ServiceTemplate 预配置模板定义
type ServiceTemplate struct {
	Service           // 嵌入基础 Service 字段
	Description string `json:"description"`
	HomeVar     string `json:"home_var"` // 环境变量名，如 CATALINA_HOME
	DefaultPort int    `json:"default_port"`
	DetectPaths []string `json:"detect_paths"` // 自动检测的安装路径
}
