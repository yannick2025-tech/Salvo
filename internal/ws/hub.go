package ws

import (
	"encoding/json"
	"sync"
)

const (
	maxClients = 100
	sendBufSize = 256
)

// Message represents a WebSocket message broadcast to clients.
type Message struct {
	Type       string `json:"type"`
	RunID      string `json:"run_id"`
	ChainID    string `json:"chain_id,omitempty"`
	NodeID     string `json:"node_id,omitempty"`
	Status     string `json:"status,omitempty"`
	DurationNs int64  `json:"duration_ns,omitempty"`
	Error      string `json:"error,omitempty"`
	LoopIndex  int    `json:"loop_index,omitempty"`
}

// Hub manages connected WebSocket clients and their subscriptions by run_id.
type Hub struct {
	mu sync.RWMutex

	// clients tracks all connected clients.
	clients map[*Client]struct{}

	// subscriptions maps run_id to the set of clients subscribed to it.
	subscriptions map[string]map[*Client]struct{}

	// register channel receives clients to add.
	register chan *Client

	// unregister channel receives clients to remove.
	unregister chan *Client
}

// NewHub creates a new Hub instance.
func NewHub() *Hub {
	return &Hub{
		clients:      make(map[*Client]struct{}),
		subscriptions: make(map[string]map[*Client]struct{}),
		register:     make(chan *Client, 16),
		unregister:   make(chan *Client, 16),
	}
}

// Run starts the Hub event loop. It should be invoked in a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if len(h.clients) >= maxClients {
				h.mu.Unlock()
				close(client.send)
				continue
			}
			h.clients[client] = struct{}{}
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				for runID := range client.subscriptions {
					if subs, ok := h.subscriptions[runID]; ok {
						delete(subs, client)
						if len(subs) == 0 {
							delete(h.subscriptions, runID)
						}
					}
				}
				close(client.send)
			}
			h.mu.Unlock()
		}
	}
}

// Register enqueues a client for registration with the Hub.
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister enqueues a client for unregistration from the Hub.
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Subscribe adds a client to the subscription set for a given run_id.
func (h *Hub) Subscribe(client *Client, runID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.subscriptions[runID]; !ok {
		h.subscriptions[runID] = make(map[*Client]struct{})
	}
	h.subscriptions[runID][client] = struct{}{}
	client.subscriptions[runID] = struct{}{}
}

// Unsubscribe removes a client from the subscription set for a given run_id.
func (h *Hub) Unsubscribe(client *Client, runID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if subs, ok := h.subscriptions[runID]; ok {
		delete(subs, client)
		if len(subs) == 0 {
			delete(h.subscriptions, runID)
		}
	}
	delete(client.subscriptions, runID)
}

// BroadcastToRun sends a JSON-encoded message to all clients subscribed
// to the given run_id.
func (h *Hub) BroadcastToRun(runID string, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	h.mu.RLock()
	subs, ok := h.subscriptions[runID]
	if !ok {
		h.mu.RUnlock()
		return
	}
	// Copy clients to avoid holding the lock while sending.
	clients := make([]*Client, 0, len(subs))
	for c := range subs {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		select {
		case c.send <- data:
		default:
			// Client send buffer full; skip this message.
		}
	}
}

// ClientCount returns the number of connected clients (for testing).
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// SubscriberCount returns the number of clients subscribed to a run_id (for testing).
func (h *Hub) SubscriberCount(runID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subscriptions[runID])
}
