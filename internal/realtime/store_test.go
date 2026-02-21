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
	store.UpsertObservedDelay(contracts.ObservedDelayV2{
		TripID:         "trip-7",
		VoznjaBusID:    7,
		StationID:      100,
		StationSeq:     1,
		ObservedTime:   now.Format(time.RFC3339Nano),
		DelaySeconds:   30,
		TrackerVersion: "v2",
	})
	store.UpsertPredictedDelay(contracts.PredictedDelayV2{
		TripID:                "trip-7",
		VoznjaBusID:           7,
		StationID:             101,
		StationSeq:            2,
		PredictedDelaySeconds: 45,
		GeneratedAt:           now.Format(time.RFC3339Nano),
		TrackerVersion:        "v2",
	})

	now = now.Add(90 * time.Second)
	positionsRemoved, observedRemoved, predictedRemoved := store.PruneExpired()
	if positionsRemoved != 1 || observedRemoved != 0 || predictedRemoved != 0 {
		t.Fatalf("removed = (%d,%d,%d), want (1,0,0)", positionsRemoved, observedRemoved, predictedRemoved)
	}

	now = now.Add(60 * time.Second)
	positionsRemoved, observedRemoved, predictedRemoved = store.PruneExpired()
	if positionsRemoved != 0 || observedRemoved != 1 || predictedRemoved != 1 {
		t.Fatalf("removed = (%d,%d,%d), want (0,1,1)", positionsRemoved, observedRemoved, predictedRemoved)
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

	store.UpsertObservedDelay(contracts.ObservedDelayV2{
		TripID:         "trip-2",
		VoznjaBusID:    2,
		StationID:      20,
		StationSeq:     2,
		ObservedTime:   "2026-02-18T10:00:00Z",
		TrackerVersion: "v2",
	})
	store.UpsertObservedDelay(contracts.ObservedDelayV2{
		TripID:         "trip-1",
		VoznjaBusID:    1,
		StationID:      10,
		StationSeq:     1,
		ObservedTime:   "2026-02-18T10:01:00Z",
		TrackerVersion: "v2",
	})
	store.UpsertPredictedDelay(contracts.PredictedDelayV2{
		TripID:         "trip-2",
		VoznjaBusID:    2,
		StationID:      30,
		StationSeq:     3,
		GeneratedAt:    "2026-02-18T10:00:00Z",
		TrackerVersion: "v2",
	})
	store.UpsertPredictedDelay(contracts.PredictedDelayV2{
		TripID:         "trip-1",
		VoznjaBusID:    1,
		StationID:      20,
		StationSeq:     2,
		GeneratedAt:    "2026-02-18T10:01:00Z",
		TrackerVersion: "v2",
	})

	snapshot := store.Snapshot()
	if snapshot.Meta.PositionsCount != 2 || snapshot.Meta.ObservedDelaysCount != 2 || snapshot.Meta.PredictedDelaysCount != 2 {
		t.Fatalf(
			"counts = (%d,%d,%d), want (2,2,2)",
			snapshot.Meta.PositionsCount,
			snapshot.Meta.ObservedDelaysCount,
			snapshot.Meta.PredictedDelaysCount,
		)
	}
	if snapshot.Positions[0].Key != "voznja_bus_id:1" || snapshot.Positions[1].Key != "voznja_bus_id:2" {
		t.Fatalf("position order = [%s,%s], want [voznja_bus_id:1,voznja_bus_id:2]", snapshot.Positions[0].Key, snapshot.Positions[1].Key)
	}
	if snapshot.ObservedDelays[0].TripID != "trip-1" || snapshot.ObservedDelays[1].TripID != "trip-2" {
		t.Fatalf("observed order = [%s,%s], want [trip-1,trip-2]", snapshot.ObservedDelays[0].TripID, snapshot.ObservedDelays[1].TripID)
	}
	if snapshot.PredictedDelays[0].TripID != "trip-1" || snapshot.PredictedDelays[1].TripID != "trip-2" {
		t.Fatalf("predicted order = [%s,%s], want [trip-1,trip-2]", snapshot.PredictedDelays[0].TripID, snapshot.PredictedDelays[1].TripID)
	}
}

