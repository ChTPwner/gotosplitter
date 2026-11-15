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

// MockLiveSplitClient captures actions and checks immediate clearing.
type MockLiveSplitClient struct {
	messages []string
	mu       sync.Mutex
}

func (c *MockLiveSplitClient) Record(msg string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, msg)
}

func (c *MockLiveSplitClient) AssertImmediateClearing() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := 0; i < len(c.messages)-1; i++ {
		if c.messages[i] != "" && c.messages[i] != "query" {
			if c.messages[i+1] != "" {
				return fmt.Errorf("action %q at index %d was not immediately cleared", c.messages[i], i)
			}
		}
	}
	return nil
}
