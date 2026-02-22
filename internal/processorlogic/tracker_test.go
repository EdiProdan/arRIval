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

func TestTrackerMissingInputFields(t *testing.T) {
	store := buildTestStore(t, baseStations(), baseTimetable())
	tracker := NewTracker(store, TrackerConfig{ServiceLocation: time.UTC})

	observedAt := time.Date(2026, 2, 21, 12, 0, 0, 0, time.UTC)
	lon := 14.4400
	lat := 45.3300
	voznjaBusID := 1001

	out := tracker.Track(TrackInput{
		ObservedAt: observedAt,
		Bus: autotrolej.LiveBus{
			Lon: &lon,
			Lat: &lat,
		},
	})
	if out.SkipReason != SkipReasonMissingVoznjaBusID {
		t.Fatalf("SkipReason = %q, want %q", out.SkipReason, SkipReasonMissingVoznjaBusID)
	}

	out = tracker.Track(TrackInput{
		ObservedAt: observedAt,
		Bus: autotrolej.LiveBus{
			VoznjaBusID: &voznjaBusID,
		},
	})
	if out.SkipReason != SkipReasonMissingCoordinates {
		t.Fatalf("SkipReason = %q, want %q", out.SkipReason, SkipReasonMissingCoordinates)
	}
}

func TestTrackerLockResolution(t *testing.T) {
	store := buildTestStore(t, baseStations(), baseTimetable())
	tracker := NewTracker(store, TrackerConfig{ServiceLocation: time.UTC})

	observedAt := time.Date(2026, 2, 21, 12, 1, 0, 0, time.UTC)
	lon := 14.4400
	lat := 45.3300
	missingTripBusID := 9999

	out := tracker.Track(TrackInput{
		ObservedAt: observedAt,
		Bus: autotrolej.LiveBus{
			VoznjaBusID: &missingTripBusID,
			Lon:         &lon,
			Lat:         &lat,
		},
	})
	if out.SkipReason != SkipReasonNoTripForVoznjaBusID {
		t.Fatalf("SkipReason = %q, want %q", out.SkipReason, SkipReasonNoTripForVoznjaBusID)
	}

	voznjaBusID := 1001
	out = tracker.Track(TrackInput{
		ObservedAt: observedAt,
		Bus: autotrolej.LiveBus{
			VoznjaBusID: &voznjaBusID,
			Lon:         &lon,
			Lat:         &lat,
		},
	})
	if out.SkipReason != SkipReasonNone {
		t.Fatalf("SkipReason = %q, want none", out.SkipReason)
	}
	if len(out.Observed) != 1 {
		t.Fatalf("observed len = %d, want 1", len(out.Observed))
	}
	if len(out.Predicted) != 3 {
		t.Fatalf("predicted len = %d, want 3", len(out.Predicted))
	}
	if out.Observed[0].TripID != "1001" {
		t.Fatalf("TripID = %q, want %q", out.Observed[0].TripID, "1001")
	}
}

func TestTrackerCoordinateResolutionFallbackAndMissing(t *testing.T) {
	stations := baseStations()
	timetable := baseTimetable()
	timetable[0].GpsX = nil
	timetable[0].GpsY = nil

	store := buildTestStore(t, stations, timetable)
	tracker := NewTracker(store, TrackerConfig{ServiceLocation: time.UTC})

	observedAt := time.Date(2026, 2, 21, 12, 1, 0, 0, time.UTC)
	voznjaBusID := 1001
	lon := 14.4400
	lat := 45.3300

	out := tracker.Track(TrackInput{
		ObservedAt: observedAt,
		Bus: autotrolej.LiveBus{
			VoznjaBusID: &voznjaBusID,
			Lon:         &lon,
			Lat:         &lat,
		},
	})
	if out.SkipReason != SkipReasonNone {
		t.Fatalf("SkipReason = %q, want none", out.SkipReason)
	}
	if len(out.Observed) != 1 || out.Observed[0].StationSeq != 1 {
		t.Fatalf("fallback match failed, observed=%+v", out.Observed)
	}

	for i := range stations {
		stations[i].GpsX = nil
		stations[i].GpsY = nil
	}
	for i := range timetable {
		timetable[i].GpsX = nil
		timetable[i].GpsY = nil
	}
	storeMissing := buildTestStore(t, stations, timetable)
	trackerMissing := NewTracker(storeMissing, TrackerConfig{ServiceLocation: time.UTC})

	out = trackerMissing.Track(TrackInput{
		ObservedAt: observedAt,
		Bus: autotrolej.LiveBus{
			VoznjaBusID: &voznjaBusID,
			Lon:         &lon,
			Lat:         &lat,
		},
	})
	if out.SkipReason != SkipReasonStopMissingCoordinates {
		t.Fatalf("SkipReason = %q, want %q", out.SkipReason, SkipReasonStopMissingCoordinates)
	}
}

