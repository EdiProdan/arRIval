package processorlogic

import (
	"testing"
	"time"
)

func TestParseScheduleTimeLocalAlignedMidnightBoundary(t *testing.T) {
	observed := time.Date(2026, 2, 22, 0, 1, 0, 0, time.UTC)

	scheduled, err := parseScheduleTimeLocalAligned("23:59:00", observed, time.UTC)
	if err != nil {
		t.Fatalf("parseScheduleTimeLocalAligned: %v", err)
	}

	expected := time.Date(2026, 2, 21, 23, 59, 0, 0, time.UTC)
	if !scheduled.Equal(expected) {
		t.Fatalf("scheduled = %s, want %s", scheduled, expected)
	}
}

func TestHaversineMetersZeroDistance(t *testing.T) {
	distance := haversineMeters(45.3300, 14.4400, 45.3300, 14.4400)
	if distance != 0 {
		t.Fatalf("distance = %f, want 0", distance)
	}
}

func TestIntPtrToInt64Ptr(t *testing.T) {
	if got := intPtrToInt64Ptr(nil); got != nil {
		t.Fatalf("intPtrToInt64Ptr(nil) = %v, want nil", got)
	}

	value := 42
	got := intPtrToInt64Ptr(&value)
	if got == nil || *got != 42 {
		t.Fatalf("intPtrToInt64Ptr(&42) = %v, want pointer to 42", got)
	}
}
