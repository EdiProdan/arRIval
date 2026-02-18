package processorlogic

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/EdiProdan/arRIval/internal/autotrolej"
	"github.com/EdiProdan/arRIval/internal/staticdata"
)

const (
	defaultStationMatchMeters = 100.0
	defaultScheduleWindow     = 15 * time.Minute
	earthRadiusMeters         = 6371000.0
)

type Config struct {
	StationMatchMeters float64
	ScheduleWindow     time.Duration
	ServiceLocation    *time.Location
}

type MatchInput struct {
	IngestedAt     time.Time
	KafkaTopic     string
	KafkaPartition int32
	KafkaOffset    int64
	Bus            autotrolej.LiveBus
}

type MatchResult struct {
	PolazakID     string
	VoznjaBusID   int64
	GBR           *int64
	StationID     int64
	StationName   string
	DistanceM     float64
	LinVarID      string
	BrojLinije    string
	ScheduledTime time.Time
	ActualTime    time.Time
	DelaySeconds  int64
}

func BuildDelay(input MatchInput, store *staticdata.Store, cfg Config) (MatchResult, bool, string) {
	if input.Bus.Lon == nil || input.Bus.Lat == nil {
		return MatchResult{}, false, "missing_coordinates"
	}
	if input.Bus.VoznjaBusID == nil {
		return MatchResult{}, false, "missing_voznja_bus_id"
	}

	matchMeters := cfg.StationMatchMeters
	if matchMeters <= 0 {
		matchMeters = defaultStationMatchMeters
	}

	window := cfg.ScheduleWindow
	if window <= 0 {
		window = defaultScheduleWindow
	}

	serviceLoc := cfg.ServiceLocation
	if serviceLoc == nil {
		serviceLoc = time.UTC
	}

	station, stationDistanceM, ok := nearestStation(*input.Bus.Lon, *input.Bus.Lat, store.Stations, matchMeters)
	if !ok {
		return MatchResult{}, false, "no_station_within_100m"
	}

	tripStops := store.DeparturesByPolazakID(strconv.Itoa(*input.Bus.VoznjaBusID))
	if len(tripStops) == 0 {
		return MatchResult{}, false, "no_line_for_polazak"
	}
	brojLinije := tripStops[0].BrojLinije

	lineStops := store.DeparturesByStationLine(station.StanicaID, brojLinije)
	if len(lineStops) == 0 {
		return MatchResult{}, false, "no_schedule_for_station_line"
	}

	actual := input.IngestedAt.UTC()
	var (
		bestStop      staticdata.TimetableStopRow
		bestScheduled time.Time
		bestDiff      = window + time.Second
		matched       bool
	)

	for _, stop := range lineStops {
		scheduled, parseErr := parseScheduleTimeLocalAligned(stop.Polazak, actual, serviceLoc)
		if parseErr != nil {
			continue
		}

		diff := actual.Sub(scheduled)
		if diff < 0 {
			diff = -diff
		}

		if diff <= window && diff < bestDiff {
			bestStop = stop
			bestScheduled = scheduled
			bestDiff = diff
			matched = true
		}
	}

	if !matched {
		return MatchResult{}, false, "no_schedule_within_window"
	}

	scheduledUTC := bestScheduled.UTC()
	actualUTC := actual.UTC()
	delaySeconds := int64(actualUTC.Sub(scheduledUTC).Seconds())
	gbr := intPtrToInt64Ptr(input.Bus.GBR)

	return MatchResult{
		PolazakID:     bestStop.PolazakID,
		VoznjaBusID:   int64(*input.Bus.VoznjaBusID),
		GBR:           gbr,
		StationID:     int64(station.StanicaID),
		StationName:   station.Naziv,
		DistanceM:     stationDistanceM,
		LinVarID:      bestStop.LinVarID,
		BrojLinije:    bestStop.BrojLinije,
		ScheduledTime: scheduledUTC,
		ActualTime:    actualUTC,
		DelaySeconds:  delaySeconds,
	}, true, ""
}

func nearestStation(lon, lat float64, stations []staticdata.Station, stationMatchMeters float64) (staticdata.Station, float64, bool) {
	var (
		bestStation  staticdata.Station
		bestDistance = math.MaxFloat64
		found        bool
	)

	for _, station := range stations {
		if station.GpsX == nil || station.GpsY == nil {
			continue
		}

		distance := haversineMeters(lat, lon, *station.GpsY, *station.GpsX)
		if distance >= stationMatchMeters {
			continue
		}

		if !found || distance < bestDistance {
			bestStation = station
			bestDistance = distance
			found = true
		}
	}

	if !found {
		return staticdata.Station{}, 0, false
	}

	return bestStation, bestDistance, true
}

func haversineMeters(lat1, lon1, lat2, lon2 float64) float64 {
	lat1Rad := degreesToRadians(lat1)
	lat2Rad := degreesToRadians(lat2)
	deltaLat := degreesToRadians(lat2 - lat1)
	deltaLon := degreesToRadians(lon2 - lon1)

	a := math.Sin(deltaLat/2)*math.Sin(deltaLat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(deltaLon/2)*math.Sin(deltaLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusMeters * c
}

func degreesToRadians(degrees float64) float64 {
	return degrees * math.Pi / 180
}

func parseScheduleTimeLocalAligned(value string, actual time.Time, loc *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("empty schedule time")
	}

	parts := strings.Split(value, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return time.Time{}, fmt.Errorf("invalid schedule time %q", value)
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid hour %q: %w", parts[0], err)
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid minute %q: %w", parts[1], err)
	}

	second := 0
	if len(parts) == 3 {
		secPart := parts[2]
		if dot := strings.Index(secPart, "."); dot >= 0 {
			secPart = secPart[:dot]
		}
		second, err = strconv.Atoi(secPart)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid second %q: %w", secPart, err)
		}
	}

	actualLocal := actual.In(loc)
	base := time.Date(actualLocal.Year(), actualLocal.Month(), actualLocal.Day(), hour, minute, second, 0, loc)
	candidates := []time.Time{base.Add(-24 * time.Hour), base, base.Add(24 * time.Hour)}
	best := candidates[0]
	bestAbs := absDuration(actual.Sub(best))

	for _, candidate := range candidates[1:] {
		candidateAbs := absDuration(actual.Sub(candidate))
		if candidateAbs < bestAbs {
			best = candidate
			bestAbs = candidateAbs
		}
	}

	return best, nil
}

func absDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}

func intPtrToInt64Ptr(value *int) *int64 {
	if value == nil {
		return nil
	}
	v := int64(*value)
	return &v
}
