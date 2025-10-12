package plugins

import (
	"fmt"
	"sort"
	"sync"
)

// Factory 构造插件实例，允许在构造时收到插件管理器。
type Factory func(*Manager) (Plugin, error)

var (
	factoryMu    sync.RWMutex
	factories    = map[string]Factory{}
	factoryNames []string
)

// RegisterFactory 注册插件工厂。重复注册或无效输入会 panic，以便在开发阶段及时暴露问题。
func RegisterFactory(name string, factory Factory) {
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

type factoryEntry struct {
	name    string
	factory Factory
}

func registeredFactories() []factoryEntry {
	factoryMu.RLock()
	defer factoryMu.RUnlock()

	entries := make([]factoryEntry, 0, len(factoryNames))
	for _, name := range factoryNames {
		entries = append(entries, factoryEntry{
			name:    name,
			factory: factories[name],
		})
	}
	return entries
}
