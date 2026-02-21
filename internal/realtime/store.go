package realtime

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/EdiProdan/arRIval/internal/contracts"
)

const (
	defaultPositionsTTL = 5 * time.Minute
	defaultDelaysTTL    = 90 * time.Minute
)

type StoreConfig struct {
	PositionsTTL time.Duration
	DelaysTTL    time.Duration
	Now          func() time.Time
}

type positionEntry struct {
	value     contracts.RealtimePosition
	updatedAt time.Time
}

type observedDelayEntry struct {
	value     contracts.ObservedDelayV2
	updatedAt time.Time
}

type predictedDelayEntry struct {
	value     contracts.PredictedDelayV2
	updatedAt time.Time
}

type Store struct {
	mu           sync.RWMutex
	positionsTTL time.Duration
	delaysTTL    time.Duration
	now          func() time.Time
	positions    map[string]positionEntry
	observed     map[string]observedDelayEntry
	predicted    map[string]predictedDelayEntry
}

func NewStore(cfg StoreConfig) *Store {
	positionsTTL := cfg.PositionsTTL
	if positionsTTL <= 0 {
		positionsTTL = defaultPositionsTTL
	}

	delaysTTL := cfg.DelaysTTL
	if delaysTTL <= 0 {
		delaysTTL = defaultDelaysTTL
	}

	now := cfg.Now
	if now == nil {
		now = time.Now
	}

	return &Store{
		positionsTTL: positionsTTL,
		delaysTTL:    delaysTTL,
		now:          now,
		positions:    make(map[string]positionEntry),
		observed:     make(map[string]observedDelayEntry),
		predicted:    make(map[string]predictedDelayEntry),
	}
}

func (s *Store) UpsertPositions(positions []contracts.RealtimePosition) {
	s.mu.Lock()
	defer s.mu.Unlock()

	updatedAt := s.now().UTC()
	for _, position := range positions {
		key := position.Key
		if key == "" {
			key = positionKey(position.VoznjaBusID, position.GBR)
			position.Key = key
		}
		s.positions[key] = positionEntry{
			value:     position,
			updatedAt: updatedAt,
		}
	}
}

func (s *Store) UpsertObservedDelay(event contracts.ObservedDelayV2) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := delayV2Key(event.TripID, event.StationID)
	s.observed[key] = observedDelayEntry{
		value:     event,
		updatedAt: s.now().UTC(),
	}

	for predictedKey, entry := range s.predicted {
		if entry.value.TripID == event.TripID && entry.value.StationSeq <= event.StationSeq {
			delete(s.predicted, predictedKey)
		}
	}
}

func (s *Store) UpsertPredictedDelay(event contracts.PredictedDelayV2) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := delayV2Key(event.TripID, event.StationID)
	s.predicted[key] = predictedDelayEntry{
		value:     event,
		updatedAt: s.now().UTC(),
	}
}

func (s *Store) PruneExpired() (int, int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.pruneExpiredLocked(s.now().UTC())
}

func (s *Store) Snapshot() contracts.RealtimeSnapshotV2 {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now().UTC()
	s.pruneExpiredLocked(now)

	positions := make([]contracts.RealtimePosition, 0, len(s.positions))
	for _, entry := range s.positions {
		positions = append(positions, entry.value)
	}
	sort.Slice(positions, func(i, j int) bool {
		return positions[i].Key < positions[j].Key
	})

	observed := make([]contracts.ObservedDelayV2, 0, len(s.observed))
	for _, entry := range s.observed {
		observed = append(observed, entry.value)
	}
	sort.Slice(observed, func(i, j int) bool {
		if observed[i].TripID != observed[j].TripID {
			return observed[i].TripID < observed[j].TripID
		}
		if observed[i].StationSeq != observed[j].StationSeq {
			return observed[i].StationSeq < observed[j].StationSeq
		}
		return observed[i].ObservedTime < observed[j].ObservedTime
	})

	predicted := make([]contracts.PredictedDelayV2, 0, len(s.predicted))
	for _, entry := range s.predicted {
		predicted = append(predicted, entry.value)
	}
	sort.Slice(predicted, func(i, j int) bool {
		if predicted[i].TripID != predicted[j].TripID {
			return predicted[i].TripID < predicted[j].TripID
		}
		if predicted[i].StationSeq != predicted[j].StationSeq {
			return predicted[i].StationSeq < predicted[j].StationSeq
		}
		return predicted[i].GeneratedAt < predicted[j].GeneratedAt
	})

	return contracts.RealtimeSnapshotV2{
		GeneratedAt:     now.Format(time.RFC3339Nano),
		Positions:       positions,
		ObservedDelays:  observed,
		PredictedDelays: predicted,
		Meta: contracts.RealtimeSnapshotMetaV2{
			PositionsCount:       len(positions),
			ObservedDelaysCount:  len(observed),
			PredictedDelaysCount: len(predicted),
		},
	}
}

func (s *Store) Counts() (int, int, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.positions), len(s.observed), len(s.predicted)
}

func (s *Store) pruneExpiredLocked(now time.Time) (int, int, int) {
	positionsRemoved := 0
	for key, entry := range s.positions {
		if now.Sub(entry.updatedAt) > s.positionsTTL {
			delete(s.positions, key)
			positionsRemoved++
		}
	}

	observedRemoved := 0
	for key, entry := range s.observed {
		if now.Sub(entry.updatedAt) > s.delaysTTL {
			delete(s.observed, key)
			observedRemoved++
		}
	}

	predictedRemoved := 0
	for key, entry := range s.predicted {
		if now.Sub(entry.updatedAt) > s.delaysTTL {
			delete(s.predicted, key)
			predictedRemoved++
		}
	}

	return positionsRemoved, observedRemoved, predictedRemoved
}

func positionKey(voznjaBusID, gbr *int64) string {
	if voznjaBusID != nil {
		return fmt.Sprintf("voznja_bus_id:%d", *voznjaBusID)
	}
	if gbr != nil {
		return fmt.Sprintf("gbr:%d", *gbr)
	}
	return "unknown"
}

func delayV2Key(tripID string, stationID int64) string {
	return fmt.Sprintf("%s:%d", tripID, stationID)
}