func TestTrackerNoStopWithinMatchRadius(t *testing.T) {
	store := buildTestStore(t, baseStations(), baseTimetable())
	tracker := NewTracker(store, TrackerConfig{ServiceLocation: time.UTC})

	observedAt := time.Date(2026, 2, 21, 12, 1, 0, 0, time.UTC)
	voznjaBusID := 1001
	lon := 0.0
	lat := 0.0

	out := tracker.Track(TrackInput{
		ObservedAt: observedAt,
		Bus: autotrolej.LiveBus{
			VoznjaBusID: &voznjaBusID,
			Lon:         &lon,
			Lat:         &lat,
		},
	})
	if out.SkipReason != SkipReasonNoStopWithinMatchRadius {
		t.Fatalf("SkipReason = %q, want %q", out.SkipReason, SkipReasonNoStopWithinMatchRadius)
	}
}

func TestTrackerProgressionAndInvalidReset(t *testing.T) {
	store := buildTestStore(t, baseStations(), baseTimetable())
	tracker := NewTracker(store, TrackerConfig{ServiceLocation: time.UTC})

	voznjaBusID := 1001
	t0 := time.Date(2026, 2, 21, 12, 0, 30, 0, time.UTC)

	out := tracker.Track(inputForStop(voznjaBusID, 14.4400, 45.3300, t0))
	if len(out.Observed) != 1 || out.Observed[0].StationSeq != 1 {
		t.Fatalf("first progression failed: %+v", out)
	}

	out = tracker.Track(inputForStop(voznjaBusID, 14.4410, 45.3310, t0.Add(5*time.Minute)))
	if len(out.Observed) != 1 || out.Observed[0].StationSeq != 2 {
		t.Fatalf("second progression failed: %+v", out)
	}

	out = tracker.Track(inputForStop(voznjaBusID, 14.4410, 45.3310, t0.Add(5*time.Minute+10*time.Second)))
	if out.SkipReason != SkipReasonDuplicateStationSeq {
		t.Fatalf("SkipReason = %q, want %q", out.SkipReason, SkipReasonDuplicateStationSeq)
	}

	out = tracker.Track(inputForStop(voznjaBusID, 14.4400, 45.3300, t0.Add(5*time.Minute+20*time.Second)))
	if out.SkipReason != SkipReasonBackwardStationSeq {
		t.Fatalf("SkipReason = %q, want %q", out.SkipReason, SkipReasonBackwardStationSeq)
	}
	out = tracker.Track(inputForStop(voznjaBusID, 14.4400, 45.3300, t0.Add(5*time.Minute+30*time.Second)))
	if out.SkipReason != SkipReasonBackwardStationSeq {
		t.Fatalf("SkipReason = %q, want %q", out.SkipReason, SkipReasonBackwardStationSeq)
	}
	out = tracker.Track(inputForStop(voznjaBusID, 14.4400, 45.3300, t0.Add(5*time.Minute+40*time.Second)))
	if out.SkipReason != SkipReasonResetAfterInvalidProgress {
		t.Fatalf("SkipReason = %q, want %q", out.SkipReason, SkipReasonResetAfterInvalidProgress)
	}

	out = tracker.Track(inputForStop(voznjaBusID, 14.4400, 45.3300, t0.Add(5*time.Minute+50*time.Second)))
	if out.SkipReason != SkipReasonNone || len(out.Observed) != 1 || out.Observed[0].StationSeq != 1 {
		t.Fatalf("relock after reset failed: %+v", out)
	}
}

