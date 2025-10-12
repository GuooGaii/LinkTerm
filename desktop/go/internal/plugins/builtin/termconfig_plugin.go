package builtin

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

// TermConfigPlugin 管理终端外观和行为设置的命令插件。
type TermConfigPlugin struct {
	*plugins.BasePlugin
}

// NewTermConfigPlugin 创建 termconfig 插件。
func NewTermConfigPlugin() *TermConfigPlugin {
	return &TermConfigPlugin{
		BasePlugin: plugins.NewBasePlugin(
			"termconfig",
			"终端配置命令 - 管理终端外观和行为设置",
			"1.0.0",
		),
	}
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
  font <字体名称>      设置字体 (可用编号选择)
  font-size [大小]     设置字号 (8-48)
  theme [主题]         设置主题 (dark/light/monokai/dracula)
  opacity <透明度>     设置透明度 (0.1-1.0)
  cursor [on|off]      开关光标闪烁
  line-height <行高>   设置行高 (1.0-2.0)

  list                 显示当前配置
  reset                重置为默认配置

示例:
  termconfig font                  # 交互式选择字体
  termconfig font "JetBrains Mono"
  termconfig font 2
  termconfig font-size 16
  termconfig theme dark
  termconfig reset`
}

func (p *TermConfigPlugin) SubCommands() []string {
	return []string{"font", "font-size", "theme", "opacity", "cursor", "line-height", "list", "reset"}
}

func (p *TermConfigPlugin) Init(ctx context.Context) error {
	log.Printf("   初始化 termconfig 插件")
	return p.BasePlugin.Init(ctx)
}

func (p *TermConfigPlugin) Start(ctx context.Context) error {
	log.Printf("   启动 termconfig 插件")
	return p.BasePlugin.Start(ctx)
}

func (p *TermConfigPlugin) Stop(ctx context.Context) error {
	log.Printf("   停止 termconfig 插件")
	return p.BasePlugin.Stop(ctx)
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

	fonts := config.AvailableFonts()

	if len(args) == 0 {
		return p.runFontSurvey(ctx, cfg, fonts)
	}

	input := strings.Join(args, " ")
	selectedFont, err := p.resolveFontSelection(input, fonts)
	if err != nil {
		return false, err.Error()
	}

	cfg.Terminal.FontFamily = fmt.Sprintf("\"%s\", monospace", selectedFont)
	return p.saveAndPush(ctx, cfg, fmt.Sprintf("✅ 字体已设置为: %s", selectedFont))
}

func (p *TermConfigPlugin) runFontSurvey(ctx plugins.CommandContext, cfg *config.Config, fonts []string) (bool, string) {
	mainFont := extractMainFont(cfg.Terminal.FontFamily)
	defaultOption := 0
	for i, font := range fonts {
		if strings.EqualFold(font, mainFont) {
			defaultOption = i
			break
		}
	}

	prompt := &survey.Select{
		Message: "选择终端字体:",
		Options: fonts,
		Default: fonts[defaultOption],
	}

	var result string
	if err := survey.AskOne(prompt, &result); err != nil {
		return false, "操作已取消"
	}

	cfg.Terminal.FontFamily = fmt.Sprintf("\"%s\", monospace", result)
	return p.saveAndPush(ctx, cfg, fmt.Sprintf("✅ 字体已设置为: %s", result))
}

func (p *TermConfigPlugin) resolveFontSelection(input string, fonts []string) (string, error) {
	if index, err := strconv.Atoi(input); err == nil {
		index--
		if index < 0 || index >= len(fonts) {
			return "", fmt.Errorf("字体编号必须在 1-%d 之间", len(fonts))
		}
		return fonts[index], nil
	}

	trimmed := strings.TrimSpace(strings.Trim(input, `"'`))
	for _, font := range fonts {
		if strings.EqualFold(font, trimmed) {
			return font, nil
		}
	}

	return "", fmt.Errorf("未知字体: %s", input)
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
		return false, "字号必须是 8-48 之间的整数"
	}

	cfg.Terminal.FontSize = size
	return p.saveAndPush(ctx, cfg, fmt.Sprintf("✅ 字号已设置为: %d", size))
}

func (p *TermConfigPlugin) handleTheme(ctx plugins.CommandContext, args []string) (bool, string) {
	cfg, err := p.loadConfig(ctx)
	if err != nil {
		return false, fmt.Sprintf("❌ 加载配置失败: %v", err)
	}

	available := config.AvailableThemes()
	sort.Strings(available)

	if len(args) == 0 {
		return false, fmt.Sprintf("当前主题: %s\n用法: termconfig theme <%s>", cfg.Terminal.Theme, strings.Join(available, "|"))
	}

	theme := strings.ToLower(args[0])
	if !config.HasTheme(theme) {
		return false, fmt.Sprintf("未知主题: %s，可选: %s", theme, strings.Join(available, "/"))
	}

	cfg.Terminal.Theme = theme
	return p.saveAndPush(ctx, cfg, fmt.Sprintf("✅ 主题已设置为: %s", theme))
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
