package app

import (
	"fmt"

	"terminal-client/internal/plugins"
	builtininfo "terminal-client/internal/plugins/builtin/info"
	builtinTermconfig "terminal-client/internal/plugins/builtin/termconfig"
)

// BuildPluginManager constructs the plugin manager with built-in plugins.
func BuildPluginManager() (*plugins.Manager, error) {
	manager := plugins.NewManager()

	termConfigPlugin, err := builtinTermconfig.New()
	if err != nil {
		return nil, fmt.Errorf("加载 termconfig 插件失败: %w", err)
	}
	if err := manager.Register(termConfigPlugin); err != nil {
		return nil, fmt.Errorf("注册 termconfig 插件失败: %w", err)
	}

	infoPlugin, err := builtininfo.New(manager)
	if err != nil {
		return nil, fmt.Errorf("加载 info 插件失败: %w", err)
	}
	if err := manager.Register(infoPlugin); err != nil {
		return nil, fmt.Errorf("注册 info 插件失败: %w", err)
	}

	return manager, nil
}
