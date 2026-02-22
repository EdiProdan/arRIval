package contracts

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestObservedDelayMarshalOmitsNilGBR(t *testing.T) {
	event := ObservedDelay{
		TripID:         "trip-1",
		VoznjaBusID:    101,
		LinVarID:       "2A-1",
		BrojLinije:     "2A",
		StationID:      410,
		StationName:    "Delta",
		StationSeq:     7,
		ScheduledTime:  "2026-02-21T10:12:00Z",
		ObservedTime:   "2026-02-21T10:16:30Z",
		DelaySeconds:   270,
		DistanceM:      28.4,
		TrackerVersion: "current",
	}

	payload, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal observed delay: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal observed delay map: %v", err)
	}

	if _, ok := decoded["gbr"]; ok {
		t.Fatalf("expected gbr to be omitted when nil")
	}
}

func TestObservedDelayRoundTripDelayValues(t *testing.T) {
	delays := []int64{-120, 0, 540}
	for _, delaySeconds := range delays {
		t.Run(fmt.Sprintf("delay_seconds_%d", delaySeconds), func(t *testing.T) {
			gbr := int64(77)
			in := ObservedDelay{
				TripID:         "trip-2",
				VoznjaBusID:    202,
				GBR:            &gbr,
				LinVarID:       "8-2",
				BrojLinije:     "8",
				StationID:      115,
				StationName:    "Trsat",
				StationSeq:     3,
				ScheduledTime:  "2026-02-21T11:00:00Z",
				ObservedTime:   "2026-02-21T11:00:00Z",
				DelaySeconds:   delaySeconds,
				DistanceM:      34.1,
				TrackerVersion: "current",
			}

			payload, err := json.Marshal(in)
			if err != nil {
				t.Fatalf("marshal observed delay: %v", err)
			}

			var out ObservedDelay
			if err := json.Unmarshal(payload, &out); err != nil {
				t.Fatalf("unmarshal observed delay: %v", err)
			}

			if out.DelaySeconds != delaySeconds {
				t.Fatalf("DelaySeconds = %d, want %d", out.DelaySeconds, delaySeconds)
			}
			if out.GBR == nil || *out.GBR != gbr {
				t.Fatalf("GBR not preserved after round trip")
			}
		})
	}
}

func TestPredictedDelayRoundTrip(t *testing.T) {
	in := PredictedDelay{
		TripID:                "trip-3",
		VoznjaBusID:           303,
		LinVarID:              "6-1",
		BrojLinije:            "6",
		StationID:             220,
		StationName:           "Zamet",
		StationSeq:            12,
		ScheduledTime:         "2026-02-21T12:10:00Z",
		PredictedTime:         "2026-02-21T12:13:20Z",
		PredictedDelaySeconds: 200,
		GeneratedAt:           "2026-02-21T12:00:00Z",
		TrackerVersion:        "current",
	}

	payload, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal predicted delay: %v", err)
	}

	var out PredictedDelay
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("unmarshal predicted delay: %v", err)
	}

	if out != in {
		t.Fatalf("predicted delay mismatch after round trip: got %+v want %+v", out, in)
	}
}
