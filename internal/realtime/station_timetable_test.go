package realtime

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/EdiProdan/arRIval/internal/contracts"
)

type stationFixtureRow struct {
	StanicaID int64  `json:"StanicaId"`
	Naziv     string `json:"Naziv"`
}

type timetableFixtureRow struct {
	PolazakID        string `json:"PolazakId"`
	StanicaID        int64  `json:"StanicaId"`
	BrojLinije       string `json:"BrojLinije"`
	RedniBrojStanice int64  `json:"RedniBrojStanice"`
	Polazak          string `json:"Polazak"`
	Naziv            string `json:"Naziv"`
}

func TestServerStationTimetableEndpoint_LiveAndScheduledMerge(t *testing.T) {
	now := time.Date(2026, 2, 22, 20, 0, 0, 0, time.UTC)

	stations := []stationFixtureRow{
		{StanicaID: 100, Naziv: "Target Station"},
	}
	timetable := []timetableFixtureRow{
		{PolazakID: "trip-live", StanicaID: 100, BrojLinije: "7", RedniBrojStanice: 5, Polazak: "21:30:00.0000000", Naziv: "Target Station"},
		{PolazakID: "trip-scheduled", StanicaID: 100, BrojLinije: "6", RedniBrojStanice: 3, Polazak: "21:40:00.0000000", Naziv: "Target Station"},
		{PolazakID: "trip-outside", StanicaID: 100, BrojLinije: "6", RedniBrojStanice: 7, Polazak: "23:10:00.0000000", Naziv: "Target Station"},
	}

	predicted := []contracts.PredictedDelay{
		{
			TripID:                "trip-live",
			VoznjaBusID:           1001,
			LinVarID:              "7-A-0",
			BrojLinije:            "7",
			StationID:             100,
			StationName:           "Target Station",
			StationSeq:            5,
			ScheduledTime:         "2026-02-22T20:30:00Z",
			PredictedTime:         "2026-02-22T20:32:00Z",
			PredictedDelaySeconds: 120,
			GeneratedAt:           "2026-02-22T20:00:00Z",
			TrackerVersion:        "current",
		},
	}

	httpServer := setupStationTimetableHTTPServer(t, now, stations, timetable, predicted)
	response, status := getStationTimetable(t, httpServer.URL+"/v1/station-arrivals?station_id=100&window_minutes=60")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}

	if response.StationID != 100 {
		t.Fatalf("StationID = %d, want 100", response.StationID)
	}
	if response.StationName != "Target Station" {
		t.Fatalf("StationName = %q, want %q", response.StationName, "Target Station")
	}
	if response.WindowMinutes != 60 {
		t.Fatalf("WindowMinutes = %d, want 60", response.WindowMinutes)
	}
	if len(response.Rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(response.Rows))
	}

	first := response.Rows[0]
	if first.Status != "live" || first.TripID != "trip-live" {
		t.Fatalf("first row = (%s,%s), want (live,trip-live)", first.Status, first.TripID)
	}
	if first.DelaySeconds == nil || *first.DelaySeconds != 120 {
		t.Fatalf("first delay = %v, want 120", first.DelaySeconds)
	}

	second := response.Rows[1]
	if second.Status != "scheduled" || second.TripID != "trip-scheduled" {
		t.Fatalf("second row = (%s,%s), want (scheduled,trip-scheduled)", second.Status, second.TripID)
	}
	if second.DelaySeconds != nil {
		t.Fatalf("second delay = %v, want nil", second.DelaySeconds)
	}
}

