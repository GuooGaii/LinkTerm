package plugins

import (
	"terminal-client/types"
)

// CommandContext 为命令插件暴露运行时能力。
type CommandContext interface {
	LoadConfig() (*types.Config, error)
	SaveConfig(*types.Config) error
	ConfigPath() string
	NotifyConfigUpdated(*types.TerminalConfig)
}

// CommandPlugin 描述命令型插件需要实现的能力。
type CommandPlugin interface {
	CommandName() string
	Usage() string
	Execute(CommandContext, []string) (bool, string)
	SubCommands() []string
}
