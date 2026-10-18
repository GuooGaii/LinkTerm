package main

import (
	"fmt"
	"github.com/aymanbagabas/go-pty"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

// The PTY belongs to a tmux CLIENT, never the long-lived AI process.
type PTYManager struct {
	pty    pty.Pty
	cmd    *pty.Cmd
	output chan []byte
	closed chan struct{}
	once   sync.Once
}

func NewPTYManager() (*PTYManager, error) {
	return &PTYManager{output: make(chan []byte, 16), closed: make(chan struct{})}, nil
}
func (m *PTYManager) Start() error { return m.StartSized(80, 30) }
func (m *PTYManager) StartSized(cols, rows int) error {
	session := os.Getenv("LINKTERM_TMUX_SESSION")
	if session == "" {
		session = "claude-login"
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(session) {
		return fmt.Errorf("invalid tmux session name")
	}
	if err := exec.Command("tmux", "has-session", "-t", "="+session).Run(); err != nil {
		return fmt.Errorf("tmux session %s does not exist; start it first", session)
	}
	var err error
	m.pty, err = pty.New()
	if err != nil {
		return err
	}
	if err = m.pty.Resize(cols, rows); err != nil {
		m.pty.Close()
		return err
	}
	m.cmd = m.pty.Command("tmux", "-u", "attach-session", "-t", "="+session)
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "TERM=") && !strings.HasPrefix(e, "COLORTERM=") && !strings.HasPrefix(e, "TMUX=") {
			m.cmd.Env = append(m.cmd.Env, e)
		}
	}
	m.cmd.Env = append(m.cmd.Env, "TERM=xterm-256color", "COLORTERM=truecolor")
	if err = m.cmd.Start(); err != nil {
		m.pty.Close()
		return err
	}
	go m.readOutput()
	go func() { m.cmd.Wait(); m.finish() }()
	return nil
}
func (m *PTYManager) finish() { m.once.Do(func() { close(m.closed) }) }
func (m *PTYManager) readOutput() {
	buf := make([]byte, 4096)
	for {
		n, err := m.pty.Read(buf)
		if n > 0 {
			b := append([]byte(nil), buf[:n]...)
			select {
			case m.output <- b:
			case <-m.closed:
				return
			}
		}
		if err != nil {
			return
		}
	}
}
func (m *PTYManager) Write(b []byte) error { _, err := m.pty.Write(b); return err }
func (m *PTYManager) Resize(cols, rows int) error {
	if m.pty != nil {
		return m.pty.Resize(cols, rows)
	}
	return nil
}
func (m *PTYManager) Close() error {
	m.finish()
	if m.cmd != nil && m.cmd.Process != nil {
		if err := m.cmd.Process.Kill(); err != nil {
			log.Printf("tmux client already stopped")
		}
	}
	if m.pty != nil {
		return m.pty.Close()
	}
	return nil
}
func (m *PTYManager) Output() <-chan []byte   { return m.output }
func (m *PTYManager) Closed() <-chan struct{} { return m.closed }
