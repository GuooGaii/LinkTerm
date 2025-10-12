package plugins

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
)

// Manager keeps track of all registered plugins and their lifecycle.
type Manager struct {
	plugins          map[string]Plugin
	commandPlugins   map[string]CommandPlugin
	eventHooks       map[string][]EventHook
	extensionPlugins map[string]ExtensionPlugin
	enabledPlugins   map[string]bool
	mu               sync.RWMutex
	ctx              context.Context
	cancel           context.CancelFunc
	started          bool
}

// NewManager creates a fresh plugin manager.
func NewManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		plugins:          make(map[string]Plugin),
		commandPlugins:   make(map[string]CommandPlugin),
		eventHooks:       make(map[string][]EventHook),
		extensionPlugins: make(map[string]ExtensionPlugin),
		enabledPlugins:   make(map[string]bool),
		ctx:              ctx,
		cancel:           cancel,
	}
}

// Register adds a plugin to the manager.
func (pm *Manager) Register(plugin Plugin) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	name := plugin.Name()
	if name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}

	if _, exists := pm.plugins[name]; exists {
		return fmt.Errorf("plugin %s already registered", name)
	}

	pm.plugins[name] = plugin
	pm.enabledPlugins[name] = true

	if cmdPlugin, ok := plugin.(CommandPlugin); ok {
		cmdName := cmdPlugin.CommandName()
		if cmdName == "" {
			return NewPluginError(name, "command name cannot be empty", nil)
		}
		if _, exists := pm.commandPlugins[cmdName]; exists {
			return NewPluginError(name, fmt.Sprintf("command %s already registered", cmdName), nil)
		}
		pm.commandPlugins[cmdName] = cmdPlugin
		log.Printf("✅ 已注册命令插件: %s -> %s", name, cmdName)
	}

	if hookPlugin, ok := plugin.(EventHook); ok {
		for _, eventType := range hookPlugin.EventTypes() {
			pm.eventHooks[eventType] = append(pm.eventHooks[eventType], hookPlugin)
		}
		for eventType := range pm.eventHooks {
			sort.Slice(pm.eventHooks[eventType], func(i, j int) bool {
				return pm.eventHooks[eventType][i].Priority() < pm.eventHooks[eventType][j].Priority()
			})
		}
		log.Printf("✅ 已注册事件钩子插件: %s", name)
	}

	if extPlugin, ok := plugin.(ExtensionPlugin); ok {
		extType := extPlugin.ExtensionType()
		if extType == "" {
			return NewPluginError(name, "extension type cannot be empty", nil)
		}
		pm.extensionPlugins[extType] = extPlugin
		log.Printf("✅ 已注册扩展插件: %s [%s]", name, extType)
	}

	log.Printf("📦 已注册插件: %s v%s - %s", name, plugin.Version(), plugin.Description())
	return nil
}

// Unregister removes a plugin.
func (pm *Manager) Unregister(name string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	plugin, exists := pm.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	if cmdPlugin, ok := plugin.(CommandPlugin); ok {
		delete(pm.commandPlugins, cmdPlugin.CommandName())
	}

	if hookPlugin, ok := plugin.(EventHook); ok {
		for _, eventType := range hookPlugin.EventTypes() {
			hooks := pm.eventHooks[eventType]
			for i, hook := range hooks {
				if hook.Name() == name {
					pm.eventHooks[eventType] = append(hooks[:i], hooks[i+1:]...)
					break
				}
			}
		}
	}

	if extPlugin, ok := plugin.(ExtensionPlugin); ok {
		delete(pm.extensionPlugins, extPlugin.ExtensionType())
	}

	delete(pm.enabledPlugins, name)
	delete(pm.plugins, name)
	log.Printf("🗑️  已取消注册插件: %s", name)
	return nil
}

// InitAll initialises plugins respecting dependencies.
func (pm *Manager) InitAll() error {
	order, err := pm.resolveDependencies()
	if err != nil {
		return err
	}

	for _, name := range order {
		plugin := pm.plugins[name]
		log.Printf("⚙️  初始化插件: %s", name)
		if err := plugin.Init(pm.ctx); err != nil {
			return NewPluginError(name, "初始化失败", err)
		}
	}

	return nil
}

// StartAll starts all plugins.
func (pm *Manager) StartAll() error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.started {
		return nil
	}

	order, err := pm.resolveDependencies()
	if err != nil {
		return err
	}

	for _, name := range order {
		plugin := pm.plugins[name]
		log.Printf("🚀 启动插件: %s", name)
		if err := plugin.Start(pm.ctx); err != nil {
			return NewPluginError(name, "启动失败", err)
		}
		pm.enabledPlugins[name] = true
	}

	pm.started = true
	return nil
}