func TestServerStationTimetableEndpoint_StrictStationID_NoNameMerge(t *testing.T) {
	now := time.Date(2026, 2, 22, 20, 0, 0, 0, time.UTC)

	stations := []stationFixtureRow{
		{StanicaID: 100, Naziv: "Shared Name"},
		{StanicaID: 200, Naziv: "Shared Name"},
	}
	timetable := []timetableFixtureRow{
		{PolazakID: "trip-other", StanicaID: 200, BrojLinije: "7", RedniBrojStanice: 8, Polazak: "21:15:00.0000000", Naziv: "Shared Name"},
	}
	predicted := []contracts.PredictedDelay{
		{
			TripID:                "trip-other-live",
			VoznjaBusID:           2001,
			LinVarID:              "7-B-0",
			BrojLinije:            "7",
			StationID:             200,
			StationName:           "Shared Name",
			StationSeq:            9,
			ScheduledTime:         "2026-02-22T20:20:00Z",
			PredictedTime:         "2026-02-22T20:21:00Z",
			PredictedDelaySeconds: 60,
			GeneratedAt:           "2026-02-22T20:00:00Z",
			TrackerVersion:        "current",
		},
	}

	httpServer := setupStationTimetableHTTPServer(t, now, stations, timetable, predicted)
	response, status := getStationTimetable(t, httpServer.URL+"/v1/station-arrivals?station_id=100&window_minutes=60")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}

	if response.StationName != "Shared Name" {
		t.Fatalf("StationName = %q, want %q", response.StationName, "Shared Name")
	}
	if len(response.Rows) != 0 {
		t.Fatalf("len(rows) = %d, want 0", len(response.Rows))
	}
}

func TestServerStationTimetableEndpoint_WindowCrossMidnight(t *testing.T) {
	now := time.Date(2026, 2, 22, 22, 50, 0, 0, time.UTC) // 23:50 Europe/Zagreb

	stations := []stationFixtureRow{
		{StanicaID: 100, Naziv: "Night Station"},
	}
	timetable := []timetableFixtureRow{
		{PolazakID: "trip-midnight", StanicaID: 100, BrojLinije: "7A", RedniBrojStanice: 4, Polazak: "00:10:00.0000000", Naziv: "Night Station"},
		{PolazakID: "trip-outside", StanicaID: 100, BrojLinije: "7A", RedniBrojStanice: 5, Polazak: "01:20:00.0000000", Naziv: "Night Station"},
	}
	predicted := []contracts.PredictedDelay{
		{
			TripID:                "trip-live",
			VoznjaBusID:           1002,
			LinVarID:              "7A-A-0",
			BrojLinije:            "7A",
			StationID:             100,
			StationName:           "Night Station",
			StationSeq:            6,
			ScheduledTime:         "2026-02-22T23:10:00Z",
			PredictedTime:         "2026-02-22T23:15:00Z",
			PredictedDelaySeconds: 300,
			GeneratedAt:           "2026-02-22T22:50:00Z",
			TrackerVersion:        "current",
		},
	}

	httpServer := setupStationTimetableHTTPServer(t, now, stations, timetable, predicted)
	response, status := getStationTimetable(t, httpServer.URL+"/v1/station-arrivals?station_id=100&window_minutes=60")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if len(response.Rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(response.Rows))
	}

	first := response.Rows[0]
	if first.Status != "scheduled" || first.TripID != "trip-midnight" {
		t.Fatalf("first row = (%s,%s), want (scheduled,trip-midnight)", first.Status, first.TripID)
	}
	if !strings.HasPrefix(first.ScheduledTime, "2026-02-22T23:10:00") {
		t.Fatalf("first ScheduledTime = %q, want prefix 2026-02-22T23:10:00", first.ScheduledTime)
	}

	second := response.Rows[1]
	if second.Status != "live" || second.TripID != "trip-live" {
		t.Fatalf("second row = (%s,%s), want (live,trip-live)", second.Status, second.TripID)
	}
}

