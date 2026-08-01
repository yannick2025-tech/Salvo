package ws

import (
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 30 * time.Second // increased from 10s for poor network conditions
	pongWait       = 90 * time.Second // increased from 60s to match writeWait headroom
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

	// outbox is a deduplicating send queue that replaces the old
	// buffered channel. Messages keyed by (run_id,chain_id,node_id)
	// are coalesced so slow networks never cause silent drops.
	outbox *SendQueue

	// subscriptions tracks the run_ids this client is subscribed to.
	subscriptions map[string]struct{}
}

// NewClient creates a new Client bound to the given hub and connection.
func NewClient(hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		hub:           hub,
		conn:          conn,
		outbox:        NewSendQueue(),
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

// WritePump writes messages from the outbox queue to the WebSocket
// connection. It drains all pending messages in a batch before waiting
// for the next signal, which minimizes frame overhead and ensures
// no message is left behind when the network is slow.
// It also sends periodic pings. It should be invoked in a goroutine.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		// Drain all pending messages before waiting.
		for {
			data, ok := c.outbox.TryPop()
			if !ok {
				break
			}
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		}

		// If the queue was closed, send close frame and exit.
		if c.outbox.IsClosed() {
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}

		// Wait for new messages or the next ping tick.
		select {
		case <-c.outbox.Signal():
			// New message(s) available — loop back to drain.
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
