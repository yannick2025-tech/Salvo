package ws

import (
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 30 * time.Second
	maxMessageSize = 512
)

// inboundMsg is a client-sent message parsed from WebSocket.
type inboundMsg struct {
	Type  string `json:"type"`
	RunID string `json:"run_id"`
}

// Client represents a connected WebSocket client.
type Client struct {
	hub  *Hub
	conn *websocket.Conn

	// send buffers outbound messages as []byte (JSON-encoded).
	send chan []byte

	// subscriptions tracks the run_ids this client is subscribed to.
	subscriptions map[string]struct{}
}

// NewClient creates a new Client bound to the given hub and connection.
func NewClient(hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		hub:           hub,
		conn:          conn,
		send:          make(chan []byte, sendBufSize),
		subscriptions: make(map[string]struct{}),
	}
}

// ReadPump reads messages from the WebSocket connection and processes
// subscribe/unsubscribe commands. It should be invoked in a goroutine.
func (c *Client) ReadPump() {
	defer c.Close()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		var msg inboundMsg
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case "subscribe":
			if msg.RunID != "" {
				c.hub.Subscribe(c, msg.RunID)
			}
		case "unsubscribe":
			if msg.RunID != "" {
				c.hub.Unsubscribe(c, msg.RunID)
			}
		}
	}
}

// WritePump writes messages from the send channel to the WebSocket connection.
// It also sends periodic pings. It should be invoked in a goroutine.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Close cleans up the client's subscriptions and unregisters from the Hub.
func (c *Client) Close() {
	c.hub.Unregister(c)
}
