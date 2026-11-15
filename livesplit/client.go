package livesplit

import (
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
    // Return true if connected to websocket
    return c.conn != nil
}

func (c *Client) Send(action string) error {
    // Send action like "Start", "Split", etc.
    return nil
}
