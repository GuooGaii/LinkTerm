package app

import (
	"fmt"

	"terminal-client/internal/config"
	"terminal-client/internal/plugins"
)

// ConfigNotifier defines a callback for configuration updates.
type ConfigNotifier interface {
	NotifyConfigUpdated(*config.TerminalConfig)
}

// CommandRuntime implements plugins.CommandContext.
type CommandRuntime struct {
	manager  *config.Manager
	notifier ConfigNotifier
}

// NewCommandRuntime creates a runtime context with optional notifier.
func NewCommandRuntime(manager *config.Manager, notifier ConfigNotifier) *CommandRuntime {
	return &CommandRuntime{
		manager:  manager,
		notifier: notifier,
	}
}

func (r *CommandRuntime) LoadConfig() (*config.Config, error) {
	if r.manager == nil {
		return nil, fmt.Errorf("config manager unavailable")
	}
	return r.manager.Load()
}

func (r *CommandRuntime) SaveConfig(cfg *config.Config) error {
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

func (r *CommandRuntime) NotifyConfigUpdated(cfg *config.TerminalConfig) {
	if r.notifier != nil {
		r.notifier.NotifyConfigUpdated(cfg)
	}
}

var _ plugins.CommandContext = (*CommandRuntime)(nil)
