package plugins

import (
	"context"
	"fmt"
	"sync"

	"terminal-client/internal/config"
)

// CommandContext exposes runtime helpers to command plugins.
type CommandContext interface {
	LoadConfig() (*config.Config, error)
	SaveConfig(*config.Config) error
	ConfigPath() string
	NotifyConfigUpdated(*config.TerminalConfig)
}

// Plugin represents the base behaviour for all plugins.
type Plugin interface {
	Name() string
	Description() string
	Version() string
	Init(context.Context) error
	Start(context.Context) error
	Stop(context.Context) error
	Dependencies() []string
}

// CommandPlugin defines terminal command behaviour.
type CommandPlugin interface {
	Plugin

	CommandName() string
	Usage() string
	Execute(CommandContext, []string) (bool, string)
	SubCommands() []string
}

// EventHook plugins react to emitted events.
type EventHook interface {
	Plugin
	EventTypes() []string
	OnEvent(context.Context, *Event) error
	Priority() int
}

// ExtensionPlugin extends the runtime with additional capabilities.
type ExtensionPlugin interface {
	Plugin
	ExtensionType() string
	Apply(context.Context, interface{}) error
}

// ConfigurablePlugin allows plugins to describe and receive configuration.
type ConfigurablePlugin interface {
	Plugin
	DefaultConfig() map[string]interface{}
	ValidateConfig(map[string]interface{}) error
	ApplyConfig(map[string]interface{}) error
}

// Event is used by hooks and extensions to exchange data.
type Event struct {
	Type      string
	Source    string
	Data      map[string]interface{}
	Timestamp int64
}

// PluginMetadata summarises a plugin for listing purposes.
type PluginMetadata struct {
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Version      string                 `json:"version"`
	Author       string                 `json:"author"`
	Dependencies []string               `json:"dependencies"`
	Config       map[string]string      `json:"config"`
	Persistent   map[string]interface{} `json:"persistent,omitempty"`
	Enabled      bool                   `json:"enabled"`
}

// PluginError wraps underlying plugin failures.
type PluginError struct {
	Plugin  string
	Message string
	Err     error
}

func (e *PluginError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("plugin %s: %s: %v", e.Plugin, e.Message, e.Err)
	}
	return fmt.Sprintf("plugin %s: %s", e.Plugin, e.Message)
}

func (e *PluginError) Unwrap() error {
	return e.Err
}

// NewPluginError creates a plugin error helper.
func NewPluginError(plugin, message string, err error) *PluginError {
	return &PluginError{
		Plugin:  plugin,
		Message: message,
		Err:     err,
	}
}

// BasePlugin offers a reusable implementation scaffold.
type BasePlugin struct {
	name         string
	description  string
	version      string
	metadataPath string
	metadataMu   sync.RWMutex
	metadata     *FileMetadata
}

// NewBasePlugin instantiates a helper implementing the basic Plugin contract.
func NewBasePlugin(name, description, version string) *BasePlugin {
	return &BasePlugin{
		name:        name,
		description: description,
		version:     version,
	}
}

// NewBasePluginFromMetadata loads plugin details from a metadata.json file.
func NewBasePluginFromMetadata(metadataPath string) (*BasePlugin, error) {
	if metadataPath == "" {
		return nil, fmt.Errorf("metadata path cannot be empty")
	}

	metadata, err := LoadMetadata(metadataPath)
	if err != nil {
		return nil, err
	}

	return &BasePlugin{
		name:         metadata.Name,
		description:  metadata.Description,
		version:      metadata.Version,
		metadataPath: metadataPath,
		metadata: &FileMetadata{
			Name:        metadata.Name,
			Version:     metadata.Version,
			Description: metadata.Description,
			Persistent:  clonePersistentState(metadata.Persistent),
		},
	}, nil
}

func (p *BasePlugin) Name() string {
	return p.name
}

func (p *BasePlugin) Description() string {
	return p.description
}

func (p *BasePlugin) Version() string {
	return p.version
}

func (p *BasePlugin) Init(context.Context) error {
	return nil
}

func (p *BasePlugin) Start(context.Context) error {
	return nil
}

func (p *BasePlugin) Stop(context.Context) error {
	return nil
}

func (p *BasePlugin) Dependencies() []string {
	return []string{}
}

// MetadataPath returns the file path storing metadata, if any.
func (p *BasePlugin) MetadataPath() string {
	p.metadataMu.RLock()
	defer p.metadataMu.RUnlock()
	return p.metadataPath
}

// PersistentState exposes a copy of the plugin's persistent variables.
func (p *BasePlugin) PersistentState() map[string]interface{} {
	p.metadataMu.RLock()
	defer p.metadataMu.RUnlock()
	if p.metadata == nil {
		return map[string]interface{}{}
	}
	return clonePersistentState(p.metadata.Persistent)
}

// UpdatePersistentState merges the provided values into the metadata file.
func (p *BasePlugin) UpdatePersistentState(updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	p.metadataMu.Lock()
	defer p.metadataMu.Unlock()

	if p.metadataPath == "" {
		return fmt.Errorf("plugin %s has no metadata path configured", p.name)
	}

	if p.metadata == nil {
		p.metadata = &FileMetadata{
			Name:        p.name,
			Version:     p.version,
			Description: p.description,
			Persistent:  map[string]interface{}{},
		}
	}

	if p.metadata.Persistent == nil {
		p.metadata.Persistent = map[string]interface{}{}
	}

	for key, value := range updates {
		if value == nil {
			delete(p.metadata.Persistent, key)
			continue
		}
		p.metadata.Persistent[key] = value
	}

	// Keep metadata descriptive fields in sync with the plugin definition.
	p.metadata.Name = p.name
	p.metadata.Version = p.version
	p.metadata.Description = p.description

	return SaveMetadata(p.metadataPath, p.metadata)
}
