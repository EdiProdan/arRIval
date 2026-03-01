package realtime

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/EdiProdan/arRIval/internal/contracts"
)

const (
	defaultStationTimetableWindowMinutes = 60
	minStationTimetableWindowMinutes     = 1
	maxStationTimetableWindowMinutes     = 120
	stationTimetableTimezone             = "Europe/Zagreb"
)

type stationTimetableScheduleRow struct {
	TripID       string
	Line         string
	StationID    int64
	StationSeq   int64
	StationName  string
	SecondsOfDay int
}

type stationTimetableIndex struct {
	location        *time.Location
	stationNameByID map[int64]string
	rowsByStationID map[int64][]stationTimetableScheduleRow
}

type stationJSONRow struct {
	StanicaID int64  `json:"StanicaId"`
	Naziv     string `json:"Naziv"`
}

type timetableJSONRow struct {
	PolazakID        string `json:"PolazakId"`
	StanicaID        int64  `json:"StanicaId"`
	BrojLinije       string `json:"BrojLinije"`
	RedniBrojStanice int64  `json:"RedniBrojStanice"`
	Polazak          string `json:"Polazak"`
	Naziv            string `json:"Naziv"`
}

type timetableSortRow struct {
	row           contracts.StationTimetableRow
	etaTime       time.Time
	scheduledTime time.Time
}

func loadStationTimetableIndex(stationsPath, timetablePath string) (*stationTimetableIndex, error) {
	path := strings.TrimSpace(timetablePath)
	if path == "" {
		return nil, fmt.Errorf("empty timetable path")
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read timetable %q: %w", path, err)
	}

	var timetableRows []timetableJSONRow
	if err := json.Unmarshal(payload, &timetableRows); err != nil {
		return nil, fmt.Errorf("unmarshal timetable %q: %w", path, err)
	}

	location, err := time.LoadLocation(stationTimetableTimezone)
	if err != nil {
		location = time.UTC
	}

	stationNameByID := loadStationNames(stationsPath)
	index := &stationTimetableIndex{
		location:        location,
		stationNameByID: stationNameByID,
		rowsByStationID: make(map[int64][]stationTimetableScheduleRow),
	}

	for _, row := range timetableRows {
		tripID := strings.TrimSpace(row.PolazakID)
		line := strings.TrimSpace(row.BrojLinije)
		if tripID == "" || line == "" || row.StanicaID <= 0 || row.RedniBrojStanice <= 0 {
			continue
		}

		secondsOfDay, ok := parseTimetableSecondsOfDay(row.Polazak)
		if !ok {
			continue
		}

		stationName := strings.TrimSpace(row.Naziv)
		if stationName == "" {
			stationName = stationNameByID[row.StanicaID]
		}
		if stationName != "" && stationNameByID[row.StanicaID] == "" {
			stationNameByID[row.StanicaID] = stationName
		}

		index.rowsByStationID[row.StanicaID] = append(index.rowsByStationID[row.StanicaID], stationTimetableScheduleRow{
			TripID:       tripID,
			Line:         line,
			StationID:    row.StanicaID,
			StationSeq:   row.RedniBrojStanice,
			StationName:  stationName,
			SecondsOfDay: secondsOfDay,
		})
	}

	for stationID := range index.rowsByStationID {
		rows := index.rowsByStationID[stationID]
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].SecondsOfDay != rows[j].SecondsOfDay {
				return rows[i].SecondsOfDay < rows[j].SecondsOfDay
			}
			if rows[i].Line != rows[j].Line {
				return rows[i].Line < rows[j].Line
			}
			if rows[i].TripID != rows[j].TripID {
				return rows[i].TripID < rows[j].TripID
			}
			return rows[i].StationSeq < rows[j].StationSeq
		})
		index.rowsByStationID[stationID] = rows
	}

	return index, nil
}

func loadStationNames(stationsPath string) map[int64]string {
	stationNameByID := make(map[int64]string)
	path := strings.TrimSpace(stationsPath)
	if path == "" {
		return stationNameByID
	}

	payload, err := os.ReadFile(path)
	if err != nil {
		return stationNameByID
	}

	var rows []stationJSONRow
	if err := json.Unmarshal(payload, &rows); err != nil {
		return stationNameByID
	}

	for _, row := range rows {
		if row.StanicaID <= 0 {
			continue
		}
		name := strings.TrimSpace(row.Naziv)
		if name == "" {
			continue
		}
		stationNameByID[row.StanicaID] = name
	}

	return stationNameByID
}

func parseTimetableSecondsOfDay(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}

	parts := strings.Split(value, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, false
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, false
	}

	second := 0
	if len(parts) == 3 {
		secondPart := parts[2]
		if dot := strings.Index(secondPart, "."); dot >= 0 {
			secondPart = secondPart[:dot]
		}
		second, err = strconv.Atoi(secondPart)
		if err != nil || second < 0 || second > 59 {
			return 0, false
		}
	}

	return hour*3600 + minute*60 + second, true
}

