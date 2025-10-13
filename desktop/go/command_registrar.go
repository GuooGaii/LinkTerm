package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

// CommandRegistrar manages temporary command registrations for the current session.
type CommandRegistrar struct {
	tempDir     string
	programPath string
	commands    []string
}

// NewCommandRegistrar builds a new registrar with the provided command list.
func NewCommandRegistrar(commands []string) *CommandRegistrar {
	programPath, _ := os.Executable()

	return &CommandRegistrar{
		programPath: programPath,
		commands:    commands,
	}
}

// Register writes helper wrappers and amends PATH for the current session.
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

// Unregister cleans up all temporary commands.
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
