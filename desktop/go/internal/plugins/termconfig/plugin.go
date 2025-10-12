package termconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"

	"terminal-client/internal/config"
	"terminal-client/internal/plugins"
)

const (
	pluginName        = "termconfig"
	pluginVersion     = "1.0.0"
	pluginDescription = "终端配置命令 - 管理终端外观和行为设置"
)

// TermConfigPlugin 管理终端外观和行为设置的命令插件。
type TermConfigPlugin struct{}

// New 创建 termconfig 插件。
func New() (*TermConfigPlugin, error) {
	return &TermConfigPlugin{}, nil
}

func (p *TermConfigPlugin) Name() string {
	return pluginName
}

func (p *TermConfigPlugin) Description() string {
	return pluginDescription
}

func (p *TermConfigPlugin) Version() string {
	return pluginVersion
}

func (p *TermConfigPlugin) Dependencies() []string {
	return nil
}

func (p *TermConfigPlugin) CommandName() string {
	return "termconfig"
}

func (p *TermConfigPlugin) Usage() string {
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

func (p *TermConfigPlugin) SubCommands() []string {
	return []string{"font", "font-size", "theme", "opacity", "cursor", "line-height", "list", "reset"}
}

func (p *TermConfigPlugin) Init(ctx context.Context) error {
	log.Printf("   初始化 termconfig 插件")
	return nil
}

func (p *TermConfigPlugin) Start(ctx context.Context) error {
	log.Printf("   启动 termconfig 插件")
	return nil
}

func (p *TermConfigPlugin) Stop(ctx context.Context) error {
	log.Printf("   停止 termconfig 插件")
	return nil
}

func (p *TermConfigPlugin) Execute(ctx plugins.CommandContext, args []string) (bool, string) {
	if len(args) == 0 {
		return true, p.Usage()
	}

	subCmd := args[0]
	subArgs := []string{}
	if len(args) > 1 {
		subArgs = args[1:]
	}

	switch subCmd {
	case "font", "font-family":
		return p.handleFont(ctx, subArgs)
	case "font-size", "size":
		return p.handleFontSize(ctx, subArgs)
	case "theme":
		return p.handleTheme(ctx, subArgs)
	case "opacity":
		return p.handleOpacity(ctx, subArgs)
	case "cursor":
		return p.handleCursor(ctx, subArgs)
	case "line-height":
		return p.handleLineHeight(ctx, subArgs)
	case "list", "show", "current":
		return p.handleList(ctx)
	case "reset":
		return p.handleReset(ctx)
	default:
		return false, fmt.Sprintf("未知命令: %s\n\n%s", subCmd, p.Usage())
	}
}

func (p *TermConfigPlugin) handleFont(ctx plugins.CommandContext, args []string) (bool, string) {
	cfg, err := p.loadConfig(ctx)
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

		return p.saveAndPush(ctx, cfg, p.updateFontFamily(cfg, choice))
	}

	font := args[0]
	for _, candidate := range sortedFonts {
		if strings.EqualFold(candidate, font) {
			return p.saveAndPush(ctx, cfg, p.updateFontFamily(cfg, candidate))
		}
	}

	return false, fmt.Sprintf("❌ 未找到字体: %s\n可用字体: %v", font, sortedFonts)
}

func (p *TermConfigPlugin) updateFontFamily(cfg *config.Config, font string) string {
	if cfg.Terminal == nil {
		cfg.Terminal = config.CloneTerminal(config.DefaultTerminal())
	}

	cfg.Terminal.FontFamily = fmt.Sprintf("%s, %s", font, config.DefaultTerminal().FontFamily)

	return fmt.Sprintf("✅ 字体已设置为: %s", font)
}

func (p *TermConfigPlugin) handleFontSize(ctx plugins.CommandContext, args []string) (bool, string) {
	cfg, err := p.loadConfig(ctx)
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
	return p.saveAndPush(ctx, cfg, fmt.Sprintf("✅ 字体大小已设置为: %d", size))
}

func (p *TermConfigPlugin) handleTheme(ctx plugins.CommandContext, args []string) (bool, string) {
	cfg, err := p.loadConfig(ctx)
	if err != nil {
		return false, fmt.Sprintf("❌ 加载配置失败: %v", err)
	}

	themes := []string{"light", "dark", "dracula", "nord", "monokai"}
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
		return p.saveAndPush(ctx, cfg, p.updateTheme(cfg, choice))
	}

	theme := strings.ToLower(args[0])
	for _, candidate := range themes {
		if candidate == theme {
			return p.saveAndPush(ctx, cfg, p.updateTheme(cfg, candidate))
		}
	}

	return false, fmt.Sprintf("❌ 未找到主题: %s\n可用主题: %v", theme, themes)
}

