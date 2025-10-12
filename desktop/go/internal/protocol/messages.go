package protocol

import "encoding/json"

type MessageType string

const (
	MessageTypeConnectionPing    MessageType = "connection.ping"
	MessageTypeConnectionPong    MessageType = "connection.pong"
	MessageTypeConnectionDevices MessageType = "connection.devices"
	MessageTypeDataInput         MessageType = "data.input"
	MessageTypeDataOutput        MessageType = "data.output"
	MessageTypeSystemInit        MessageType = "system.init"
	MessageTypeSystemResize      MessageType = "system.resize"
	MessageTypeSystemCommand     MessageType = "system.command"
	MessageTypeSystemConfig      MessageType = "system.config"
)

type Message struct {
	Type MessageType     `json:"type"`
	To   string          `json:"to"`
	Data json.RawMessage `json:"data"`
}

type ConnectionDevicesData struct {
	DeviceType string            `json:"deviceType,omitempty"`
	Status     string            `json:"status,omitempty"`
	Devices    map[string]string `json:"devices,omitempty"`
	TerminalID string            `json:"terminalId,omitempty"`
}

type DataInputData struct {
	TerminalID string `json:"terminalId,omitempty"`
	Input      string `json:"input"`
}

type DataOutputData struct {
	TerminalID string `json:"terminalId,omitempty"`
	Output     string `json:"output"`
}

type SystemInitData struct {
	TerminalID string `json:"terminalId,omitempty"`
	Cols       int    `json:"cols,omitempty"`
	Rows       int    `json:"rows,omitempty"`
}

type SystemResizeData struct {
	TerminalID string `json:"terminalId,omitempty"`
	Cols       int    `json:"cols"`
	Rows       int    `json:"rows"`
}

type SystemCommandData struct {
	TerminalID string `json:"terminalId,omitempty"`
	Command    string `json:"command"`
}

type SystemConfigData struct {
	TerminalID  string            `json:"terminalId,omitempty"`
	FontFamily  string            `json:"fontFamily,omitempty"`
	FontSize    int               `json:"fontSize,omitempty"`
	Theme       string            `json:"theme,omitempty"`
	ThemeColors map[string]string `json:"themeColors,omitempty"`
	Opacity     float64           `json:"opacity,omitempty"`
	CursorBlink bool              `json:"cursorBlink,omitempty"`
	LineHeight  float64           `json:"lineHeight,omitempty"`
}

func NewMessage(msgType MessageType, to string, data interface{}) (*Message, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	return &Message{
		Type: msgType,
		To:   to,
		Data: dataBytes,
	}, nil
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
