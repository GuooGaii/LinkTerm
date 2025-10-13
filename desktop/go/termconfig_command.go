package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"terminal-client/types"
)

// ConfigNotifier defines the contract for propagating config updates.
type ConfigNotifier interface {
	NotifyConfigUpdated(*types.TerminalConfig)
}

// TermConfigCommand 提供终端外观配置的内置命令实现。
type TermConfigCommand struct {
	manager  *ConfigManager
	notifier ConfigNotifier
}

// NewTermConfigCommand 创建 termconfig 命令处理器。
func NewTermConfigCommand(manager *ConfigManager, notifier ConfigNotifier) *TermConfigCommand {
	return &TermConfigCommand{
		manager:  manager,
		notifier: notifier,
	}
}

// Usage 返回命令帮助信息。
func (c *TermConfigCommand) Usage() string {
	return `termconfig - 终端配置工具

用法:
  termconfig <命令> [参数]

命令:
  font                 显示交互式字体选择器
  font-size [大小]     设置字号 (8-48)
  theme                显示交互式主题选择器
  opacity <透明度>     设置透明度 (0.1-1.0)
  cursor [on|off]      开关光标闪烁
  line-height <行高>   设置行高 (1.0-2.0)

  list                 显示当前配置
  reset                重置为默认配置

示例:
  termconfig font                  # 交互式选择字体
  termconfig theme                 # 交互式选择主题
  termconfig font-size 16
  termconfig opacity 0.8
  termconfig cursor on
  termconfig reset`
}

// SubCommands 返回支持的子命令列表。
func (c *TermConfigCommand) SubCommands() []string {
	return []string{"font", "font-size", "theme", "opacity", "cursor", "line-height", "list", "reset"}
}

// Execute 运行 termconfig 命令。
func (c *TermConfigCommand) Execute(args []string) (bool, string) {
	if len(args) == 0 {
		return true, c.Usage()
	}

	subCmd := args[0]
	subArgs := []string{}
	if len(args) > 1 {
		subArgs = args[1:]
	}

	switch subCmd {
	case "font", "font-family":
		return c.handleFont(subArgs)
	case "font-size", "size":
		return c.handleFontSize(subArgs)
	case "theme":
		return c.handleTheme(subArgs)
	case "opacity":
		return c.handleOpacity(subArgs)
	case "cursor":
		return c.handleCursor(subArgs)
	case "line-height":
		return c.handleLineHeight(subArgs)
	case "list", "show", "current":
		return c.handleList()
	case "reset":
		return c.handleReset()
	default:
		return false, fmt.Sprintf("未知命令: %s\n\n%s", subCmd, c.Usage())
	}
}

func (c *TermConfigCommand) handleFont(args []string) (bool, string) {
	cfg, err := c.loadConfig()
	if err != nil {
		return false, fmt.Sprintf("❌ 加载配置失败: %v", err)
	}

	sortedFonts := getSortedFonts()
	if len(args) == 0 {
		choice := ""
		prompt := &survey.Select{
			Message: "选择字体",
			Options: sortedFonts,
			Default: extractMainFont(cfg.Terminal.FontFamily),
		}

		if err := survey.AskOne(prompt, &choice); err != nil {
			return false, fmt.Sprintf("❌ 选择字体失败: %v", err)
		}

		return c.saveAndPush(cfg, c.updateFontFamily(cfg, choice))
	}

	font := args[0]
	for _, candidate := range sortedFonts {
		if strings.EqualFold(candidate, font) {
			return c.saveAndPush(cfg, c.updateFontFamily(cfg, candidate))
		}
	}

	return false, fmt.Sprintf("❌ 未找到字体: %s\n可用字体: %v", font, sortedFonts)
}

func (c *TermConfigCommand) updateFontFamily(cfg *types.Config, font string) string {
	if cfg.Terminal == nil {
		cfg.Terminal = types.CloneTerminal(types.DefaultTerminal())
	}

	cfg.Terminal.FontFamily = fmt.Sprintf("%s, %s", font, types.DefaultTerminal().FontFamily)

	return fmt.Sprintf("✅ 字体已设置为: %s", font)
}

func (c *TermConfigCommand) handleFontSize(args []string) (bool, string) {
	cfg, err := c.loadConfig()
	if err != nil {
		return false, fmt.Sprintf("❌ 加载配置失败: %v", err)
	}

	if len(args) == 0 {
		return false, fmt.Sprintf("当前字体大小: %d\n用法: termconfig font-size <8-48>", cfg.Terminal.FontSize)
	}

	size, err := strconv.Atoi(args[0])
	if err != nil || size < 8 || size > 48 {
		return false, "字体大小必须是 8-48 之间的数字"
	}

	cfg.Terminal.FontSize = size
	return c.saveAndPush(cfg, fmt.Sprintf("✅ 字体大小已设置为: %d", size))
}

func (c *TermConfigCommand) handleTheme(args []string) (bool, string) {
	cfg, err := c.loadConfig()
	if err != nil {
		return false, fmt.Sprintf("❌ 加载配置失败: %v", err)
	}

	themes := types.AvailableThemes()
	if len(args) == 0 {
		choice := ""
		prompt := &survey.Select{
			Message: "选择主题",
			Options: themes,
			Default: cfg.Terminal.Theme,
		}

		if err := survey.AskOne(prompt, &choice); err != nil {
			return false, fmt.Sprintf("❌ 选择主题失败: %v", err)
		}
		return c.saveAndPush(cfg, c.updateTheme(cfg, choice))
	}

	theme := strings.ToLower(args[0])
	if types.HasTheme(theme) {
		return c.saveAndPush(cfg, c.updateTheme(cfg, theme))
	}

	return false, fmt.Sprintf("❌ 未找到主题: %s\n可用主题: %v", theme, themes)
}

