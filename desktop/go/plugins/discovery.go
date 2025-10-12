package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
)

// Manifest 描述插件的元数据信息。
type Manifest struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Description  string   `json:"description"`
	CommandName  string   `json:"commandName,omitempty"`
	Dependencies []string `json:"dependencies,omitempty"`
	EntryPoint   string   `json:"entryPoint"`
	Enabled      bool     `json:"-"` // 运行时状态，不序列化
}

// PluginEntry 包含插件的清单和实例。
type PluginEntry struct {
	Manifest Manifest
	Plugin   CommandPlugin
}

// DiscoverPlugins 扫描 plugins 目录，自动发现并加载所有插件。
func DiscoverPlugins(pluginsDir string) (map[string]*PluginEntry, error) {
	entries := make(map[string]*PluginEntry)

	// 检查插件目录是否存在
	if _, err := os.Stat(pluginsDir); os.IsNotExist(err) {
		return entries, nil
	}

	// 遍历插件目录
	dirEntries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil, fmt.Errorf("读取插件目录失败: %w", err)
	}

	for _, dirEntry := range dirEntries {
		if !dirEntry.IsDir() {
			continue
		}

		pluginDir := filepath.Join(pluginsDir, dirEntry.Name())
		manifestPath := filepath.Join(pluginDir, "manifest.json")

		// 检查是否存在 manifest.json
		if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
			continue
		}

		// 读取并解析清单
		manifest, err := loadManifest(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("加载插件 %s 清单失败: %w", dirEntry.Name(), err)
		}

		// 加载插件实例
		plugin, err := loadPlugin(dirEntry.Name(), manifest)
		if err != nil {
			return nil, fmt.Errorf("加载插件 %s 失败: %w", manifest.Name, err)
		}

		entries[manifest.Name] = &PluginEntry{
			Manifest: *manifest,
			Plugin:   plugin,
		}
	}

	return entries, nil
}

// loadManifest 读取并解析插件清单文件。
func loadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取清单文件失败: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("解析清单文件失败: %w", err)
	}

	// 验证必需字段
	if manifest.Name == "" {
		return nil, fmt.Errorf("清单缺少 name 字段")
	}
	if manifest.Version == "" {
		return nil, fmt.Errorf("清单缺少 version 字段")
	}
	if manifest.EntryPoint == "" {
		return nil, fmt.Errorf("清单缺少 entryPoint 字段")
	}

	return &manifest, nil
}

// loadPlugin 根据清单信息加载插件实例。
func loadPlugin(packageName string, manifest *Manifest) (CommandPlugin, error) {
	// 根据包名获取插件工厂函数
	// 这里使用约定：每个插件包必须导出 NewPlugin 函数
	factory := getPluginFactory(packageName)
	if factory == nil {
		return nil, fmt.Errorf("插件 %s 未找到工厂函数 %s", manifest.Name, manifest.EntryPoint)
	}

	// 调用工厂函数创建插件实例
	pluginInterface := factory(nil)
	if pluginInterface == nil {
		return nil, fmt.Errorf("插件 %s 工厂函数返回 nil", manifest.Name)
	}

	// 类型断言获取 CommandPlugin 接口
	plugin, ok := pluginInterface.(CommandPlugin)
	if !ok {
		return nil, fmt.Errorf("插件 %s 未实现 CommandPlugin 接口", manifest.Name)
	}

	return plugin, nil
}

// getPluginFactory 根据包名获取插件工厂函数。
// 这里使用静态注册表，避免复杂的反射或代码生成。
func getPluginFactory(packageName string) func(interface{}) interface{} {
	// 从内置插件映射中获取
	builtins := GetBuiltinPlugins()
	if factory, exists := builtins[packageName]; exists {
		return factory
	}
	return nil
}

// GetBuiltinPlugins 返回所有内置插件的工厂函数映射。
// 注意：这个函数将在每个插件包中通过 init() 注册。
var builtinPlugins = make(map[string]func(interface{}) interface{})

func GetBuiltinPlugins() map[string]func(interface{}) interface{} {
	return builtinPlugins
}

// RegisterPlugin 注册插件工厂函数（供插件包在 init() 中调用）。
func RegisterPlugin(name string, factory func(interface{}) interface{}) {
	builtinPlugins[name] = factory
}

// ResolvePluginDependencies 解析插件依赖关系并返回正确的加载顺序。
func ResolvePluginDependencies(entries map[string]*PluginEntry) ([]string, error) {
	visited := make(map[string]bool)
	visiting := make(map[string]bool)
	order := []string{}

	var visit func(string) error
	visit = func(name string) error {
		if visited[name] {
			return nil
		}
		if visiting[name] {
			return fmt.Errorf("检测到循环依赖: %s", name)
		}

		entry, exists := entries[name]
		if !exists {
			return fmt.Errorf("插件 %s 不存在", name)
		}

		visiting[name] = true

		for _, dep := range entry.Manifest.Dependencies {
			if _, exists := entries[dep]; !exists {
				return fmt.Errorf("插件 %s 依赖的插件 %s 不存在", name, dep)
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

	for name := range entries {
		if err := visit(name); err != nil {
			return nil, err
		}
	}

	return order, nil
}

// validatePluginInterface 验证插件是否正确实现了接口（用于调试）。
func validatePluginInterface(plugin interface{}) error {
	pluginType := reflect.TypeOf(plugin)

	// 检查是否实现了 CommandPlugin 接口
	pluginInterface := reflect.TypeOf((*CommandPlugin)(nil)).Elem()
	if !pluginType.Implements(pluginInterface) {
		return fmt.Errorf("插件未实现 CommandPlugin 接口")
	}

	return nil
}
