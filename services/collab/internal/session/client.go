package session

import (
	"sync"

	"github.com/gorilla/websocket"

	"collab/internal/models"
)

type Client struct {
	Conn *websocket.Conn
	mu   sync.Mutex
	hook func(models.WSFrame)
}

func NewClient(conn *websocket.Conn) *Client { return &Client{Conn: conn} }

// SetSendHook replaces the default WebSocket sender (used in tests).
func (c *Client) SetSendHook(fn func(models.WSFrame)) {
	c.mu.Lock()
	c.hook = fn
	c.mu.Unlock()
}

func (c *Client) Send(frame models.WSFrame) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.hook != nil {
		c.hook(frame)
		return
	}
	if c.Conn == nil {
		return
	}
	_ = c.Conn.WriteJSON(frame)
}
