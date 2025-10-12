package main

import "strings"

// Config holds the persisted application configuration.
type Config struct {
	ServerURL string          `json:"serverUrl"`
	WebURL    string          `json:"webUrl"`
	DeviceID  string          `json:"deviceId"`
	Terminal  *TerminalConfig `json:"terminal,omitempty"`
}

// TerminalConfig controls the behaviour and appearance of the pseudo terminal.
type TerminalConfig struct {
	FontFamily  string  `json:"fontFamily"`
	FontSize    int     `json:"fontSize"`
	Theme       string  `json:"theme"`
	Opacity     float64 `json:"opacity"`
	CursorBlink bool    `json:"cursorBlink"`
	LineHeight  float64 `json:"lineHeight"`
}

var (
	defaultTerminalConfig = TerminalConfig{
		FontFamily:  "\"Cascadia Code\", \"JetBrains Mono\", \"Fira Code\", monospace",
		FontSize:    14,
		Theme:       "dark",
		Opacity:     1.0,
		CursorBlink: true,
		LineHeight:  1.2,
	}

	availableThemes = map[string]map[string]string{
		"dark": {
			"background": "#0a0a0a",
			"foreground": "#ffffff",
			"cursor":     "#10b981",
		},
		"light": {
			"background": "#ffffff",
			"foreground": "#000000",
			"cursor":     "#0066cc",
		},
		"monokai": {
			"background": "#272822",
			"foreground": "#f8f8f2",
			"cursor":     "#f92672",
		},
		"dracula": {
			"background": "#282a36",
			"foreground": "#f8f8f2",
			"cursor":     "#50fa7b",
		},
	}

	availableFonts = []string{
		"Cascadia Code",
		"JetBrains Mono",
		"Fira Code",
		"Monaco",
		"Consolas",
		"Courier New",
		"Ubuntu Mono",
		"Source Code Pro",
		"SF Mono",
		"Menlo",
	}
)

// CloneTerminal copies the provided terminal configuration.
func CloneTerminal(cfg TerminalConfig) *TerminalConfig {
	copy := cfg
	return &copy
}

// DefaultTerminal returns the default terminal configuration.
func DefaultTerminal() TerminalConfig {
	return defaultTerminalConfig
}

// NormalizeTerminal cleans up a potentially incomplete terminal configuration and
// fills missing values using defaults.
func NormalizeTerminal(cfg *TerminalConfig) *TerminalConfig {
	result := defaultTerminalConfig
	if cfg != nil {
		if strings.TrimSpace(cfg.FontFamily) != "" {
			result.FontFamily = cfg.FontFamily
		}
		if cfg.FontSize > 0 {
			result.FontSize = cfg.FontSize
		}
		if strings.TrimSpace(cfg.Theme) != "" {
			result.Theme = cfg.Theme
		}
		if cfg.Opacity > 0 {
			result.Opacity = cfg.Opacity
		}
		if cfg.LineHeight > 0 {
			result.LineHeight = cfg.LineHeight
		}
		result.CursorBlink = cfg.CursorBlink
	}
	return CloneTerminal(result)
}

// ThemeColors returns the palette for the given theme.
func ThemeColors(theme string) map[string]string {
	if colors, ok := availableThemes[theme]; ok {
		return colors
	}
	return availableThemes["dark"]
}

// AvailableThemes returns the configured theme names.
func AvailableThemes() []string {
	names := make([]string, 0, len(availableThemes))
	for name := range availableThemes {
		names = append(names, name)
	}
	return names
}

// HasTheme reports whether the theme exists.
func HasTheme(theme string) bool {
	_, ok := availableThemes[theme]
	return ok
}

// AvailableFonts returns the predefined font families.
func AvailableFonts() []string {
	return availableFonts
}
