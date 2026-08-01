package ws

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// SendQueue is a thread-safe, deduplicating message queue for WebSocket
// outbound messages. It solves the slow-network message-loss problem by:
//
//  1. Deduplicating: messages with the same key (run_id/chain_id/node_id)
//     are coalesced — only the latest data is kept, eliminating stale
//     intermediate states (e.g. "running" superseded by "ok").
//
//  2. Never dropping: unlike a fixed-size channel that silently discards
//     messages when full, the queue grows to accommodate all unique keys.
//     The practical upper bound is the number of distinct (run,chain,node)
//     tuples across all active subscriptions — typically O(nodes × chains).
//
// WritePump drains the queue via TryPop in a loop, ensuring all pending
// messages are flushed before waiting for the next signal.
type SendQueue struct {
	mu      sync.Mutex
	pending map[string][]byte // dedup_key → latest serialized message
	order   []string          // FIFO insertion order of dedup keys
	signal  chan struct{}     // buffered[1], notifies WritePump of pending data
	closed  bool
	seq     atomic.Int64      // monotonic counter for no-dedup keys
}

// NewSendQueue creates a new SendQueue.
func NewSendQueue() *SendQueue {
	return &SendQueue{
		pending: make(map[string][]byte),
		order:   make([]string, 0, 64),
		signal:  make(chan struct{}, 1),
	}
}

// Push adds or updates a message in the queue.
//
//   - If key is non-empty and a message with the same key already exists,
//     its data is replaced in-place (the FIFO position is preserved).
//     This is the dedup path used for span_update messages keyed by
//     (run_id, chain_id, node_id).
//
//   - If key is empty, a unique synthetic key is generated so the message
//     is never deduplicated. Used for one-shot messages like initial
//     state sync.
func (q *SendQueue) Push(key string, data []byte) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return
	}

	if key == "" {
		key = fmt.Sprintf("_seq_%d", q.seq.Add(1))
	}

	if _, exists := q.pending[key]; !exists {
		q.order = append(q.order, key)
	}
	q.pending[key] = data

	// Non-blocking signal: buffered[1] ensures at most one notification
	// is pending; WritePump will drain all available messages.
	select {
	case q.signal <- struct{}{}:
	default:
	}
}

// TryPop returns the next message in FIFO order, or (nil, false) if the
// queue is empty. The returned data is the latest version for that key.
func (q *SendQueue) TryPop() ([]byte, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.order) == 0 {
		return nil, false
	}

	key := q.order[0]
	q.order = q.order[1:]
	data, ok := q.pending[key]
	delete(q.pending, key)

	return data, ok
}

// Signal returns the notification channel that WritePump should select on.
func (q *SendQueue) Signal() <-chan struct{} {
	return q.signal
}

// IsClosed returns whether the queue has been closed.
func (q *SendQueue) IsClosed() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.closed
}

// Close marks the queue as closed and signals WritePump so it can exit.
func (q *SendQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.closed = true
	select {
	case q.signal <- struct{}{}:
	default:
	}
}

// Len returns the number of pending messages (for diagnostics/testing).
func (q *SendQueue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.order)
}
