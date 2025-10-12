package plugins

import (
	"context"

	"terminal-client/types"
)

// CommandContext 为命令插件暴露运行时能力。
type CommandContext interface {
	LoadConfig() (*types.Config, error)
	SaveConfig(*types.Config) error
	ConfigPath() string
	NotifyConfigUpdated(*types.TerminalConfig)
}

// Plugin 描述插件公共生命周期。
type Plugin interface {
	Init(context.Context) error
	Start(context.Context) error
	Stop(context.Context) error
}

// CommandPlugin 描述命令型插件需要实现的额外能力。
type CommandPlugin interface {
	Plugin

	CommandName() string
	Usage() string
	Execute(CommandContext, []string) (bool, string)
	SubCommands() []string
}
