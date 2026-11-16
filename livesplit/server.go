package livesplit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// ClientConnection represents a connected LiveSplit One instance.
type ClientConnection struct {
	conn *websocket.Conn
	send chan []string
}

// RunMetadata contains run details reported by a LiveSplit One client.
type RunMetadata struct {
	GameName         string
	CategoryName     string
	CurrentSplitName string
}

type metadataMessage struct {
	GameName          *string `json:"gameName"`
	GameNameSnake     *string `json:"game_name"`
	CategoryName      *string `json:"categoryName"`
	CategoryNameSnake *string `json:"category_name"`
	CurrentSplitName  *string `json:"currentSplitName"`
	SplitNameSnake    *string `json:"current_split_name"`
	SplitName         *string `json:"splitName"`
	Type              string  `json:"type"`
	Command           string  `json:"command"`
}

// Server coordinates LiveSplit One client connections.
type Server struct {
	addr       string
	httpServer *http.Server
	clients    map[*ClientConnection]bool
	mu         sync.Mutex
	onStatus   func(connected bool, count int)
	metadata   RunMetadata
	onMetadata func(RunMetadata)
	serveErr   chan error
	started    bool
	stopped    bool
}

// NewServer creates a new LiveSplit One WebSocket server instance.
func NewServer(addr string, onStatus func(connected bool, count int)) *Server {
	addr = strings.TrimPrefix(addr, "ws://")
	return &Server{
		addr:     addr,
		clients:  make(map[*ClientConnection]bool),
		onStatus: onStatus,
		serveErr: make(chan error, 1),
	}
}

// Start begins listening on the configured address.
func (s *Server) Start() error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return errors.New("LiveSplit server already started")
	}
	if s.stopped {
		s.mu.Unlock()
		return errors.New("LiveSplit server already stopped")
	}

	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to listen on %s: %w", s.addr, err)
	}
	s.addr = listener.Addr().String()
	s.httpServer = &http.Server{Handler: http.HandlerFunc(s.ServeHTTP)}
	s.started = true
	httpServer := s.httpServer
	s.mu.Unlock()

	go func() {
		if err := httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.serveErr <- fmt.Errorf("LiveSplit HTTP server failed: %w", err)
		}
	}()

	return nil
}

// Errors reports errors that occur after the server starts serving requests.
func (s *Server) Errors() <-chan error {
	return s.serveErr
}

// URL returns the WebSocket URL for this server.
func (s *Server) URL() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return "ws://" + s.addr
}

// Addr returns the bound TCP address.
func (s *Server) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.addr
}

// Stop shuts down the server and disconnects all clients.
func (s *Server) Stop() error {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return nil
	}
	s.stopped = true
	httpServer := s.httpServer
	clients := make([]*ClientConnection, 0, len(s.clients))
	for client := range s.clients {
		delete(s.clients, client)
		close(client.send)
		clients = append(clients, client)
	}
	hadClients := len(clients) > 0
	s.mu.Unlock()

	for _, client := range clients {
		_ = client.conn.Close()
	}

	var err error
	if httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err = httpServer.Shutdown(ctx)
		cancel()
	}
	if hadClients && s.onStatus != nil {
		s.onStatus(false, 0)
	}
	return err
}

// ConnectedClients returns the count of currently connected LiveSplit One clients.
func (s *Server) ConnectedClients() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.clients)
}

// Metadata returns the most recently reported LiveSplit One run metadata.
func (s *Server) Metadata() RunMetadata {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.metadata
}

// SetMetadataCallback sets a callback invoked when run metadata changes.
func (s *Server) SetMetadataCallback(callback func(RunMetadata)) {
	s.mu.Lock()
	s.onMetadata = callback
	s.mu.Unlock()
}

// BroadcastAction dispatches an action to all connected clients and immediately clears it.
func normalizeAction(action string) string {
	action = strings.TrimSpace(action)
	switch strings.ToLower(action) {
	case "start":
		return "starttimer"
	case "split":
		return "split"
	default:
		return action
	}
}

