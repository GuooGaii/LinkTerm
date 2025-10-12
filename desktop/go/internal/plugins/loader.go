package plugins

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

const metadataFilename = "metadata.json"

// pluginsRoot resolves the absolute path to the plugin directory.
func pluginsRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("plugins: unable to determine loader source path")
	}
	return filepath.Dir(file), nil
}

// LoadAll discovers plugin metadata under the plugin directory and loads any
// plugins with registered factories.
func LoadAll(manager *Manager) error {
	if manager == nil {
		return fmt.Errorf("plugins: manager cannot be nil")
	}

	root, err := pluginsRoot()
	if err != nil {
		return err
	}

	return loadFromDir(manager, root)
}

func loadFromDir(manager *Manager, root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			log.Printf("⚠️  插件目录不存在: %s", root)
			return nil
		}
		return fmt.Errorf("plugins: read directory %s: %w", root, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		metadataPath := filepath.Join(root, entry.Name(), metadataFilename)
		metadata, err := LoadMetadata(metadataPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return fmt.Errorf("plugins: load metadata for %s: %w", entry.Name(), err)
		}

		factory, ok := lookupFactory(metadata.Name)
		if !ok {
			log.Printf("⚠️  未找到插件工厂: %s (metadata: %s)", metadata.Name, metadataPath)
			continue
		}

		plugin, err := factory(manager)
		if err != nil {
			return fmt.Errorf("plugins: instantiate plugin %s: %w", metadata.Name, err)
		}

		if err := manager.Register(plugin); err != nil {
			return fmt.Errorf("plugins: register plugin %s: %w", metadata.Name, err)
		}
	}

	return nil
}
