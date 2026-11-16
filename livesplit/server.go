package livesplit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const protocolTimeout = 2 * time.Second

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// SplitStatus describes the current timer state reported by LiveSplit One.
type SplitStatus struct {
	Running          bool
	CurrentSplitName string
}

// ClientConnection represents a connected LiveSplit One instance.
type ClientConnection struct {
	conn      *websocket.Conn
	done      chan struct{}
	closeOnce sync.Once

	requestMu sync.Mutex
	pendingMu sync.Mutex
	pending   chan protocolResult
}

// Server coordinates LiveSplit One client connections.
type Server struct {
	addr       string
	httpServer *http.Server
	clients    map[*ClientConnection]bool
	mu         sync.Mutex
	onStatus   func(connected bool, count int)
	onSplit    func(SplitStatus)
	onError    func(error)
	serveErr   chan error
	started    bool
	stopped    bool

	splitRequestPending bool
	splitRequestID      uint64
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
	s.splitRequestID++
	s.splitRequestPending = false
	httpServer := s.httpServer
	clients := make([]*ClientConnection, 0, len(s.clients))
	for client := range s.clients {
		delete(s.clients, client)
		clients = append(clients, client)
	}
	hadClients := len(clients) > 0
	s.mu.Unlock()

	for _, client := range clients {
		client.close()
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

// SetSplitCallback sets the callback invoked after a current-split query completes.
func (s *Server) SetSplitCallback(callback func(SplitStatus)) {
	s.mu.Lock()
	s.onSplit = callback
	s.mu.Unlock()
}

// SetErrorCallback sets the callback used to report asynchronous protocol errors.
func (s *Server) SetErrorCallback(callback func(error)) {
	s.mu.Lock()
	s.onError = callback
	s.mu.Unlock()
}

// RequestCurrentSplitName queries timer state and, when active, its current segment name.
func (s *Server) RequestCurrentSplitName() error {
	s.mu.Lock()
	if s.splitRequestPending {
		s.mu.Unlock()
		return nil
	}
	var client *ClientConnection
	for connected := range s.clients {
		client = connected
		break
	}
	if client == nil {
		s.mu.Unlock()
		return errors.New("no LiveSplit One client connected")
	}

	s.splitRequestPending = true
	s.splitRequestID++
	requestID := s.splitRequestID
	s.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*protocolTimeout)
		defer cancel()
		status, err := client.currentSplit(ctx)

		s.mu.Lock()
		if s.splitRequestID != requestID || s.stopped {
			s.mu.Unlock()
			return
		}
		s.splitRequestPending = false
		callback := s.onSplit
		onError := s.onError
		s.mu.Unlock()

		if err != nil {
			if onError != nil {
				onError(err)
			} else {
				log.Printf("LiveSplit One split query failed: %v", err)
			}
			return
		}
		if callback != nil {
			callback(status)
		}
	}()
	return nil
}

// DispatchAction sends one protocol command to each connected LiveSplit One client.
func (s *Server) DispatchAction(action string) error {
	command, err := actionCommand(action)
	if err != nil {
		return err
	}
	return s.dispatchCommand(command)
}

// BroadcastAction is an alias for DispatchAction.
func (s *Server) BroadcastAction(action string) error {
	return s.DispatchAction(action)
}

// ResetAttempt resets the timer with an explicit attempt-history decision.
func (s *Server) ResetAttempt(saveAttempt bool) error {
	command := protocolCommand{Command: "reset", SaveAttempt: &saveAttempt}
	return s.dispatchCommand(command)
}

func (s *Server) dispatchCommand(command protocolCommand) error {
	message, err := encodeCommand(command)
	if err != nil {
		return err
	}

	s.mu.Lock()
	clients := make([]*ClientConnection, 0, len(s.clients))
	for client := range s.clients {
		clients = append(clients, client)
	}
	s.mu.Unlock()
	if len(clients) == 0 {
		return errors.New("no LiveSplit One client connected")
	}

	errs := make(chan error, len(clients))
	var wg sync.WaitGroup
	for index, client := range clients {
		wg.Add(1)
		go func(index int, client *ClientConnection) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), protocolTimeout)
			defer cancel()
			if _, err := client.request(ctx, message); err != nil {
				errs <- fmt.Errorf("LiveSplit One client %d: %w", index+1, err)
			}
		}(index, client)
	}
	wg.Wait()
	close(errs)

	var failures []error
	for err := range errs {
		failures = append(failures, err)
	}
	return errors.Join(failures...)
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
		done: make(chan struct{}),
	}

	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		client.close()
		return
	}
	s.clients[client] = true
	count := len(s.clients)
	s.mu.Unlock()

	if s.onStatus != nil {
		s.onStatus(true, count)
	}
	go client.readPump(s)
}