func TestTrackerStaleResetBoundary(t *testing.T) {
	store := buildTestStore(t, baseStations(), baseTimetable())
	voznjaBusID := 1001
	t0 := time.Date(2026, 2, 21, 12, 0, 10, 0, time.UTC)

	trackerNoReset := NewTracker(store, TrackerConfig{
		ServiceLocation: time.UTC,
		StaleAfter:      15 * time.Minute,
	})
	out := trackerNoReset.Track(inputForStop(voznjaBusID, 14.4400, 45.3300, t0))
	if len(out.Observed) != 1 {
		t.Fatalf("initial progress failed: %+v", out)
	}
	out = trackerNoReset.Track(inputForStop(voznjaBusID, 14.4410, 45.3310, t0.Add(14*time.Minute+59*time.Second)))
	if out.SkipReason != SkipReasonNone || len(out.Observed) != 1 {
		t.Fatalf("14m59s should not reset stale state: %+v", out)
	}

	trackerReset := NewTracker(store, TrackerConfig{
		ServiceLocation: time.UTC,
		StaleAfter:      15 * time.Minute,
	})
	out = trackerReset.Track(inputForStop(voznjaBusID, 14.4400, 45.3300, t0))
	if len(out.Observed) != 1 {
		t.Fatalf("initial progress failed: %+v", out)
	}
	out = trackerReset.Track(inputForStop(voznjaBusID, 14.4410, 45.3310, t0.Add(15*time.Minute)))
	if out.SkipReason != SkipReasonResetAfterStaleState {
		t.Fatalf("SkipReason = %q, want %q", out.SkipReason, SkipReasonResetAfterStaleState)
	}
	out = trackerReset.Track(inputForStop(voznjaBusID, 14.4410, 45.3310, t0.Add(15*time.Minute+time.Second)))
	if out.SkipReason != SkipReasonNone || len(out.Observed) != 1 {
		t.Fatalf("reacquire after stale reset failed: %+v", out)
	}
}

func TestTrackerEMAAndResetBehavior(t *testing.T) {
	store := buildTestStore(t, baseStations(), baseTimetable())
	voznjaBusID := 1001

	tracker := NewTracker(store, TrackerConfig{ServiceLocation: time.UTC})

	out := tracker.Track(inputForStop(voznjaBusID, 14.4400, 45.3300, time.Date(2026, 2, 21, 12, 1, 0, 0, time.UTC)))
	if len(out.Predicted) == 0 || out.Predicted[0].PredictedDelaySeconds != 60 {
		t.Fatalf("first EMA value mismatch: %+v", out.Predicted)
	}

	out = tracker.Track(inputForStop(voznjaBusID, 14.4410, 45.3310, time.Date(2026, 2, 21, 12, 15, 0, 0, time.UTC)))
	if len(out.Predicted) != 2 {
		t.Fatalf("predicted len = %d, want 2", len(out.Predicted))
	}
	// EMA with alpha 0.35: 0.35*600 + 0.65*60 = 249
	if out.Predicted[0].PredictedDelaySeconds != 249 {
		t.Fatalf("PredictedDelaySeconds = %d, want %d", out.Predicted[0].PredictedDelaySeconds, 249)
	}

	trackerReset := NewTracker(store, TrackerConfig{
		ServiceLocation: time.UTC,
		StaleAfter:      5 * time.Minute,
	})
	out = trackerReset.Track(inputForStop(voznjaBusID, 14.4400, 45.3300, time.Date(2026, 2, 21, 12, 1, 0, 0, time.UTC)))
	if len(out.Observed) != 1 {
		t.Fatalf("initial progression failed: %+v", out)
	}
	out = trackerReset.Track(inputForStop(voznjaBusID, 14.4410, 45.3310, time.Date(2026, 2, 21, 12, 7, 0, 0, time.UTC)))
	if out.SkipReason != SkipReasonResetAfterStaleState {
		t.Fatalf("SkipReason = %q, want %q", out.SkipReason, SkipReasonResetAfterStaleState)
	}
	out = trackerReset.Track(inputForStop(voznjaBusID, 14.4400, 45.3300, time.Date(2026, 2, 21, 12, 7, 10, 0, time.UTC)))
	if out.SkipReason != SkipReasonNone || len(out.Predicted) == 0 {
		t.Fatalf("reacquire after stale reset failed: %+v", out)
	}
	if out.Predicted[0].PredictedDelaySeconds != out.Observed[0].DelaySeconds {
		t.Fatalf("EMA should reset on relock: predicted=%d observed=%d", out.Predicted[0].PredictedDelaySeconds, out.Observed[0].DelaySeconds)
	}
}

