package processorlogic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/EdiProdan/arRIval/internal/autotrolej"
	"github.com/EdiProdan/arRIval/internal/staticdata"
)

func TestV2TrackerReplaySyntheticFixture(t *testing.T) {
	fixture := loadV2ReplayFixture(t, "testdata/v2/synthetic/fixture.json")
	store := buildV2ReplayStore(t, fixture)
	tracker := NewV2Tracker(store, V2TrackerConfig{ServiceLocation: time.UTC})

	var (
		observedCount  int
		predictedCount int
		latestByKey    = make(map[string]int64)
	)

	for _, input := range fixture.Inputs {
		observedAt, err := time.Parse(time.RFC3339Nano, input.ObservedAt)
		if err != nil {
			t.Fatalf("parse observed_at %q: %v", input.ObservedAt, err)
		}

		out := tracker.Track(V2TrackInput{
			ObservedAt: observedAt,
			Bus:        input.Bus,
		})

		observedCount += len(out.Observed)
		predictedCount += len(out.Predicted)
		for _, p := range out.Predicted {
			key := p.TripID + ":" + strconv.FormatInt(p.StationID, 10)
			latestByKey[key] = p.PredictedDelaySeconds
		}
	}

	if observedCount != 3 {
		t.Fatalf("observedCount = %d, want 3", observedCount)
	}
	if predictedCount != 6 {
		t.Fatalf("predictedCount = %d, want 6", predictedCount)
	}
	if latestByKey["1001:4"] != 116 {
		t.Fatalf("latest predicted delay for trip 1001 station 4 = %d, want 116", latestByKey["1001:4"])
	}
}

func TestV2TrackerReplayRealCaptureSmoke(t *testing.T) {
	fixture := loadV2ReplayFixture(t, "testdata/v2/real_capture/fixture.json")
	store := buildV2ReplayStore(t, fixture)
	tracker := NewV2Tracker(store, V2TrackerConfig{ServiceLocation: time.UTC})

	var (
		observedCount int
		predCount     int
		noTripSkips   int
	)

	for _, input := range fixture.Inputs {
		observedAt, err := time.Parse(time.RFC3339Nano, input.ObservedAt)
		if err != nil {
			t.Fatalf("parse observed_at %q: %v", input.ObservedAt, err)
		}

		out := tracker.Track(V2TrackInput{
			ObservedAt: observedAt,
			Bus:        input.Bus,
		})
		if out.SkipReason == V2SkipReasonNoTripForVoznjaBusID {
			noTripSkips++
		}
		observedCount += len(out.Observed)
		predCount += len(out.Predicted)
	}

	if noTripSkips != 0 {
		t.Fatalf("no_trip_for_voznja_bus_id skips = %d, want 0", noTripSkips)
	}
	if observedCount == 0 {
		t.Fatalf("expected at least one observed event from real capture fixture")
	}
	if predCount == 0 {
		t.Fatalf("expected at least one predicted event from real capture fixture")
	}
}

type v2ReplayFixture struct {
	Stations  []staticdata.Station          `json:"stations"`
	LinePaths []staticdata.LinePathRow      `json:"line_paths"`
	Timetable []staticdata.TimetableStopRow `json:"timetable"`
	Inputs    []v2ReplayInput               `json:"inputs"`
}

type v2ReplayInput struct {
	ObservedAt string             `json:"observed_at"`
	Bus        autotrolej.LiveBus `json:"bus"`
}

func loadV2ReplayFixture(t *testing.T, relativePath string) v2ReplayFixture {
	t.Helper()

	raw, err := os.ReadFile(filepath.Clean(relativePath))
	if err != nil {
		t.Fatalf("read fixture %s: %v", relativePath, err)
	}

	var fixture v2ReplayFixture
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("unmarshal fixture %s: %v", relativePath, err)
	}

	return fixture
}

func buildV2ReplayStore(t *testing.T, fixture v2ReplayFixture) *staticdata.Store {
	t.Helper()

	linePaths := fixture.LinePaths
	if len(linePaths) == 0 {
		linePaths = buildLinePathsFromTimetable(fixture.Timetable)
	}

	dir := t.TempDir()
	writeJSONFixture(t, filepath.Join(dir, "stanice.json"), fixture.Stations)
	writeJSONFixture(t, filepath.Join(dir, "linije.json"), linePaths)
	writeJSONFixture(t, filepath.Join(dir, "voznired_dnevni.json"), fixture.Timetable)

	store, err := staticdata.LoadFromDir(dir)
	if err != nil {
		t.Fatalf("load fixture static store: %v", err)
	}

	return store
}
