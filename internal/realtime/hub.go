package realtime

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/EdiProdan/arRIval/internal/contracts"
)

const (
	defaultWSPingInterval = 20 * time.Second
	defaultWSClientBuffer = 128
)

type HubCallbacks struct {
	OnConnect     func()
	OnDisconnect  func(reason string)
	OnMessageSent func(messageType string)
	OnDropped     func(reason string)
}

type HubConfig struct {
	PingInterval time.Duration
	ClientBuffer int
	Now          func() time.Time
	Callbacks    HubCallbacks
}

type outboundMessage struct {
	messageType string
	payload     []byte
}

type client struct {
	conn *wsConn
	send chan outboundMessage
	once sync.Once
}

type Hub struct {
	mu           sync.RWMutex
	clients      map[*client]struct{}
	pingInterval time.Duration
	clientBuffer int
	now          func() time.Time
	callbacks    HubCallbacks
}

func NewHub(cfg HubConfig) *Hub {
	pingInterval := cfg.PingInterval
	if pingInterval <= 0 {
		pingInterval = defaultWSPingInterval
	}

	clientBuffer := cfg.ClientBuffer
	if clientBuffer <= 0 {
		clientBuffer = defaultWSClientBuffer
	}

	now := cfg.Now
	if now == nil {
		now = time.Now
	}

	return &Hub{
		clients:      make(map[*client]struct{}),
		pingInterval: pingInterval,
		clientBuffer: clientBuffer,
		now:          now,
		callbacks:    cfg.Callbacks,
	}
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgradeWebSocket(w, r)
	if err != nil {
		http.Error(w, "websocket upgrade failed", http.StatusBadRequest)
		return
	}

	client := &client{
		conn: conn,
		send: make(chan outboundMessage, h.clientBuffer),
	}

	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()

	if h.callbacks.OnConnect != nil {
		h.callbacks.OnConnect()
	}

	go h.readPump(client)
	go h.writePump(client)
}

func (h *Hub) Broadcast(messageType string, payload []byte) {
	dropped := make([]*client, 0)
	h.mu.RLock()
	for c := range h.clients {
		select {
		case c.send <- outboundMessage{messageType: messageType, payload: payload}:
		default:
			dropped = append(dropped, c)
		}
	}
	h.mu.RUnlock()

	for _, c := range dropped {
		h.closeClient(c, "slow_client")
		if h.callbacks.OnDropped != nil {
			h.callbacks.OnDropped("slow_client")
		}
	}
}

func (h *Hub) Close() {
	h.mu.RLock()
	clients := make([]*client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.RUnlock()

	for _, c := range clients {
		h.closeClient(c, "server_shutdown")
	}
}

func (h *Hub) readPump(c *client) {
	pongWait := 2 * h.pingInterval

	if c.conn != nil {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		c.conn.SetPongHandler(func() {
			_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		})
	}

	for {
		if c.conn == nil {
			h.closeClient(c, "read_error")
			return
		}

		if _, _, err := c.conn.ReadMessage(); err != nil {
			if errors.Is(err, errWSClosed) {
				h.closeClient(c, "client_closed")
				return
			}

			if errors.Is(err, net.ErrClosed) {
				h.closeClient(c, "client_closed")
				return
			}

			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				h.closeClient(c, "stale_client")
				return
			}

			h.closeClient(c, "read_error")
			return
		}
	}
}

func (h *Hub) writePump(c *client) {
	ticker := time.NewTicker(h.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				return
			}

			if c.conn == nil {
				h.closeClient(c, "write_error")
				return
			}

			_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := c.conn.WriteText(message.payload); err != nil {
				h.closeClient(c, "write_error")
				return
			}

			if h.callbacks.OnMessageSent != nil {
				h.callbacks.OnMessageSent(message.messageType)
			}
		case <-ticker.C:
			if c.conn == nil {
				h.closeClient(c, "write_error")
				return
			}

			heartbeatPayload, err := json.Marshal(contracts.RealtimeEnvelope{
				Type: "heartbeat",
				TS:   h.now().UTC().Format(time.RFC3339Nano),
				Data: contracts.RealtimeHeartbeat{
					Status: "ok",
				},
			})
			if err != nil {
				continue
			}

			_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := c.conn.WriteText(heartbeatPayload); err != nil {
				h.closeClient(c, "write_error")
				return
			}

			if h.callbacks.OnMessageSent != nil {
				h.callbacks.OnMessageSent("heartbeat")
			}

			_ = c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := c.conn.WritePing(); err != nil {
				h.closeClient(c, "stale_client")
				return
			}
		}
	}
}

func (h *Hub) closeClient(c *client, reason string) {
	c.once.Do(func() {
		removed := false
		h.mu.Lock()
		if _, ok := h.clients[c]; ok {
			delete(h.clients, c)
			removed = true
		}
		h.mu.Unlock()

		close(c.send)
		if c.conn != nil {
			_ = c.conn.WriteClose()
			_ = c.conn.Close()
		}

		if removed && h.callbacks.OnDisconnect != nil {
			h.callbacks.OnDisconnect(reason)
		}
	})
}
