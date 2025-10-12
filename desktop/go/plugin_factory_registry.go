package main

import (
	"fmt"
	"sort"
	"sync"
)

// PluginFactory 构造插件实例，允许在构造时收到插件管理器。
type PluginFactory func(*PluginManager) (Plugin, error)

var (
	factoryMu    sync.RWMutex
	factories    = map[string]PluginFactory{}
	factoryNames []string
)

// RegisterPluginFactory 注册插件工厂。重复注册或无效输入会 panic，以便在开发阶段及时暴露问题。
func RegisterPluginFactory(name string, factory PluginFactory) {
	if name == "" {
		panic("plugins: factory name cannot be empty")
	}
	if factory == nil {
		panic(fmt.Sprintf("plugins: factory for %q cannot be nil", name))
	}

	factoryMu.Lock()
	defer factoryMu.Unlock()

	if _, exists := factories[name]; exists {
		panic(fmt.Sprintf("plugins: factory %q already registered", name))
	}

	factories[name] = factory
	factoryNames = append(factoryNames, name)
	sort.Strings(factoryNames)
}

type pluginFactoryEntry struct {
	name    string
	factory PluginFactory
}

func registeredFactories() []pluginFactoryEntry {
	factoryMu.RLock()
	defer factoryMu.RUnlock()

	entries := make([]pluginFactoryEntry, 0, len(factoryNames))
	for _, name := range factoryNames {
		entries = append(entries, pluginFactoryEntry{
			name:    name,
			factory: factories[name],
		})
	}
	return entries
}
