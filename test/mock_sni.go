package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// MockSNI represents a mock QUsb2Snes / SNI server for integration testing
type MockSNI struct {
	upgrader    websocket.Upgrader
	connections map[*websocket.Conn]bool
	mutex       sync.RWMutex
	port        string
	listener    net.Listener
	httpServer  *http.Server
	stopChan    chan struct{}

	// Emulated memory
	wram          []byte          // 128KB WRAM buffer ($7E:0000 - $7F:FFFF)
	extraRam      map[uint32]byte // Sparse map for non-WRAM addresses
	devices       []string
	attachedDev   string
	clientName    string
	readRequests  []MemoryReadRequest
	resetRequests int
}

type MemoryReadRequest struct {
	Address uint32
	Size    uint32
}

// NewMockSNI creates a new mock SNI server
func NewMockSNI(port string) *MockSNI {
	return &MockSNI{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		connections: make(map[*websocket.Conn]bool),
		port:        port,
		stopChan:    make(chan struct{}),
		wram:        make([]byte, 128*1024), // 128KB WRAM
		extraRam:    make(map[uint32]byte),
		devices:     []string{"RetroArch", "SD2SNES COM3"},
	}
}

// Start starts the mock SNI server on its configured port (or ephemeral if "0" / empty)
func (m *MockSNI) Start() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	bindAddr := m.port
	if bindAddr == "" || bindAddr == "0" {
		bindAddr = "127.0.0.1:0"
	} else if !strings.Contains(bindAddr, ":") {
		bindAddr = "127.0.0.1:" + bindAddr
	}

	l, err := net.Listen("tcp", bindAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", bindAddr, err)
	}
	m.listener = l
	_, portStr, _ := net.SplitHostPort(l.Addr().String())
	m.port = portStr

	mux := http.NewServeMux()
	mux.HandleFunc("/", m.handleConnection)

	m.httpServer = &http.Server{
		Handler: mux,
	}

	go func() {
		_ = m.httpServer.Serve(l)
	}()

	return nil
}

// URL returns the WebSocket URL to connect to this mock SNI server
func (m *MockSNI) URL() string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return fmt.Sprintf("ws://127.0.0.1:%s", m.port)
}

// Port returns the active listening port
func (m *MockSNI) Port() string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.port
}

// Stop stops the mock SNI server and closes all active connections
func (m *MockSNI) Stop() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	select {
	case <-m.stopChan:
		return
	default:
		close(m.stopChan)
	}

	if m.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_ = m.httpServer.Shutdown(ctx)
	}

	for conn := range m.connections {
		_ = conn.Close()
		delete(m.connections, conn)
	}
}

// SetByte sets a single byte at the normalized memory address
func (m *MockSNI) SetByte(addr uint32, val byte) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.setByteLocked(addr, val)
}

// SetWord sets a 16-bit word (little-endian) at the normalized memory address
func (m *MockSNI) SetWord(addr uint32, val uint16) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.setByteLocked(addr, byte(val&0xFF))
	m.setByteLocked(addr+1, byte((val>>8)&0xFF))
}

// SetMemory writes a byte slice starting at the normalized memory address
func (m *MockSNI) SetMemory(addr uint32, data []byte) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for i, b := range data {
		m.setByteLocked(addr+uint32(i), b)
	}
}

// GetByte reads a single byte from the normalized memory address
func (m *MockSNI) GetByte(addr uint32) byte {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.getByteLocked(addr)
}

// GetWord reads a 16-bit word (little-endian) from the normalized memory address
func (m *MockSNI) GetWord(addr uint32) uint16 {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	b0 := uint16(m.getByteLocked(addr))
	b1 := uint16(m.getByteLocked(addr + 1))
	return b0 | (b1 << 8)
}

func (m *MockSNI) GetAddressRequests() []MemoryReadRequest {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return append([]MemoryReadRequest(nil), m.readRequests...)
}

func (m *MockSNI) ResetRequests() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.resetRequests
}

func (m *MockSNI) WaitForResetRequest(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if m.ResetRequests() > 0 {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for SNI reset request")
}

// ResetMemory clears all simulated memory back to zero
func (m *MockSNI) ResetMemory() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for i := range m.wram {
		m.wram[i] = 0
	}
	m.extraRam = make(map[uint32]byte)
}

// ReadMemory directly implements the engine.MemoryReader interface for in-process testing
func (m *MockSNI) ReadMemory(address uint32, size int) ([]byte, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	buf := make([]byte, size)
	for i := 0; i < size; i++ {
		buf[i] = m.getByteLocked(address + uint32(i))
	}
	return buf, nil
}

