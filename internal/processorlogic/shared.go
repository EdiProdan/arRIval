package processorlogic

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	defaultStationMatchMeters = 100.0
	earthRadiusMeters         = 6371000.0
)

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

func parseScheduleTimeLocalAligned(value string, observed time.Time, loc *time.Location) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("empty schedule time")
	}
	if loc == nil {
		loc = time.UTC
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

	observedLocal := observed.In(loc)
	base := time.Date(observedLocal.Year(), observedLocal.Month(), observedLocal.Day(), hour, minute, second, 0, loc)
	candidates := []time.Time{base.Add(-24 * time.Hour), base, base.Add(24 * time.Hour)}
	best := candidates[0]
	bestAbs := absDuration(observed.Sub(best))

	for _, candidate := range candidates[1:] {
		candidateAbs := absDuration(observed.Sub(candidate))
		if candidateAbs < bestAbs {
			best = candidate
			bestAbs = candidateAbs
		}
	}

	return best, nil
}

func degreesToRadians(degrees float64) float64 {
	return degrees * math.Pi / 180
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
