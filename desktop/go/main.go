package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"terminal-client/plugins"
	_ "terminal-client/plugins/info"       // 自动注册 info 插件
	_ "terminal-client/plugins/termconfig" // 自动注册 termconfig 插件
	"terminal-client/types"
)

// 为了方便在 main 包中使用，创建类型别名
type Config = types.Config
type TerminalConfig = types.TerminalConfig

func main() {
	if handleCLIMode(os.Args) {
		return
	}

	serverURL := flag.String("server", "", "服务器 WebSocket 地址 (例如: wss://terminal.example.workers.dev)")
	webURL := flag.String("web", "", "Web 访问地址 (例如: https://terminal.example.workers.dev)")
	configPath := flag.String("config", "config.json", "配置文件路径")
	flag.Parse()

	cfgManager := NewConfigManager(*configPath)

	var cfg *Config
	var err error

	if *serverURL != "" && *webURL != "" {
		existing, _ := cfgManager.Load()
		cfg = CreateConfigWithDeviceID(existing, *serverURL, *webURL)
		if err := cfgManager.Save(cfg); err != nil {
			log.Fatalf("保存配置失败: %v", err)
		}
		log.Println("使用命令行参数配置")
	} else {
		cfg, err = cfgManager.Load()
		if err != nil {
			if os.IsNotExist(err) {
				cfg = nil
			} else {
				log.Fatalf("加载配置失败: %v", err)
			}
		}
	}

	runServer(cfgManager, cfg)
}

func handleCLIMode(args []string) bool {
	if len(args) <= 1 {
		return false
	}

	configPath := "config.json"

	firstArg := args[1]
	switch firstArg {
	case "config":
		executePluginCommand("termconfig", args[2:], configPath)
		return true
	case "plugin":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "用法: terminal plugin <插件名> [参数]")
			os.Exit(1)
		}
		executePluginCommand(args[2], args[3:], configPath)
		return true
	}

	if !strings.HasPrefix(firstArg, "-") {
		executePluginCommand(firstArg, args[2:], configPath)
		return true
	}

	return false
}

// BuildPluginManager 构建插件管理器并自动发现加载插件。
func BuildPluginManager() (*PluginManager, error) {
	manager := NewPluginManager()

	// 尝试多个可能的插件目录位置
	var pluginsDir string
	var entries map[string]*plugins.PluginEntry
	var err error

	// 1. 优先尝试当前工作目录（开发环境）
	if cwd, _ := os.Getwd(); cwd != "" {
		pluginsDir = filepath.Join(cwd, "plugins")
		if stat, statErr := os.Stat(pluginsDir); statErr == nil && stat.IsDir() {
			entries, err = plugins.DiscoverPlugins(pluginsDir)
			if err == nil && len(entries) > 0 {
				log.Printf("从工作目录加载插件: %s", pluginsDir)
			}
		}
	}

	// 2. 如果工作目录没找到，尝试可执行文件所在目录（生产环境）
	if entries == nil || len(entries) == 0 {
		if exePath, exeErr := os.Executable(); exeErr == nil {
			exeDir := filepath.Dir(exePath)
			pluginsDir = filepath.Join(exeDir, "plugins")
			if stat, statErr := os.Stat(pluginsDir); statErr == nil && stat.IsDir() {
				entries, err = plugins.DiscoverPlugins(pluginsDir)
				if err == nil && len(entries) > 0 {
					log.Printf("从可执行文件目录加载插件: %s", pluginsDir)
				}
			}
		}
	}

	// 3. 如果都找不到，返回错误
	if entries == nil || len(entries) == 0 {
		return nil, fmt.Errorf("未找到插件目录或插件为空")
	}

	if err != nil {
		return nil, fmt.Errorf("发现插件失败: %w", err)
	}

	// 注册所有发现的插件
	for _, entry := range entries {
		if err := manager.Register(entry); err != nil {
			return nil, fmt.Errorf("注册插件 %s 失败: %w", entry.Manifest.Name, err)
		}
	}

	return manager, nil
}

func executePluginCommand(command string, args []string, configPath string) {
	cfgManager := NewConfigManager(configPath)

	prevOutput := log.Writer()
	log.SetOutput(io.Discard) // 禁用日志输出
	defer log.SetOutput(prevOutput)

	pluginManager, err := BuildPluginManager()
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化插件失败: %v\n", err)
		os.Exit(1)
	}

	if err := pluginManager.InitAll(); err != nil {
		fmt.Fprintf(os.Stderr, "初始化插件失败: %v\n", err)
		os.Exit(1)
	}

	ctx := NewCommandRuntime(cfgManager, nil)
	success, output := pluginManager.ExecuteCommand(command, ctx, args)
	fmt.Print(output)
	if !success {
		os.Exit(1)
	}
}

func runServer(cfgManager *ConfigManager, cfg *Config) {
	if cfg == nil {
		fmt.Println("\n❌ 错误：未配置服务器地址")
		fmt.Println("请使用以下方式之一进行配置：")
		fmt.Println("  1. 使用命令行参数：-server <url> -web <url>")
		fmt.Println("  2. 使用配置命令：./terminal termconfig set server <url>")
		fmt.Println("\n示例：")
		fmt.Println("  ./terminal -server wss://xxx.workers.dev -web https://xxx.workers.dev")
		os.Exit(1)
	}

	fmt.Println("\n正在启动服务...")

	pluginManager, err := BuildPluginManager()
	if err != nil {
		log.Fatalf("构建插件管理器失败: %v", err)
	}

	if err := pluginManager.InitAll(); err != nil {
		log.Fatalf("初始化插件失败: %v", err)
	}

	if err := pluginManager.StartAll(); err != nil {
		log.Fatalf("启动插件失败: %v", err)
	}

	var registrar *CommandRegistrar
	commands := pluginManager.ListCommands()
	if len(commands) > 0 {
		if reg := NewCommandRegistrar(commands); reg != nil {
			if err := reg.Register(); err != nil {
				log.Printf("警告: 注册临时命令失败: %v", err)
			} else {
				registrar = reg
			}
		}
	}

	client := NewWebSocketClient(cfg, cfgManager)
	client.SetPluginManager(pluginManager)
	client.SetCommandContext(NewCommandRuntime(cfgManager, client))

	if err := client.Start(); err != nil {
		if registrar != nil {
			registrar.Unregister()
		}
		pluginManager.StopAll()
		log.Fatalf("启动客户端失败: %v", err)
	}

	fmt.Println("\n✅ 服务已启动")
	fmt.Println("按 Ctrl+C 停止服务")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n正在关闭...")

	if registrar != nil {
		registrar.Unregister()
	}

	pluginManager.StopAll()
	client.Stop()
}