func TestTrackerMidnightCrossing(t *testing.T) {
	stations := []staticdata.Station{
		{StanicaID: 11, Naziv: "Night A", GpsX: float64Ptr(14.5000), GpsY: float64Ptr(45.3200)},
		{StanicaID: 12, Naziv: "Night B", GpsX: float64Ptr(14.5005), GpsY: float64Ptr(45.3205)},
		{StanicaID: 13, Naziv: "Night C", GpsX: float64Ptr(14.5010), GpsY: float64Ptr(45.3210)},
	}
	timetable := []staticdata.TimetableStopRow{
		{PolazakID: "2001", StanicaID: 11, LinVarID: "n-1", Polazak: "23:59:00", RedniBrojStanice: 1, BrojLinije: "N1", Naziv: "Night A", GpsX: float64Ptr(14.5000), GpsY: float64Ptr(45.3200)},
		{PolazakID: "2001", StanicaID: 12, LinVarID: "n-1", Polazak: "00:01:00", RedniBrojStanice: 2, BrojLinije: "N1", Naziv: "Night B", GpsX: float64Ptr(14.5005), GpsY: float64Ptr(45.3205)},
		{PolazakID: "2001", StanicaID: 13, LinVarID: "n-1", Polazak: "00:03:00", RedniBrojStanice: 3, BrojLinije: "N1", Naziv: "Night C", GpsX: float64Ptr(14.5010), GpsY: float64Ptr(45.3210)},
	}

	store := buildTestStore(t, stations, timetable)
	tracker := NewTracker(store, TrackerConfig{ServiceLocation: time.UTC})

	voznjaBusID := 2001
	out := tracker.Track(inputForStop(voznjaBusID, 14.5000, 45.3200, time.Date(2026, 2, 22, 0, 0, 0, 0, time.UTC)))
	if out.SkipReason != SkipReasonNone || len(out.Observed) != 1 || len(out.Predicted) < 1 {
		t.Fatalf("unexpected output: %+v", out)
	}

	observedScheduled, err := time.Parse(time.RFC3339Nano, out.Observed[0].ScheduledTime)
	if err != nil {
		t.Fatalf("parse observed scheduled time: %v", err)
	}
	if !observedScheduled.Equal(time.Date(2026, 2, 21, 23, 59, 0, 0, time.UTC)) {
		t.Fatalf("observed scheduled = %s, want 2026-02-21T23:59:00Z", observedScheduled)
	}

	predictedScheduled, err := time.Parse(time.RFC3339Nano, out.Predicted[0].ScheduledTime)
	if err != nil {
		t.Fatalf("parse predicted scheduled time: %v", err)
	}
	if !predictedScheduled.Equal(time.Date(2026, 2, 22, 0, 1, 0, 0, time.UTC)) {
		t.Fatalf("predicted scheduled = %s, want 2026-02-22T00:01:00Z", predictedScheduled)
	}
}

