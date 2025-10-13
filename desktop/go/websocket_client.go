package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const defaultTerminalID = "default"

type WebSocketClient struct {
	config        *Config
	configManager *ConfigManager
	ws            *websocket.Conn
	sessionID     string
	mu            sync.Mutex
	stopChan      chan struct{}
	reconnectCh   chan struct{}
	lastConfigMod time.Time
	sessions      map[string]*terminalSession
}

type terminalSession struct {
	id      string
	manager *PTYManager
}

func NewWebSocketClient(cfg *Config, manager *ConfigManager) *WebSocketClient {
	client := &WebSocketClient{
		config:        cfg,
		configManager: manager,
		sessionID:     cfg.DeviceID,
		stopChan:      make(chan struct{}),
		reconnectCh:   make(chan struct{}, 1),
		sessions:      make(map[string]*terminalSession),
	}
	return client
}

func (c *WebSocketClient) normalizeTerminalID(id string) string {
	if strings.TrimSpace(id) == "" {
		return defaultTerminalID
	}
	return strings.TrimSpace(id)
}

func (c *WebSocketClient) ensureSession(id string) (*terminalSession, error) {
	terminalID := c.normalizeTerminalID(id)

	c.mu.Lock()
	if session, ok := c.sessions[terminalID]; ok {
		c.mu.Unlock()
		return session, nil
	}
	c.mu.Unlock()

	manager, err := NewPTYManager()
	if err != nil {
		return nil, fmt.Errorf("创建 PTY 失败: %w", err)
	}

	if err := manager.Start(); err != nil {
		manager.Close()
		return nil, fmt.Errorf("启动 PTY 失败: %w", err)
	}

	session := &terminalSession{
		id:      terminalID,
		manager: manager,
	}

	c.mu.Lock()
	if existing, ok := c.sessions[terminalID]; ok {
		c.mu.Unlock()
		manager.Close()
		return existing, nil
	}
	c.sessions[terminalID] = session
	c.mu.Unlock()

	go c.handleSessionOutput(session.id, session.manager)
	c.sendTerminalInit(session.id, 80, 30)
	log.Printf("🆕 创建终端会话: %s", session.id)

	return session, nil
}



func (c *WebSocketClient) closeSession(id string) {
	terminalID := c.normalizeTerminalID(id)

	c.mu.Lock()
	session, ok := c.sessions[terminalID]
	if ok {
		delete(c.sessions, terminalID)
	}
	c.mu.Unlock()

	if !ok {
		return
	}

	if session.manager != nil {
		session.manager.Close()
	}

	log.Printf("🛑 终端会话已关闭: %s", terminalID)
}

func (c *WebSocketClient) restartSession(id string) {
	terminalID := c.normalizeTerminalID(id)

	c.closeSession(terminalID)
	if _, err := c.ensureSession(terminalID); err != nil {
		log.Printf("重启终端 %s 失败: %v", terminalID, err)
		return
	}

	log.Printf("✅ 终端进程已重启: %s", terminalID)
}

func (c *WebSocketClient) handleSessionOutput(terminalID string, manager *PTYManager) {
	for {
		select {
		case data := <-manager.Output():
			if len(data) == 0 {
				continue
			}
			c.sendTerminalOutput(terminalID, string(data))
		case <-manager.Closed():
			c.mu.Lock()
			if session, ok := c.sessions[terminalID]; ok && session.manager == manager {
				delete(c.sessions, terminalID)
			}
			c.mu.Unlock()
			log.Printf("👋 终端会话结束: %s", terminalID)
			return
		case <-c.stopChan:
			return
		}
	}
}

func (c *WebSocketClient) sendTerminalOutput(terminalID string, output string) {
	if output == "" {
		return
	}
	if err := c.send(NewOutput(output)); err != nil {
		log.Printf("发送终端输出失败 [%s]: %v", terminalID, err)
	}
}

func (c *WebSocketClient) sendTerminalInit(terminalID string, cols, rows int) {
	if err := c.send(NewInit(cols, rows)); err != nil {
		log.Printf("发送终端初始化失败 [%s]: %v", terminalID, err)
	}
}

func (c *WebSocketClient) Start() error {
	log.Println("🚀 Terminal 启动中...")

	// 创建默认终端会话
	if _, err := c.ensureSession(defaultTerminalID); err != nil {
		return err
	}

	// 记录配置文件初始修改时间
	if c.configManager != nil {
		if stat, err := os.Stat(c.configManager.Path()); err == nil {
			c.lastConfigMod = stat.ModTime()
		}
	}

	// 启动配置文件监听
	if c.configManager != nil {
		go c.watchConfigFile()
	}

	// 启动 WebSocket 连接
	go c.connectLoop()

	return nil
}

