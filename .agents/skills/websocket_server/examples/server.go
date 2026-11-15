package examples

import (
	"context"
	"log"
	"net/http"
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
			s.mu.RLock()
			for client := range s.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(s.clients, client)
				}
			}
			s.mu.RUnlock()
		}
	}
}

// DispatchAction sends an action to LiveSplit One and immediately clears it.
func (s *Server) DispatchAction(action string) {
	if action == "" {
		return
	}
	// Step 1: Send the trigger action
	s.broadcast <- []byte(action)

	// Step 2: Mandatory immediate clearing to avoid repeated triggers
	s.broadcast <- []byte("")
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
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}