func inputForStop(voznjaBusID int, lon, lat float64, observedAt time.Time) TrackInput {
	return TrackInput{
		ObservedAt: observedAt,
		Bus: autotrolej.LiveBus{
			VoznjaBusID: intPtr(voznjaBusID),
			Lon:         float64Ptr(lon),
			Lat:         float64Ptr(lat),
		},
	}
}

func baseStations() []staticdata.Station {
	return []staticdata.Station{
		{StanicaID: 1, Naziv: "Stop 1", GpsX: float64Ptr(14.4400), GpsY: float64Ptr(45.3300)},
		{StanicaID: 2, Naziv: "Stop 2", GpsX: float64Ptr(14.4410), GpsY: float64Ptr(45.3310)},
		{StanicaID: 3, Naziv: "Stop 3", GpsX: float64Ptr(14.4420), GpsY: float64Ptr(45.3320)},
		{StanicaID: 4, Naziv: "Stop 4", GpsX: float64Ptr(14.4430), GpsY: float64Ptr(45.3330)},
	}
}

func baseTimetable() []staticdata.TimetableStopRow {
	return []staticdata.TimetableStopRow{
		{ID: 1, PolazakID: "1001", StanicaID: 1, LinVarID: "lv-1", Polazak: "12:00:00", RedniBrojStanice: 1, BrojLinije: "2A", Naziv: "Stop 1", GpsX: float64Ptr(14.4400), GpsY: float64Ptr(45.3300)},
		{ID: 2, PolazakID: "1001", StanicaID: 2, LinVarID: "lv-1", Polazak: "12:05:00", RedniBrojStanice: 2, BrojLinije: "2A", Naziv: "Stop 2", GpsX: float64Ptr(14.4410), GpsY: float64Ptr(45.3310)},
		{ID: 3, PolazakID: "1001", StanicaID: 3, LinVarID: "lv-1", Polazak: "12:10:00", RedniBrojStanice: 3, BrojLinije: "2A", Naziv: "Stop 3", GpsX: float64Ptr(14.4420), GpsY: float64Ptr(45.3320)},
		{ID: 4, PolazakID: "1001", StanicaID: 4, LinVarID: "lv-1", Polazak: "12:15:00", RedniBrojStanice: 4, BrojLinije: "2A", Naziv: "Stop 4", GpsX: float64Ptr(14.4430), GpsY: float64Ptr(45.3330)},
	}
}

func buildTestStore(t *testing.T, stations []staticdata.Station, timetable []staticdata.TimetableStopRow) *staticdata.Store {
	t.Helper()

	linePaths := buildLinePathsFromTimetable(timetable)

	dir := t.TempDir()
	writeJSONFixture(t, filepath.Join(dir, "stanice.json"), stations)
	writeJSONFixture(t, filepath.Join(dir, "linije.json"), linePaths)
	writeJSONFixture(t, filepath.Join(dir, "voznired_dnevni.json"), timetable)

	store, err := staticdata.LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}

	return store
}

func writeJSONFixture(t *testing.T, path string, value any) {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal fixture %s: %v", path, err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}

func buildLinePathsFromTimetable(rows []staticdata.TimetableStopRow) []staticdata.LinePathRow {
	result := make([]staticdata.LinePathRow, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))

	for i, row := range rows {
		key := row.LinVarID + ":" + strconv.Itoa(row.StanicaID) + ":" + strconv.Itoa(row.RedniBrojStanice)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, staticdata.LinePathRow{
			ID:                   i + 1,
			LinVarID:             row.LinVarID,
			BrojLinije:           row.BrojLinije,
			NazivVarijanteLinije: row.NazivVarijanteLinije,
			Smjer:                row.Smjer,
			StanicaID:            row.StanicaID,
			RedniBrojStanice:     row.RedniBrojStanice,
			Varijanta:            row.Varijanta,
		})
	}

	return result
}

func float64Ptr(value float64) *float64 {
	v := value
	return &v
}

func intPtr(value int) *int {
	v := value
	return &v
}
