package session

import (
	"sync"

	"github.com/gorilla/websocket"

	"collab/internal/models"
)

type Client struct {
	Conn *websocket.Conn
	mu   sync.Mutex
}

func NewClient(conn *websocket.Conn) *Client { return &Client{Conn: conn} }

func (c *Client) Send(frame models.WSFrame) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.Conn.WriteJSON(frame)
}