func (c *WebSocketClient) connectLoop() {
	for {
		if err := c.connect(); err != nil {
			log.Printf("连接失败: %v，3秒后重试...", err)
			time.Sleep(3 * time.Second)
			continue
		}

		// 等待重连信号
		select {
		case <-c.reconnectCh:
			log.Println("❌ 与服务器断开连接，3秒后重连...")
			time.Sleep(3 * time.Second)
		case <-c.stopChan:
			return
		}
	}
}

func (c *WebSocketClient) connect() error {
	u, err := url.Parse(c.config.ServerURL)
	if err != nil {
		return err
	}

	u.Path = "/ws"
	q := u.Query()
	q.Set("session", c.sessionID)
	u.RawQuery = q.Encode()

	log.Printf("连接服务器: %s", u.String())

	ws, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.ws = ws
	c.mu.Unlock()

	log.Println("✅ 已连接到服务器")

	// 注册设备
	if err := c.send(NewSystem(SysCmdRegister, RegisterArgs{
		DeviceType: "desktop",
		SessionID:  c.sessionID,
	})); err != nil {
		ws.Close()
		return err
	}

	webURL := fmt.Sprintf("%s?session=%s", c.config.WebURL, c.sessionID)
	log.Println("🔗 访问链接:")
	log.Printf("   %s", webURL)
	log.Println("")
	log.Println("💡 在手机或其他设备的浏览器中打开上述链接即可使用")

	// 发送初始配置到前端
	if c.config != nil && c.config.Terminal != nil {
		c.SendConfigUpdate(c.config.Terminal)
	}

	// 启动消息接收
	go c.handleMessages()

	return nil
}

func (c *WebSocketClient) handleMessages() {
	defer func() {
		c.mu.Lock()
		if c.ws != nil {
			c.ws.Close()
			c.ws = nil
		}
		c.mu.Unlock()

		// 触发重连
		select {
		case c.reconnectCh <- struct{}{}:
		default:
		}
	}()

	for {
		c.mu.Lock()
		ws := c.ws
		c.mu.Unlock()

		if ws == nil {
			return
		}

		var msg Message
		if err := ws.ReadJSON(&msg); err != nil {
			log.Printf("读取消息失败: %v", err)
			return
		}

		c.handleMessage(&msg)
	}
}

func (c *WebSocketClient) handleMessage(msg *Message) {
	switch msg.Type {
	case TypeInput:
		if data, ok := msg.Data.(string); ok {
			if err := c.writeToSession(defaultTerminalID, []byte(data)); err != nil {
				log.Printf("写入终端失败: %v", err)
			}
		}

	case TypeSystem:
		c.handleSystemMessage(msg.Data)

	default:
		log.Printf("未知消息类型: %s", msg.Type)
	}
}

func (c *WebSocketClient) handleSystemMessage(data interface{}) {
	// 尝试解析为结构化数据
	jsonData, _ := json.Marshal(data)
	var sysData SystemData
	if err := json.Unmarshal(jsonData, &sysData); err != nil {
		// 可能是字符串格式
		if str, ok := data.(string); ok {
			sysData.Cmd = str
		} else {
			log.Printf("解析系统消息失败: %v", err)
			return
		}
	}

	switch sysData.Cmd {
	case SysCmdResize:
		// 调整大小
		if args, ok := sysData.Args.(map[string]interface{}); ok {
			cols := int(args["cols"].(float64))
			rows := int(args["rows"].(float64))

			sess, err := c.ensureSession(defaultTerminalID)
			if err != nil {
				log.Printf("确保终端会话失败: %v", err)
				return
			}

			if sess.manager != nil && cols > 0 && rows > 0 {
				if err := sess.manager.Resize(cols, rows); err != nil {
					log.Printf("调整终端尺寸失败: %v", err)
				} else {
					log.Printf("🔄 终端尺寸调整为: %dx%d", cols, rows)
				}
			}
		}

	case SysCmdClear:
		// 清屏
		if err := c.writeToSession(defaultTerminalID, []byte("\x1b[2J\x1b[H")); err != nil {
			log.Printf("清屏失败: %v", err)
		}

	case SysCmdRestart:
		c.restartSession(defaultTerminalID)

	case SysCmdPing:
		c.send(NewSystem(SysCmdPong, nil))

	case SysCmdConnected:
		// 设备连接通知
		if args, ok := sysData.Args.(map[string]interface{}); ok {
			if deviceType, ok := args["deviceType"].(string); ok && deviceType == "web" {
				log.Println("🌐 网页端已连接")
				// 网页端连接后，发送当前配置
				if c.config != nil && c.config.Terminal != nil {
					c.SendConfigUpdate(c.config.Terminal)
				}
			}
		}

	case "ctrl_c":
		if err := c.writeToSession(defaultTerminalID, []byte("\x03")); err != nil {
			log.Printf("发送 Ctrl+C 失败: %v", err)
		}

	default:
		c.handleSystemCommand(sysData.Cmd, defaultTerminalID)
	}
}

