package plugins

import (
	"context"
	"fmt"

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
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Version      string            `json:"version"`
	Author       string            `json:"author"`
	Dependencies []string          `json:"dependencies"`
	Config       map[string]string `json:"config"`
	Enabled      bool              `json:"enabled"`
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
	name        string
	description string
	version     string
}

// NewBasePlugin instantiates a helper implementing the basic Plugin contract.
func NewBasePlugin(name, description, version string) *BasePlugin {
	return &BasePlugin{
		name:        name,
		description: description,
		version:     version,
	}
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
