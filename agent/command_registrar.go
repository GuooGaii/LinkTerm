package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

// CommandRegistrar 用于管理当前会话的临时命令注册。
type CommandRegistrar struct {
	tempDir     string   // 临时目录路径
	programPath string   // 程序本身的路径
	commands    []string // 命令列表
}

// NewCommandRegistrar 使用提供的命令列表创建一个新的注册器。
func NewCommandRegistrar(commands []string) *CommandRegistrar {
	programPath, _ := os.Executable()

	return &CommandRegistrar{
		programPath: programPath,
		commands:    commands,
	}
}

// Register 写入辅助脚本并为当前会话修改 PATH。
func (r *CommandRegistrar) Register() error {
	tempDir, err := os.MkdirTemp("", "terminal-commands-")
	if err != nil {
		return fmt.Errorf("创建临时目录失败: %w", err)
	}
	r.tempDir = tempDir

	if runtime.GOOS == "windows" {
		return r.registerCommands(".bat", windowsWrapperTemplate, ";")
	}
	return r.registerCommands("", unixWrapperTemplate, ":")
}

// registerCommands 为所有命令写入脚本，并修改 PATH 环境变量。
func (r *CommandRegistrar) registerCommands(suffix, scriptTemplate, pathSeparator string) error {
	for _, cmdName := range r.commands {
		scriptPath := filepath.Join(r.tempDir, cmdName+suffix)

		content := fmt.Sprintf(scriptTemplate, r.programPath, cmdName)

		if err := os.WriteFile(scriptPath, []byte(content), 0o755); err != nil {
			return fmt.Errorf("创建命令脚本 %s 失败: %w", cmdName, err)
		}

		log.Printf("✅ %s 命令已注册: %s", cmdName, scriptPath)
	}

	currentPath := os.Getenv("PATH")
	newPath := r.tempDir + pathSeparator + currentPath
	os.Setenv("PATH", newPath)
	log.Printf("   提示: 现在可以在终端中使用这些命令了: %v", r.commands)

	return nil
}

// Unregister 清理所有临时命令。
func (r *CommandRegistrar) Unregister() {
	if r.tempDir != "" {
		if err := os.RemoveAll(r.tempDir); err != nil {
			log.Printf("清理临时目录失败: %v", err)
		} else {
			log.Printf("✅ 已清理命令: %v", r.commands)
		}
	}
}

const (
	windowsWrapperTemplate = `@echo off
"%s" %s %%*
`
	unixWrapperTemplate = `#!/bin/bash
"%s" %s "$@"
`
)
