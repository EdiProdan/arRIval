package aggregatorlogic

import (
	"strings"
	"testing"
	"time"

	"github.com/EdiProdan/arRIval/internal/contracts"
)

func TestConsumeEventInvalidObservedTime(t *testing.T) {
	state := make(map[AggregateKey]*AggregateBucket)
	err := ConsumeEvent(state, contracts.ObservedDelayV2{
		BrojLinije:   "2A",
		ObservedTime: "invalid",
		DelaySeconds: 10,
	}, 300)
	if err == nil {
		t.Fatalf("ConsumeEvent error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "parse observed_time") {
		t.Fatalf("ConsumeEvent error = %q, want parse observed_time", err.Error())
	}
}

func TestConsumeEventBlankRouteFallsBackToUnknown(t *testing.T) {
	state := make(map[AggregateKey]*AggregateBucket)
	err := ConsumeEvent(state, contracts.ObservedDelayV2{
		BrojLinije:   " ",
		ObservedTime: "2025-01-02T10:15:00Z",
		DelaySeconds: 120,
	}, 300)
	if err != nil {
		t.Fatalf("ConsumeEvent: %v", err)
	}

	rowsByDate := ComputeRowsByDate(state, time.Date(2025, 1, 2, 10, 30, 0, 0, time.UTC))
	rows := rowsByDate["2025-01-02"]
	if len(rows) != 1 {
		t.Fatalf("rows len = %d, want %d", len(rows), 1)
	}
	if rows[0].Route != "unknown" {
		t.Fatalf("Route = %q, want %q", rows[0].Route, "unknown")
	}
	if rows[0].HourStartUTC != "2025-01-02T10:00:00Z" {
		t.Fatalf("HourStartUTC = %q, want %q", rows[0].HourStartUTC, "2025-01-02T10:00:00Z")
	}
}

func TestComputeRowsByDateStats(t *testing.T) {
	state := make(map[AggregateKey]*AggregateBucket)
	events := []contracts.ObservedDelayV2{
		{BrojLinije: "2A", ObservedTime: "2025-01-02T10:01:00Z", DelaySeconds: -120},
		{BrojLinije: "2A", ObservedTime: "2025-01-02T10:02:00Z", DelaySeconds: 0},
		{BrojLinije: "2A", ObservedTime: "2025-01-02T10:03:00Z", DelaySeconds: 60},
		{BrojLinije: "2A", ObservedTime: "2025-01-02T10:04:00Z", DelaySeconds: 300},
	}
	for _, event := range events {
		if err := ConsumeEvent(state, event, 60); err != nil {
			t.Fatalf("ConsumeEvent: %v", err)
		}
	}

	rowsByDate := ComputeRowsByDate(state, time.Date(2025, 1, 2, 11, 0, 0, 123456000, time.UTC))
	rows := rowsByDate["2025-01-02"]
	if len(rows) != 1 {
		t.Fatalf("rows len = %d, want %d", len(rows), 1)
	}

	row := rows[0]
	if row.SampleCount != 4 {
		t.Fatalf("SampleCount = %d, want %d", row.SampleCount, 4)
	}
	if row.AvgDelaySeconds != 60 {
		t.Fatalf("AvgDelaySeconds = %v, want %v", row.AvgDelaySeconds, 60.0)
	}
	if row.P95DelaySeconds != 300 {
		t.Fatalf("P95DelaySeconds = %d, want %d", row.P95DelaySeconds, 300)
	}
	if row.P99DelaySeconds != 300 {
		t.Fatalf("P99DelaySeconds = %d, want %d", row.P99DelaySeconds, 300)
	}
	if row.OnTimePercent != 50 {
		t.Fatalf("OnTimePercent = %v, want %v", row.OnTimePercent, 50.0)
	}
	if row.WrittenAt != "2025-01-02T11:00:00.123456Z" {
		t.Fatalf("WrittenAt = %q, want %q", row.WrittenAt, "2025-01-02T11:00:00.123456Z")
	}
}
