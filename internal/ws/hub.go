package ws

import (
	"encoding/json"
	"fmt"
	"sync"
)

const maxClients = 100

// SpanStateFunc retrieves the current span states for a given run_id.
// It is called after a client subscribes to push the existing state,
// eliminating the race between subscription and broadcast.
// Returns a slice of Messages representing the current state of all
// spans in the run, or nil if the run is not found.
type SpanStateFunc func(runID string) []Message

// HubOption configures a Hub during creation.
type HubOption func(*Hub)

// WithSpanState sets the callback that provides current span states
// for a run_id. When set, Subscribe will push existing states to the
// newly subscribed client, ensuring no state is missed even if the
// client subscribes after execution has started.
func WithSpanState(fn SpanStateFunc) HubOption {
	return func(h *Hub) { h.spanState = fn }
}

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

// dedupKey returns a deduplication key for span_update messages.
// Two messages with the same key represent the same (run, chain, node)
// triple, and the later one supersedes the earlier.
// Returns "" for non-span_update messages (no dedup).
func (m Message) dedupKey() string {
	if m.Type != "span_update" {
		return ""
	}
	return fmt.Sprintf("%s/%s/%s", m.RunID, m.ChainID, m.NodeID)
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

	// spanState is an optional callback that returns the current span
	// states for a given run_id. Used to push initial state on subscribe.
	spanState SpanStateFunc
}

// NewHub creates a new Hub instance with optional configuration.
func NewHub(opts ...HubOption) *Hub {
	h := &Hub{
		clients:      make(map[*Client]struct{}),
		subscriptions: make(map[string]map[*Client]struct{}),
		register:     make(chan *Client, 16),
		unregister:   make(chan *Client, 16),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// SetSpanState sets the callback that provides current span states for a
// run_id. This is called after construction to break the circular
// dependency between Hub and Tracer (tracer needs hub.BroadcastToRun,
// hub needs tracer for initial state push on Subscribe).
func (h *Hub) SetSpanState(fn SpanStateFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.spanState = fn
}

// Run starts the Hub event loop. It should be invoked in a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if len(h.clients) >= maxClients {
				h.mu.Unlock()
				client.outbox.Close()
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
				client.outbox.Close()
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
// After subscribing, if a SpanStateFunc is configured, the current
// span states for the run are pushed to the client's outbox. This
// eliminates the race where the client subscribes after the runner
// has already started broadcasting events.
func (h *Hub) Subscribe(client *Client, runID string) {
	h.mu.Lock()

	if _, ok := h.subscriptions[runID]; !ok {
		h.subscriptions[runID] = make(map[*Client]struct{})
	}
	h.subscriptions[runID][client] = struct{}{}
	client.subscriptions[runID] = struct{}{}

	// Capture spanState while holding the lock (it's set once at creation).
	stateFn := h.spanState
	h.mu.Unlock()

	// Push current state to the client outside the lock.
	if stateFn != nil {
		h.pushCurrentState(client, runID, stateFn)
	}
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

// pushCurrentState sends the current span states for a run to a client.
// This is called immediately after Subscribe to ensure the client has
// the complete state, even if it connected after execution started.
func (h *Hub) pushCurrentState(client *Client, runID string, stateFn SpanStateFunc) {
	msgs := stateFn(runID)
	if len(msgs) == 0 {
		return
	}

	for _, msg := range msgs {
		data, err := json.Marshal(msg)
		if err != nil {
			continue
		}
		key := msg.dedupKey()
		client.outbox.Push(key, data)
	}
}

// BroadcastToRun sends a message to all clients subscribed to the given
// run_id. Messages are pushed into each client's deduplicating outbox,
// keyed by (run_id, chain_id, node_id) for span_update messages. This
// ensures that slow networks never cause message loss — if the same
// node updates multiple times, only the latest state is kept.
func (h *Hub) BroadcastToRun(runID string, msg Message) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	dedupKey := msg.dedupKey()

	h.mu.RLock()
	subs, ok := h.subscriptions[runID]
	if !ok {
		h.mu.RUnlock()
		return
	}
	// Copy clients to avoid holding the lock while pushing.
	clients := make([]*Client, 0, len(subs))
	for c := range subs {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		c.outbox.Push(dedupKey, data)
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
