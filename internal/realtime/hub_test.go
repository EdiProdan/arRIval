package realtime

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestHubBroadcastDropsSlowClient(t *testing.T) {
	var dropped int64
	var disconnected int64

	hub := NewHub(HubConfig{
		Callbacks: HubCallbacks{
			OnDropped: func(string) {
				atomic.AddInt64(&dropped, 1)
			},
			OnDisconnect: func(reason string) {
				if reason == "slow_client" {
					atomic.AddInt64(&disconnected, 1)
				}
			},
		},
	})

	c := &client{
		send: make(chan outboundMessage, 1),
	}
	c.send <- outboundMessage{messageType: "positions_batch", payload: []byte(`{}`)}

	hub.mu.Lock()
	hub.clients[c] = struct{}{}
	hub.mu.Unlock()

	hub.Broadcast("positions_batch", []byte(`{"type":"positions_batch"}`))

	if atomic.LoadInt64(&dropped) != 1 {
		t.Fatalf("dropped = %d, want 1", atomic.LoadInt64(&dropped))
	}
	if atomic.LoadInt64(&disconnected) != 1 {
		t.Fatalf("disconnected = %d, want 1", atomic.LoadInt64(&disconnected))
	}
}

func TestHubServeWSConnectDisconnect(t *testing.T) {
	var connected int64
	var disconnected int64

	hub := NewHub(HubConfig{
		PingInterval: 100 * time.Millisecond,
		Callbacks: HubCallbacks{
			OnConnect: func() { atomic.AddInt64(&connected, 1) },
			OnDisconnect: func(reason string) {
				if reason == "client_closed" || reason == "read_error" {
					atomic.AddInt64(&disconnected, 1)
				}
			},
		},
	})

	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hub.ServeWS(w, r)
	}))
	defer httpServer.Close()
	defer hub.Close()

	conn := dialTestWS(t, httpServer.URL, "/")

	if !waitFor(t, time.Second, func() bool { return atomic.LoadInt64(&connected) == 1 }) {
		t.Fatalf("connected callback not observed")
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("close websocket: %v", err)
	}

	if !waitFor(t, 2*time.Second, func() bool { return atomic.LoadInt64(&disconnected) == 1 }) {
		t.Fatalf("disconnected callback not observed")
	}
}

func waitFor(t *testing.T, timeout time.Duration, check func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if check() {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return check()
}