func TestStoreObservedUpsertRemovesMatchingPredictedStop(t *testing.T) {
	now := time.Date(2026, 2, 18, 10, 0, 0, 0, time.UTC)
	store := NewStore(StoreConfig{
		Now: func() time.Time { return now },
	})

	store.UpsertPredictedDelay(contracts.PredictedDelayV2{
		TripID:         "trip-121",
		VoznjaBusID:    121,
		StationID:      5,
		StationSeq:     5,
		GeneratedAt:    "2026-02-18T10:00:00Z",
		TrackerVersion: "v2",
	})
	store.UpsertObservedDelay(contracts.ObservedDelayV2{
		TripID:         "trip-121",
		VoznjaBusID:    121,
		StationID:      5,
		StationSeq:     5,
		ObservedTime:   "2026-02-18T10:02:00Z",
		TrackerVersion: "v2",
	})

	snapshot := store.Snapshot()
	if snapshot.Meta.ObservedDelaysCount != 1 {
		t.Fatalf("ObservedDelaysCount = %d, want 1", snapshot.Meta.ObservedDelaysCount)
	}
	if snapshot.Meta.PredictedDelaysCount != 0 {
		t.Fatalf("PredictedDelaysCount = %d, want 0", snapshot.Meta.PredictedDelaysCount)
	}
}

func TestStoreObservedUpsertRemovesProgressedPredictions(t *testing.T) {
	now := time.Date(2026, 2, 18, 10, 0, 0, 0, time.UTC)
	store := NewStore(StoreConfig{
		Now: func() time.Time { return now },
	})

	store.UpsertPredictedDelay(contracts.PredictedDelayV2{
		TripID:         "trip-121",
		VoznjaBusID:    121,
		StationID:      30,
		StationSeq:     3,
		GeneratedAt:    "2026-02-18T10:00:00Z",
		TrackerVersion: "v2",
	})
	store.UpsertPredictedDelay(contracts.PredictedDelayV2{
		TripID:         "trip-121",
		VoznjaBusID:    121,
		StationID:      40,
		StationSeq:     4,
		GeneratedAt:    "2026-02-18T10:00:00Z",
		TrackerVersion: "v2",
	})
	store.UpsertPredictedDelay(contracts.PredictedDelayV2{
		TripID:         "trip-121",
		VoznjaBusID:    121,
		StationID:      50,
		StationSeq:     5,
		GeneratedAt:    "2026-02-18T10:00:00Z",
		TrackerVersion: "v2",
	})
	store.UpsertPredictedDelay(contracts.PredictedDelayV2{
		TripID:         "trip-121",
		VoznjaBusID:    121,
		StationID:      60,
		StationSeq:     6,
		GeneratedAt:    "2026-02-18T10:00:00Z",
		TrackerVersion: "v2",
	})
	store.UpsertPredictedDelay(contracts.PredictedDelayV2{
		TripID:         "trip-999",
		VoznjaBusID:    999,
		StationID:      40,
		StationSeq:     4,
		GeneratedAt:    "2026-02-18T10:00:00Z",
		TrackerVersion: "v2",
	})

	store.UpsertObservedDelay(contracts.ObservedDelayV2{
		TripID:         "trip-121",
		VoznjaBusID:    121,
		StationID:      50,
		StationSeq:     5,
		ObservedTime:   "2026-02-18T10:02:00Z",
		TrackerVersion: "v2",
	})

	snapshot := store.Snapshot()
	if snapshot.Meta.PredictedDelaysCount != 2 {
		t.Fatalf("PredictedDelaysCount = %d, want 2", snapshot.Meta.PredictedDelaysCount)
	}

	for _, event := range snapshot.PredictedDelays {
		if event.TripID == "trip-121" && event.StationSeq <= 5 {
			t.Fatalf("found stale predicted entry: trip=%s seq=%d", event.TripID, event.StationSeq)
		}
	}
}
