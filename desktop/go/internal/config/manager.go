package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/uuid"
)

// Manager provides helpers to work with the configuration file on disk.
type Manager struct {
	path string
}

// NewManager constructs a configuration manager for the given path.
func NewManager(path string) *Manager {
	return &Manager{path: path}
}

// Path returns the configured file path.
func (m *Manager) Path() string {
	return m.path
}

// Load reads configuration from disk and normalises terminal settings.
func (m *Manager) Load() (*Config, error) {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	cfg.Terminal = NormalizeTerminal(cfg.Terminal)

	if cfg.ServerURL == "" || cfg.WebURL == "" {
		return nil, fmt.Errorf("配置不完整：缺少服务器地址")
	}

	if EnsureDeviceID(&cfg) {
		if err := m.Save(&cfg); err != nil {
			return nil, fmt.Errorf("保存配置失败: %w", err)
		}
	}

	return &cfg, nil
}

// Save writes configuration back to disk ensuring defaults are present.
func (m *Manager) Save(cfg *Config) error {
	if cfg.Terminal == nil {
		cfg.Terminal = CloneTerminal(DefaultTerminal())
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.path, data, 0o644)
}

// EnsureDeviceID guarantees the configuration carries a device identifier.
// It returns true when the configuration was mutated.
func EnsureDeviceID(cfg *Config) bool {
	if cfg.DeviceID != "" {
		return false
	}
	cfg.DeviceID = uuid.New().String()
	return true
}

// CreateWithDeviceID builds a config using supplied endpoints and an optional
// existing configuration for reusing the DeviceID.
func CreateWithDeviceID(existing *Config, serverURL, webURL string) *Config {
	deviceID := ""
	if existing != nil {
		deviceID = existing.DeviceID
	}
	if deviceID == "" {
		deviceID = uuid.New().String()
	}

	return &Config{
		ServerURL: serverURL,
		WebURL:    webURL,
		DeviceID:  deviceID,
		Terminal:  CloneTerminal(DefaultTerminal()),
	}
}
