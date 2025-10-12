package info

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"terminal-client/internal/plugins"
)

const (
	pluginName        = "info"
	pluginVersion     = "1.0.0"
	pluginDescription = "显示系统和插件信息"
)

// InfoPlugin exposes runtime and plugin details.
type InfoPlugin struct {
	pluginManager *plugins.Manager
}

// New constructs the info command plugin.
func New(pm *plugins.Manager) (*InfoPlugin, error) {
	return &InfoPlugin{
		pluginManager: pm,
	}, nil
}

func (p *InfoPlugin) Name() string {
	return pluginName
}

func (p *InfoPlugin) Description() string {
	return pluginDescription
}

func (p *InfoPlugin) Version() string {
	return pluginVersion
}

func (p *InfoPlugin) Dependencies() []string {
	return nil
}

func (p *InfoPlugin) CommandName() string {
	return "info"
}

func (p *InfoPlugin) Usage() string {
	return `info - 显示系统和插件信息

用法:
  info                 显示系统信息
  info plugins         列出所有已加载的插件
  info commands        列出所有可用命令
  info version         显示程序版本信息

示例:
  info
  info plugins
  info commands`
}

func (p *InfoPlugin) Execute(_ plugins.CommandContext, args []string) (bool, string) {
	if len(args) == 0 {
		return true, p.getSystemInfo()
	}

	switch args[0] {
	case "plugins":
		return true, p.listPlugins()
	case "commands":
		return true, p.listCommands()
	case "version":
		return true, p.getVersionInfo()
	default:
		return true, fmt.Sprintf("未知子命令: %s\n\n%s", args[0], p.Usage())
	}
}

func (p *InfoPlugin) SubCommands() []string {
	return []string{"plugins", "commands", "version"}
}

func (p *InfoPlugin) Init(ctx context.Context) error {
	return nil
}

func (p *InfoPlugin) Start(ctx context.Context) error {
	return nil
}

func (p *InfoPlugin) Stop(ctx context.Context) error {
	return nil
}

func (p *InfoPlugin) getSystemInfo() string {
	var sb strings.Builder
	sb.WriteString("=== 系统信息 ===\n\n")
	sb.WriteString(fmt.Sprintf("操作系统:   %s\n", runtime.GOOS))
	sb.WriteString(fmt.Sprintf("架构:       %s\n", runtime.GOARCH))
	sb.WriteString(fmt.Sprintf("Go 版本:    %s\n", runtime.Version()))
	sb.WriteString(fmt.Sprintf("CPU 核心:   %d\n", runtime.NumCPU()))
	sb.WriteString(fmt.Sprintf("Goroutines: %d\n", runtime.NumGoroutine()))

	if p.pluginManager != nil {
		enabled := make([]string, 0)
		for _, metadata := range p.pluginManager.ListPlugins() {
			if metadata.Enabled {
				enabled = append(enabled, fmt.Sprintf("📦 %s (v%s)", metadata.Name, metadata.Version))
			}
		}

		sb.WriteString("\n=== 已启用的插件 ===\n\n")
		if len(enabled) == 0 {
			sb.WriteString("没有已启用的插件\n")
		} else {
			for _, item := range enabled {
				sb.WriteString(item)
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}

func (p *InfoPlugin) listPlugins() string {
	if p.pluginManager == nil {
		return "插件管理器不可用"
	}

	pluginInfos := p.pluginManager.ListPlugins()
	if len(pluginInfos) == 0 {
		return "没有已加载的插件"
	}

	enabledCount := 0
	for _, info := range pluginInfos {
		if info.Enabled {
			enabledCount++
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== 已加载的插件 (%d) | 已启用: %d ===\n\n", len(pluginInfos), enabledCount))

	for _, metadata := range pluginInfos {
		status := "停用"
		if metadata.Enabled {
			status = "启用"
		}
		sb.WriteString(fmt.Sprintf("📦 %s (v%s) [%s]\n", metadata.Name, metadata.Version, status))
		sb.WriteString(fmt.Sprintf("   %s\n", metadata.Description))
		if len(metadata.Dependencies) > 0 {
			sb.WriteString(fmt.Sprintf("   依赖: %v\n", metadata.Dependencies))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (p *InfoPlugin) listCommands() string {
	if p.pluginManager == nil {
		return "插件管理器不可用"
	}

	commands := p.pluginManager.ListCommands()
	if len(commands) == 0 {
		return "没有可用的命令"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== 可用命令 (%d) ===\n\n", len(commands)))

	for _, cmd := range commands {
		if plugin, exists := p.pluginManager.GetCommand(cmd); exists {
			sb.WriteString(fmt.Sprintf("  %s - %s\n", cmd, plugin.Description()))
		}
	}

	sb.WriteString("\n提示: 使用 '<命令> --help' 查看详细用法\n")
	return sb.String()
}

func (p *InfoPlugin) getVersionInfo() string {
	return `Terminal 远程终端客户端
版本: 1.0.0
构建: Go插件化架构

项目地址: https://chatnext.ai/terminal
微信: agentready`
}

func init() {
	plugins.RegisterFactory("info", func(pm *plugins.Manager) (plugins.Plugin, error) {
		return New(pm)
	})
}
