package livesplit

import (
	"fmt"
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn    *websocket.Conn
	url     string
	writeMu sync.Mutex
}

func NewClient(url string) *Client {
	return &Client{url: url}
}

func (c *Client) IsConnected() bool {
	return c.conn != nil
}

func (c *Client) Connect() error {
	if c.url == "" {
		return fmt.Errorf("no websocket URL configured")
	}
	conn, _, err := websocket.DefaultDialer.Dial(c.url, nil)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

func (c *Client) Close() error {
	if c.conn == nil {
		return nil
	}
	if err := c.conn.Close(); err != nil {
		return err
	}
	c.conn = nil
	return nil
}

func (c *Client) Send(action string) error {
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	command, err := actionCommand(action)
	if err != nil {
		return err
	}
	message, err := encodeCommand(command)
	if err != nil {
		return err
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
		return fmt.Errorf("send LiveSplit One command: %w", err)
	}
	return nil
}

// ResetAttempt sends a timer reset with an explicit attempt-history decision.
func (c *Client) ResetAttempt(saveAttempt bool) error {
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	message, err := encodeCommand(protocolCommand{Command: "reset", SaveAttempt: &saveAttempt})
	if err != nil {
		return err
	}

	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
		return fmt.Errorf("send LiveSplit One reset command: %w", err)
	}
	return nil
}
