package realtime

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/EdiProdan/arRIval/internal/autotrolej"
	"github.com/EdiProdan/arRIval/internal/contracts"
)

func TestServerStationsEndpoint(t *testing.T) {
	store := NewStore(StoreConfig{})
	hub := NewHub(HubConfig{PingInterval: 100 * time.Millisecond})

	tmpDir := t.TempDir()
	stationsPath := filepath.Join(tmpDir, "stanice.json")
	if err := os.WriteFile(stationsPath, []byte(`[{"StanicaId":1,"Naziv":"Main","Kratki":"M","GpsX":14.4,"GpsY":45.3}]`), 0o644); err != nil {
		t.Fatalf("write stations fixture: %v", err)
	}

	server := NewServer(ServerConfig{
		Store:        store,
		Hub:          hub,
		StationsPath: stationsPath,
	})
	defer server.Close()

	httpServer := httptest.NewServer(server.Routes())
	defer httpServer.Close()

	resp, err := http.Get(httpServer.URL + "/v1/stations")
	if err != nil {
		t.Fatalf("GET /v1/stations: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var stations []map[string]any
	if err := json.Unmarshal(body, &stations); err != nil {
		t.Fatalf("unmarshal stations: %v body=%s", err, string(body))
	}
	if len(stations) != 1 {
		t.Fatalf("len(stations) = %d, want 1", len(stations))
	}
}

func TestServerLineMapEndpoint(t *testing.T) {
	store := NewStore(StoreConfig{})
	hub := NewHub(HubConfig{PingInterval: 100 * time.Millisecond})

	tmpDir := t.TempDir()
	lineMapPath := filepath.Join(tmpDir, "voznired_dnevni.json")
	payload := []byte(`[
		{"PolazakId":"1001","BrojLinije":"2A","NazivVarijanteLinije":"A -> B"},
		{"PolazakId":"1001","BrojLinije":"2A"},
		{"PolazakId":"1002","BrojLinije":"7","NazivVarijanteLinije":"C -> D"},
		{"PolazakId":"1003","BrojLinije":""},
		{"PolazakId":"","BrojLinije":"1"}
	]`)
	if err := os.WriteFile(lineMapPath, payload, 0o644); err != nil {
		t.Fatalf("write line map fixture: %v", err)
	}

	server := NewServer(ServerConfig{
		Store:       store,
		Hub:         hub,
		LineMapPath: lineMapPath,
	})
	defer server.Close()

	httpServer := httptest.NewServer(server.Routes())
	defer httpServer.Close()

	resp, err := http.Get(httpServer.URL + "/v1/line-map")
	if err != nil {
		t.Fatalf("GET /v1/line-map: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Deprecation"); got != "true" {
		t.Fatalf("Deprecation header = %q, want %q", got, "true")
	}
	if got := resp.Header.Get("Sunset"); got != deprecatedSunset {
		t.Fatalf("Sunset header = %q, want %q", got, deprecatedSunset)
	}

	type lineMapValue struct {
		Line      string `json:"line"`
		RouteName string `json:"route_name,omitempty"`
	}

	var lineMap map[string]lineMapValue
	if err := json.NewDecoder(resp.Body).Decode(&lineMap); err != nil {
		t.Fatalf("decode line map: %v", err)
	}

	if got := lineMap["1001"].Line; got != "2A" {
		t.Fatalf("lineMap[1001].line = %q, want %q", got, "2A")
	}
	if got := lineMap["1001"].RouteName; got != "A -> B" {
		t.Fatalf("lineMap[1001].route_name = %q, want %q", got, "A -> B")
	}
	if got := lineMap["1002"].Line; got != "7" {
		t.Fatalf("lineMap[1002].line = %q, want %q", got, "7")
	}
	if got := lineMap["1002"].RouteName; got != "C -> D" {
		t.Fatalf("lineMap[1002].route_name = %q, want %q", got, "C -> D")
	}
	if _, exists := lineMap["1003"]; exists {
		t.Fatalf("lineMap should not contain empty-line entry for key 1003")
	}
	if _, exists := lineMap[""]; exists {
		t.Fatalf("lineMap should not contain empty key")
	}
}

func TestServerLineMapEndpointUsesPreloadedCache(t *testing.T) {
	store := NewStore(StoreConfig{})
	hub := NewHub(HubConfig{PingInterval: 100 * time.Millisecond})

	tmpDir := t.TempDir()
	lineMapPath := filepath.Join(tmpDir, "voznired_dnevni.json")
	payload := []byte(`[{"PolazakId":"1001","BrojLinije":"2A","NazivVarijanteLinije":"A -> B"}]`)
	if err := os.WriteFile(lineMapPath, payload, 0o644); err != nil {
		t.Fatalf("write line map fixture: %v", err)
	}

	server := NewServer(ServerConfig{
		Store:       store,
		Hub:         hub,
		LineMapPath: lineMapPath,
	})
	defer server.Close()

	if err := os.Remove(lineMapPath); err != nil {
		t.Fatalf("remove line map fixture: %v", err)
	}

	httpServer := httptest.NewServer(server.Routes())
	defer httpServer.Close()

	resp, err := http.Get(httpServer.URL + "/v1/line-map")
	if err != nil {
		t.Fatalf("GET /v1/line-map: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	type lineMapValue struct {
		Line      string `json:"line"`
		RouteName string `json:"route_name,omitempty"`
	}

	var lineMap map[string]lineMapValue
	if err := json.NewDecoder(resp.Body).Decode(&lineMap); err != nil {
		t.Fatalf("decode line map: %v", err)
	}
	if got := lineMap["1001"].Line; got != "2A" {
		t.Fatalf("lineMap[1001].line = %q, want %q", got, "2A")
	}
}

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
	server := NewServer(ServerConfig{
		Store:             store,
		Hub:               hub,
		SourceInterval:    5 * time.Second,
		HeartbeatInterval: 20 * time.Second,
	})
	defer server.Close()

	httpServer := httptest.NewServer(server.Routes())
	defer httpServer.Close()

	initial := getSnapshot(t, httpServer.URL+"/v1/snapshot")
	if initial.Meta.PositionsCount != 0 || initial.Meta.ObservedDelaysCount != 0 || initial.Meta.PredictedDelaysCount != 0 {
		t.Fatalf(
			"initial counts = (%d,%d,%d), want (0,0,0)",
			initial.Meta.PositionsCount,
			initial.Meta.ObservedDelaysCount,
			initial.Meta.PredictedDelaysCount,
		)
	}
	if initial.Meta.SourceIntervalMS != 5000 {
		t.Fatalf("initial SourceIntervalMS = %d, want 5000", initial.Meta.SourceIntervalMS)
	}
	if initial.Meta.HeartbeatIntervalMS != 20000 {
		t.Fatalf("initial HeartbeatIntervalMS = %d, want 20000", initial.Meta.HeartbeatIntervalMS)
	}

	observedAt := time.Date(2026, 2, 18, 11, 0, 0, 0, time.UTC)
	if err := server.HandlePositionsRecord("bus-positions-raw", mustPositionsPayload(t), observedAt); err != nil {
		t.Fatalf("HandlePositionsRecord: %v", err)
	}
	if err := server.HandlePredictedDelayRecord("bus-delay-predicted", mustPredictedPayload(t, 5), observedAt); err != nil {
		t.Fatalf("HandlePredictedDelayRecord: %v", err)
	}
	if err := server.HandleObservedDelayRecord("bus-delay-observed", mustObservedPayload(t, 5), observedAt); err != nil {
		t.Fatalf("HandleObservedDelayRecord: %v", err)
	}

	snapshot := getSnapshot(t, httpServer.URL+"/v1/snapshot")
	if snapshot.Meta.PositionsCount != 1 || snapshot.Meta.ObservedDelaysCount != 1 || snapshot.Meta.PredictedDelaysCount != 0 {
		t.Fatalf(
			"snapshot counts = (%d,%d,%d), want (1,1,0)",
			snapshot.Meta.PositionsCount,
			snapshot.Meta.ObservedDelaysCount,
			snapshot.Meta.PredictedDelaysCount,
		)
	}
	if snapshot.Meta.SourceIntervalMS != 5000 {
		t.Fatalf("snapshot SourceIntervalMS = %d, want 5000", snapshot.Meta.SourceIntervalMS)
	}
	if snapshot.Meta.HeartbeatIntervalMS != 20000 {
		t.Fatalf("snapshot HeartbeatIntervalMS = %d, want 20000", snapshot.Meta.HeartbeatIntervalMS)
	}
}

func TestServerWebsocketReceivesUpdatesAndHeartbeat(t *testing.T) {
	store := NewStore(StoreConfig{})
	connected := make(chan struct{}, 1)
	hub := NewHub(HubConfig{
		PingInterval: 50 * time.Millisecond,
		Callbacks: HubCallbacks{
			OnConnect: func() {
				select {
				case connected <- struct{}{}:
				default:
				}
			},
		},
	})
	server := NewServer(ServerConfig{Store: store, Hub: hub})
	defer server.Close()

	httpServer := httptest.NewServer(server.Routes())
	defer httpServer.Close()

	conn := dialTestWS(t, httpServer.URL, "/v1/ws")
	defer conn.Close()

	select {
	case <-connected:
	case <-time.After(2 * time.Second):
		t.Fatalf("websocket client was not registered in hub before publish")
	}

	observedAt := time.Date(2026, 2, 18, 11, 10, 0, 0, time.UTC)
	if err := server.HandlePositionsRecord("bus-positions-raw", mustPositionsPayload(t), observedAt); err != nil {
		t.Fatalf("HandlePositionsRecord: %v", err)
	}
	if err := server.HandlePredictedDelayRecord("bus-delay-predicted", mustPredictedPayload(t, 6), observedAt); err != nil {
		t.Fatalf("HandlePredictedDelayRecord: %v", err)
	}
	if err := server.HandleObservedDelayRecord("bus-delay-observed", mustObservedPayload(t, 5), observedAt); err != nil {
		t.Fatalf("HandleObservedDelayRecord: %v", err)
	}

	foundPositions := false
	foundObserved := false
	foundPredicted := false
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
		case contracts.RealtimeEventDelayObservedUpdate:
			foundObserved = true
		case contracts.RealtimeEventDelayPredictionUpdate:
			foundPredicted = true
		case "heartbeat":
			foundHeartbeat = true
		}

		if foundPositions && foundObserved && foundPredicted && foundHeartbeat {
			break
		}
	}

	if !foundPositions || !foundObserved || !foundPredicted || !foundHeartbeat {
		t.Fatalf(
			"found types positions=%v observed=%v predicted=%v heartbeat=%v, want all true",
			foundPositions,
			foundObserved,
			foundPredicted,
			foundHeartbeat,
		)
	}
}

func TestServerSnapshotObservedSupersedesProgressedPredictions(t *testing.T) {
	store := NewStore(StoreConfig{})
	hub := NewHub(HubConfig{PingInterval: 100 * time.Millisecond})
	server := NewServer(ServerConfig{Store: store, Hub: hub})
	defer server.Close()

	httpServer := httptest.NewServer(server.Routes())
	defer httpServer.Close()

	observedAt := time.Date(2026, 2, 18, 11, 20, 0, 0, time.UTC)
	if err := server.HandlePredictedDelayRecord("bus-delay-predicted", mustPredictedPayload(t, 4), observedAt); err != nil {
		t.Fatalf("HandlePredictedDelayRecord seq=4: %v", err)
	}
	if err := server.HandlePredictedDelayRecord("bus-delay-predicted", mustPredictedPayload(t, 5), observedAt); err != nil {
		t.Fatalf("HandlePredictedDelayRecord seq=5: %v", err)
	}
	if err := server.HandlePredictedDelayRecord("bus-delay-predicted", mustPredictedPayload(t, 6), observedAt); err != nil {
		t.Fatalf("HandlePredictedDelayRecord seq=6: %v", err)
	}
	if err := server.HandleObservedDelayRecord("bus-delay-observed", mustObservedPayload(t, 5), observedAt); err != nil {
		t.Fatalf("HandleObservedDelayRecord seq=5: %v", err)
	}

	snapshot := getSnapshot(t, httpServer.URL+"/v1/snapshot")
	if snapshot.Meta.ObservedDelaysCount != 1 {
		t.Fatalf("ObservedDelaysCount = %d, want 1", snapshot.Meta.ObservedDelaysCount)
	}
	if snapshot.Meta.PredictedDelaysCount != 1 {
		t.Fatalf("PredictedDelaysCount = %d, want 1", snapshot.Meta.PredictedDelaysCount)
	}
	if snapshot.PredictedDelays[0].StationSeq != 6 {
		t.Fatalf("remaining predicted StationSeq = %d, want 6", snapshot.PredictedDelays[0].StationSeq)
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

func mustObservedPayload(t *testing.T, stationSeq int64) []byte {
	t.Helper()
	payload, err := json.Marshal(contracts.ObservedDelay{
		TripID:         "trip-121",
		VoznjaBusID:    121,
		StationID:      1000 + stationSeq,
		StationName:    "Main",
		StationSeq:     stationSeq,
		DistanceM:      15,
		LinVarID:       "L1A",
		BrojLinije:     "1",
		ScheduledTime:  "2026-02-18T11:05:00Z",
		ObservedTime:   "2026-02-18T11:10:00Z",
		DelaySeconds:   300,
		TrackerVersion: "current",
	})
	if err != nil {
		t.Fatalf("marshal observed payload: %v", err)
	}
	return payload
}

func mustPredictedPayload(t *testing.T, stationSeq int64) []byte {
	t.Helper()
	payload, err := json.Marshal(contracts.PredictedDelay{
		TripID:                "trip-121",
		VoznjaBusID:           121,
		StationID:             1000 + stationSeq,
		StationName:           "Main",
		StationSeq:            stationSeq,
		LinVarID:              "L1A",
		BrojLinije:            "1",
		ScheduledTime:         "2026-02-18T11:05:00Z",
		PredictedTime:         "2026-02-18T11:10:00Z",
		PredictedDelaySeconds: 300,
		GeneratedAt:           "2026-02-18T11:00:00Z",
		TrackerVersion:        "current",
	})
	if err != nil {
		t.Fatalf("marshal predicted payload: %v", err)
	}
	return payload
}
