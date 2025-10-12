package app

import (
	"fmt"

	"terminal-client/internal/plugins"
	"terminal-client/internal/plugins/register"
)

// BuildPluginManager constructs the plugin manager with dynamically discovered plugins.
func BuildPluginManager() (*plugins.Manager, error) {
	manager := plugins.NewManager()

	register.Register()

	if err := plugins.LoadAll(manager); err != nil {
		return nil, fmt.Errorf("加载插件失败: %w", err)
	}

	return manager, nil
}
