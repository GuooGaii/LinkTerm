package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

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

func executePluginCommand(command string, args []string, configPath string) {
	cfgManager := NewConfigManager(configPath)

	prevOutput := log.Writer()
	log.SetOutput(io.Discard)
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