func TestServerStationTimetableEndpoint_LiveOverridesScheduledByTripStationSeq(t *testing.T) {
	now := time.Date(2026, 2, 22, 20, 0, 0, 0, time.UTC)

	stations := []stationFixtureRow{
		{StanicaID: 100, Naziv: "Loop Station"},
	}
	timetable := []timetableFixtureRow{
		{PolazakID: "trip-loop", StanicaID: 100, BrojLinije: "6", RedniBrojStanice: 5, Polazak: "21:15:00.0000000", Naziv: "Loop Station"},
		{PolazakID: "trip-loop", StanicaID: 100, BrojLinije: "6", RedniBrojStanice: 6, Polazak: "21:45:00.0000000", Naziv: "Loop Station"},
	}
	predicted := []contracts.PredictedDelay{
		{
			TripID:                "trip-loop",
			VoznjaBusID:           1003,
			LinVarID:              "6-A-0",
			BrojLinije:            "6",
			StationID:             100,
			StationName:           "Loop Station",
			StationSeq:            5,
			ScheduledTime:         "2026-02-22T20:15:00Z",
			PredictedTime:         "2026-02-22T20:16:00Z",
			PredictedDelaySeconds: 60,
			GeneratedAt:           "2026-02-22T20:00:00Z",
			TrackerVersion:        "current",
		},
	}

	httpServer := setupStationTimetableHTTPServer(t, now, stations, timetable, predicted)
	response, status := getStationTimetable(t, httpServer.URL+"/v1/station-arrivals?station_id=100&window_minutes=60")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if len(response.Rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(response.Rows))
	}

	if response.Rows[0].Status != "live" || response.Rows[0].StationSeq != 5 {
		t.Fatalf("first row = (%s,seq=%d), want (live,seq=5)", response.Rows[0].Status, response.Rows[0].StationSeq)
	}
	if response.Rows[1].Status != "scheduled" || response.Rows[1].StationSeq != 6 {
		t.Fatalf("second row = (%s,seq=%d), want (scheduled,seq=6)", response.Rows[1].Status, response.Rows[1].StationSeq)
	}
}

func TestStationTimetableBuilder_LoopTripRepeatedStationKeepsUnmatchedScheduledRow(t *testing.T) {
	now := time.Date(2026, 2, 22, 20, 0, 0, 0, time.UTC)
	stations := []stationFixtureRow{
		{StanicaID: 100, Naziv: "Builder Station"},
	}
	timetable := []timetableFixtureRow{
		{PolazakID: "trip-loop", StanicaID: 100, BrojLinije: "7", RedniBrojStanice: 1, Polazak: "21:05:00.0000000", Naziv: "Builder Station"},
		{PolazakID: "trip-loop", StanicaID: 100, BrojLinije: "7", RedniBrojStanice: 3, Polazak: "21:35:00.0000000", Naziv: "Builder Station"},
	}

	tmpDir := t.TempDir()
	stationsPath, timetablePath := writeStationTimetableFixtures(t, tmpDir, stations, timetable)
	index, err := loadStationTimetableIndex(stationsPath, timetablePath)
	if err != nil {
		t.Fatalf("loadStationTimetableIndex: %v", err)
	}

	predicted := []contracts.PredictedDelay{
		{
			TripID:                "trip-loop",
			VoznjaBusID:           1004,
			LinVarID:              "7-A-0",
			BrojLinije:            "7",
			StationID:             100,
			StationName:           "Builder Station",
			StationSeq:            1,
			ScheduledTime:         "2026-02-22T20:05:00Z",
			PredictedTime:         "2026-02-22T20:06:00Z",
			PredictedDelaySeconds: 60,
			GeneratedAt:           "2026-02-22T20:00:00Z",
			TrackerVersion:        "current",
		},
	}

	response := index.Build(100, now, 60*time.Minute, predicted)
	if len(response.Rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(response.Rows))
	}
	if response.Rows[0].Status != "live" || response.Rows[0].StationSeq != 1 {
		t.Fatalf("first row = (%s,seq=%d), want (live,seq=1)", response.Rows[0].Status, response.Rows[0].StationSeq)
	}
	if response.Rows[1].Status != "scheduled" || response.Rows[1].StationSeq != 3 {
		t.Fatalf("second row = (%s,seq=%d), want (scheduled,seq=3)", response.Rows[1].Status, response.Rows[1].StationSeq)
	}
}

