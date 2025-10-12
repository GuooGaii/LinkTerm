package plugins

import (
	"fmt"
	"sync"
)

// Factory constructs a plugin instance and allows factories to receive the
// plugin manager for additional context if necessary.
type Factory func(*Manager) (Plugin, error)

var (
	factoryMu sync.RWMutex
	factories = map[string]Factory{}
)

// RegisterFactory registers a factory for the given plugin name. It panics if
// the name is empty or already registered, since factories are typically
// declared during init() and such errors should surface immediately.
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
}

// lookupFactory returns the factory registered for the given name.
func lookupFactory(name string) (Factory, bool) {
	factoryMu.RLock()
	defer factoryMu.RUnlock()
	factory, ok := factories[name]
	return factory, ok
}
