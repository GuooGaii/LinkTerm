package main

import (
	"context"
	"fmt"

	"terminal-client/plugins"
)

// pluginAdapter 将通用插件适配为 main.Plugin 和 main.CommandPlugin 接口
type pluginAdapter struct {
	raw interface{}
}

func (pa *pluginAdapter) Name() string {
	type named interface{ Name() string }
	if n, ok := pa.raw.(named); ok {
		return n.Name()
	}
	return "unknown"
}

func (pa *pluginAdapter) Description() string {
	type described interface{ Description() string }
	if d, ok := pa.raw.(described); ok {
		return d.Description()
	}
	return ""
}

func (pa *pluginAdapter) Version() string {
	type versioned interface{ Version() string }
	if v, ok := pa.raw.(versioned); ok {
		return v.Version()
	}
	return "0.0.0"
}

func (pa *pluginAdapter) Dependencies() []string {
	type dependent interface{ Dependencies() []string }
	if d, ok := pa.raw.(dependent); ok {
		return d.Dependencies()
	}
	return nil
}

func (pa *pluginAdapter) Init(ctx context.Context) error {
	type initable interface{ Init(context.Context) error }
	if i, ok := pa.raw.(initable); ok {
		return i.Init(ctx)
	}
	return nil
}

func (pa *pluginAdapter) Start(ctx context.Context) error {
	type startable interface{ Start(context.Context) error }
	if s, ok := pa.raw.(startable); ok {
		return s.Start(ctx)
	}
	return nil
}

func (pa *pluginAdapter) Stop(ctx context.Context) error {
	type stoppable interface{ Stop(context.Context) error }
	if s, ok := pa.raw.(stoppable); ok {
		return s.Stop(ctx)
	}
	return nil
}

func (pa *pluginAdapter) CommandName() string {
	type commanded interface{ CommandName() string }
	if c, ok := pa.raw.(commanded); ok {
		return c.CommandName()
	}
	return ""
}

func (pa *pluginAdapter) Usage() string {
	type usable interface{ Usage() string }
	if u, ok := pa.raw.(usable); ok {
		return u.Usage()
	}
	return ""
}

func (pa *pluginAdapter) SubCommands() []string {
	type subcommanded interface{ SubCommands() []string }
	if s, ok := pa.raw.(subcommanded); ok {
		return s.SubCommands()
	}
	return nil
}

func (pa *pluginAdapter) Execute(ctx CommandContext, args []string) (bool, string) {
	// 使用反射样式的方法调用，通过结构化的 duck typing
	type anyExecutable interface {
		Execute(interface{}, []string) (bool, string)
	}

	// 尝试直接调用，传递 ctx 作为 interface{}
	if e, ok := pa.raw.(anyExecutable); ok {
		return e.Execute(ctx, args)
	}

	return false, "plugin does not support execution"
}

// BuildPluginManager constructs the plugin manager with dynamically discovered plugins.
func BuildPluginManager() (*PluginManager, error) {
	manager := NewPluginManager()

	// 从 plugins 包加载所有内置插件
	builtinPlugins := plugins.GetBuiltinPlugins()

	for pluginName, pluginFactory := range builtinPlugins {
		// 必须在闭包外捕获变量以避免循环变量问题
		name := pluginName
		factory := pluginFactory

		RegisterPluginFactory(name, func(pm *PluginManager) (Plugin, error) {
			plugin, err := factory(pm)
			if err != nil {
				return nil, err
			}
			// 使用适配器包装插件
			return &pluginAdapter{raw: plugin}, nil
		})
	}

	// 加载所有已注册的插件
	if err := LoadAllPlugins(manager); err != nil {
		return nil, fmt.Errorf("加载插件失败: %w", err)
	}

	return manager, nil
}
