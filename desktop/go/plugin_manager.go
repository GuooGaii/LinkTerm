package main

import (
	"fmt"
	"log"
	"sort"
	"sync"

	"terminal-client/plugins"
)

// PluginInfo 用于对外展示插件列表信息。
type PluginInfo struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Version      string   `json:"version"`
	Dependencies []string `json:"dependencies"`
	Enabled      bool     `json:"enabled"`
}

// PluginManager keeps track of all registered plugins.
type PluginManager struct {
	entries        map[string]*plugins.PluginEntry // 插件条目（清单+实例）
	commandPlugins map[string]plugins.CommandPlugin
	mu             sync.RWMutex
}

// NewPluginManager creates a fresh plugin manager.
func NewPluginManager() *PluginManager {
	return &PluginManager{
		entries:        make(map[string]*plugins.PluginEntry),
		commandPlugins: make(map[string]plugins.CommandPlugin),
	}
}

// Register adds a plugin entry to the manager.
func (pm *PluginManager) Register(entry *plugins.PluginEntry) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	name := entry.Manifest.Name
	if name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}

	if _, exists := pm.entries[name]; exists {
		return fmt.Errorf("plugin %s already registered", name)
	}

	pm.entries[name] = entry
	entry.Manifest.Enabled = true

	// 注册命令插件
	cmdName := entry.Plugin.CommandName()
	if cmdName == "" {
		return fmt.Errorf("plugin %s: command name cannot be empty", name)
	}
	if _, exists := pm.commandPlugins[cmdName]; exists {
		return fmt.Errorf("plugin %s: command %s already registered", name, cmdName)
	}
	pm.commandPlugins[cmdName] = entry.Plugin
	log.Printf("✅ 已注册命令插件: %s -> %s", name, cmdName)

	log.Printf("📦 已注册插件: %s v%s - %s", name, entry.Manifest.Version, entry.Manifest.Description)
	return nil
}


// GetCommand retrieves a command plugin by name.
func (pm *PluginManager) GetCommand(name string) (plugins.CommandPlugin, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	plugin, exists := pm.commandPlugins[name]
	return plugin, exists
}

// ListCommands returns registered command names sorted alphabetically.
func (pm *PluginManager) ListCommands() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	commands := make([]string, 0, len(pm.commandPlugins))
	for name := range pm.commandPlugins {
		commands = append(commands, name)
	}

	sort.Strings(commands)
	return commands
}

// GetPlugin returns a plugin entry by name.
func (pm *PluginManager) GetPlugin(name string) (*plugins.PluginEntry, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	entry, exists := pm.entries[name]
	return entry, exists
}

// ListPlugins collects metadata for registered plugins.
func (pm *PluginManager) ListPlugins() []PluginInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	metadata := make([]PluginInfo, 0, len(pm.entries))
	for _, entry := range pm.entries {
		metadata = append(metadata, PluginInfo{
			Name:         entry.Manifest.Name,
			Description:  entry.Manifest.Description,
			Version:      entry.Manifest.Version,
			Dependencies: entry.Manifest.Dependencies,
			Enabled:      entry.Manifest.Enabled,
		})
	}

	sort.Slice(metadata, func(i, j int) bool {
		return metadata[i].Name < metadata[j].Name
	})

	return metadata
}

func (pm *PluginManager) resolveDependencies() ([]string, error) {
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

		entry, exists := pm.entries[name]
		if !exists {
			return fmt.Errorf("plugin %s not found", name)
		}

		visiting[name] = true

		for _, dep := range entry.Manifest.Dependencies {
			if _, exists := pm.entries[dep]; !exists {
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

	for name := range pm.entries {
		if err := visit(name); err != nil {
			return nil, err
		}
	}

	return order, nil
}

// GetCommandHelp returns extended help for commands.
func (pm *PluginManager) GetCommandHelp(commandName string) string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	if plugin, exists := pm.commandPlugins[commandName]; exists {
		// 查找对应的清单信息
		for _, entry := range pm.entries {
			if entry.Plugin.CommandName() == commandName {
				return fmt.Sprintf("%s - %s\n\n%s",
					commandName,
					entry.Manifest.Description,
					plugin.Usage())
			}
		}
		return plugin.Usage()
	}

	return fmt.Sprintf("命令 '%s' 不存在", commandName)
}

// ExecuteCommand runs a command plugin.
func (pm *PluginManager) ExecuteCommand(commandName string, ctx plugins.CommandContext, args []string) (bool, string) {
	plugin, exists := pm.GetCommand(commandName)
	if !exists {
		return false, fmt.Sprintf("未知命令: %s\n\n可用命令: %v", commandName, pm.ListCommands())
	}

	return plugin.Execute(ctx, args)
}
