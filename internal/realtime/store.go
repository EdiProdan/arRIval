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

type delayEntry struct {
	value     contracts.DelayEvent
	updatedAt time.Time
}

type Store struct {
	mu           sync.RWMutex
	positionsTTL time.Duration
	delaysTTL    time.Duration
	now          func() time.Time
	positions    map[string]positionEntry
	delays       map[string]delayEntry
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
		delays:       make(map[string]delayEntry),
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

func (s *Store) UpsertDelay(event contracts.DelayEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := delayKey(event.VoznjaBusID, event.StationID)
	s.delays[key] = delayEntry{
		value:     event,
		updatedAt: s.now().UTC(),
	}
}

func (s *Store) PruneExpired() (int, int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.pruneExpiredLocked(s.now().UTC())
}

func (s *Store) Snapshot() contracts.RealtimeSnapshot {
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

	delays := make([]contracts.DelayEvent, 0, len(s.delays))
	for _, entry := range s.delays {
		delays = append(delays, entry.value)
	}
	sort.Slice(delays, func(i, j int) bool {
		if delays[i].VoznjaBusID != delays[j].VoznjaBusID {
			return delays[i].VoznjaBusID < delays[j].VoznjaBusID
		}
		if delays[i].StationID != delays[j].StationID {
			return delays[i].StationID < delays[j].StationID
		}
		return delays[i].ActualTime < delays[j].ActualTime
	})

	return contracts.RealtimeSnapshot{
		GeneratedAt: now.Format(time.RFC3339Nano),
		Positions:   positions,
		Delays:      delays,
		Meta: contracts.RealtimeSnapshotMeta{
			PositionsCount: len(positions),
			DelaysCount:    len(delays),
		},
	}
}

func (s *Store) Counts() (int, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.positions), len(s.delays)
}

func (s *Store) pruneExpiredLocked(now time.Time) (int, int) {
	positionsRemoved := 0
	for key, entry := range s.positions {
		if now.Sub(entry.updatedAt) > s.positionsTTL {
			delete(s.positions, key)
			positionsRemoved++
		}
	}

	delaysRemoved := 0
	for key, entry := range s.delays {
		if now.Sub(entry.updatedAt) > s.delaysTTL {
			delete(s.delays, key)
			delaysRemoved++
		}
	}

	return positionsRemoved, delaysRemoved
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

func delayKey(voznjaBusID, stationID int64) string {
	return fmt.Sprintf("%d:%d", voznjaBusID, stationID)
}