func TestServerStationArrivalsEndpoint_Validation(t *testing.T) {
	now := time.Date(2026, 2, 22, 20, 0, 0, 0, time.UTC)

	stations := []stationFixtureRow{{StanicaID: 100, Naziv: "Target Station"}}
	timetable := []timetableFixtureRow{{PolazakID: "trip-1", StanicaID: 100, BrojLinije: "7", RedniBrojStanice: 1, Polazak: "21:00:00"}}
	httpServer := setupStationTimetableHTTPServer(t, now, stations, timetable, nil)

	_, status := getStationTimetable(t, httpServer.URL+"/v1/station-arrivals")
	if status != http.StatusBadRequest {
		t.Fatalf("missing station_id status = %d, want %d", status, http.StatusBadRequest)
	}

	_, status = getStationTimetable(t, httpServer.URL+"/v1/station-arrivals?station_id=abc")
	if status != http.StatusBadRequest {
		t.Fatalf("invalid station_id status = %d, want %d", status, http.StatusBadRequest)
	}

	_, status = getStationTimetable(t, httpServer.URL+"/v1/station-arrivals?station_id=100&window_minutes=oops")
	if status != http.StatusBadRequest {
		t.Fatalf("invalid window_minutes status = %d, want %d", status, http.StatusBadRequest)
	}
}

func TestServerStationArrivalsEndpoint_ClampsWindow(t *testing.T) {
	now := time.Date(2026, 2, 22, 20, 0, 0, 0, time.UTC)

	stations := []stationFixtureRow{{StanicaID: 100, Naziv: "Target Station"}}
	timetable := []timetableFixtureRow{{PolazakID: "trip-1", StanicaID: 100, BrojLinije: "7", RedniBrojStanice: 1, Polazak: "21:00:00"}}
	httpServer := setupStationTimetableHTTPServer(t, now, stations, timetable, nil)

	response, status := getStationTimetable(t, httpServer.URL+"/v1/station-arrivals?station_id=100&window_minutes=0")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if response.WindowMinutes != minStationTimetableWindowMinutes {
		t.Fatalf("WindowMinutes = %d, want %d", response.WindowMinutes, minStationTimetableWindowMinutes)
	}

	response, status = getStationTimetable(t, httpServer.URL+"/v1/station-arrivals?station_id=100&window_minutes=999")
	if status != http.StatusOK {
		t.Fatalf("status = %d, want %d", status, http.StatusOK)
	}
	if response.WindowMinutes != maxStationTimetableWindowMinutes {
		t.Fatalf("WindowMinutes = %d, want %d", response.WindowMinutes, maxStationTimetableWindowMinutes)
	}
}

