package realtime

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/EdiProdan/arRIval/internal/contracts"
)

type ServerCallbacks struct {
	OnKafkaRecord      func(topic string)
	OnInvalidPositions func(count int)
	OnSnapshotRequest  func()
	OnStateChange      func(positions, observedDelays, predictedDelays int)
}

type ServerConfig struct {
	Store             *Store
	Hub               *Hub
	StationsPath      string
	LineMapPath       string
	TimetablePath     string
	SourceInterval    time.Duration
	HeartbeatInterval time.Duration
	Now               func() time.Time
	Callbacks         ServerCallbacks
}

type Server struct {
	store            *Store
	hub              *Hub
	stationsPath     string
	sourceIntervalMs int
	heartbeatMs      int
	now              func() time.Time
	callbacks        ServerCallbacks
	stationTimetable *stationTimetableIndex
	lineMapByBusID   map[string]lineMapValue
	ready            atomic.Bool
}

type lineMapRow struct {
	PolazakID            string `json:"PolazakId"`
	BrojLinije           string `json:"BrojLinije"`
	NazivVarijanteLinije string `json:"NazivVarijanteLinije"`
}

type lineMapValue struct {
	Line      string `json:"line"`
	RouteName string `json:"route_name,omitempty"`
}

const deprecatedSunset = "Mon, 30 Jun 2026 00:00:00 GMT"

func NewServer(cfg ServerConfig) *Server {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}

	var stationTimetable *stationTimetableIndex
	if strings.TrimSpace(cfg.TimetablePath) != "" {
		loadedIndex, err := loadStationTimetableIndex(cfg.StationsPath, cfg.TimetablePath)
		if err != nil {
			log.Printf("status=error stage=station_timetable_load err=%v", err)
		} else {
			stationTimetable = loadedIndex
		}
	}

	var lineMapByBusID map[string]lineMapValue
	if strings.TrimSpace(cfg.LineMapPath) != "" {
		loadedLineMap, err := loadLineMapIndex(cfg.LineMapPath)
		if err != nil {
			log.Printf("status=error stage=line_map_load err=%v", err)
		} else {
			lineMapByBusID = loadedLineMap
		}
	}

	return &Server{
		store:            cfg.Store,
		hub:              cfg.Hub,
		stationsPath:     cfg.StationsPath,
		sourceIntervalMs: durationToMillis(cfg.SourceInterval),
		heartbeatMs:      durationToMillis(cfg.HeartbeatInterval),
		now:              now,
		callbacks:        cfg.Callbacks,
		stationTimetable: stationTimetable,
		lineMapByBusID:   lineMapByBusID,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/readyz", s.handleReadyz)
	mux.HandleFunc("/v1/snapshot", s.handleSnapshot)
	mux.HandleFunc("/v1/stations", s.handleStations)
	mux.HandleFunc("/v1/station-arrivals", s.handleStationArrivals)
	mux.HandleFunc("/v1/line-map", s.handleLineMap)
	mux.HandleFunc("/v1/station-timetable", s.handleStationTimetable)
	mux.HandleFunc("/v1/ws", s.handleWS)
	mux.Handle("/", newUIHandler())
	return mux
}

func (s *Server) SetReady(value bool) {
	s.ready.Store(value)
}

func (s *Server) HandlePositionsRecord(topic string, payload []byte, observedAt time.Time) error {
	if s.callbacks.OnKafkaRecord != nil {
		s.callbacks.OnKafkaRecord(topic)
	}

	positions, invalid, err := ParsePositionsRecord(payload, observedAt)
	if invalid > 0 && s.callbacks.OnInvalidPositions != nil {
		s.callbacks.OnInvalidPositions(invalid)
	}
	if err != nil {
		return err
	}
	if len(positions) == 0 {
		return nil
	}

	s.store.UpsertPositions(positions)
	s.store.PruneExpired()
	s.emitStateCounts()

	payloadBytes, err := marshalEnvelope("positions_batch", observedAt.UTC(), contracts.RealtimePositionsBatch{
		Positions: positions,
	})
	if err != nil {
		return err
	}

	s.hub.Broadcast("positions_batch", payloadBytes)
	return nil
}

func (s *Server) HandleObservedDelayRecord(topic string, payload []byte, observedAt time.Time) error {
	if s.callbacks.OnKafkaRecord != nil {
		s.callbacks.OnKafkaRecord(topic)
	}

	event, err := ParseObservedDelayRecord(payload)
	if err != nil {
		return err
	}

	s.store.UpsertObservedDelay(event)
	s.store.PruneExpired()
	s.emitStateCounts()

	payloadBytes, err := marshalEnvelope(contracts.RealtimeEventDelayObservedUpdate, observedAt.UTC(), contracts.RealtimeObservedDelayUpdate{
		ObservedDelay: event,
	})
	if err != nil {
		return err
	}

	s.hub.Broadcast(contracts.RealtimeEventDelayObservedUpdate, payloadBytes)
	return nil
}

