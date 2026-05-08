package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/ognev-dev/goplease/app/ds"
)

// ─── Upgrader ─────────────────────────────────────────────────────────────────

var upgrader = websocket.Upgrader{
	ReadBufferSize:   1024,
	WriteBufferSize:  1024,
	HandshakeTimeout: 10 * time.Second,
	// In production replace with an origin whitelist check.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ─── Wire message types (client ↔ server) ────────────────────────────────────

type InMessage struct {
	Action Action          `json:"action"`
	Data   json.RawMessage `json:"data"`
}

type OutMessage struct {
	Action Action `json:"action"`
	Data   any    `json:"data"`
}

// ─── Client ───────────────────────────────────────────────────────────────────

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 4096
)

// Client represents one connected browser/game client.
type Client struct {
	ID       string
	PlayerID ds.ID
	RoomID   string

	hub  *Hub
	conn *websocket.Conn
	send chan []byte // buffered outbound queue
}

func newClient(hub *Hub, conn *websocket.Conn, playerID ds.ID) *Client {
	return &Client{
		ID:       uuid.NewString(),
		PlayerID: playerID,
		hub:      hub,
		conn:     conn,
		send:     make(chan []byte, 128),
	}
}

// Send enqueues a message for this client (non-blocking).
func (c *Client) Send(msg OutMessage) {
	b, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[ws] marshal error for client %s: %v", c.ID, err)
		return
	}
	select {
	case c.send <- b:
	default:
		// Buffer full — drop and let the write pump detect the dead connection.
		log.Printf("[ws] send buffer full for client %s, dropping message", c.ID)
	}
}

// readPump reads messages from the WebSocket and dispatches them to the hub.
// Runs in its own goroutine; closes the connection when done.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[ws] unexpected close from client %s: %v", c.ID, err)
			}
			break
		}

		var msg InMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			log.Printf("[ws] invalid JSON from client %s: %v", c.ID, err)
			c.Send(OutMessage{Action: ErrorAction, Data: map[string]string{"message": "invalid JSON"}})
			continue
		}

		c.hub.dispatch <- Envelope{Client: c, Message: msg}
	}
}

// writePump drains the send channel and writes to the WebSocket.
// Runs in its own goroutine.
func (c *Client) writePump() {
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
				// Hub closed the channel.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Printf("[ws] write error for client %s: %v", c.ID, err)
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ─── Events (Hub → GameServer) ───────────────────────────────────────────────

type EventKind int

const (
	EventConnected EventKind = iota
	EventDisconnected
	EventMessage
)

type Event struct {
	Kind   EventKind
	Client *Client
	Msg    InMessage // populated only for EventMessage
}

// ─── Hub ──────────────────────────────────────────────────────────────────────

// Envelope bundles an incoming message with its sender.
type Envelope struct {
	Client  *Client
	Message InMessage
}

// RoomBroadcast sends a message to every client in a specific room.
type RoomBroadcast struct {
	RoomID  string
	Message OutMessage
}

// Hub is the central registry of all active WebSocket clients.
// It has no knowledge of game logic — it only manages connections and routing.
//
// Two independent loops run concurrently:
//   - registryLoop  — owns the clients map; handles connect/disconnect/dispatch
//   - broadcastLoop — drains the broadcast channel; reads clients map under RLock
//
// This split ensures that a slow GameServer consumer never blocks room broadcasts.
type Hub struct {
	mu             sync.RWMutex
	clients        map[string]*Client // keyed by Client.ID
	clientByPlayer map[ds.ID]*Client

	register   chan *Client
	unregister chan *Client
	dispatch   chan Envelope
	broadcast  chan RoomBroadcast

	// Events is read by GameServer.Run(). Buffered to decouple the two loops.
	Events chan Event
}

func NewHub() *Hub {
	return &Hub{
		clients:        make(map[string]*Client),
		clientByPlayer: make(map[ds.ID]*Client),
		register:       make(chan *Client, 16),
		unregister:     make(chan *Client, 16),
		dispatch:       make(chan Envelope, 256),
		broadcast:      make(chan RoomBroadcast, 256),
		Events:         make(chan Event, 256),
	}
}

// Run launches both internal loops. Call once in a goroutine.
func (h *Hub) Run() {
	go h.broadcastLoop()
	h.registryLoop() // blocks
}

// registryLoop serialises all mutations to the clients map and forwards
// connect/disconnect/message events to the Events channel for GameServer.
func (h *Hub) registryLoop() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			h.clients[c.ID] = c
			h.clientByPlayer[c.PlayerID] = c
			h.mu.Unlock()
			log.Printf("[hub] connected: %s (player %s)", c.ID, c.PlayerID)
			h.Events <- Event{Kind: EventConnected, Client: c}

		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c.ID]; ok {
				delete(h.clients, c.ID)
				delete(h.clientByPlayer, c.PlayerID)
				close(c.send)
			}
			h.mu.Unlock()
			log.Printf("[hub] disconnected: %s (player %s)", c.ID, c.PlayerID)
			h.Events <- Event{Kind: EventDisconnected, Client: c}

		case env := <-h.dispatch:
			h.Events <- Event{Kind: EventMessage, Client: env.Client, Msg: env.Message}
		}
	}
}

// broadcastLoop is the only writer to client.send for room-wide messages.
// It runs independently so a full Events channel never stalls broadcasts.
func (h *Hub) broadcastLoop() {
	for bc := range h.broadcast {
		h.mu.RLock()
		for _, c := range h.clients {
			if c.RoomID == bc.RoomID {
				c.Send(bc.Message)
			}
		}
		h.mu.RUnlock()
	}
}

// Broadcast enqueues a message for every client in a room.
func (h *Hub) Broadcast(roomID string, msg OutMessage) {
	h.broadcast <- RoomBroadcast{RoomID: roomID, Message: msg}
}

// ClientByPlayerID finds a connected client by its game-level player ID.
func (h *Hub) ClientByPlayerID(playerID ds.ID) *Client {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.clientByPlayer[playerID]
}

// ─── HTTP handler ─────────────────────────────────────────────────────────────

// ServeWS upgrades an HTTP request to a WebSocket connection and registers the
// resulting client with the hub.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	playerID := ds.NewID()
	idString := r.URL.Query().Get("player_id")
	if idString != "" {
		var err error
		playerID, err = ds.ParseID(idString)
		if err != nil {
			log.Printf("invalid UUID: %v", err)
			return
		}
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[ws] upgrade failed for player %s: %v", playerID, err)
		return
	}

	c := newClient(h, conn, playerID)
	h.register <- c

	go c.writePump()
	go c.readPump()
}
