package main

import (
	"io"
	"log"
	"os"
	"runtime"

	"github.com/aymanbagabas/go-pty"
)

// PTYManager 协调伪终端（Pseudo Terminal, PTY）的生命周期管理。
type PTYManager struct {
	pty    pty.Pty
	cmd    *pty.Cmd
	output chan []byte
	closed chan struct{}
}

// NewPTYManager 构造一个新的PTY管理器实例。
func NewPTYManager() (*PTYManager, error) {
	return &PTYManager{
		output: make(chan []byte, 100),
		closed: make(chan struct{}),
	}, nil
}

// Start 启动底层 shell 并开始流式传输输出。
func (m *PTYManager) Start() error {
	var shell string
	if runtime.GOOS == "windows" {
		shell = "C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe"
	} else {
		shell = "/bin/bash"
	}

	ptyInst, err := pty.New()
	if err != nil {
		return err
	}
	m.pty = ptyInst

	if err := m.pty.Resize(80, 30); err != nil {
		m.pty.Close()
		return err
	}

	if runtime.GOOS == "windows" {
		m.cmd = m.pty.Command(shell, "-NoLogo", "-ExecutionPolicy", "Bypass")
	} else {
		m.cmd = m.pty.Command(shell)
	}
	m.cmd.Env = os.Environ()

	if err := m.cmd.Start(); err != nil {
		m.pty.Close()
		return err
	}

	go m.readOutput()
	go m.waitForExit()

	return nil
}

// readOutput 循环读取PTY的输出，把内容写到output通道中。
func (m *PTYManager) readOutput() {
	buf := make([]byte, 4096)
	for {
		select {
		case <-m.closed:
			return
		default:
			n, err := m.pty.Read(buf)
			if err != nil {
				if err != io.EOF && err.Error() != "EOF" {
					log.Printf("PTY读取错误: %v", err)
				}
				if err == io.EOF {
					return
				}
				continue
			}
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				select {
				case m.output <- data:
				case <-m.closed:
					return
				}
			}
		}
	}
}

// waitForExit 等待 shell 进程退出并关闭closed通道。
func (m *PTYManager) waitForExit() {
	m.cmd.Wait()
	close(m.closed)
}

// Write 将输入数据转发到PTY。
func (m *PTYManager) Write(data []byte) error {
	_, err := m.pty.Write(data)
	return err
}

// Resize 调整终端窗口大小。
func (m *PTYManager) Resize(cols, rows int) error {
	if m.pty != nil {
		return m.pty.Resize(cols, rows)
	}
	return nil
}

// Close 关闭PTY会话。
func (m *PTYManager) Close() error {
	if m.pty != nil {
		select {
		case <-m.closed:
		default:
			close(m.closed)
			if m.cmd.Process != nil {
				m.cmd.Process.Kill()
			}
			m.pty.Close()
		}
	}
	return nil
}

// Output 返回PTY输出数据的通道（只读）。
func (m *PTYManager) Output() <-chan []byte {
	return m.output
}

// Closed 返回一个信号通道，当PTY命令终止时关闭。
func (m *PTYManager) Closed() <-chan struct{} {
	return m.closed
}
