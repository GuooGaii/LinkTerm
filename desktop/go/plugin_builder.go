package main

import (
	"fmt"
)

// BuildPluginManager constructs the plugin manager with dynamically discovered plugins.
func BuildPluginManager() (*PluginManager, error) {
	manager := NewPluginManager()

	// Plugins are registered via init() functions in plugin_info.go and plugin_termconfig.go
	if err := LoadAllPlugins(manager); err != nil {
		return nil, fmt.Errorf("加载插件失败: %w", err)
	}

	return manager, nil
}
