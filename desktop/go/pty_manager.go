package main

import (
	"io"
	"log"
	"os"
	"runtime"

	"github.com/aymanbagabas/go-pty"
)

// PTYManager orchestrates the pseudo terminal lifecycle.
type PTYManager struct {
	pty    pty.Pty
	cmd    *pty.Cmd
	output chan []byte
	closed chan struct{}
}

// NewPTYManager constructs a PTY manager instance.
func NewPTYManager() (*PTYManager, error) {
	return &PTYManager{
		output: make(chan []byte, 100),
		closed: make(chan struct{}),
	}, nil
}

// Start spawns the underlying shell and begins streaming output.
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

func (m *PTYManager) waitForExit() {
	m.cmd.Wait()
	close(m.closed)
}

// Write forwards input to the PTY.
func (m *PTYManager) Write(data []byte) error {
	_, err := m.pty.Write(data)
	return err
}

// Resize adjusts the terminal size.
func (m *PTYManager) Resize(cols, rows int) error {
	if m.pty != nil {
		return m.pty.Resize(cols, rows)
	}
	return nil
}

// Close shuts down the PTY session.
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

// Output streams PTY output data.
func (m *PTYManager) Output() <-chan []byte {
	return m.output
}

// Closed signals when the PTY command has terminated.
func (m *PTYManager) Closed() <-chan struct{} {
	return m.closed
}
