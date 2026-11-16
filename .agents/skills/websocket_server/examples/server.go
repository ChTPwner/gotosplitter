package examples

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow LiveSplit One web client
	},
}

// Client represents a connected LiveSplit One instance.
type Client struct {
	server *Server
	conn   *websocket.Conn
	send   chan []byte
}

// Server coordinates LiveSplit One client connections.
type Server struct {
	addr       string
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte
	mu         sync.RWMutex
	onStatus   func(connected bool, count int)
}

func NewServer(addr string, onStatus func(connected bool, count int)) *Server {
	return &Server{
		addr:       addr,
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, 32),
		onStatus:   onStatus,
	}
}

// Start runs the server loop and HTTP listener.
func (s *Server) Start(ctx context.Context) error {
	httpServer := &http.Server{
		Addr:    s.addr,
		Handler: s,
	}

	go s.run(ctx)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	log.Printf("[LiveSplit WS] Listening on %s", s.addr)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			for c := range s.clients {
				close(c.send)
				delete(s.clients, c)
			}
			s.mu.Unlock()
			return

		case client := <-s.register:
			s.mu.Lock()
			s.clients[client] = true
			count := len(s.clients)
			s.mu.Unlock()
			if s.onStatus != nil {
				s.onStatus(true, count)
			}

		case client := <-s.unregister:
			s.mu.Lock()
			if _, ok := s.clients[client]; ok {
				delete(s.clients, client)
				close(client.send)
			}
			count := len(s.clients)
			s.mu.Unlock()
			if s.onStatus != nil {
				s.onStatus(count > 0, count)
			}

		case message := <-s.broadcast:
			s.mu.Lock()
			for client := range s.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(s.clients, client)
				}
			}
			s.mu.Unlock()
		}
	}
}

// DispatchAction sends one JSON command and does not send an empty clearing frame.
func (s *Server) DispatchAction(action string) error {
	command := strings.ToLower(strings.TrimSpace(action))
	switch command {
	case "start", "starttimer":
		command = "start"
	case "split", "pause", "resume":
	case "splitorstart":
		command = "splitOrStart"
	case "undo":
		command = "undoSplit"
	case "skip":
		command = "skipSplit"
	case "reset":
		return errors.New("reset requires an explicit save-attempt decision")
	default:
		return errors.New("unsupported LiveSplit One action")
	}

	return s.dispatchCommand(command, nil)
}

func (s *Server) ResetAttempt(saveAttempt bool) error {
	return s.dispatchCommand("reset", &saveAttempt)
}

func (s *Server) dispatchCommand(command string, saveAttempt *bool) error {
	message, err := json.Marshal(struct {
		Command     string `json:"command"`
		SaveAttempt *bool  `json:"saveAttempt,omitempty"`
	}{Command: command, SaveAttempt: saveAttempt})
	if err != nil {
		return err
	}
	select {
	case s.broadcast <- message:
		return nil
	default:
		return errors.New("LiveSplit command queue is full")
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[LiveSplit WS] Upgrade error: %v", err)
		return
	}
	client := &Client{server: s, conn: conn, send: make(chan []byte, 16)}
	s.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) writePump() {
	defer func() {
		_ = c.conn.Close()
	}()
	for message := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
			return
		}
	}
}

func (c *Client) readPump() {
	defer func() {
		c.server.unregister <- c
		_ = c.conn.Close()
	}()
	for {
		messageType, message, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage {
			log.Printf("[LiveSplit WS] Ignoring non-text protocol response")
			continue
		}
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(message, &envelope); err != nil {
			log.Printf("[LiveSplit WS] Invalid protocol response: %v", err)
			continue
		}
		switch {
		case envelope["event"] != nil:
			log.Printf("[LiveSplit WS] Timer event: %s", envelope["event"])
		case envelope["error"] != nil:
			log.Printf("[LiveSplit WS] Command error: %s", envelope["error"])
		case envelope["success"] != nil:
			log.Printf("[LiveSplit WS] Command succeeded: %s", envelope["success"])
		default:
			log.Printf("[LiveSplit WS] Unrecognized protocol response: %s", message)
		}
	}
}
