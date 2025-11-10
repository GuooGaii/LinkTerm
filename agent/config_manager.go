package main

import (
	"encoding/json"
	"fmt"
	"os"

	"terminal-client/types"

	"github.com/google/uuid"
)

// ConfigManager 提供用于处理磁盘配置文件的辅助方法。
type ConfigManager struct {
	path string
}

// NewConfigManager 为给定路径构造一个配置管理器。
func NewConfigManager(path string) *ConfigManager {
	return &ConfigManager{path: path}
}

// Path 返回配置的文件路径。
func (m *ConfigManager) Path() string {
	return m.path
}

// Load 从磁盘读取配置并标准化终端设置。
func (m *ConfigManager) Load() (*types.Config, error) {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return nil, err
	}

	var cfg types.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	cfg.Terminal = types.NormalizeTerminal(cfg.Terminal)

	if cfg.ServerURL == "" || cfg.WebURL == "" {
		return nil, fmt.Errorf("配置不完整：缺少服务器地址")
	}

	// 如果需要，生成并保存 DeviceID
	if EnsureDeviceID(&cfg) {
		if err := m.Save(&cfg); err != nil {
			return nil, fmt.Errorf("保存配置失败: %w", err)
		}
	}

	return &cfg, nil
}

// Save 将配置写回到磁盘，并确保默认值的存在。
func (m *ConfigManager) Save(cfg *types.Config) error {
	if cfg.Terminal == nil {
		cfg.Terminal = types.CloneTerminal(types.DefaultTerminal())
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.path, data, 0o644)
}

// EnsureDeviceID 保证配置包含设备标识符。
// 如果配置被更改，则返回 true。
func EnsureDeviceID(cfg *types.Config) bool {
	if cfg.DeviceID != "" {
		return false
	}
	cfg.DeviceID = uuid.New().String()
	return true
}

// CreateConfigWithDeviceID 使用提供的 endpoint 和可选的
// 已有 DeviceID 的配置来构建配置对象。
func CreateConfigWithDeviceID(existing *types.Config, serverURL, webURL string) *types.Config {
	cfg := &types.Config{
		ServerURL: serverURL,
		WebURL:    webURL,
		Terminal:  types.CloneTerminal(types.DefaultTerminal()),
	}

	if existing != nil {
		cfg.DeviceID = existing.DeviceID
	}

	EnsureDeviceID(cfg)
	return cfg
}