func (c *TermConfigCommand) updateTheme(cfg *types.Config, theme string) string {
	if cfg.Terminal == nil {
		cfg.Terminal = types.CloneTerminal(types.DefaultTerminal())
	}

	cfg.Terminal.Theme = theme
	return fmt.Sprintf("✅ 主题已设置为: %s", theme)
}

func (c *TermConfigCommand) handleOpacity(args []string) (bool, string) {
	cfg, err := c.loadConfig()
	if err != nil {
		return false, fmt.Sprintf("❌ 加载配置失败: %v", err)
	}

	if len(args) == 0 {
		return false, fmt.Sprintf("当前透明度: %.1f\n用法: termconfig opacity <0.1-1.0>", cfg.Terminal.Opacity)
	}

	opacity, err := strconv.ParseFloat(args[0], 64)
	if err != nil || opacity < 0.1 || opacity > 1.0 {
		return false, "透明度必须是 0.1-1.0 之间的数字"
	}

	cfg.Terminal.Opacity = opacity
	return c.saveAndPush(cfg, fmt.Sprintf("✅ 透明度已设置为: %.1f", opacity))
}

func (c *TermConfigCommand) handleCursor(args []string) (bool, string) {
	cfg, err := c.loadConfig()
	if err != nil {
		return false, fmt.Sprintf("❌ 加载配置失败: %v", err)
	}

	if len(args) == 0 {
		status := "关闭"
		if cfg.Terminal.CursorBlink {
			status = "开启"
		}
		return false, fmt.Sprintf("当前光标闪烁: %s\n用法: termconfig cursor [on|off]", status)
	}

	switch strings.ToLower(args[0]) {
	case "on", "true", "1":
		cfg.Terminal.CursorBlink = true
		return c.saveAndPush(cfg, "✅ 已开启光标闪烁")
	case "off", "false", "0":
		cfg.Terminal.CursorBlink = false
		return c.saveAndPush(cfg, "✅ 已关闭光标闪烁")
	default:
		return false, "用法: termconfig cursor [on|off]"
	}
}

func (c *TermConfigCommand) handleLineHeight(args []string) (bool, string) {
	cfg, err := c.loadConfig()
	if err != nil {
		return false, fmt.Sprintf("❌ 加载配置失败: %v", err)
	}

	if len(args) == 0 {
		return false, fmt.Sprintf("当前行高: %.2f\n用法: termconfig line-height <1.0-2.0>", cfg.Terminal.LineHeight)
	}

	height, err := strconv.ParseFloat(args[0], 64)
	if err != nil || height < 1.0 || height > 2.0 {
		return false, "行高必须是 1.0-2.0 之间的数字"
	}

	cfg.Terminal.LineHeight = height
	return c.saveAndPush(cfg, fmt.Sprintf("✅ 行高已设置为: %.2f", height))
}

func (c *TermConfigCommand) handleList() (bool, string) {
	cfg, err := c.loadConfig()
	if err != nil {
		return false, fmt.Sprintf("❌ 加载配置失败: %v", err)
	}

	data, _ := json.MarshalIndent(cfg.Terminal, "", "  ")
	return true, fmt.Sprintf("当前配置:\n%s", string(data))
}

func (c *TermConfigCommand) handleReset() (bool, string) {
	prompt := &survey.Confirm{
		Message: "确定要重置为默认配置吗？",
		Default: false,
	}

	var result bool
	if err := survey.AskOne(prompt, &result); err != nil || !result {
		return true, "已取消"
	}

	cfg, err := c.loadConfig()
	if err != nil {
		return false, fmt.Sprintf("❌ 加载配置失败: %v", err)
	}

	cfg.Terminal = types.CloneTerminal(types.DefaultTerminal())
	return c.saveAndPush(cfg, "✅ 配置已重置为默认值")
}

func (c *TermConfigCommand) loadConfig() (*types.Config, error) {
	if c.manager == nil {
		return nil, errors.New("配置管理器不可用")
	}

	cfg, err := c.manager.Load()
	if err != nil {
		return &types.Config{
			Terminal: types.CloneTerminal(types.DefaultTerminal()),
		}, nil
	}

	return cfg, nil
}

func (c *TermConfigCommand) saveAndPush(cfg *types.Config, message string) (bool, string) {
	if c.manager == nil {
		return false, "配置管理器不可用"
	}

	if err := c.manager.Save(cfg); err != nil {
		return false, fmt.Sprintf("❌ 保存配置失败: %v", err)
	}

	if cfg.Terminal != nil && c.notifier != nil {
		c.notifier.NotifyConfigUpdated(cfg.Terminal)
	}

	return true, message
}

// extractMainFont 从字体族字符串中提取主字体名称。
func extractMainFont(fontFamily string) string {
	fontFamily = strings.Trim(fontFamily, `"`)
	fonts := strings.Split(fontFamily, ",")

	if len(fonts) > 0 {
		mainFont := strings.TrimSpace(fonts[0])
		mainFont = strings.Trim(mainFont, `"'\`)
		return mainFont
	}

	return ""
}

func getSortedFonts() []string {
	fonts := types.AvailableFonts()
	set := make(map[string]struct{}, len(fonts))
	result := make([]string, 0, len(fonts))

	for _, font := range fonts {
		trimmed := strings.TrimSpace(font)
		if trimmed == "" {
			continue
		}
		if _, exists := set[trimmed]; exists {
			continue
		}
		set[trimmed] = struct{}{}
		result = append(result, trimmed)
	}

	sort.Strings(result)
	return result
}
