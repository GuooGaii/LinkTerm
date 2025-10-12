package termconfig

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
)

const (
	termconfigPluginName        = "termconfig"
	termconfigPluginVersion     = "1.0.0"
	termconfigPluginDescription = "终端配置命令 - 管理终端外观和行为设置"
)

// CommandContext 接口定义，避免循环依赖
type CommandContext interface {
	LoadConfig() (*Config, error)
	SaveConfig(*Config) error
	ConfigPath() string
	NotifyConfigUpdated(*TerminalConfig)
}

// Config 配置结构
type Config struct {
	ServerURL string          `json:"serverUrl"`
	WebURL    string          `json:"webUrl"`
	DeviceID  string          `json:"deviceId"`
	Terminal  *TerminalConfig `json:"terminal"`
}

// TerminalConfig 终端配置结构
type TerminalConfig struct {
	FontFamily  string  `json:"fontFamily"`
	FontSize    int     `json:"fontSize"`
	Theme       string  `json:"theme"`
	Opacity     float64 `json:"opacity"`
	CursorBlink bool    `json:"cursorBlink"`
	LineHeight  float64 `json:"lineHeight"`
}

// TermConfigPlugin 管理终端外观和行为设置的命令插件。
type TermConfigPlugin struct{}

// NewPlugin 创建 termconfig 插件（导出供插件加载器使用）。
func NewPlugin(pm interface{}) (interface{}, error) {
	return &TermConfigPlugin{}, nil
}

func (p *TermConfigPlugin) Name() string {
	return termconfigPluginName
}

func (p *TermConfigPlugin) Description() string {
	return termconfigPluginDescription
}

