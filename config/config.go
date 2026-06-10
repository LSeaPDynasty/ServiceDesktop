package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Config 应用配置
type Config struct {
	mu                 sync.Mutex                `json:"-"`
	Language           string                    `json:"language"`
	AutoStart          bool                      `json:"auto_start"`
	UserServices       []UserServiceConf         `json:"user_services"`
	DiscoveredServices []DiscoveredServiceConf   `json:"discovered_services"` // 自动发现后持久化的服务
	PathOverrides      map[string]string         `json:"path_overrides"`
	StartProfiles      map[string]map[string]string `json:"start_profiles"`
}

// UserServiceConf 用户自定义服务的持久化配置
type UserServiceConf struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Category    string `json:"category"`
	InstallPath string `json:"install_path"`
	StartCmd    string `json:"start_cmd"`
	StopCmd     string `json:"stop_cmd"`
	Port        int    `json:"port"`
	LogFile     string `json:"log_file"`
	Args        string `json:"args"`
	EnvVars     string `json:"env_vars"`   // KEY=VAL;KEY2=VAL2
}

// DiscoveredServiceConf 自动发现服务的持久化配置（轻量，只存关键字段）
type DiscoveredServiceConf struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Category    string `json:"category"`
	InstallPath string `json:"install_path"`
	StartCmd    string `json:"start_cmd"`
	StopCmd     string `json:"stop_cmd"`
	LogFile     string `json:"log_file"`
	Port        int    `json:"port"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Language:           "zh",
		AutoStart:          false,
		UserServices:       make([]UserServiceConf, 0),
		DiscoveredServices: make([]DiscoveredServiceConf, 0),
		PathOverrides:      make(map[string]string),
		StartProfiles:      make(map[string]map[string]string),
	}
}

// GetProfileNames 获取服务的所有 Profile 名称列表
func (c *Config) GetProfileNames(serviceID string) []string {
	if c.StartProfiles == nil || c.StartProfiles[serviceID] == nil {
		return nil
	}
	names := make([]string, 0, len(c.StartProfiles[serviceID]))
	for name := range c.StartProfiles[serviceID] {
		names = append(names, name)
	}
	return names
}

// configPath 返回配置文件路径
func configPath() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		appData = filepath.Join(home, ".config")
	}
	dir := filepath.Join(appData, "ServiceDesktop")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建配置目录失败: %w", err)
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load 从文件加载配置，文件不存在时返回默认配置
func Load() *Config {
	path, err := configPath()
	if err != nil {
		return DefaultConfig()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return DefaultConfig()
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return DefaultConfig()
	}
	return cfg
}

// Save 保存配置到文件
func (c *Config) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	path, err := configPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}
	return nil
}
