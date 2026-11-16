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
	conn             *websocket.Conn
	mutex            sync.RWMutex
	port             string
	targetURL        string
	messageQueue     []string
	messageMutex     sync.RWMutex
	stopChan         chan struct{}
	connected        bool
	currentSplitName string
	timerState       string
	currentIndex     int
	lastSaveAttempt  *bool
}

// NewMockLiveSplit creates a new mock LiveSplit client
func NewMockLiveSplit(port string) *MockLiveSplit {
	return &MockLiveSplit{
		port:         port,
		messageQueue: make([]string, 0),
		stopChan:     make(chan struct{}),
		timerState:   "NotRunning",
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

		messageType, message, err := conn.ReadMessage()
		if err != nil {
			m.mutex.Lock()
			m.connected = false
			m.mutex.Unlock()
			return
		}
		if messageType != websocket.TextMessage {
			_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":{"code":"InvalidCommand"}}`))
			continue
		}
		if !m.handleCommand(conn, message) {
			return
		}
	}
}

func (m *MockLiveSplit) handleCommand(conn *websocket.Conn, message []byte) bool {
	var request struct {
		Command     string `json:"command"`
		Index       *int   `json:"index"`
		SaveAttempt *bool  `json:"saveAttempt"`
	}
	if err := json.Unmarshal(message, &request); err != nil || request.Command == "" {
		return writeProtocolMessage(conn, `{"error":{"code":"InvalidCommand"}}`)
	}

	var response any = nil
	var event string
	recordAction := false
	m.mutex.Lock()
	switch request.Command {
	case "start":
		m.timerState = "Running"
		m.currentIndex = 0
		event = "Started"
		recordAction = true
	case "split":
		if m.timerState == "Running" || m.timerState == "Paused" {
			m.currentIndex++
		}
		m.currentSplitName = ""
		event = "Splitted"
		recordAction = true
	case "reset":
		m.timerState = "NotRunning"
		m.currentIndex = 0
		if request.SaveAttempt != nil {
			saveAttempt := *request.SaveAttempt
			m.lastSaveAttempt = &saveAttempt
		} else {
			m.lastSaveAttempt = nil
		}
		event = "Reset"
		recordAction = true
	case "pause":
		m.timerState = "Paused"
		event = "Paused"
		recordAction = true
	case "resume":
		m.timerState = "Running"
		event = "Resumed"
		recordAction = true
	case "undoSplit":
		if m.currentIndex > 0 {
			m.currentIndex--
		}
		event = "UndoSplit"
		recordAction = true
	case "skipSplit":
		m.currentIndex++
		event = "Skipped"
		recordAction = true
	case "getCurrentState":
		state := map[string]any{"state": m.timerState}
		if m.timerState == "Running" || m.timerState == "Paused" {
			state["index"] = m.currentIndex
		}
		response = state
	case "getSegmentName":
		response = m.currentSplitName
	default:
		m.mutex.Unlock()
		return writeProtocolMessage(conn, `{"error":{"code":"InvalidCommand"}}`)
	}
	m.mutex.Unlock()

	if recordAction {
		m.messageMutex.Lock()
		m.messageQueue = append(m.messageQueue, request.Command)
		m.messageMutex.Unlock()
	}
	if event != "" {
		if !writeProtocolJSON(conn, map[string]string{"event": event}) {
			return false
		}
	}
	return writeProtocolJSON(conn, map[string]any{"success": response})
}

// SetCurrentSplitName configures the name returned for split-name requests.
func (m *MockLiveSplit) SetCurrentSplitName(name string) {
	m.mutex.Lock()
	m.currentSplitName = name
	m.mutex.Unlock()
}

// LastSaveAttempt returns the choice sent with the most recent reset command.
func (m *MockLiveSplit) LastSaveAttempt() *bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	if m.lastSaveAttempt == nil {
		return nil
	}
	saveAttempt := *m.lastSaveAttempt
	return &saveAttempt
}

func writeProtocolMessage(conn *websocket.Conn, message string) bool {
	return conn.WriteMessage(websocket.TextMessage, []byte(message)) == nil
}

func writeProtocolJSON(conn *websocket.Conn, message any) bool {
	data, err := json.Marshal(message)
	if err != nil {
		return false
	}
	return conn.WriteMessage(websocket.TextMessage, data) == nil
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
