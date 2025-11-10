package types

import "strings"

// Config 保存持久化的应用程序配置。
type Config struct {
	ServerURL string          `json:"serverUrl"`
	WebURL    string          `json:"webUrl"`
	DeviceID  string          `json:"deviceId"`
	Terminal  *TerminalConfig `json:"terminal,omitempty"`
}

// TerminalConfig 控制伪终端的行为和外观。
type TerminalConfig struct {
	FontFamily  string  `json:"fontFamily"`
	FontSize    int     `json:"fontSize"`
	Theme       string  `json:"theme"`
	Opacity     float64 `json:"opacity"`
	CursorBlink bool    `json:"cursorBlink"`
	LineHeight  float64 `json:"lineHeight"`
}

var (
	// 默认终端配置
	defaultTerminalConfig = TerminalConfig{
		FontFamily:  "\"Cascadia Code\", \"JetBrains Mono\", \"Fira Code\", monospace",
		FontSize:    14,
		Theme:       "dark",
		Opacity:     1.0,
		CursorBlink: true,
		LineHeight:  1.2,
	}

	// 可用主题配色
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

	// 可用字体族
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

// CloneTerminal 复制提供的终端配置。
func CloneTerminal(cfg TerminalConfig) *TerminalConfig {
	copy := cfg
	return &copy
}

// DefaultTerminal 返回默认的终端配置。
func DefaultTerminal() TerminalConfig {
	return defaultTerminalConfig
}

// NormalizeTerminal 清理可能不完整的终端配置，并使用默认值填充缺失项。
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

// ThemeColors 返回指定主题的调色板。
func ThemeColors(theme string) map[string]string {
	if colors, ok := availableThemes[theme]; ok {
		return colors
	}
	return availableThemes["dark"]
}

// AvailableThemes 返回已配置的主题名称。
func AvailableThemes() []string {
	names := make([]string, 0, len(availableThemes))
	for name := range availableThemes {
		names = append(names, name)
	}
	return names
}

// HasTheme 检查主题是否存在。
func HasTheme(theme string) bool {
	_, ok := availableThemes[theme]
	return ok
}

// AvailableFonts 返回预定义的字体族列表。
func AvailableFonts() []string {
	return availableFonts
}
