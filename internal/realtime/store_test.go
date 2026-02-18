package realtime

import (
	"testing"
	"time"

	"github.com/EdiProdan/arRIval/internal/contracts"
)

func TestStoreUpsertReplacesByKey(t *testing.T) {
	now := time.Date(2026, 2, 18, 10, 0, 0, 0, time.UTC)
	store := NewStore(StoreConfig{
		Now: func() time.Time { return now },
	})

	busID := int64(101)
	store.UpsertPositions([]contracts.RealtimePosition{
		{
			Key:         "voznja_bus_id:101",
			VoznjaBusID: &busID,
			Lon:         14.1,
			Lat:         45.1,
			ObservedAt:  now.Format(time.RFC3339Nano),
		},
	})

	now = now.Add(1 * time.Minute)
	store.UpsertPositions([]contracts.RealtimePosition{
		{
			Key:         "voznja_bus_id:101",
			VoznjaBusID: &busID,
			Lon:         14.2,
			Lat:         45.2,
			ObservedAt:  now.Format(time.RFC3339Nano),
		},
	})

	snapshot := store.Snapshot()
	if len(snapshot.Positions) != 1 {
		t.Fatalf("len(snapshot.Positions) = %d, want 1", len(snapshot.Positions))
	}
	if snapshot.Positions[0].Lon != 14.2 || snapshot.Positions[0].Lat != 45.2 {
		t.Fatalf("position = (%f,%f), want (%f,%f)", snapshot.Positions[0].Lon, snapshot.Positions[0].Lat, 14.2, 45.2)
	}
}

func TestStorePruneExpired(t *testing.T) {
	now := time.Date(2026, 2, 18, 10, 0, 0, 0, time.UTC)
	store := NewStore(StoreConfig{
		PositionsTTL: 1 * time.Minute,
		DelaysTTL:    2 * time.Minute,
		Now:          func() time.Time { return now },
	})

	store.UpsertPositions([]contracts.RealtimePosition{
		{
			Key:        "gbr:1",
			Lon:        14.1,
			Lat:        45.1,
			ObservedAt: now.Format(time.RFC3339Nano),
		},
	})
	store.UpsertDelay(contracts.DelayEvent{
		VoznjaBusID:  7,
		StationID:    100,
		DelaySeconds: 30,
	})

	now = now.Add(90 * time.Second)
	positionsRemoved, delaysRemoved := store.PruneExpired()
	if positionsRemoved != 1 || delaysRemoved != 0 {
		t.Fatalf("removed = (%d,%d), want (1,0)", positionsRemoved, delaysRemoved)
	}

	now = now.Add(60 * time.Second)
	positionsRemoved, delaysRemoved = store.PruneExpired()
	if positionsRemoved != 0 || delaysRemoved != 1 {
		t.Fatalf("removed = (%d,%d), want (0,1)", positionsRemoved, delaysRemoved)
	}
}

func TestStoreSnapshotDeterministicOrderAndCounts(t *testing.T) {
	now := time.Date(2026, 2, 18, 10, 0, 0, 0, time.UTC)
	store := NewStore(StoreConfig{
		Now: func() time.Time { return now },
	})

	busID2 := int64(2)
	busID1 := int64(1)
	gbr := int64(20)
	store.UpsertPositions([]contracts.RealtimePosition{
		{
			Key:         "voznja_bus_id:2",
			VoznjaBusID: &busID2,
			Lon:         14.2,
			Lat:         45.2,
			ObservedAt:  now.Format(time.RFC3339Nano),
		},
		{
			Key:         "voznja_bus_id:1",
			VoznjaBusID: &busID1,
			GBR:         &gbr,
			Lon:         14.1,
			Lat:         45.1,
			ObservedAt:  now.Format(time.RFC3339Nano),
		},
	})

	store.UpsertDelay(contracts.DelayEvent{VoznjaBusID: 2, StationID: 20, ActualTime: "2026-02-18T10:00:00Z"})
	store.UpsertDelay(contracts.DelayEvent{VoznjaBusID: 1, StationID: 10, ActualTime: "2026-02-18T10:01:00Z"})

	snapshot := store.Snapshot()
	if snapshot.Meta.PositionsCount != 2 || snapshot.Meta.DelaysCount != 2 {
		t.Fatalf("counts = (%d,%d), want (2,2)", snapshot.Meta.PositionsCount, snapshot.Meta.DelaysCount)
	}
	if snapshot.Positions[0].Key != "voznja_bus_id:1" || snapshot.Positions[1].Key != "voznja_bus_id:2" {
		t.Fatalf("position order = [%s,%s], want [voznja_bus_id:1,voznja_bus_id:2]", snapshot.Positions[0].Key, snapshot.Positions[1].Key)
	}
	if snapshot.Delays[0].VoznjaBusID != 1 || snapshot.Delays[1].VoznjaBusID != 2 {
		t.Fatalf("delay order = [%d,%d], want [1,2]", snapshot.Delays[0].VoznjaBusID, snapshot.Delays[1].VoznjaBusID)
	}
}