func (p *TermConfigPlugin) Version() string {
	return termconfigPluginVersion
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

// DefaultTerminal 返回默认终端配置
func DefaultTerminal() *TerminalConfig {
	return &TerminalConfig{
		FontFamily:  "\"Cascadia Code\", monospace",
		FontSize:    14,
		Theme:       "dark",
		Opacity:     1.0,
		CursorBlink: true,
		LineHeight:  1.2,
	}
}

// CloneTerminal 克隆终端配置
func CloneTerminal(src *TerminalConfig) *TerminalConfig {
	if src == nil {
		return DefaultTerminal()
	}
	clone := *src
	return &clone
}

func (p *TermConfigPlugin) Execute(ctx interface{}, args []string) (bool, string) {
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

func (p *TermConfigPlugin) handleFont(ctx interface{}, args []string) (bool, string) {
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

func (p *TermConfigPlugin) updateFontFamily(cfg *Config, font string) string {
	if cfg.Terminal == nil {
		cfg.Terminal = CloneTerminal(DefaultTerminal())
	}

	cfg.Terminal.FontFamily = fmt.Sprintf("%s, %s", font, DefaultTerminal().FontFamily)

	return fmt.Sprintf("✅ 字体已设置为: %s", font)
}

func (p *TermConfigPlugin) handleFontSize(ctx interface{}, args []string) (bool, string) {
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

func (p *TermConfigPlugin) handleTheme(ctx interface{}, args []string) (bool, string) {
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

func (p *TermConfigPlugin) updateTheme(cfg *Config, theme string) string {
	if cfg.Terminal == nil {
		cfg.Terminal = CloneTerminal(DefaultTerminal())
	}

	cfg.Terminal.Theme = theme
	return fmt.Sprintf("✅ 主题已设置为: %s", theme)
}

func (p *TermConfigPlugin) handleOpacity(ctx interface{}, args []string) (bool, string) {
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

func (p *TermConfigPlugin) handleCursor(ctx interface{}, args []string) (bool, string) {
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

func (p *TermConfigPlugin) handleLineHeight(ctx interface{}, args []string) (bool, string) {
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

func (p *TermConfigPlugin) handleList(ctx interface{}) (bool, string) {
	cfg, err := p.loadConfig(ctx)
	if err != nil {
		return false, fmt.Sprintf("❌ 加载配置失败: %v", err)
	}

	data, _ := json.MarshalIndent(cfg.Terminal, "", "  ")
	return true, fmt.Sprintf("当前配置:\n%s", string(data))
}

func (p *TermConfigPlugin) handleReset(ctx interface{}) (bool, string) {
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

	cfg.Terminal = CloneTerminal(DefaultTerminal())
	return p.saveAndPush(ctx, cfg, "✅ 配置已重置为默认值")
}

func (p *TermConfigPlugin) loadConfig(ctx interface{}) (*Config, error) {
	if ctx == nil {
		return nil, errors.New("命令上下文不可用")
	}

	// 使用鸭子类型 - 查找有 LoadConfig 方法的类型
	// 使用反射风格的接口检查，因为返回类型可能不同
	type configLoaderAny interface {
		LoadConfig() (interface{}, error)
	}

	// 尝试直接调用 LoadConfig
	val := reflect.ValueOf(ctx)
	method := val.MethodByName("LoadConfig")
	if !method.IsValid() {
		return nil, errors.New("上下文不支持配置加载")
	}

	results := method.Call(nil)
	if len(results) != 2 {
		return nil, errors.New("LoadConfig 返回值格式不正确")
	}

	// 检查错误
	if !results[1].IsNil() {
		err := results[1].Interface().(error)
		if os.IsNotExist(err) {
			return &Config{
				Terminal: CloneTerminal(DefaultTerminal()),
			}, nil
		}
		return nil, err
	}

	// 转换配置
	cfgInterface := results[0].Interface()
	if cfgInterface == nil {
		return &Config{
			Terminal: CloneTerminal(DefaultTerminal()),
		}, nil
	}

	// 使用类型断言或 JSON 序列化来转换
	if cfg, ok := cfgInterface.(*Config); ok {
		if cfg.Terminal == nil {
			cfg.Terminal = CloneTerminal(DefaultTerminal())
		}
		return cfg, nil
	}

	// 尝试通过 JSON 转换
	jsonData, err := json.Marshal(cfgInterface)
	if err != nil {
		return nil, fmt.Errorf("配置类型转换失败: %v", err)
	}

	var cfg Config
	if err := json.Unmarshal(jsonData, &cfg); err != nil {
		return nil, fmt.Errorf("配置解析失败: %v", err)
	}

	if cfg.Terminal == nil {
		cfg.Terminal = CloneTerminal(DefaultTerminal())
	}
	return &cfg, nil
}

func (p *TermConfigPlugin) saveAndPush(ctx interface{}, cfg *Config, message string) (bool, string) {
	if ctx == nil {
		return false, "命令上下文不可用"
	}

	// 使用反射调用 SaveConfig
	val := reflect.ValueOf(ctx)
	saveMethod := val.MethodByName("SaveConfig")
	if !saveMethod.IsValid() {
		return false, "上下文不支持配置保存"
	}

	// 需要将 termconfig.Config 转换为 main.Config
	// 使用 JSON 序列化/反序列化来进行类型转换
	jsonData, err := json.Marshal(cfg)
	if err != nil {
		return false, fmt.Sprintf("❌ 配置序列化失败: %v", err)
	}

	// 获取 SaveConfig 方法的参数类型
	saveMethodType := saveMethod.Type()
	if saveMethodType.NumIn() != 1 {
		return false, "SaveConfig 方法签名不正确"
	}

	// 创建目标类型的实例
	paramType := saveMethodType.In(0)
	if paramType.Kind() != reflect.Ptr {
		return false, "SaveConfig 参数类型必须是指针"
	}

	// 创建新的配置实例并反序列化
	newCfgPtr := reflect.New(paramType.Elem())
	if err := json.Unmarshal(jsonData, newCfgPtr.Interface()); err != nil {
		return false, fmt.Sprintf("❌ 配置反序列化失败: %v", err)
	}

	// 调用 SaveConfig
	results := saveMethod.Call([]reflect.Value{newCfgPtr})
	if len(results) > 0 && !results[0].IsNil() {
		err := results[0].Interface().(error)
		return false, fmt.Sprintf("❌ 保存配置失败: %v", err)
	}

	// 尝试通知配置更新
	if cfg.Terminal != nil {
		notifyMethod := val.MethodByName("NotifyConfigUpdated")
		if notifyMethod.IsValid() {
			// 同样需要类型转换
			terminalJSON, _ := json.Marshal(cfg.Terminal)
			notifyMethodType := notifyMethod.Type()
			if notifyMethodType.NumIn() == 1 {
				terminalParamType := notifyMethodType.In(0)
				if terminalParamType.Kind() == reflect.Ptr {
					newTerminalPtr := reflect.New(terminalParamType.Elem())
					if err := json.Unmarshal(terminalJSON, newTerminalPtr.Interface()); err == nil {
						notifyMethod.Call([]reflect.Value{newTerminalPtr})
					}
				}
			}
		}
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
