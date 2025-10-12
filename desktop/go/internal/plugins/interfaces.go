package plugins

import (
	"context"
	"fmt"

	"terminal-client/internal/config"
)

// CommandContext 为命令插件暴露运行时能力。
type CommandContext interface {
	LoadConfig() (*config.Config, error)
	SaveConfig(*config.Config) error
	ConfigPath() string
	NotifyConfigUpdated(*config.TerminalConfig)
}

// Plugin 描述插件公共生命周期。
type Plugin interface {
	Name() string
	Description() string
	Version() string
	Init(context.Context) error
	Start(context.Context) error
	Stop(context.Context) error
	Dependencies() []string
}

// CommandPlugin 描述命令型插件需要实现的额外能力。
type CommandPlugin interface {
	Plugin

	CommandName() string
	Usage() string
	Execute(CommandContext, []string) (bool, string)
	SubCommands() []string
}

// PluginInfo 用于对外展示插件列表信息。
type PluginInfo struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Version      string   `json:"version"`
	Dependencies []string `json:"dependencies"`
	Enabled      bool     `json:"enabled"`
}

// PluginError 对插件运行期错误进行包装。
type PluginError struct {
	Plugin  string
	Message string
	Err     error
}

func (e *PluginError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("plugin %s: %s: %v", e.Plugin, e.Message, e.Err)
	}
	return fmt.Sprintf("plugin %s: %s", e.Plugin, e.Message)
}

func (e *PluginError) Unwrap() error {
	return e.Err
}

// NewPluginError 构造插件错误。
func NewPluginError(plugin, message string, err error) *PluginError {
	return &PluginError{
		Plugin:  plugin,
		Message: message,
		Err:     err,
	}
}
