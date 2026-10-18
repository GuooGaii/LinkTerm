package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type WebSocketClient struct {
	config        *Config
	configManager *ConfigManager
	mu            sync.Mutex
	ws            *websocket.Conn
	stopChan      chan struct{}
	once          sync.Once
}
type wireMessage struct {
	Type  string          `json:"type"`
	Data  json.RawMessage `json:"data"`
	Epoch string          `json:"epoch"`
}
type attachment struct {
	manager *PTYManager
	epoch   string
	credits chan struct{}
	done    chan struct{}
	once    sync.Once
}

func (a *attachment) close() { a.once.Do(func() { close(a.done); a.manager.Close() }) }
func NewWebSocketClient(cfg *Config, manager *ConfigManager) *WebSocketClient {
	return &WebSocketClient{config: cfg, configManager: manager, stopChan: make(chan struct{})}
}
func (c *WebSocketClient) Start() error {
	if os.Getenv("LINKTERM_AGENT_TOKEN_FILE") == "" {
		return fmt.Errorf("LINKTERM_AGENT_TOKEN_FILE is required")
	}
	u, err := url.Parse(c.config.ServerURL)
	if err != nil {
		return err
	}
	if u.Scheme != "wss" && !(u.Scheme == "ws" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost")) {
		return fmt.Errorf("agent requires wss (ws allowed only on localhost for tests)")
	}
	go c.loop()
	return nil
}
func (c *WebSocketClient) loop() {
	for {
		select {
		case <-c.stopChan:
			return
		default:
		}
		if err := c.connect(); err != nil {
			log.Printf("LinkTerm disconnected: %v", err)
		}
		select {
		case <-c.stopChan:
			return
		case <-time.After(3 * time.Second):
		}
	}
}
func (c *WebSocketClient) send(ws *websocket.Conn, v interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return ws.WriteJSON(v)
}
func (c *WebSocketClient) connect() error {
	token, err := os.ReadFile(os.Getenv("LINKTERM_AGENT_TOKEN_FILE"))
	if err != nil {
		return err
	}
	u, err := url.Parse(c.config.ServerURL)
	if err != nil {
		return err
	}
	u.Path = "/agent"
	u.RawQuery = ""
	h := http.Header{}
	h.Set("Authorization", "Bearer "+strings.TrimSpace(string(token)))
	ws, _, err := websocket.DefaultDialer.Dial(u.String(), h)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.ws = ws
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		if c.ws == ws {
			c.ws = nil
		}
		c.mu.Unlock()
		ws.Close()
	}()
	ws.SetReadLimit(96 * 1024)
	ws.SetReadDeadline(time.Now().Add(50 * time.Second))
	stopPing := make(chan struct{})
	defer close(stopPing)
	go func() {
		t := time.NewTicker(15 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-stopPing:
				return
			case <-t.C:
				c.mu.Lock()
				ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
				err := ws.WriteMessage(websocket.TextMessage, []byte("ping"))
				c.mu.Unlock()
				if err != nil {
					ws.Close()
					return
				}
			}
		}
	}()
	log.Printf("Connected; attach existing tmux session from %s", c.config.WebURL)
	var current *attachment
	defer func() {
		if current != nil {
			current.close()
		}
	}()
	for {
		_, b, err := ws.ReadMessage()
		if err != nil {
			return err
		}
		ws.SetReadDeadline(time.Now().Add(50 * time.Second))
		if string(b) == "pong" {
			continue
		}
		var msg wireMessage
		if json.Unmarshal(b, &msg) != nil {
			continue
		}
		if msg.Type == "ack" {
			if current != nil && current.epoch == msg.Epoch {
				select {
				case <-current.credits:
				default:
				}
			}
			continue
		}
		if msg.Type == "input" {
			if current != nil && current.epoch == msg.Epoch {
				var s string
				if json.Unmarshal(msg.Data, &s) == nil && len(s) <= 16384 {
					current.manager.Write([]byte(s))
				}
			}
			continue
		}
		if msg.Type != "system" {
			continue
		}
		var data struct {
			Cmd  string     `json:"cmd"`
			Args ResizeArgs `json:"args"`
		}
		if json.Unmarshal(msg.Data, &data) != nil {
			continue
		}
		switch data.Cmd {
		case "attach":
			if current != nil {
				current.close()
				current = nil
			}
			if data.Args.Cols < 2 || data.Args.Cols > 500 || data.Args.Rows < 2 || data.Args.Rows > 200 || msg.Epoch == "" {
				continue
			}
			m, _ := NewPTYManager()
			if err := m.StartSized(data.Args.Cols, data.Args.Rows); err != nil {
				c.send(ws, map[string]interface{}{"type": "system", "epoch": msg.Epoch, "data": map[string]interface{}{"cmd": "error", "args": map[string]string{"message": err.Error()}}})
				continue
			}
			current = &attachment{manager: m, epoch: msg.Epoch, credits: make(chan struct{}, 8), done: make(chan struct{})}
			if err := c.send(ws, map[string]interface{}{"type": "system", "epoch": msg.Epoch, "data": map[string]string{"cmd": "ready"}}); err != nil {
				return err
			}
			go c.output(ws, current)
		case "resize":
			if current != nil && current.epoch == msg.Epoch && data.Args.Cols >= 2 && data.Args.Cols <= 500 && data.Args.Rows >= 2 && data.Args.Rows <= 200 {
				current.manager.Resize(data.Args.Cols, data.Args.Rows)
			}
		case "detach":
			if current != nil && current.epoch == msg.Epoch {
				current.close()
				current = nil
			}
		}
	}
}
func (c *WebSocketClient) output(ws *websocket.Conn, a *attachment) {
	for {
		select {
		case <-a.done:
			return
		case <-a.manager.Closed():
			c.send(ws, map[string]interface{}{"type": "system", "epoch": a.epoch, "data": map[string]interface{}{"cmd": "error", "args": map[string]string{"message": "tmux attachment ended; reconnect to resume"}}})
			return
		case b := <-a.manager.Output():
			select {
			case a.credits <- struct{}{}:
			case <-a.done:
				return
			}
			if err := c.send(ws, map[string]interface{}{"type": "output", "epoch": a.epoch, "data": base64.StdEncoding.EncodeToString(b)}); err != nil {
				ws.Close()
				return
			}
		}
	}
}
func (c *WebSocketClient) Stop() {
	c.once.Do(func() { close(c.stopChan) })
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ws != nil {
		c.ws.Close()
	}
}

// Legacy termconfig commands are local configuration only in tmux mode.
func (c *WebSocketClient) SendConfigUpdate(cfg *TerminalConfig) {
	log.Print("Terminal appearance is managed by the web client in tmux mode")
}