func (p *TermConfigPlugin) updateTheme(cfg *config.Config, theme string) string {
	if cfg.Terminal == nil {
		cfg.Terminal = config.CloneTerminal(config.DefaultTerminal())
	}

	cfg.Terminal.Theme = theme
	return fmt.Sprintf("✅ 主题已设置为: %s", theme)
}

func (p *TermConfigPlugin) handleOpacity(ctx plugins.CommandContext, args []string) (bool, string) {
	cfg, err := p.loadConfig(ctx)
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
	return p.saveAndPush(ctx, cfg, fmt.Sprintf("✅ 透明度已设置为: %.1f", opacity))
}

func (p *TermConfigPlugin) handleCursor(ctx plugins.CommandContext, args []string) (bool, string) {
	cfg, err := p.loadConfig(ctx)
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
		return p.saveAndPush(ctx, cfg, "✅ 已开启光标闪烁")
	case "off", "false", "0":
		cfg.Terminal.CursorBlink = false
		return p.saveAndPush(ctx, cfg, "✅ 已关闭光标闪烁")
	default:
		return false, "用法: termconfig cursor [on|off]"
	}
}

func (p *TermConfigPlugin) handleLineHeight(ctx plugins.CommandContext, args []string) (bool, string) {
	cfg, err := p.loadConfig(ctx)
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
	return p.saveAndPush(ctx, cfg, fmt.Sprintf("✅ 行高已设置为: %.2f", height))
}

func (p *TermConfigPlugin) handleList(ctx plugins.CommandContext) (bool, string) {
	cfg, err := p.loadConfig(ctx)
	if err != nil {
		return false, fmt.Sprintf("❌ 加载配置失败: %v", err)
	}

	data, _ := json.MarshalIndent(cfg.Terminal, "", "  ")
	return true, fmt.Sprintf("当前配置:\n%s", string(data))
}

func (p *TermConfigPlugin) handleReset(ctx plugins.CommandContext) (bool, string) {
	prompt := &survey.Confirm{
		Message: "确定要重置为默认配置吗？",
		Default: false,
	}

	var result bool
	if err := survey.AskOne(prompt, &result); err != nil || !result {
		return true, "已取消"
	}

	cfg, err := p.loadConfig(ctx)
	if err != nil {
		return false, fmt.Sprintf("❌ 加载配置失败: %v", err)
	}

	cfg.Terminal = config.CloneTerminal(config.DefaultTerminal())
	return p.saveAndPush(ctx, cfg, "✅ 配置已重置为默认值")
}

func (p *TermConfigPlugin) loadConfig(ctx plugins.CommandContext) (*config.Config, error) {
	if ctx == nil {
		return nil, errors.New("命令上下文不可用")
	}

	cfg, err := ctx.LoadConfig()
	if err != nil {
		if os.IsNotExist(err) {
			return &config.Config{
				Terminal: config.CloneTerminal(config.DefaultTerminal()),
			}, nil
		}
		return nil, err
	}

	if cfg.Terminal == nil {
		cfg.Terminal = config.CloneTerminal(config.DefaultTerminal())
	}
	return cfg, nil
}

func (p *TermConfigPlugin) saveAndPush(ctx plugins.CommandContext, cfg *config.Config, message string) (bool, string) {
	if ctx == nil {
		return false, "命令上下文不可用"
	}

	if err := ctx.SaveConfig(cfg); err != nil {
		return false, fmt.Sprintf("❌ 保存配置失败: %v", err)
	}

	if cfg.Terminal != nil {
		ctx.NotifyConfigUpdated(cfg.Terminal)
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

var defaultFontCandidates = []string{
	"JetBrains Mono",
	"Fira Code",
	"Source Code Pro",
	"Cascadia Code",
	"Monaco",
	"Consolas",
	"Inconsolata",
	"Menlo",
	"Ubuntu Mono",
	"DejaVu Sans Mono",
	"IBM Plex Mono",
}

func getSortedFonts() []string {
	fontSet := map[string]struct{}{}
	for _, font := range defaultFontCandidates {
		trimmed := strings.TrimSpace(font)
		if trimmed == "" {
			continue
		}
		fontSet[trimmed] = struct{}{}
	}

	fonts := make([]string, 0, len(fontSet))
	for font := range fontSet {
		fonts = append(fonts, font)
	}

	sort.Strings(fonts)
	return fonts
}

func init() {
	plugins.RegisterFactory("termconfig", func(pm *plugins.Manager) (plugins.Plugin, error) {
		return New()
	})
}
