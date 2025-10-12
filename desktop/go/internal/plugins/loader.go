package plugins

import "fmt"

// LoadAll 实例化所有已注册的插件工厂并加入管理器。
func LoadAll(manager *Manager) error {
	if manager == nil {
		return fmt.Errorf("plugins: manager cannot be nil")
	}

	for _, entry := range registeredFactories() {
		plugin, err := entry.factory(manager)
		if err != nil {
			return fmt.Errorf("plugins: instantiate plugin %s: %w", entry.name, err)
		}
		if err := manager.Register(plugin); err != nil {
			return fmt.Errorf("plugins: register plugin %s: %w", entry.name, err)
		}
	}

	return nil
}
