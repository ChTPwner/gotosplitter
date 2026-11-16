package livesplit

import (
	"fmt"
	"strings"

	"github.com/gorilla/websocket"
)

type Client struct {
	conn *websocket.Conn
	url  string
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

	action = normalizeAction(action)
	if action == "" {
		return nil
	}

	if strings.TrimSpace(action) == "" {
		return nil
	}

	return c.conn.WriteMessage(websocket.TextMessage, []byte(action))
}
