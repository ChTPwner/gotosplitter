package examples

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type SNIRequest struct {
	Opcode   string   `json:"Opcode"`
	Space    string   `json:"Space"`
	Operands []string `json:"Operands,omitempty"`
}

type DeviceListResponse struct {
	Results []string `json:"Results"`
}

type SNIClient struct {
	addr       string
	conn       *websocket.Conn
	mu         sync.Mutex
	attachedDev string
}

func NewSNIClient(addr string) *SNIClient {
	return &SNIClient{addr: addr}
}

// Connect opens socket, registers name, enumerates devices, and attaches.
func (c *SNIClient) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	conn, _, err := websocket.DefaultDialer.Dial(c.addr, nil)
	if err != nil {
		return fmt.Errorf("failed to dial SNI: %w", err)
	}
	c.conn = conn

	// 1. Name
	if err := c.sendRequest(SNIRequest{Opcode: "Name", Space: "SNES", Operands: []string{"GotoSplitter"}}); err != nil {
		return err
	}

	// 2. DeviceList
	if err := c.sendRequest(SNIRequest{Opcode: "DeviceList", Space: "SNES"}); err != nil {
		return err
	}
	_, msg, err := c.conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("read DeviceList failed: %w", err)
	}

	var resp DeviceListResponse
	if err := json.Unmarshal(msg, &resp); err != nil {
		return fmt.Errorf("parse DeviceList failed: %w", err)
	}
	if len(resp.Results) == 0 {
		return fmt.Errorf("no SNES devices discovered")
	}

	c.attachedDev = resp.Results[0]

	// 3. Attach
	if err := c.sendRequest(SNIRequest{Opcode: "Attach", Space: "SNES", Operands: []string{c.attachedDev}}); err != nil {
		return err
	}

	log.Printf("[SNI Client] Attached to device: %s", c.attachedDev)
	return nil
}

// ReadMemory fetches requested bytes via GetAddress.
func (c *SNIClient) ReadMemory(address uint32, size int) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	req := SNIRequest{
		Opcode:   "GetAddress",
		Space:    "SNES",
		Operands: []string{fmt.Sprintf("%X", address), fmt.Sprintf("%X", size)},
	}

	if err := c.sendRequest(req); err != nil {
		return nil, err
	}

	_ = c.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	msgType, data, err := c.conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("read memory response failed: %w", err)
	}

	if msgType == websocket.BinaryMessage {
		return data, nil
	}

	return data, nil
}

func (c *SNIClient) sendRequest(req SNIRequest) error {
	data, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return c.conn.WriteMessage(websocket.TextMessage, data)
}
