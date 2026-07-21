package ws

import (
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// ServeWs handles the HTTP request that upgrades to a WebSocket connection,
// creates a Client, and registers it with the Hub.
func ServeWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := NewClient(hub, conn)
	hub.Register(client)

	go client.WritePump()
	go client.ReadPump()
}
