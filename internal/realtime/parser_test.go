package realtime

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/EdiProdan/arRIval/internal/autotrolej"
	"github.com/EdiProdan/arRIval/internal/contracts"
)

func TestParsePositionsRecord(t *testing.T) {
	lon1 := 14.4
	lat1 := 45.3
	gbr1 := 10
	voznjaBusID1 := 123

	lon2 := 14.5
	lat2 := 45.4
	gbr2 := 11

	response := autotrolej.AutobusiResponse{
		Msg: "ok",
		Err: false,
		Res: []autotrolej.LiveBus{
			{
				Lon:         &lon1,
				Lat:         &lat1,
				GBR:         &gbr1,
				VoznjaBusID: &voznjaBusID1,
			},
			{
				Lon: &lon2,
				Lat: &lat2,
				GBR: &gbr2,
			},
			{
				GBR: &gbr2,
				Lat: &lat2,
			},
			{
				Lon: &lon2,
				Lat: &lat2,
			},
		},
	}

	payload, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	observedAt := time.Date(2026, 2, 18, 10, 0, 0, 123000000, time.UTC)
	positions, invalid, err := ParsePositionsRecord(payload, observedAt)
	if err != nil {
		t.Fatalf("ParsePositionsRecord error: %v", err)
	}

	if invalid != 2 {
		t.Fatalf("invalid = %d, want 2", invalid)
	}

	if len(positions) != 2 {
		t.Fatalf("len(positions) = %d, want 2", len(positions))
	}

	if positions[0].Key != "voznja_bus_id:123" {
		t.Fatalf("positions[0].Key = %q, want %q", positions[0].Key, "voznja_bus_id:123")
	}
	if positions[1].Key != "gbr:11" {
		t.Fatalf("positions[1].Key = %q, want %q", positions[1].Key, "gbr:11")
	}
	if positions[0].ObservedAt != "2026-02-18T10:00:00.123Z" {
		t.Fatalf("positions[0].ObservedAt = %q, want %q", positions[0].ObservedAt, "2026-02-18T10:00:00.123Z")
	}
}

func TestParseObservedDelayRecord(t *testing.T) {
	payload, err := json.Marshal(contracts.ObservedDelay{
		TripID:         "trip-121",
		VoznjaBusID:    121,
		LinVarID:       "L1A",
		BrojLinije:     "1",
		StationID:      1001,
		StationName:    "Main",
		StationSeq:     5,
		ScheduledTime:  "2026-02-18T11:05:00Z",
		ObservedTime:   "2026-02-18T11:10:00Z",
		DelaySeconds:   300,
		DistanceM:      15,
		TrackerVersion: "current",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	event, err := ParseObservedDelayRecord(payload)
	if err != nil {
		t.Fatalf("ParseObservedDelayRecord error: %v", err)
	}
	if event.TripID != "trip-121" || event.StationSeq != 5 || event.DelaySeconds != 300 {
		t.Fatalf("parsed observed event = %+v, want trip-121 seq=5 delay=300", event)
	}
}

func TestParsePredictedDelayRecord(t *testing.T) {
	payload, err := json.Marshal(contracts.PredictedDelay{
		TripID:                "trip-121",
		VoznjaBusID:           121,
		LinVarID:              "L1A",
		BrojLinije:            "1",
		StationID:             1002,
		StationName:           "Next",
		StationSeq:            6,
		ScheduledTime:         "2026-02-18T11:12:00Z",
		PredictedTime:         "2026-02-18T11:17:00Z",
		PredictedDelaySeconds: 300,
		GeneratedAt:           "2026-02-18T11:10:00Z",
		TrackerVersion:        "current",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	event, err := ParsePredictedDelayRecord(payload)
	if err != nil {
		t.Fatalf("ParsePredictedDelayRecord error: %v", err)
	}
	if event.TripID != "trip-121" || event.StationSeq != 6 || event.PredictedDelaySeconds != 300 {
		t.Fatalf("parsed predicted event = %+v, want trip-121 seq=6 delay=300", event)
	}
}

func TestParseObservedAndPredictedDelayRecordInvalidJSON(t *testing.T) {
	if _, err := ParseObservedDelayRecord([]byte("{")); err == nil {
		t.Fatalf("ParseObservedDelayRecord error = nil, want non-nil")
	}
	if _, err := ParsePredictedDelayRecord([]byte("{")); err == nil {
		t.Fatalf("ParsePredictedDelayRecord error = nil, want non-nil")
	}
}
