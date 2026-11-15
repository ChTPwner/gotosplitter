package test

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// MockLiveSplit represents a mock LiveSplit One client for testing
type MockLiveSplit struct {
	conn         *websocket.Conn
	mutex        sync.RWMutex
	port         string
	targetURL    string
	messageQueue []string
	messageMutex sync.RWMutex
	stopChan     chan struct{}
	connected    bool
}

// NewMockLiveSplit creates a new mock LiveSplit client
func NewMockLiveSplit(port string) *MockLiveSplit {
	return &MockLiveSplit{
		port:         port,
		messageQueue: make([]string, 0),
		stopChan:     make(chan struct{}),
	}
}

// Connect dials GotoSplitter's LiveSplit server at the specified WebSocket URL
func (m *MockLiveSplit) Connect(wsURL string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !strings.HasPrefix(wsURL, "ws://") && !strings.HasPrefix(wsURL, "wss://") {
		wsURL = "ws://" + wsURL
	}
	m.targetURL = wsURL

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to dial LiveSplit server at %s: %w", wsURL, err)
	}
	m.conn = conn
	m.connected = true

	go m.readLoop(conn)
	return nil
}

// Start connects using the configured port (ws://localhost:<port>)
func (m *MockLiveSplit) Start() error {
	return m.Connect(fmt.Sprintf("ws://localhost:%s", m.port))
}

// Stop stops the mock LiveSplit client and closes the socket
func (m *MockLiveSplit) Stop() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	select {
	case <-m.stopChan:
		return
	default:
		close(m.stopChan)
	}

	m.connected = false
	if m.conn != nil {
		_ = m.conn.Close()
		m.conn = nil
	}
}

// IsConnected returns whether the client is currently connected
func (m *MockLiveSplit) IsConnected() bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.connected && m.conn != nil
}

func (m *MockLiveSplit) readLoop(conn *websocket.Conn) {
	for {
		select {
		case <-m.stopChan:
			return
		default:
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			m.mutex.Lock()
			m.connected = false
			m.mutex.Unlock()
			return
		}

		msgStr := string(message)
		m.messageMutex.Lock()
		m.messageQueue = append(m.messageQueue, msgStr)
		m.messageMutex.Unlock()
	}
}

// GetMessages returns all received messages
func (m *MockLiveSplit) GetMessages() []string {
	m.messageMutex.RLock()
	defer m.messageMutex.RUnlock()

	result := make([]string, len(m.messageQueue))
	copy(result, m.messageQueue)
	return result
}

// GetLastMessage returns the last received message
func (m *MockLiveSplit) GetLastMessage() string {
	m.messageMutex.RLock()
	defer m.messageMutex.RUnlock()

	if len(m.messageQueue) == 0 {
		return ""
	}
	return m.messageQueue[len(m.messageQueue)-1]
}

// ClearMessages clears the received message queue
func (m *MockLiveSplit) ClearMessages() {
	m.messageMutex.Lock()
	defer m.messageMutex.Unlock()
	m.messageQueue = m.messageQueue[:0]
}

// VerifyImmediateClearing verifies that every non-empty action is immediately followed by an empty frame
func (m *MockLiveSplit) VerifyImmediateClearing() error {
	m.messageMutex.RLock()
	defer m.messageMutex.RUnlock()

	for i, msg := range m.messageQueue {
		if msg != "" {
			if i+1 >= len(m.messageQueue) {
				return fmt.Errorf("action %q at index %d was not cleared (no subsequent message received)", msg, i)
			}
			nextMsg := m.messageQueue[i+1]
			if nextMsg != "" {
				return fmt.Errorf("action %q at index %d not immediately cleared; next message is %q", msg, i, nextMsg)
			}
		}
	}
	return nil
}

// WaitForAction blocks until the given action is recorded in the message queue, or times out
func (m *MockLiveSplit) WaitForAction(action string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		m.messageMutex.RLock()
		for _, msg := range m.messageQueue {
			if msg == action {
				m.messageMutex.RUnlock()
				return nil
			}
		}
		m.messageMutex.RUnlock()
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for action %q (messages received: %v)", action, m.GetMessages())
}

// SendGameInfo sends game information to GotoSplitter
func (m *MockLiveSplit) SendGameInfo(gameName, categoryName string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.conn == nil {
		return fmt.Errorf("not connected")
	}

	gameInfo := map[string]interface{}{
		"gameName":     gameName,
		"categoryName": categoryName,
	}

	data, err := json.Marshal(gameInfo)
	if err != nil {
		return err
	}

	return m.conn.WriteMessage(websocket.TextMessage, data)
}

// SendQuery requests a supported metadata field from GotoSplitter.
func (m *MockLiveSplit) SendQuery(command string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.conn == nil {
		return fmt.Errorf("not connected")
	}

	data, err := json.Marshal(map[string]string{
		"type":    "query",
		"command": command,
	})
	if err != nil {
		return err
	}
	return m.conn.WriteMessage(websocket.TextMessage, data)
}
