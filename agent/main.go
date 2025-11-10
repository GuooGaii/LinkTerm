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
	commandArgs := args[2:]

	firstArg := args[1]
	switch firstArg {
	case "config":
		executeTermConfigCommand(commandArgs, configPath)
		return true
	case "plugin":
		fmt.Fprintln(os.Stderr, "插件系统已移除")
		os.Exit(1)
	case "termconfig":
		executeTermConfigCommand(commandArgs, configPath)
		return true
	}

	if !strings.HasPrefix(firstArg, "-") {
		fmt.Fprintf(os.Stderr, "未知命令: %s\n", firstArg)
		os.Exit(1)
	}

	return false
}

func executeTermConfigCommand(args []string, configPath string) {
	cfgManager := NewConfigManager(configPath)

	prevOutput := log.Writer()
	log.SetOutput(io.Discard) // 禁用日志输出
	defer log.SetOutput(prevOutput)

	command := NewTermConfigCommand(cfgManager, nil)
	success, output := command.Execute(args)
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
		fmt.Println("  2. 编辑配置文件：config.json")
		fmt.Println("\n示例：")
		fmt.Println("  ./terminal -server wss://xxx.workers.dev -web https://xxx.workers.dev")
		os.Exit(1)
	}

	fmt.Println("\n正在启动服务...")

	var registrar *CommandRegistrar
	if reg := NewCommandRegistrar([]string{"termconfig"}); reg != nil {
		if err := reg.Register(); err != nil {
			log.Printf("警告: 注册临时命令失败: %v", err)
		} else {
			registrar = reg
		}
	}

	client := NewWebSocketClient(cfg, cfgManager)

	if err := client.Start(); err != nil {
		if registrar != nil {
			registrar.Unregister()
		}
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

	client.Stop()
}