func tripStationSeqKey(tripID string, stationID, stationSeq int64) string {
	return fmt.Sprintf("%s:%d:%d", tripID, stationID, stationSeq)
}

func (s *stationTimetableIndex) stationName(stationID int64) string {
	name := strings.TrimSpace(s.stationNameByID[stationID])
	if name != "" {
		return name
	}
	return fmt.Sprintf("Station %d", stationID)
}

func (s *stationTimetableIndex) nextOccurrence(secondsOfDay int, now time.Time) time.Time {
	nowLocal := now.In(s.location)
	dayStart := time.Date(nowLocal.Year(), nowLocal.Month(), nowLocal.Day(), 0, 0, 0, 0, s.location)
	candidateLocal := dayStart.Add(time.Duration(secondsOfDay) * time.Second)
	if candidateLocal.Before(nowLocal) {
		candidateLocal = candidateLocal.Add(24 * time.Hour)
	}
	return candidateLocal.UTC()
}

func (s *stationTimetableIndex) Build(
	stationID int64,
	now time.Time,
	window time.Duration,
	predicted []contracts.PredictedDelay,
) contracts.StationTimetableResponse {
	now = now.UTC()
	end := now.Add(window)
	if window <= 0 {
		window = time.Duration(defaultStationTimetableWindowMinutes) * time.Minute
		end = now.Add(window)
	}

	rows := make([]timetableSortRow, 0)
	liveKeys := make(map[string]struct{})

	for _, item := range predicted {
		if item.StationID != stationID {
			continue
		}

		eta, err := time.Parse(time.RFC3339Nano, item.PredictedTime)
		if err != nil {
			continue
		}
		eta = eta.UTC()
		if eta.Before(now) || eta.After(end) {
			continue
		}

		scheduled, err := time.Parse(time.RFC3339Nano, item.ScheduledTime)
		if err != nil {
			scheduled = eta
		}
		scheduled = scheduled.UTC()

		delay := item.PredictedDelaySeconds
		line := strings.TrimSpace(item.BrojLinije)
		stationName := strings.TrimSpace(item.StationName)
		if stationName == "" {
			stationName = s.stationName(stationID)
		}

		row := contracts.StationTimetableRow{
			Status:        "live",
			TripID:        item.TripID,
			Line:          line,
			StationID:     stationID,
			StationSeq:    item.StationSeq,
			ScheduledTime: scheduled.Format(time.RFC3339Nano),
			ETATime:       eta.Format(time.RFC3339Nano),
			DelaySeconds:  &delay,
		}
		rows = append(rows, timetableSortRow{
			row:           row,
			etaTime:       eta,
			scheduledTime: scheduled,
		})
		liveKeys[tripStationSeqKey(item.TripID, stationID, item.StationSeq)] = struct{}{}
	}

	scheduledRows := s.rowsByStationID[stationID]
	for _, item := range scheduledRows {
		scheduled := s.nextOccurrence(item.SecondsOfDay, now)
		if scheduled.Before(now) || scheduled.After(end) {
			continue
		}

		key := tripStationSeqKey(item.TripID, item.StationID, item.StationSeq)
		if _, exists := liveKeys[key]; exists {
			continue
		}

		stationName := strings.TrimSpace(item.StationName)
		if stationName == "" {
			stationName = s.stationName(stationID)
		}

		row := contracts.StationTimetableRow{
			Status:        "scheduled",
			TripID:        item.TripID,
			Line:          item.Line,
			StationID:     item.StationID,
			StationSeq:    item.StationSeq,
			ScheduledTime: scheduled.Format(time.RFC3339Nano),
			ETATime:       scheduled.Format(time.RFC3339Nano),
			DelaySeconds:  nil,
		}
		rows = append(rows, timetableSortRow{
			row:           row,
			etaTime:       scheduled,
			scheduledTime: scheduled,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		left := rows[i]
		right := rows[j]
		if !left.etaTime.Equal(right.etaTime) {
			return left.etaTime.Before(right.etaTime)
		}
		if !left.scheduledTime.Equal(right.scheduledTime) {
			return left.scheduledTime.Before(right.scheduledTime)
		}
		if left.row.Line != right.row.Line {
			return left.row.Line < right.row.Line
		}
		if left.row.TripID != right.row.TripID {
			return left.row.TripID < right.row.TripID
		}
		return left.row.StationSeq < right.row.StationSeq
	})

	responseRows := make([]contracts.StationTimetableRow, 0, len(rows))
	for _, item := range rows {
		responseRows = append(responseRows, item.row)
	}

	return contracts.StationTimetableResponse{
		StationID:   stationID,
		StationName: s.stationName(stationID),
		GeneratedAt: now.Format(time.RFC3339Nano),
		Rows:        responseRows,
	}
}