func (s *Server) BroadcastAction(action string) {
	action = normalizeAction(action)
	if action == "" {
		return
	}

	var dropped []*ClientConnection
	s.mu.Lock()
	for client := range s.clients {
		select {
		case client.send <- []string{action, ""}:
		default:
			delete(s.clients, client)
			close(client.send)
			dropped = append(dropped, client)
		}
	}
	count := len(s.clients)
	s.mu.Unlock()

	for _, client := range dropped {
		_ = client.conn.Close()
	}
	if len(dropped) > 0 && s.onStatus != nil {
		s.onStatus(count > 0, count)
	}
}

// DispatchAction is an alias for BroadcastAction.
func (s *Server) DispatchAction(action string) {
	s.BroadcastAction(action)
}

// ServeHTTP upgrades a request and registers the resulting LiveSplit One client.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	stopped := s.stopped
	s.mu.Unlock()
	if stopped {
		http.Error(w, "LiveSplit server is stopped", http.StatusServiceUnavailable)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := &ClientConnection{
		conn: conn,
		send: make(chan []string, 32),
	}

	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		_ = conn.Close()
		return
	}
	s.clients[client] = true
	count := len(s.clients)
	s.mu.Unlock()

	if s.onStatus != nil {
		s.onStatus(true, count)
	}
	go client.writePump(s)
	go client.readPump(s)
}

func (c *ClientConnection) writePump(server *Server) {
	defer server.removeClient(c)

	for batch := range c.send {
		for _, message := range batch {
			if err := c.conn.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
				return
			}
		}
	}
}

func (c *ClientConnection) readPump(server *Server) {
	defer server.removeClient(c)

	for {
		messageType, message, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		server.handleMessage(c, messageType, message)
	}
}

func (s *Server) handleMessage(client *ClientConnection, messageType int, message []byte) {
	if messageType != websocket.TextMessage {
		return
	}

	var payload metadataMessage
	if json.Unmarshal(message, &payload) == nil {
		s.updateMetadata(payload)
		if strings.EqualFold(payload.Type, "query") {
			s.replyToQuery(client, payload.Command)
		}
		return
	}

	s.replyToQuery(client, string(message))
}

func (s *Server) updateMetadata(payload metadataMessage) {
	s.mu.Lock()
	previous := s.metadata
	setMetadataField(&s.metadata.GameName, payload.GameName, payload.GameNameSnake)
	setMetadataField(&s.metadata.CategoryName, payload.CategoryName, payload.CategoryNameSnake)
	setMetadataField(&s.metadata.CurrentSplitName, payload.CurrentSplitName, payload.SplitNameSnake, payload.SplitName)
	metadata := s.metadata
	callback := s.onMetadata
	changed := metadata != previous
	s.mu.Unlock()

	if changed && callback != nil {
		callback(metadata)
	}
}

func setMetadataField(destination *string, values ...*string) {
	for _, value := range values {
		if value != nil {
			*destination = *value
			return
		}
	}
}

func (s *Server) replyToQuery(client *ClientConnection, command string) {
	var value string
	switch strings.ToLower(strings.TrimSpace(command)) {
	case "getgamename":
		value = s.Metadata().GameName
	case "getcategoryname":
		value = s.Metadata().CategoryName
	case "getcurrentsplitname":
		value = s.Metadata().CurrentSplitName
	default:
		return
	}

	s.mu.Lock()
	if !s.clients[client] {
		s.mu.Unlock()
		return
	}
	select {
	case client.send <- []string{value}:
		s.mu.Unlock()
	default:
		delete(s.clients, client)
		close(client.send)
		count := len(s.clients)
		s.mu.Unlock()
		_ = client.conn.Close()
		if s.onStatus != nil {
			s.onStatus(count > 0, count)
		}
	}
}

func (s *Server) removeClient(client *ClientConnection) {
	s.mu.Lock()
	if !s.clients[client] {
		s.mu.Unlock()
		return
	}
	delete(s.clients, client)
	close(client.send)
	count := len(s.clients)
	s.mu.Unlock()

	_ = client.conn.Close()
	if s.onStatus != nil {
		s.onStatus(count > 0, count)
	}
}
