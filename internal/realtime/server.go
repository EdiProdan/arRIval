package realtime

import (
	"encoding/json"
	"log"
	"net/http"
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
	Store     *Store
	Hub       *Hub
	Now       func() time.Time
	Callbacks ServerCallbacks
}

type Server struct {
	store     *Store
	hub       *Hub
	now       func() time.Time
	callbacks ServerCallbacks
	ready     atomic.Bool
}

func NewServer(cfg ServerConfig) *Server {
	now := cfg.Now
	if now == nil {
		now = time.Now
	}

	return &Server{
		store:     cfg.Store,
		hub:       cfg.Hub,
		now:       now,
		callbacks: cfg.Callbacks,
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/readyz", s.handleReadyz)
	mux.HandleFunc("/v1/snapshot", s.handleSnapshot)
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

	payloadBytes, err := marshalEnvelope(contracts.RealtimeEventDelayObservedUpdateV2, observedAt.UTC(), contracts.RealtimeObservedDelayUpdate{
		ObservedDelay: event,
	})
	if err != nil {
		return err
	}

	s.hub.Broadcast(contracts.RealtimeEventDelayObservedUpdateV2, payloadBytes)
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

	payloadBytes, err := marshalEnvelope(contracts.RealtimeEventDelayPredictionUpdateV2, observedAt.UTC(), contracts.RealtimePredictedDelayUpdate{
		PredictedDelay: event,
	})
	if err != nil {
		return err
	}

	s.hub.Broadcast(contracts.RealtimeEventDelayPredictionUpdateV2, payloadBytes)
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
	s.emitStateCounts()
	writeJSON(w, http.StatusOK, snapshot)
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