func (c *WebSocketClient) handleSystemCommand(command string, terminalID string) {
	targetID := c.normalizeTerminalID(terminalID)

	switch command {
	case "open_terminal":
		if _, err := c.ensureSession(targetID); err != nil {
			log.Printf("创建终端 %s 失败: %v", targetID, err)
		}
	case "close_terminal":
		c.closeSession(targetID)
	case "restart":
		log.Printf("🔄 重启终端进程: %s", targetID)
		c.restartSession(targetID)
	case "clear":
		if err := c.writeToSession(targetID, []byte("\x1b[2J\x1b[H")); err != nil {
			log.Printf("清屏失败 [%s]: %v", targetID, err)
		}
	case "ctrl_c":
		if err := c.writeToSession(targetID, []byte("\x03")); err != nil {
			log.Printf("发送 Ctrl+C 失败 [%s]: %v", targetID, err)
		}
	default:
		if !c.handleCommand(command, targetID) {
			log.Printf("未知系统命令: %s", command)
		}
	}
}

func (c *WebSocketClient) writeToSession(terminalID string, data []byte) error {
	session, err := c.ensureSession(terminalID)
	if err != nil {
		return err
	}
	if session.manager == nil {
		return fmt.Errorf("终端会话未准备好: %s", terminalID)
	}
	return session.manager.Write(data)
}

// handleCommand 处理内置命令
func (c *WebSocketClient) handleCommand(commandLine string, terminalID string) bool {
	// 解析命令行：termconfig font Consolas
	parts := strings.Fields(commandLine)
	if len(parts) == 0 {
		return false
	}

	cmdName := parts[0]
	args := parts[1:]

	switch cmdName {
	case "termconfig", "config":
		if c.configManager == nil {
			log.Printf("⚠️  配置管理器不可用，无法执行: %s", commandLine)
			return true
		}

		cmd := NewTermConfigCommand(c.configManager, c)
		success, output := cmd.Execute(args)

		if !success {
			log.Printf("❌ termconfig 命令执行失败: %s", strings.TrimSpace(output))
		} else {
			log.Printf("✅ termconfig 命令执行成功: %s", strings.TrimSpace(output))
		}

		// 将输出发送到前端
		c.sendTerminalOutput(terminalID, output+"\n")
		return true
	default:
		return false
	}
}

func (c *WebSocketClient) send(msg *Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.ws != nil {
		return c.ws.WriteJSON(msg)
	}

	return fmt.Errorf("websocket 未连接")
}

func (c *WebSocketClient) Stop() {
	close(c.stopChan)

	c.mu.Lock()
	ws := c.ws
	c.ws = nil
	sessions := c.sessions
	c.sessions = make(map[string]*terminalSession)
	c.mu.Unlock()

	if ws != nil {
		ws.Close()
	}

	for _, sess := range sessions {
		if sess != nil && sess.manager != nil {
			sess.manager.Close()
		}
	}
}

// SendConfigUpdate 发送配置更新到Web端
func (c *WebSocketClient) SendConfigUpdate(cfg *TerminalConfig) {
	if cfg == nil {
		log.Printf("警告: 配置为空，跳过发送")
		return
	}

	log.Printf("准备发送配置到Web端: FontFamily=%s, FontSize=%d, Theme=%s",
		cfg.FontFamily, cfg.FontSize, cfg.Theme)

	if err := c.send(NewConfig(cfg)); err != nil {
		log.Printf("发送配置更新失败: %v", err)
	} else {
		log.Printf("配置更新已发送到Web端")
	}
}

// NotifyConfigUpdated implements ConfigNotifier.
func (c *WebSocketClient) NotifyConfigUpdated(cfg *TerminalConfig) {
	c.SendConfigUpdate(cfg)
}

func (c *WebSocketClient) watchConfigFile() {
	if c.configManager == nil {
		return
	}

	configPath := strings.TrimSpace(c.configManager.Path())
	if configPath == "" {
		return
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			stat, err := os.Stat(configPath)
			if err != nil {
				continue
			}

			modTime := stat.ModTime()
			if !modTime.After(c.lastConfigMod) {
				continue
			}

			updatedConfig, err := c.configManager.Load()
			if err != nil {
				log.Printf("重新加载配置失败: %v", err)
				continue
			}

			c.mu.Lock()
			c.config = updatedConfig
			c.sessionID = updatedConfig.DeviceID
			c.lastConfigMod = modTime
			c.mu.Unlock()

			c.SendConfigUpdate(updatedConfig.Terminal)
		}
	}
}
