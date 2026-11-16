package examples

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"

	"github.com/gorilla/websocket"
)

var mockUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// MockSNIServer emulates a QUsb2Snes / SNI backend.
type MockSNIServer struct {
	server  *httptest.Server
	ram     map[uint32]byte
	mu      sync.RWMutex
	devices []string
}

func NewMockSNIServer() *MockSNIServer {
	s := &MockSNIServer{
		ram:     make(map[uint32]byte),
		devices: []string{"MockSNES", "RetroArch"},
	}
	s.server = httptest.NewServer(http.HandlerFunc(s.handleWS))
	return s
}

func (s *MockSNIServer) URL() string {
	return "ws" + s.server.URL[4:]
}

func (s *MockSNIServer) Close() {
	s.server.Close()
}

func (s *MockSNIServer) SetByte(addr uint32, val byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ram[addr] = val
}

func (s *MockSNIServer) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := mockUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var req struct {
			Opcode   string   `json:"Opcode"`
			Operands []string `json:"Operands"`
		}
		if err := json.Unmarshal(msg, &req); err != nil {
			continue
		}

		switch req.Opcode {
		case "DeviceList":
			resp, _ := json.Marshal(map[string][]string{"Results": s.devices})
			_ = conn.WriteMessage(websocket.TextMessage, resp)
		case "Attach":
			// Acknowledge attach
		case "GetAddress":
			if len(req.Operands) >= 2 {
				var addr uint32
				var size int
				fmt.Sscanf(req.Operands[0], "%X", &addr)
				fmt.Sscanf(req.Operands[1], "%X", &size)

				buf := make([]byte, size)
				s.mu.RLock()
				for i := 0; i < size; i++ {
					buf[i] = s.ram[addr+uint32(i)]
				}
				s.mu.RUnlock()

				_ = conn.WriteMessage(websocket.BinaryMessage, buf)
			}
		}
	}
}

// MockLiveSplitClient handles the JSON command protocol used by LiveSplit One.
type MockLiveSplitClient struct {
	commands []string
	state    string
	index    int
	segment  string
	save     *bool
	mu       sync.Mutex
}

func (c *MockLiveSplitClient) HandleCommand(message []byte) (event, response []byte, err error) {
	var request struct {
		Command     string `json:"command"`
		Index       *int   `json:"index"`
		SaveAttempt *bool  `json:"saveAttempt"`
	}
	if err := json.Unmarshal(message, &request); err != nil || request.Command == "" {
		response, err = json.Marshal(map[string]any{"error": map[string]string{"code": "InvalidCommand"}})
		return nil, response, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == "" {
		c.state = "NotRunning"
	}
	var result any
	var timerEvent string
	switch request.Command {
	case "start":
		c.state = "Running"
		c.index = 0
		timerEvent = "Started"
		c.commands = append(c.commands, request.Command)
	case "split":
		c.index++
		timerEvent = "Splitted"
		c.commands = append(c.commands, request.Command)
	case "reset":
		if request.SaveAttempt == nil {
			response, err = json.Marshal(map[string]any{"error": map[string]string{"code": "InvalidCommand"}})
			return nil, response, err
		}
		save := *request.SaveAttempt
		c.save = &save
		c.state = "NotRunning"
		c.index = 0
		timerEvent = "Reset"
		c.commands = append(c.commands, request.Command)
	case "getCurrentState":
		result = map[string]any{"state": c.state}
		if c.state == "Running" || c.state == "Paused" {
			result.(map[string]any)["index"] = c.index
		}
	case "getSegmentName":
		result = c.segment
	default:
		response, err = json.Marshal(map[string]any{"error": map[string]string{"code": "InvalidCommand"}})
		return nil, response, err
	}

	if timerEvent != "" {
		event, err = json.Marshal(map[string]string{"event": timerEvent})
		if err != nil {
			return nil, nil, err
		}
	}
	response, err = json.Marshal(map[string]any{"success": result})
	return event, response, err
}

func (c *MockLiveSplitClient) Commands() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	commands := make([]string, len(c.commands))
	copy(commands, c.commands)
	return commands
}

func (c *MockLiveSplitClient) SetSegmentName(name string) {
	c.mu.Lock()
	c.segment = name
	c.mu.Unlock()
}

func (c *MockLiveSplitClient) LastSaveAttempt() *bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.save == nil {
		return nil
	}
	save := *c.save
	return &save
}