// StopAll stops all plugins in reverse order.
func (pm *Manager) StopAll() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if !pm.started {
		return
	}

	order, err := pm.resolveDependencies()
	if err != nil {
		log.Printf("停止插件时解析依赖失败: %v", err)
		order = make([]string, 0, len(pm.plugins))
		for name := range pm.plugins {
			order = append(order, name)
		}
	}

	for i := len(order) - 1; i >= 0; i-- {
		name := order[i]
		plugin := pm.plugins[name]
		log.Printf("⛔ 停止插件: %s", name)
		if err := plugin.Stop(pm.ctx); err != nil {
			log.Printf("⚠️  停止插件 %s 失败: %v", name, err)
		}
		pm.enabledPlugins[name] = false
	}

	pm.started = false
	pm.cancel()
}

// GetCommand retrieves a command plugin by name.
func (pm *Manager) GetCommand(name string) (CommandPlugin, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	plugin, exists := pm.commandPlugins[name]
	return plugin, exists
}

// ListCommands returns registered command names sorted alphabetically.
func (pm *Manager) ListCommands() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	commands := make([]string, 0, len(pm.commandPlugins))
	for name := range pm.commandPlugins {
		commands = append(commands, name)
	}

	sort.Strings(commands)
	return commands
}

// EmitEvent broadcasts an event to subscribed hooks.
func (pm *Manager) EmitEvent(event *Event) error {
	pm.mu.RLock()
	hooks := pm.eventHooks[event.Type]
	pm.mu.RUnlock()

	for _, hook := range hooks {
		if err := hook.OnEvent(pm.ctx, event); err != nil {
			log.Printf("⚠️  钩子 %s 处理事件 %s 失败: %v", hook.Name(), event.Type, err)
		}
	}

	return nil
}

// GetExtension fetches an extension by type.
func (pm *Manager) GetExtension(extensionType string) (ExtensionPlugin, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	plugin, exists := pm.extensionPlugins[extensionType]
	return plugin, exists
}

// GetPlugin returns a plugin by name.
func (pm *Manager) GetPlugin(name string) (Plugin, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	plugin, exists := pm.plugins[name]
	return plugin, exists
}

// ListPlugins collects metadata for registered plugins.
func (pm *Manager) ListPlugins() []PluginMetadata {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	metadata := make([]PluginMetadata, 0, len(pm.plugins))
	for _, plugin := range pm.plugins {
		name := plugin.Name()
		metadata = append(metadata, PluginMetadata{
			Name:         name,
			Description:  plugin.Description(),
			Version:      plugin.Version(),
			Dependencies: plugin.Dependencies(),
			Enabled:      pm.enabledPlugins[name],
		})
	}

	sort.Slice(metadata, func(i, j int) bool {
		return metadata[i].Name < metadata[j].Name
	})

	return metadata
}

func (pm *Manager) resolveDependencies() ([]string, error) {
	visited := make(map[string]bool)
	visiting := make(map[string]bool)
	order := []string{}

	var visit func(string) error
	visit = func(name string) error {
		if visited[name] {
			return nil
		}
		if visiting[name] {
			return fmt.Errorf("circular dependency detected: %s", name)
		}

		plugin, exists := pm.plugins[name]
		if !exists {
			return fmt.Errorf("plugin %s not found", name)
		}

		visiting[name] = true

		for _, dep := range plugin.Dependencies() {
			if _, exists := pm.plugins[dep]; !exists {
				return fmt.Errorf("plugin %s depends on missing plugin %s", name, dep)
			}
			if err := visit(dep); err != nil {
				return err
			}
		}

		visiting[name] = false
		visited[name] = true
		order = append(order, name)
		return nil
	}

	for name := range pm.plugins {
		if err := visit(name); err != nil {
			return nil, err
		}
	}

	return order, nil
}

// GetCommandHelp returns extended help for commands.
func (pm *Manager) GetCommandHelp(commandName string) string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if plugin, exists := pm.commandPlugins[commandName]; exists {
		return fmt.Sprintf("%s - %s\n\n%s",
			commandName,
			plugin.Description(),
			plugin.Usage())
	}

	return fmt.Sprintf("命令 '%s' 不存在", commandName)
}

// ExecuteCommand runs a command plugin.
func (pm *Manager) ExecuteCommand(commandName string, ctx CommandContext, args []string) (bool, string) {
	plugin, exists := pm.GetCommand(commandName)
	if !exists {
		return false, fmt.Sprintf("未知命令: %s\n\n可用命令: %v", commandName, pm.ListCommands())
	}

	return plugin.Execute(ctx, args)
}
