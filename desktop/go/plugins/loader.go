package plugins

import (
	"terminal-client/plugins/info"
	"terminal-client/plugins/termconfig"
)

// PluginFactory 插件工厂函数类型
type PluginFactory func(interface{}) (interface{}, error)

// GetBuiltinPlugins 返回所有内置插件的工厂函数映射
func GetBuiltinPlugins() map[string]PluginFactory {
	return map[string]PluginFactory{
		"info":       info.NewPlugin,
		"termconfig": termconfig.NewPlugin,
	}
}
