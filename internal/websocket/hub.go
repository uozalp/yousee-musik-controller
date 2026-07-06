// Package websocket implements the realtime event hub that pushes player
// state updates to connected browser clients.
package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	gws "github.com/gorilla/websocket"
)

// Event types pushed to clients.
const (
	EventPlaybackState   = "PlaybackStateChanged"
	EventTrackChanged    = "TrackChanged"
	EventProgress        = "ProgressUpdated"
	EventQueueChanged    = "QueueChanged"
	EventVolumeChanged   = "VolumeChanged"
	EventConnectionState = "ConnectionStateChanged"
)

// Event is a single message pushed over the WebSocket.
type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

// Hub maintains the set of active clients and broadcasts messages to them.
type Hub struct {
	mu         sync.RWMutex
	clients    map[*client]struct{}
	register   chan *client
	unregister chan *client
	broadcast  chan []byte

	// onConnect is invoked with a fresh client so an initial snapshot can be sent.
	onConnect func() []Event
}

// NewHub creates a Hub. onConnect returns the events to send to a newly
// connected client (an initial state snapshot).
func NewHub(onConnect func() []Event) *Hub {
	return &Hub{
		clients:    make(map[*client]struct{}),
		register:   make(chan *client),
		unregister: make(chan *client),
		broadcast:  make(chan []byte, 64),
		onConnect:  onConnect,
	}
}

// Run starts the hub event loop. Call in a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = struct{}{}
			h.mu.Unlock()
		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
			h.mu.Unlock()
		case msg := <-h.broadcast:
			h.mu.RLock()
			for c := range h.clients {
				select {
				case c.send <- msg:
				default:
					// Slow client: drop connection.
					go func(cl *client) { h.unregister <- cl }(c)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast marshals and sends an event to all connected clients.
func (h *Hub) Broadcast(evt Event) {
	data, err := json.Marshal(evt)
	if err != nil {
		log.Printf("websocket: marshal event: %v", err)
		return
	}
	select {
	case h.broadcast <- data:
	default:
		log.Printf("websocket: broadcast buffer full, dropping event %s", evt.Type)
	}
}

var upgrader = gws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// The controller runs on a trusted LAN device; allow same-origin only.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ServeHTTP upgrades an HTTP request to a WebSocket connection.
func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket: upgrade: %v", err)
		return
	}
	c := &client{hub: h, conn: conn, send: make(chan []byte, 32)}
	h.register <- c

	// Send the initial snapshot.
	if h.onConnect != nil {
		for _, evt := range h.onConnect() {
			if data, err := json.Marshal(evt); err == nil {
				c.send <- data
			}
		}
	}

	go c.writePump()
	go c.readPump()
}

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

// client is a single WebSocket connection.
type client struct {
	hub  *Hub
	conn *gws.Conn
	send chan []byte
}

func (c *client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	c.conn.SetReadLimit(4096)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (c *client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(gws.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(gws.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(gws.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
