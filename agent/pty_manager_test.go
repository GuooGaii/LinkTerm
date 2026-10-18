package main

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestAttachResizeReconnectPreservesExistingSession(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux required for PTY integration")
	}
	t.Setenv("TMUX_TMPDIR", t.TempDir())
	t.Setenv("LINKTERM_TMUX_SESSION", "linkterm-test")
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("LC_ALL", "C")
	if b, err := exec.Command("tmux", "new-session", "-d", "-s", "linkterm-test", "sh -c 'printf ORIGINAL_SESSION中文验证; exec sleep 120'").CombinedOutput(); err != nil {
		t.Fatalf("%s: %v", b, err)
	}
	defer exec.Command("tmux", "kill-server").Run()
	for i := 0; i < 2; i++ {
		m, _ := NewPTYManager()
		if err := m.StartSized(91, 31); err != nil {
			t.Fatal(err)
		}
		var output strings.Builder
		timer := time.NewTimer(5 * time.Second)
		for !strings.Contains(output.String(), "ORIGINAL_SESSION中文验证") {
			select {
			case b := <-m.Output():
				output.Write(b)
			case <-timer.C:
				m.Close()
				t.Fatal("existing session was not repainted")
			}
		}
		timer.Stop()
		if err := m.Resize(101, 35); err != nil {
			t.Fatal(err)
		}
		deadline := time.Now().Add(2 * time.Second)
		for {
			b, _ := exec.Command("tmux", "display-message", "-p", "-t", "linkterm-test:0", "#{window_width}").CombinedOutput()
			if strings.TrimSpace(string(b)) == "101" {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("resize not delivered: %s", b)
			}
			time.Sleep(20 * time.Millisecond)
		}
		m.Close()
		if err := exec.Command("tmux", "has-session", "-t", "=linkterm-test").Run(); err != nil {
			t.Fatal("disconnect killed session")
		}
	}
}
