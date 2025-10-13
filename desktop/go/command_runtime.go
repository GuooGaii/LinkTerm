package main

import (
	"fmt"
	"terminal-client/types"
)

// ConfigNotifier defines a callback for configuration updates.
type ConfigNotifier interface {
	NotifyConfigUpdated(*types.TerminalConfig)
}

// CommandRuntime 为内置命令提供配置读写能力。
type CommandRuntime struct {
	manager  *ConfigManager
	notifier ConfigNotifier
}

// NewCommandRuntime creates a runtime context with optional notifier.
func NewCommandRuntime(manager *ConfigManager, notifier ConfigNotifier) *CommandRuntime {
	return &CommandRuntime{
		manager:  manager,
		notifier: notifier,
	}
}

func (r *CommandRuntime) LoadConfig() (*types.Config, error) {
	if r.manager == nil {
		return nil, fmt.Errorf("config manager unavailable")
	}
	return r.manager.Load()
}

func (r *CommandRuntime) SaveConfig(cfg *types.Config) error {
	if r.manager == nil {
		return fmt.Errorf("config manager unavailable")
	}
	return r.manager.Save(cfg)
}

func (r *CommandRuntime) ConfigPath() string {
	if r.manager == nil {
		return ""
	}
	return r.manager.Path()
}

func (r *CommandRuntime) NotifyConfigUpdated(cfg *types.TerminalConfig) {
	if r.notifier != nil {
		r.notifier.NotifyConfigUpdated(cfg)
	}
}
