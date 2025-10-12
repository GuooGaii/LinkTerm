package main

import "encoding/json"

// Message 简化的消息结构 - 只有3种类型
type Message struct {
	Type string      `json:"type"` // "input", "output", "system"
	Data interface{} `json:"data"`
}

// 消息类型常量
const (
	TypeInput  = "input"  // 用户输入 -> PTY
	TypeOutput = "output" // PTY输出 -> 用户
	TypeSystem = "system" // 系统控制
)

// SystemData 系统消息数据
type SystemData struct {
	Cmd  string      `json:"cmd"`            // 命令类型
	Args interface{} `json:"args,omitempty"` // 命令参数
}

// 系统命令类型
const (
	SysCmdInit     = "init"     // 初始化
	SysCmdResize   = "resize"   // 调整大小
	SysCmdConfig   = "config"   // 配置更新
	SysCmdClear    = "clear"    // 清屏
	SysCmdRestart  = "restart"  // 重启
	SysCmdPing     = "ping"     // 心跳
	SysCmdPong     = "pong"     // 心跳响应
	SysCmdRegister = "register" // 注册设备
	SysCmdConnected = "connected" // 设备已连接
)

// ResizeArgs 调整大小参数
type ResizeArgs struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

// ConfigArgs 配置参数
type ConfigArgs struct {
	FontFamily  string            `json:"fontFamily,omitempty"`
	FontSize    int               `json:"fontSize,omitempty"`
	Theme       string            `json:"theme,omitempty"`
	ThemeColors map[string]string `json:"themeColors,omitempty"`
	Opacity     float64           `json:"opacity,omitempty"`
	CursorBlink bool              `json:"cursorBlink,omitempty"`
	LineHeight  float64           `json:"lineHeight,omitempty"`
}

// RegisterArgs 注册参数
type RegisterArgs struct {
	DeviceType string `json:"deviceType"` // "desktop" or "web"
	SessionID  string `json:"sessionId,omitempty"`
}

// InitArgs 初始化参数
type InitArgs struct {
	Cols int `json:"cols"`
	Rows int `json:"rows"`
}

// ConnectedArgs 连接通知参数
type ConnectedArgs struct {
	DeviceType string `json:"deviceType"` // "desktop" or "web"
}

// 辅助函数
func NewMessage(msgType string, data interface{}) (*Message, error) {
	return &Message{Type: msgType, Data: data}, nil
}

func (m *Message) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}

func ParseMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

// 快捷创建函数
func NewInput(data string) *Message {
	return &Message{Type: TypeInput, Data: data}
}

func NewOutput(data string) *Message {
	return &Message{Type: TypeOutput, Data: data}
}

func NewSystem(cmd string, args interface{}) *Message {
	return &Message{
		Type: TypeSystem,
		Data: SystemData{Cmd: cmd, Args: args},
	}
}

func NewResize(cols, rows int) *Message {
	return NewSystem(SysCmdResize, ResizeArgs{Cols: cols, Rows: rows})
}

func NewConfig(cfg *TerminalConfig) *Message {
	args := &ConfigArgs{
		FontFamily:  cfg.FontFamily,
		FontSize:    cfg.FontSize,
		Theme:       cfg.Theme,
		ThemeColors: ThemeColors(cfg.Theme),
		Opacity:     cfg.Opacity,
		CursorBlink: cfg.CursorBlink,
		LineHeight:  cfg.LineHeight,
	}
	return NewSystem(SysCmdConfig, args)
}

func NewInit(cols, rows int) *Message {
	return NewSystem(SysCmdInit, InitArgs{Cols: cols, Rows: rows})
}