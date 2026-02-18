package processorlogic

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/EdiProdan/arRIval/internal/autotrolej"
	"github.com/EdiProdan/arRIval/internal/staticdata"
)

func TestBuildDelayMissingCoordinates(t *testing.T) {
	store := testStore(t, "12:00:00")
	voznjaBusID := 123

	_, ok, reason := BuildDelay(MatchInput{
		IngestedAt: time.Date(2025, 1, 2, 12, 0, 0, 0, time.UTC),
		Bus: autotrolej.LiveBus{
			VoznjaBusID: &voznjaBusID,
		},
	}, store, Config{ServiceLocation: time.UTC})
	if ok {
		t.Fatalf("ok = true, want false")
	}
	if reason != "missing_coordinates" {
		t.Fatalf("reason = %q, want %q", reason, "missing_coordinates")
	}
}

func TestBuildDelayMissingVoznjaBusID(t *testing.T) {
	store := testStore(t, "12:00:00")
	lon := 14.44
	lat := 45.33

	_, ok, reason := BuildDelay(MatchInput{
		IngestedAt: time.Date(2025, 1, 2, 12, 0, 0, 0, time.UTC),
		Bus: autotrolej.LiveBus{
			Lon: &lon,
			Lat: &lat,
		},
	}, store, Config{ServiceLocation: time.UTC})
	if ok {
		t.Fatalf("ok = true, want false")
	}
	if reason != "missing_voznja_bus_id" {
		t.Fatalf("reason = %q, want %q", reason, "missing_voznja_bus_id")
	}
}

func TestBuildDelayHappyPath(t *testing.T) {
	store := testStore(t, "12:00:00")
	voznjaBusID := 123
	gbr := 77
	lon := 14.44
	lat := 45.33
	ingestedAt := time.Date(2025, 1, 2, 12, 1, 0, 0, time.UTC)

	result, ok, reason := BuildDelay(MatchInput{
		IngestedAt: ingestedAt,
		Bus: autotrolej.LiveBus{
			GBR:         &gbr,
			Lon:         &lon,
			Lat:         &lat,
			VoznjaBusID: &voznjaBusID,
		},
	}, store, Config{
		StationMatchMeters: 100,
		ScheduleWindow:     15 * time.Minute,
		ServiceLocation:    time.UTC,
	})
	if !ok {
		t.Fatalf("ok = false, want true (reason=%s)", reason)
	}
	if result.PolazakID != "123" {
		t.Fatalf("PolazakID = %q, want %q", result.PolazakID, "123")
	}
	if result.DelaySeconds != 60 {
		t.Fatalf("DelaySeconds = %d, want %d", result.DelaySeconds, 60)
	}
	if result.StationName != "Test Station" {
		t.Fatalf("StationName = %q, want %q", result.StationName, "Test Station")
	}
	if result.GBR == nil || *result.GBR != int64(gbr) {
		t.Fatalf("GBR = %v, want %d", result.GBR, gbr)
	}
}

func TestBuildDelayMidnightAlignmentUsesNearestDay(t *testing.T) {
	store := testStore(t, "23:59:00")
	voznjaBusID := 123
	lon := 14.44
	lat := 45.33
	ingestedAt := time.Date(2025, 1, 2, 0, 1, 0, 0, time.UTC)

	result, ok, reason := BuildDelay(MatchInput{
		IngestedAt: ingestedAt,
		Bus: autotrolej.LiveBus{
			Lon:         &lon,
			Lat:         &lat,
			VoznjaBusID: &voznjaBusID,
		},
	}, store, Config{
		StationMatchMeters: 100,
		ScheduleWindow:     15 * time.Minute,
		ServiceLocation:    time.UTC,
	})
	if !ok {
		t.Fatalf("ok = false, want true (reason=%s)", reason)
	}

	expectedScheduled := time.Date(2025, 1, 1, 23, 59, 0, 0, time.UTC)
	if !result.ScheduledTime.Equal(expectedScheduled) {
		t.Fatalf("ScheduledTime = %s, want %s", result.ScheduledTime, expectedScheduled)
	}
	if result.DelaySeconds != 120 {
		t.Fatalf("DelaySeconds = %d, want %d", result.DelaySeconds, 120)
	}
}

func testStore(t *testing.T, schedule string) *staticdata.Store {
	t.Helper()

	dir := t.TempDir()

	stations := `[{"StanicaId":1,"Naziv":"Test Station","Kratki":"TS","GpsX":14.44,"GpsY":45.33}]`
	if err := os.WriteFile(filepath.Join(dir, "stanice.json"), []byte(stations), 0o644); err != nil {
		t.Fatalf("write stanice.json: %v", err)
	}

	linePaths := `[{"Id":1,"LinVarId":"lv-1","BrojLinije":"2A","NazivVarijanteLinije":"Test","Smjer":"A","StanicaId":1,"RedniBrojStanice":1,"Varijanta":"V"}]`
	if err := os.WriteFile(filepath.Join(dir, "linije.json"), []byte(linePaths), 0o644); err != nil {
		t.Fatalf("write linije.json: %v", err)
	}

	timetable := `[{"Id":1,"PolazakId":"123","StanicaId":1,"LinVarId":"lv-1","Polazak":"` + schedule + `","RedniBrojStanice":1,"BrojLinije":"2A","Smjer":"A","Varijanta":"V","NazivVarijanteLinije":"Test","PodrucjePrometa":"Rijeka","GpsX":14.44,"GpsY":45.33,"Naziv":"Test Station"}]`
	if err := os.WriteFile(filepath.Join(dir, "voznired_dnevni.json"), []byte(timetable), 0o644); err != nil {
		t.Fatalf("write voznired_dnevni.json: %v", err)
	}

	store, err := staticdata.LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	return store
}
