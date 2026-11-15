package sni

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Request represents a QUsb2Snes / SNI JSON command frame
type Request struct {
	Opcode   string   `json:"Opcode"`
	Space    string   `json:"Space"`
	Operands []string `json:"Operands,omitempty"`
}

// DeviceListResponse represents the response frame from DeviceList
type DeviceListResponse struct {
	Results []string `json:"Results"`
}

// SNI represents the client bridge to QUsb2Snes or SNI server
type SNI struct {
	addr        string
	conn        *websocket.Conn
	mu          sync.Mutex
	attachedDev string
	connected   bool
}

// NewClient creates a new SNI client instance
func NewClient(addr string) *SNI {
	// Ensure websocket scheme
	if !strings.HasPrefix(addr, "ws://") && !strings.HasPrefix(addr, "wss://") {
		addr = "ws://" + addr
	}
	return &SNI{addr: addr}
}

// Connect dials the SNI server, registers client name, enumerates devices, and attaches
func (s *SNI) Connect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, err := url.Parse(s.addr)
	if err != nil {
		return fmt.Errorf("invalid SNI address %q: %w", s.addr, err)
	}

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to dial SNI at %s: %w", s.addr, err)
	}
	s.conn = conn
	s.connected = true

	// 1. Name registration
	if err := s.sendRequestLocked(Request{Opcode: "Name", Space: "SNES", Operands: []string{"GotoSplitter"}}); err != nil {
		s.closeLocked()
		return fmt.Errorf("name registration failed: %w", err)
	}

	// 2. DeviceList
	if err := s.sendRequestLocked(Request{Opcode: "DeviceList", Space: "SNES"}); err != nil {
		s.closeLocked()
		return fmt.Errorf("devicelist request failed: %w", err)
	}

	_ = s.conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := s.conn.ReadMessage()
	if err != nil {
		s.closeLocked()
		return fmt.Errorf("reading devicelist failed: %w", err)
	}

	var resp DeviceListResponse
	if err := json.Unmarshal(msg, &resp); err != nil {
		s.closeLocked()
		return fmt.Errorf("unmarshal devicelist failed: %w", err)
	}

	if len(resp.Results) == 0 {
		s.closeLocked()
		return fmt.Errorf("no SNES devices discovered")
	}

	s.attachedDev = resp.Results[0]

	// 3. Attach
	if err := s.sendRequestLocked(Request{Opcode: "Attach", Space: "SNES", Operands: []string{s.attachedDev}}); err != nil {
		s.closeLocked()
		return fmt.Errorf("attach failed: %w", err)
	}

	return nil
}

// IsConnected returns whether the SNI connection is open
func (s *SNI) IsConnected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.connected && s.conn != nil
}

// AttachedDevice returns the name of the attached hardware/emulator
func (s *SNI) AttachedDevice() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.attachedDev
}

// ReadMemory fetches requested bytes via GetAddress opcode
func (s *SNI) ReadMemory(address uint32, length int) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.connected || s.conn == nil {
		return nil, fmt.Errorf("SNI client not connected")
	}

	req := Request{
		Opcode:   "GetAddress",
		Space:    "SNES",
		Operands: []string{fmt.Sprintf("%X", normalizeWRAMAddress(address)), fmt.Sprintf("%X", length)},
	}

	if err := s.sendRequestLocked(req); err != nil {
		return nil, err
	}

	_ = s.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	msgType, data, err := s.conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("read memory response failed: %w", err)
	}

	// If binary frame, return payload directly
	if msgType == websocket.BinaryMessage {
		return data, nil
	}

	// If text frame (some older mocks or bridges return JSON {"Data": [...]})
	var jsonResp struct {
		Data []byte `json:"Data"`
	}
	if err := json.Unmarshal(data, &jsonResp); err == nil && len(jsonResp.Data) > 0 {
		return jsonResp.Data, nil
	}

	return data, nil
}

func normalizeWRAMAddress(address uint32) uint32 {
	switch {
	case address >= 0xF50000 && address <= 0xF6FFFF:
		return address
	case address >= 0x7E0000 && address <= 0x7FFFFF:
		return 0xF50000 + address - 0x7E0000
	case address < 0x20000:
		return 0xF50000 + address
	default:
		return address
	}
}

// CheckEvents checks legacy event definitions
func (s *SNI) CheckEvents(events []EventDefinition) []DetectedEvent {
	var ret []DetectedEvent
	for _, e := range events {
		data, err := s.ReadMemory(e.Address, len(e.Value))
		if err == nil && matches(data, e.Value) {
			ret = append(ret, DetectedEvent{Action: e.Action})
		}
	}
	return ret
}

// Close closes the WebSocket connection
func (s *SNI) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closeLocked()
}

func (s *SNI) closeLocked() error {
	s.connected = false
	if s.conn != nil {
		err := s.conn.Close()
		s.conn = nil
		return err
	}
	return nil
}

func (s *SNI) sendRequestLocked(req Request) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return s.conn.WriteMessage(websocket.TextMessage, data)
}
