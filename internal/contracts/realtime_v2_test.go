package contracts

import (
	"encoding/json"
	"testing"
)

func TestRealtimeSnapshotV2JSONShape(t *testing.T) {
	snapshot := RealtimeSnapshotV2{
		GeneratedAt: "2026-02-21T12:00:00Z",
		Positions: []RealtimePosition{
			{
				Key:        "voznja_bus_id:101",
				Lon:        14.4,
				Lat:        45.3,
				ObservedAt: "2026-02-21T12:00:00Z",
			},
		},
		ObservedDelays: []ObservedDelayV2{
			{
				TripID:         "trip-1",
				VoznjaBusID:    101,
				LinVarID:       "2A-1",
				BrojLinije:     "2A",
				StationID:      410,
				StationName:    "Delta",
				StationSeq:     7,
				ScheduledTime:  "2026-02-21T12:05:00Z",
				ObservedTime:   "2026-02-21T12:06:30Z",
				DelaySeconds:   90,
				DistanceM:      16.8,
				TrackerVersion: "v2",
			},
		},
		PredictedDelays: []PredictedDelayV2{
			{
				TripID:                "trip-1",
				VoznjaBusID:           101,
				LinVarID:              "2A-1",
				BrojLinije:            "2A",
				StationID:             415,
				StationName:           "Centar",
				StationSeq:            8,
				ScheduledTime:         "2026-02-21T12:10:00Z",
				PredictedTime:         "2026-02-21T12:11:30Z",
				PredictedDelaySeconds: 90,
				GeneratedAt:           "2026-02-21T12:00:00Z",
				TrackerVersion:        "v2",
			},
		},
		Meta: RealtimeSnapshotMetaV2{
			PositionsCount:       1,
			ObservedDelaysCount:  1,
			PredictedDelaysCount: 1,
		},
	}

	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("unmarshal snapshot map: %v", err)
	}

	if _, ok := decoded["observed_delays"]; !ok {
		t.Fatalf("snapshot missing observed_delays field")
	}
	if _, ok := decoded["predicted_delays"]; !ok {
		t.Fatalf("snapshot missing predicted_delays field")
	}
	if _, ok := decoded["delays"]; ok {
		t.Fatalf("snapshot should not include V1 delays field")
	}

	metaRaw, ok := decoded["meta"].(map[string]any)
	if !ok {
		t.Fatalf("snapshot meta not an object")
	}
	if _, ok := metaRaw["observed_delays_count"]; !ok {
		t.Fatalf("snapshot meta missing observed_delays_count")
	}
	if _, ok := metaRaw["predicted_delays_count"]; !ok {
		t.Fatalf("snapshot meta missing predicted_delays_count")
	}
}

func TestRealtimeV2WSUpdatePayloads(t *testing.T) {
	observed := ObservedDelayV2{
		TripID:         "trip-observed",
		VoznjaBusID:    5001,
		LinVarID:       "1A",
		BrojLinije:     "1",
		StationID:      1,
		StationName:    "Main",
		StationSeq:     1,
		ScheduledTime:  "2026-02-21T12:00:00Z",
		ObservedTime:   "2026-02-21T12:02:00Z",
		DelaySeconds:   120,
		DistanceM:      11.5,
		TrackerVersion: "v2",
	}
	predicted := PredictedDelayV2{
		TripID:                "trip-predicted",
		VoznjaBusID:           5001,
		LinVarID:              "1A",
		BrojLinije:            "1",
		StationID:             2,
		StationName:           "West",
		StationSeq:            2,
		ScheduledTime:         "2026-02-21T12:05:00Z",
		PredictedTime:         "2026-02-21T12:07:00Z",
		PredictedDelaySeconds: 120,
		GeneratedAt:           "2026-02-21T12:00:00Z",
		TrackerVersion:        "v2",
	}

	tests := []struct {
		name       string
		eventType  string
		data       any
		payloadKey string
	}{
		{
			name:      "observed",
			eventType: RealtimeEventDelayObservedUpdateV2,
			data: RealtimeObservedDelayUpdate{
				ObservedDelay: observed,
			},
			payloadKey: "observed_delay",
		},
		{
			name:      "prediction",
			eventType: RealtimeEventDelayPredictionUpdateV2,
			data: RealtimePredictedDelayUpdate{
				PredictedDelay: predicted,
			},
			payloadKey: "predicted_delay",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envelope := RealtimeEnvelope{
				Type: tt.eventType,
				TS:   "2026-02-21T12:00:00Z",
				Data: tt.data,
			}

			payload, err := json.Marshal(envelope)
			if err != nil {
				t.Fatalf("marshal envelope: %v", err)
			}

			var decoded map[string]any
			if err := json.Unmarshal(payload, &decoded); err != nil {
				t.Fatalf("unmarshal envelope map: %v", err)
			}

			gotType, ok := decoded["type"].(string)
			if !ok {
				t.Fatalf("missing string type field")
			}
			if gotType != tt.eventType {
				t.Fatalf("type = %q, want %q", gotType, tt.eventType)
			}

			dataRaw, ok := decoded["data"].(map[string]any)
			if !ok {
				t.Fatalf("data is not an object")
			}
			if _, ok := dataRaw[tt.payloadKey]; !ok {
				t.Fatalf("payload missing %q", tt.payloadKey)
			}
		})
	}
}
