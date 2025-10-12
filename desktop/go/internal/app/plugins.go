package app

import (
	"fmt"

	"terminal-client/internal/plugins"
	"terminal-client/internal/plugins/builtin"
)

// BuildPluginManager constructs the plugin manager with built-in plugins.
func BuildPluginManager() (*plugins.Manager, error) {
	manager := plugins.NewManager()

	if err := manager.Register(builtin.NewTermConfigPlugin()); err != nil {
		return nil, fmt.Errorf("注册 termconfig 插件失败: %w", err)
	}

	if err := manager.Register(builtin.NewInfoPlugin(manager)); err != nil {
		return nil, fmt.Errorf("注册 info 插件失败: %w", err)
	}

	return manager, nil
}
