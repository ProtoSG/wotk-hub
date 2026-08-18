// Package wshub implements a minimal per-team WebSocket fanout hub used by
// the competitive games (emoji movies, daily riddle) to push live session
// updates to both partners without polling. Deliberately generic on the
// broadcast payload (any) instead of importing modules/games types — this
// package has no knowledge of games/riddle/session shapes, mirroring
// shared/team's role as a small cross-module utility with no upward
// dependency on the modules that use it.
package wshub

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// writeWait bounds how long a single write to a client can take before
	// it's considered dead.
	writeWait = 10 * time.Second
	// pongWait bounds how long we wait for a pong (or any client frame)
	// before considering the connection dead. pingPeriod must stay
	// comfortably under this or every connection times out between pings.
	pongWait = 60 * time.Second
	// pingPeriod is how often the write pump proactively pings an idle
	// connection to keep intermediary proxies/load balancers from closing
	// it for inactivity, and to detect a dead peer before pongWait expires.
	pingPeriod = (pongWait * 9) / 10
	// sendBufferSize bounds each client's outbound queue. A slow or stuck
	// reader drops (its connection is closed) rather than letting
	// BroadcastToTeam block or an unbounded backlog grow in memory.
	sendBufferSize = 16
)

// Client is one registered WebSocket connection, scoped to a team.
type Client struct {
	teamID int64
	conn   *websocket.Conn
	send   chan []byte
	hub    *Hub
}

// Hub fans JSON-marshaled events out to every connection registered under a
// given team. A couple's two accounts share one teamID (see
// shared/team.ResolveTeamID), so "broadcast to a team" reaches both
// partners' open tabs/devices at once.
type Hub struct {
	mu      sync.RWMutex
	clients map[int64]map[*Client]struct{}
}

// New returns an empty Hub, ready to Register connections and
// BroadcastToTeam. One Hub instance is meant to live for the process
// lifetime, same as the *sql.DB it sits alongside.
func New() *Hub {
	return &Hub{clients: make(map[int64]map[*Client]struct{})}
}

// Register adds an already-upgraded connection under teamID and starts its
// read and write pumps. The caller (an HTTP handler) has nothing further to
// do afterward — the pumps own the connection's lifecycle from here,
// including Unregistering and closing it once the peer disconnects.
func (h *Hub) Register(teamID int64, conn *websocket.Conn) *Client {
	c := &Client{teamID: teamID, conn: conn, send: make(chan []byte, sendBufferSize), hub: h}

	h.mu.Lock()
	if h.clients[teamID] == nil {
		h.clients[teamID] = make(map[*Client]struct{})
	}
	h.clients[teamID][c] = struct{}{}
	h.mu.Unlock()

	go c.writePump()
	go c.readPump()
	return c
}

// Unregister removes a client from the hub and closes its send channel so
// writePump can exit. Idempotent — safe to call more than once for the same
// client (readPump's normal-disconnect path and a forced drop from
// BroadcastToTeam can both race to call this).
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	clients, ok := h.clients[c.teamID]
	if !ok {
		return
	}
	if _, ok := clients[c]; !ok {
		return
	}
	delete(clients, c)
	close(c.send)
	if len(clients) == 0 {
		delete(h.clients, c.teamID)
	}
}

// BroadcastToTeam marshals event to JSON and fans it out to every
// connection currently registered under teamID. A client whose send buffer
// is already full (a stuck/slow reader) is dropped instead of allowed to
// block the broadcast for everyone else.
func (h *Hub) BroadcastToTeam(teamID int64, event any) {
	payload, err := json.Marshal(event)
	if err != nil {
		log.Printf("wshub: marshal event failed: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients[teamID] {
		select {
		case c.send <- payload:
		default:
			log.Printf("wshub: send buffer full for team %d, dropping connection", teamID)
			go h.Unregister(c)
		}
	}
}

// readPump drains client-sent frames — this hub is server→client push only,
// there's nothing for a client to say — and handles pong/close so a dead
// peer is detected instead of leaking a connection forever. Must run in its
// own goroutine: ReadMessage blocks until a frame arrives or the connection
// errors/closes.
func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

// writePump owns every write to the underlying connection —
// gorilla/websocket forbids concurrent writes on the same *Conn, so both
// broadcast payloads and keepalive pings must funnel through this one
// goroutine per client rather than writing directly from BroadcastToTeam.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Unregister closed our channel — tell the peer we're done
				// and stop.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
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