func (m *MockSNI) normalizeAddress(addr uint32) (isWRAM bool, offset uint32) {
	if addr >= 0xF50000 && addr <= 0xF6FFFF {
		return true, addr - 0xF50000
	}
	if addr >= 0x7E0000 && addr <= 0x7FFFFF {
		return true, addr - 0x7E0000
	}
	if addr < uint32(len(m.wram)) {
		return true, addr
	}
	return false, addr
}

func (m *MockSNI) setByteLocked(addr uint32, val byte) {
	isWRAM, offset := m.normalizeAddress(addr)
	if isWRAM && int(offset) < len(m.wram) {
		m.wram[offset] = val
	} else {
		m.extraRam[addr] = val
	}
}

func (m *MockSNI) getByteLocked(addr uint32) byte {
	isWRAM, offset := m.normalizeAddress(addr)
	if isWRAM && int(offset) < len(m.wram) {
		return m.wram[offset]
	}
	return m.extraRam[addr]
}

// handleConnection handles incoming WebSocket connections
func (m *MockSNI) handleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := m.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	m.mutex.Lock()
	m.connections[conn] = true
	m.mutex.Unlock()

	defer func() {
		m.mutex.Lock()
		delete(m.connections, conn)
		m.mutex.Unlock()
	}()

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			return
		}

		if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
			continue
		}

		var request map[string]interface{}
		if err := json.Unmarshal(message, &request); err != nil {
			continue
		}

		opcode, _ := request["Opcode"].(string)
		switch opcode {
		case "Name":
			m.handleName(request)
		case "DeviceList":
			resp := m.handleDeviceList()
			_ = conn.WriteMessage(websocket.TextMessage, resp)
		case "Attach":
			m.handleAttach(request)
		case "Reset":
			m.mutex.Lock()
			m.resetRequests++
			m.mutex.Unlock()
		case "GetAddress":
			rawBytes, jsonFallback := m.handleGetAddress(request)
			if rawBytes != nil {
				// Standard SNI returns binary WebSocket frame
				_ = conn.WriteMessage(websocket.BinaryMessage, rawBytes)
			} else if jsonFallback != nil {
				_ = conn.WriteMessage(websocket.TextMessage, jsonFallback)
			}
		}
	}
}

func (m *MockSNI) handleName(request map[string]interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if operands, ok := request["Operands"].([]interface{}); ok && len(operands) > 0 {
		if name, ok := operands[0].(string); ok {
			m.clientName = name
		}
	}
}

func (m *MockSNI) handleDeviceList() []byte {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	response := map[string]interface{}{
		"Results": m.devices,
	}
	data, _ := json.Marshal(response)
	return data
}

func (m *MockSNI) handleAttach(request map[string]interface{}) []byte {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	if operands, ok := request["Operands"].([]interface{}); ok && len(operands) > 0 {
		if dev, ok := operands[0].(string); ok {
			m.attachedDev = dev
		}
	}
	response := map[string]interface{}{
		"Status": "Success",
	}
	data, _ := json.Marshal(response)
	return data
}

func (m *MockSNI) handleGetAddress(request map[string]interface{}) ([]byte, []byte) {
	operands, ok := request["Operands"].([]interface{})
	if !ok || len(operands) < 2 {
		return nil, nil
	}

	addressStr, ok1 := operands[0].(string)
	sizeStr, ok2 := operands[1].(string)
	if !ok1 || !ok2 {
		return nil, nil
	}

	addr := parseFlexibleHex(addressStr)
	size := parseFlexibleHex(sizeStr)
	if size == 0 {
		size = 1
	}

	m.mutex.Lock()
	m.readRequests = append(m.readRequests, MemoryReadRequest{Address: addr, Size: size})
	buf := make([]byte, size)
	for i := uint32(0); i < size; i++ {
		buf[i] = m.getByteLocked(addr + i)
	}
	m.mutex.Unlock()

	return buf, nil
}

// parseFlexibleHex parses hex with or without 0x/0X prefix
func parseFlexibleHex(hexStr string) uint32 {
	hexStr = strings.TrimSpace(hexStr)
	hexStr = strings.TrimPrefix(hexStr, "0x")
	hexStr = strings.TrimPrefix(hexStr, "0X")
	if hexStr == "" {
		return 0
	}
	val, err := strconv.ParseUint(hexStr, 16, 32)
	if err != nil {
		// Fallback sscanf
		var res uint32
		fmt.Sscanf(hexStr, "%x", &res)
		return res
	}
	return uint32(val)
}