func TestServerStationTimetableAliasMatchesStationArrivalsWithDeprecationHeaders(t *testing.T) {
	now := time.Date(2026, 2, 22, 20, 0, 0, 0, time.UTC)

	stations := []stationFixtureRow{
		{StanicaID: 100, Naziv: "Target Station"},
	}
	timetable := []timetableFixtureRow{
		{PolazakID: "trip-live", StanicaID: 100, BrojLinije: "7", RedniBrojStanice: 5, Polazak: "21:30:00.0000000", Naziv: "Target Station"},
	}
	predicted := []contracts.PredictedDelay{
		{
			TripID:                "trip-live",
			VoznjaBusID:           1001,
			LinVarID:              "7-A-0",
			BrojLinije:            "7",
			StationID:             100,
			StationName:           "Target Station",
			StationSeq:            5,
			ScheduledTime:         "2026-02-22T20:30:00Z",
			PredictedTime:         "2026-02-22T20:32:00Z",
			PredictedDelaySeconds: 120,
			GeneratedAt:           "2026-02-22T20:00:00Z",
			TrackerVersion:        "current",
		},
	}

	httpServer := setupStationTimetableHTTPServer(t, now, stations, timetable, predicted)

	arrivalsResponse, arrivalsStatus, _ := getStationTimetableWithHeaders(t, httpServer.URL+"/v1/station-arrivals?station_id=100&window_minutes=60")
	if arrivalsStatus != http.StatusOK {
		t.Fatalf("station-arrivals status = %d, want %d", arrivalsStatus, http.StatusOK)
	}

	aliasResponse, aliasStatus, headers := getStationTimetableWithHeaders(t, httpServer.URL+"/v1/station-timetable?station_id=100&window_minutes=60")
	if aliasStatus != http.StatusOK {
		t.Fatalf("station-timetable status = %d, want %d", aliasStatus, http.StatusOK)
	}
	if !reflect.DeepEqual(aliasResponse, arrivalsResponse) {
		t.Fatalf("alias payload differs from station-arrivals payload")
	}
	if got := headers.Get("Deprecation"); got != "true" {
		t.Fatalf("Deprecation header = %q, want %q", got, "true")
	}
	if got := headers.Get("Sunset"); got != deprecatedSunset {
		t.Fatalf("Sunset header = %q, want %q", got, deprecatedSunset)
	}
}

func setupStationTimetableHTTPServer(
	t *testing.T,
	now time.Time,
	stations []stationFixtureRow,
	timetable []timetableFixtureRow,
	predicted []contracts.PredictedDelay,
) *httptest.Server {
	t.Helper()

	store := NewStore(StoreConfig{
		Now: func() time.Time { return now },
	})
	for _, event := range predicted {
		store.UpsertPredictedDelay(event)
	}

	hub := NewHub(HubConfig{PingInterval: 100 * time.Millisecond})

	tmpDir := t.TempDir()
	stationsPath, timetablePath := writeStationTimetableFixtures(t, tmpDir, stations, timetable)

	server := NewServer(ServerConfig{
		Store:         store,
		Hub:           hub,
		StationsPath:  stationsPath,
		TimetablePath: timetablePath,
		Now: func() time.Time {
			return now
		},
	})
	t.Cleanup(server.Close)

	httpServer := httptest.NewServer(server.Routes())
	t.Cleanup(httpServer.Close)
	return httpServer
}

func writeStationTimetableFixtures(
	t *testing.T,
	dir string,
	stations []stationFixtureRow,
	timetable []timetableFixtureRow,
) (string, string) {
	t.Helper()

	stationsPath := filepath.Join(dir, "stanice.json")
	stationsPayload, err := json.Marshal(stations)
	if err != nil {
		t.Fatalf("marshal stations fixture: %v", err)
	}
	if err := os.WriteFile(stationsPath, stationsPayload, 0o644); err != nil {
		t.Fatalf("write stations fixture: %v", err)
	}

	timetablePath := filepath.Join(dir, "voznired_dnevni.json")
	timetablePayload, err := json.Marshal(timetable)
	if err != nil {
		t.Fatalf("marshal timetable fixture: %v", err)
	}
	if err := os.WriteFile(timetablePath, timetablePayload, 0o644); err != nil {
		t.Fatalf("write timetable fixture: %v", err)
	}

	return stationsPath, timetablePath
}

func getStationTimetable(t *testing.T, url string) (contracts.StationTimetableResponse, int) {
	t.Helper()

	payload, status, _ := getStationTimetableWithHeaders(t, url)
	return payload, status
}

func getStationTimetableWithHeaders(t *testing.T, url string) (contracts.StationTimetableResponse, int, http.Header) {
	t.Helper()

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var payload contracts.StationTimetableResponse
	if resp.StatusCode == http.StatusOK {
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("unmarshal response: %v body=%s", err, string(body))
		}
	}

	return payload, resp.StatusCode, resp.Header.Clone()
}
