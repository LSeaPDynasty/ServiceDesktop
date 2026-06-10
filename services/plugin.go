package services

import "encoding/json"

// ServicePlugin 定义每个服务类型的差异化行为
// 每个服务类型（Tomcat/Redis/Kafka 等）实现此接口并调用 Register 注册
type ServicePlugin interface {
	// ID 返回唯一标识，对应 Service.ID 前缀（如 "tomcat", "redis"）
	ID() string

	// Template 返回预置服务模板
	Template() ServiceTemplate

	// ConfigSchema 返回前端渲染配置表单用的 JSON Schema
	// 返回 null 表示无额外配置项
	ConfigSchema() json.RawMessage

	// ConfigFiles 返回该服务在 installPath 下的配置文件相对路径
	ConfigFiles(installPath string) []string

	// BeforeStart 启动前的预检/初始化回调，返回 error 则中止启动
	BeforeStart(svc *Service) error

	// AfterStop 停止后的清理回调
	AfterStop(svc *Service) error
}

// registry 全局插件注册表
var registry = map[string]ServicePlugin{}

// Register 注册一个服务插件（在插件的 init() 中调用）
func Register(p ServicePlugin) {
	registry[p.ID()] = p
}

// GetAllPlugins 返回所有已注册的插件
func GetAllPlugins() []ServicePlugin {
	out := make([]ServicePlugin, 0, len(registry))
	for _, p := range registry {
		out = append(out, p)
	}
	return out
}

// GetPlugin 根据 ID 获取插件
func GetPlugin(id string) (ServicePlugin, bool) {
	p, ok := registry[id]
	return p, ok
}
