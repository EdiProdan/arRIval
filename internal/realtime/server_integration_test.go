package realtime

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/EdiProdan/arRIval/internal/autotrolej"
	"github.com/EdiProdan/arRIval/internal/contracts"
)

func TestServerHealthAndReadyz(t *testing.T) {
	store := NewStore(StoreConfig{})
	hub := NewHub(HubConfig{PingInterval: 100 * time.Millisecond})
	server := NewServer(ServerConfig{Store: store, Hub: hub})
	defer server.Close()

	httpServer := httptest.NewServer(server.Routes())
	defer httpServer.Close()

	status := getStatusCode(t, httpServer.URL+"/healthz")
	if status != http.StatusOK {
		t.Fatalf("healthz status = %d, want %d", status, http.StatusOK)
	}

	status = getStatusCode(t, httpServer.URL+"/readyz")
	if status != http.StatusServiceUnavailable {
		t.Fatalf("readyz status = %d, want %d", status, http.StatusServiceUnavailable)
	}

	server.SetReady(true)
	status = getStatusCode(t, httpServer.URL+"/readyz")
	if status != http.StatusOK {
		t.Fatalf("readyz status = %d, want %d", status, http.StatusOK)
	}
}

func TestServerSnapshotEmptyAndPopulated(t *testing.T) {
	store := NewStore(StoreConfig{})
	hub := NewHub(HubConfig{PingInterval: 100 * time.Millisecond})
	server := NewServer(ServerConfig{Store: store, Hub: hub})
	defer server.Close()

	httpServer := httptest.NewServer(server.Routes())
	defer httpServer.Close()

	initial := getSnapshot(t, httpServer.URL+"/v1/snapshot")
	if initial.Meta.PositionsCount != 0 || initial.Meta.DelaysCount != 0 {
		t.Fatalf("initial counts = (%d,%d), want (0,0)", initial.Meta.PositionsCount, initial.Meta.DelaysCount)
	}

	observedAt := time.Date(2026, 2, 18, 11, 0, 0, 0, time.UTC)
	if err := server.HandlePositionsRecord("bus-positions-raw", mustPositionsPayload(t), observedAt); err != nil {
		t.Fatalf("HandlePositionsRecord: %v", err)
	}
	if err := server.HandleDelayRecord("bus-delays", mustDelayPayload(t), observedAt); err != nil {
		t.Fatalf("HandleDelayRecord: %v", err)
	}

	snapshot := getSnapshot(t, httpServer.URL+"/v1/snapshot")
	if snapshot.Meta.PositionsCount != 1 || snapshot.Meta.DelaysCount != 1 {
		t.Fatalf("snapshot counts = (%d,%d), want (1,1)", snapshot.Meta.PositionsCount, snapshot.Meta.DelaysCount)
	}
}

func TestServerWebsocketReceivesUpdatesAndHeartbeat(t *testing.T) {
	store := NewStore(StoreConfig{})
	hub := NewHub(HubConfig{PingInterval: 50 * time.Millisecond})
	server := NewServer(ServerConfig{Store: store, Hub: hub})
	defer server.Close()

	httpServer := httptest.NewServer(server.Routes())
	defer httpServer.Close()

	conn := dialTestWS(t, httpServer.URL, "/v1/ws")
	defer conn.Close()

	observedAt := time.Date(2026, 2, 18, 11, 10, 0, 0, time.UTC)
	if err := server.HandlePositionsRecord("bus-positions-raw", mustPositionsPayload(t), observedAt); err != nil {
		t.Fatalf("HandlePositionsRecord: %v", err)
	}
	if err := server.HandleDelayRecord("bus-delays", mustDelayPayload(t), observedAt); err != nil {
		t.Fatalf("HandleDelayRecord: %v", err)
	}

	foundPositions := false
	foundDelay := false
	foundHeartbeat := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, readErr := conn.ReadText(250 * time.Millisecond)
		if readErr != nil {
			continue
		}

		var envelope contracts.RealtimeEnvelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			t.Fatalf("unmarshal envelope: %v", err)
		}

		switch envelope.Type {
		case "positions_batch":
			foundPositions = true
		case "delay_update":
			foundDelay = true
		case "heartbeat":
			foundHeartbeat = true
		}

		if foundPositions && foundDelay && foundHeartbeat {
			break
		}
	}

	if !foundPositions || !foundDelay || !foundHeartbeat {
		t.Fatalf("found types positions=%v delay=%v heartbeat=%v, want all true", foundPositions, foundDelay, foundHeartbeat)
	}
}

func TestServerWebsocketStaleClientIsClosed(t *testing.T) {
	store := NewStore(StoreConfig{})
	hub := NewHub(HubConfig{PingInterval: 30 * time.Millisecond})
	server := NewServer(ServerConfig{Store: store, Hub: hub})
	defer server.Close()

	httpServer := httptest.NewServer(server.Routes())
	defer httpServer.Close()

	conn := dialTestWS(t, httpServer.URL, "/v1/ws")
	defer conn.Close()

	time.Sleep(250 * time.Millisecond)

	deadline := time.Now().Add(2 * time.Second)
	closed := false
	for time.Now().Before(deadline) {
		_, readErr := conn.ReadText(100 * time.Millisecond)
		if readErr != nil {
			closed = true
			break
		}
	}

	if !closed {
		t.Fatalf("expected stale websocket to be closed by server")
	}
}

func getStatusCode(t *testing.T, url string) int {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

func getSnapshot(t *testing.T, url string) contracts.RealtimeSnapshot {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var snapshot contracts.RealtimeSnapshot
	if err := json.Unmarshal(body, &snapshot); err != nil {
		t.Fatalf("unmarshal snapshot: %v body=%s", err, string(body))
	}
	return snapshot
}

func mustPositionsPayload(t *testing.T) []byte {
	t.Helper()
	lon := 14.4
	lat := 45.3
	gbr := 21
	voznjaBusID := 121

	payload, err := json.Marshal(autotrolej.AutobusiResponse{
		Msg: "ok",
		Err: false,
		Res: []autotrolej.LiveBus{
			{
				Lon:         &lon,
				Lat:         &lat,
				GBR:         &gbr,
				VoznjaBusID: &voznjaBusID,
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal positions payload: %v", err)
	}
	return payload
}

func mustDelayPayload(t *testing.T) []byte {
	t.Helper()
	payload, err := json.Marshal(contracts.DelayEvent{
		PolazakID:     "121",
		VoznjaBusID:   121,
		StationID:     1001,
		StationName:   "Main",
		DistanceM:     15,
		LinVarID:      "L1A",
		BrojLinije:    "1",
		ScheduledTime: "2026-02-18T11:05:00Z",
		ActualTime:    "2026-02-18T11:10:00Z",
		DelaySeconds:  300,
	})
	if err != nil {
		t.Fatalf("marshal delay payload: %v", err)
	}
	return payload
}