func (c *ClientConnection) request(ctx context.Context, message []byte) (json.RawMessage, error) {
	c.requestMu.Lock()
	defer c.requestMu.Unlock()

	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("send LiveSplit One command: %w", err)
	}
	select {
	case <-c.done:
		return nil, errors.New("LiveSplit One client disconnected")
	default:
	}

	response := make(chan protocolResult, 1)
	c.pendingMu.Lock()
	c.pending = response
	c.pendingMu.Unlock()

	if err := c.conn.SetWriteDeadline(time.Now().Add(protocolTimeout)); err != nil {
		c.clearPending(response)
		c.close()
		return nil, fmt.Errorf("set LiveSplit One write deadline: %w", err)
	}
	if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
		c.clearPending(response)
		c.close()
		return nil, fmt.Errorf("send LiveSplit One command: %w", err)
	}
	if err := c.conn.SetWriteDeadline(time.Time{}); err != nil {
		c.clearPending(response)
		c.close()
		return nil, fmt.Errorf("clear LiveSplit One write deadline: %w", err)
	}

	timer := time.NewTimer(protocolTimeout)
	defer timer.Stop()
	select {
	case result := <-response:
		c.clearPending(response)
		return result.Success, result.Err
	case <-ctx.Done():
		c.clearPending(response)
		c.close()
		return nil, fmt.Errorf("wait for LiveSplit One response: %w", ctx.Err())
	case <-timer.C:
		c.clearPending(response)
		c.close()
		return nil, fmt.Errorf("wait for LiveSplit One response: timed out after %s", protocolTimeout)
	case <-c.done:
		c.clearPending(response)
		return nil, errors.New("LiveSplit One client disconnected before responding")
	}
}

func (c *ClientConnection) currentSplit(ctx context.Context) (SplitStatus, error) {
	message, err := encodeCommand(protocolCommand{Command: "getCurrentState"})
	if err != nil {
		return SplitStatus{}, err
	}
	response, err := c.request(ctx, message)
	if err != nil {
		return SplitStatus{}, err
	}

	var state struct {
		State string `json:"state"`
		Index *int   `json:"index"`
	}
	if err := json.Unmarshal(response, &state); err != nil {
		return SplitStatus{}, fmt.Errorf("decode LiveSplit One current state: %w", err)
	}
	switch state.State {
	case "Running", "Paused":
		if state.Index == nil {
			return SplitStatus{}, errors.New("LiveSplit One returned an active state without a split index")
		}
		message, err := encodeCommand(protocolCommand{
			Command: "getSegmentName",
			Index:   state.Index,
		})
		if err != nil {
			return SplitStatus{}, err
		}
		response, err = c.request(ctx, message)
		if err != nil {
			return SplitStatus{}, err
		}

		var name string
		if err := json.Unmarshal(response, &name); err != nil {
			return SplitStatus{}, fmt.Errorf("decode LiveSplit One segment name: %w", err)
		}
		return SplitStatus{Running: true, CurrentSplitName: name}, nil
	case "NotRunning", "Ended":
		return SplitStatus{}, nil
	default:
		return SplitStatus{}, fmt.Errorf("LiveSplit One returned unknown timer state %q", state.State)
	}
}

func (c *ClientConnection) readPump(server *Server) {
	defer server.removeClient(c)

	for {
		messageType, message, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage {
			server.reportProtocolError(fmt.Errorf("received unsupported LiveSplit One frame type %d", messageType))
			continue
		}
		result, err := decodeProtocolResult(message)
		if err != nil {
			server.reportProtocolError(err)
			continue
		}
		if result.Event {
			continue
		}
		if !c.deliver(result) {
			server.reportProtocolError(errors.New("received an unsolicited LiveSplit One response"))
		}
	}
}

func (c *ClientConnection) deliver(result protocolResult) bool {
	c.pendingMu.Lock()
	pending := c.pending
	c.pendingMu.Unlock()
	if pending == nil {
		return false
	}
	select {
	case pending <- result:
		return true
	case <-c.done:
		return false
	}
}

func (c *ClientConnection) clearPending(response chan protocolResult) {
	c.pendingMu.Lock()
	if c.pending == response {
		c.pending = nil
	}
	c.pendingMu.Unlock()
}

func (c *ClientConnection) close() {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}

func (s *Server) reportProtocolError(err error) {
	s.mu.Lock()
	callback := s.onError
	s.mu.Unlock()
	if callback != nil {
		callback(err)
		return
	}
	log.Printf("LiveSplit One protocol error: %v", err)
}

func (s *Server) removeClient(client *ClientConnection) {
	s.mu.Lock()
	if !s.clients[client] {
		s.mu.Unlock()
		client.close()
		return
	}
	delete(s.clients, client)
	count := len(s.clients)
	s.mu.Unlock()

	client.close()
	if s.onStatus != nil {
		s.onStatus(count > 0, count)
	}
}