func (s *Server) HandlePredictedDelayRecord(topic string, payload []byte, observedAt time.Time) error {
	if s.callbacks.OnKafkaRecord != nil {
		s.callbacks.OnKafkaRecord(topic)
	}

	event, err := ParsePredictedDelayRecord(payload)
	if err != nil {
		return err
	}

	s.store.UpsertPredictedDelay(event)
	s.store.PruneExpired()
	s.emitStateCounts()

	payloadBytes, err := marshalEnvelope(contracts.RealtimeEventDelayPredictionUpdate, observedAt.UTC(), contracts.RealtimePredictedDelayUpdate{
		PredictedDelay: event,
	})
	if err != nil {
		return err
	}

	s.hub.Broadcast(contracts.RealtimeEventDelayPredictionUpdate, payloadBytes)
	return nil
}

func (s *Server) Close() {
	s.hub.Close()
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	if !s.ready.Load() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) handleSnapshot(w http.ResponseWriter, _ *http.Request) {
	if s.callbacks.OnSnapshotRequest != nil {
		s.callbacks.OnSnapshotRequest()
	}

	snapshot := s.store.Snapshot()
	snapshot.Meta.SourceIntervalMS = s.sourceIntervalMs
	snapshot.Meta.HeartbeatIntervalMS = s.heartbeatMs
	s.emitStateCounts()
	writeJSON(w, http.StatusOK, snapshot)
}

func (s *Server) handleStations(w http.ResponseWriter, _ *http.Request) {
	payload, err := os.ReadFile(s.stationsPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "stations_unavailable"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(payload); err != nil {
		log.Printf("status=error stage=http_write status=%d err=%v", http.StatusOK, err)
	}
}

func (s *Server) handleLineMap(w http.ResponseWriter, _ *http.Request) {
	markDeprecatedEndpoint(w, "/v1/line-map")

	if s.lineMapByBusID == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "line_map_unavailable"})
		return
	}

	writeJSON(w, http.StatusOK, s.lineMapByBusID)
}

func (s *Server) handleStationArrivals(w http.ResponseWriter, r *http.Request) {
	stationIDRaw := strings.TrimSpace(r.URL.Query().Get("station_id"))
	if stationIDRaw == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "station_id_required"})
		return
	}

	stationID, err := strconv.ParseInt(stationIDRaw, 10, 64)
	if err != nil || stationID <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "station_id_invalid"})
		return
	}

	windowMinutes := defaultStationTimetableWindowMinutes
	windowRaw := strings.TrimSpace(r.URL.Query().Get("window_minutes"))
	if windowRaw != "" {
		parsedWindow, parseErr := strconv.Atoi(windowRaw)
		if parseErr != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "window_minutes_invalid"})
			return
		}
		windowMinutes = parsedWindow
	}
	if windowMinutes < minStationTimetableWindowMinutes {
		windowMinutes = minStationTimetableWindowMinutes
	}
	if windowMinutes > maxStationTimetableWindowMinutes {
		windowMinutes = maxStationTimetableWindowMinutes
	}

	if s.stationTimetable == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "station_arrivals_unavailable"})
		return
	}

	window := time.Duration(windowMinutes) * time.Minute
	snapshot := s.store.Snapshot()
	response := s.stationTimetable.Build(stationID, s.now().UTC(), window, snapshot.PredictedDelays)
	response.WindowMinutes = windowMinutes

	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleStationTimetable(w http.ResponseWriter, r *http.Request) {
	markDeprecatedEndpoint(w, "/v1/station-timetable")
	s.handleStationArrivals(w, r)
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}
	s.hub.ServeWS(w, r)
}

func (s *Server) emitStateCounts() {
	if s.callbacks.OnStateChange == nil {
		return
	}
	positions, observedDelays, predictedDelays := s.store.Counts()
	s.callbacks.OnStateChange(positions, observedDelays, predictedDelays)
}

func loadLineMapIndex(path string) (map[string]lineMapValue, error) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return nil, nil
	}

	payload, err := os.ReadFile(trimmedPath)
	if err != nil {
		return nil, err
	}

	var rows []lineMapRow
	if err := json.Unmarshal(payload, &rows); err != nil {
		return nil, err
	}

	byBusID := make(map[string]lineMapValue, len(rows))
	for _, row := range rows {
		busID := strings.TrimSpace(row.PolazakID)
		line := strings.TrimSpace(row.BrojLinije)
		routeName := strings.TrimSpace(row.NazivVarijanteLinije)
		if busID == "" || line == "" {
			continue
		}
		if _, exists := byBusID[busID]; exists {
			continue
		}
		byBusID[busID] = lineMapValue{
			Line:      line,
			RouteName: routeName,
		}
	}

	return byBusID, nil
}

func durationToMillis(value time.Duration) int {
	if value <= 0 {
		return 0
	}
	return int(value / time.Millisecond)
}

func markDeprecatedEndpoint(w http.ResponseWriter, endpoint string) {
	w.Header().Set("Deprecation", "true")
	w.Header().Set("Sunset", deprecatedSunset)
	log.Printf("status=warn stage=http_deprecated endpoint=%s", endpoint)
}

func marshalEnvelope(messageType string, ts time.Time, data any) ([]byte, error) {
	payload, err := json.Marshal(contracts.RealtimeEnvelope{
		Type: messageType,
		TS:   ts.UTC().Format(time.RFC3339Nano),
		Data: data,
	})
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("status=error stage=http_write status=%d err=%v", status, err)
	}
}
