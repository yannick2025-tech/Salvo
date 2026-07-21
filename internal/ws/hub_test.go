package ws

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

func newTestHub(t *testing.T) *Hub {
	t.Helper()
	hub := NewHub()
	go hub.Run()
	t.Cleanup(func() {
		// Hub has no Stop method; it runs until the process ends.
		// For tests we rely on GC / process teardown.
	})
	return hub
}

func dialWS(t *testing.T, handler http.HandlerFunc) *websocket.Conn {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestHub_SubscribeUnsubscribe(t *testing.T) {
	hub := newTestHub(t)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r)
	})

	conn := dialWS(t, handler)

	// Give the hub time to register the client.
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, hub.ClientCount())

	// Subscribe to run_id.
	subMsg, _ := json.Marshal(inboundMsg{Type: "subscribe", RunID: "run-1"})
	assert.NoError(t, conn.WriteMessage(websocket.TextMessage, subMsg))
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, hub.SubscriberCount("run-1"))

	// Broadcast a message to run-1; client should receive it.
	hub.BroadcastToRun("run-1", Message{Type: "span_update", RunID: "run-1", NodeID: "node-1", Status: "ok"})
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	assert.NoError(t, err)
	var received Message
	assert.NoError(t, json.Unmarshal(msg, &received))
	assert.Equal(t, "span_update", received.Type)
	assert.Equal(t, "run-1", received.RunID)
	assert.Equal(t, "node-1", received.NodeID)

	// Unsubscribe.
	unsubMsg, _ := json.Marshal(inboundMsg{Type: "unsubscribe", RunID: "run-1"})
	assert.NoError(t, conn.WriteMessage(websocket.TextMessage, unsubMsg))
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 0, hub.SubscriberCount("run-1"))
}

func TestHub_BroadcastToRun(t *testing.T) {
	hub := newTestHub(t)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r)
	})

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	dial := func() *websocket.Conn {
		t.Helper()
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		assert.NoError(t, err)
		t.Cleanup(func() { conn.Close() })
		return conn
	}

	conn1 := dial()
	conn2 := dial()
	time.Sleep(50 * time.Millisecond)

	// conn1 subscribes to run-a, conn2 subscribes to run-b.
	subA, _ := json.Marshal(inboundMsg{Type: "subscribe", RunID: "run-a"})
	subB, _ := json.Marshal(inboundMsg{Type: "subscribe", RunID: "run-b"})
	assert.NoError(t, conn1.WriteMessage(websocket.TextMessage, subA))
	assert.NoError(t, conn2.WriteMessage(websocket.TextMessage, subB))
	time.Sleep(50 * time.Millisecond)

	// Broadcast to run-a: only conn1 should receive.
	hub.BroadcastToRun("run-a", Message{Type: "span_update", RunID: "run-a", Status: "running"})
	conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn1.ReadMessage()
	assert.NoError(t, err)
	var m1 Message
	assert.NoError(t, json.Unmarshal(msg, &m1))
	assert.Equal(t, "run-a", m1.RunID)

	// Broadcast to run-b: only conn2 should receive.
	hub.BroadcastToRun("run-b", Message{Type: "span_update", RunID: "run-b", Status: "ok"})
	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err = conn2.ReadMessage()
	assert.NoError(t, err)
	var m2 Message
	assert.NoError(t, json.Unmarshal(msg, &m2))
	assert.Equal(t, "run-b", m2.RunID)
}

func TestHub_MaxClients(t *testing.T) {
	hub := newTestHub(t)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r)
	})

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	var conns []*websocket.Conn
	for i := 0; i < maxClients; i++ {
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if assert.NoError(t, err) {
			conns = append(conns, conn)
		}
	}
	defer func() {
		for _, c := range conns {
			c.Close()
		}
	}()

	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, maxClients, hub.ClientCount())

	// The 101st client should be rejected (send channel closed).
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn101, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		conn101.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, _, err = conn101.ReadMessage()
		assert.Error(t, err, "101st client should be disconnected")
		conn101.Close()
	}
}

func TestHub_ClientDisconnect(t *testing.T) {
	hub := newTestHub(t)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ServeWs(hub, w, r)
	})

	conn := dialWS(t, handler)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, hub.ClientCount())

	// Subscribe before disconnecting.
	subMsg, _ := json.Marshal(inboundMsg{Type: "subscribe", RunID: "run-x"})
	assert.NoError(t, conn.WriteMessage(websocket.TextMessage, subMsg))
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, 1, hub.SubscriberCount("run-x"))

	// Close the connection (simulate disconnect).
	conn.Close()
	time.Sleep(100 * time.Millisecond)

	// Hub should have cleaned up.
	assert.Equal(t, 0, hub.ClientCount())
	assert.Equal(t, 0, hub.SubscriberCount("run-x"))
}
