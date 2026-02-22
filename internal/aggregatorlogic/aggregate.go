package aggregatorlogic

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/EdiProdan/arRIval/internal/contracts"
)

type AggregateKey struct {
	Date         string
	HourStartUTC string
	Route        string
}

type AggregateBucket struct {
	delays      []int64
	onTimeCount int64
}

type StatsRow struct {
	Date            string  `parquet:"name=date, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	HourStartUTC    string  `parquet:"name=hour_start_utc, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	Route           string  `parquet:"name=route, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	SampleCount     int64   `parquet:"name=sample_count, type=INT64"`
	AvgDelaySeconds float64 `parquet:"name=avg_delay_seconds, type=DOUBLE"`
	P95DelaySeconds int64   `parquet:"name=p95_delay_seconds, type=INT64"`
	P99DelaySeconds int64   `parquet:"name=p99_delay_seconds, type=INT64"`
	OnTimePercent   float64 `parquet:"name=on_time_percentage, type=DOUBLE"`
	WrittenAt       string  `parquet:"name=written_at, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
}

func ConsumeEvent(state map[AggregateKey]*AggregateBucket, event contracts.ObservedDelay, onTimeSeconds int) error {
	actual, err := parseTimestampUTC(event.ObservedTime)
	if err != nil {
		return fmt.Errorf("parse observed_time %q: %w", event.ObservedTime, err)
	}

	route := strings.TrimSpace(event.BrojLinije)
	if route == "" {
		route = "unknown"
	}

	hourStart := actual.UTC().Truncate(time.Hour)
	key := AggregateKey{
		Date:         hourStart.Format("2006-01-02"),
		HourStartUTC: hourStart.Format(time.RFC3339),
		Route:        route,
	}

	bucket, ok := state[key]
	if !ok {
		bucket = &AggregateBucket{}
		state[key] = bucket
	}

	bucket.delays = append(bucket.delays, event.DelaySeconds)
	if absInt64(event.DelaySeconds) <= int64(onTimeSeconds) {
		bucket.onTimeCount++
	}

	return nil
}

func ComputeRowsByDate(state map[AggregateKey]*AggregateBucket, writtenAt time.Time) map[string][]StatsRow {
	if len(state) == 0 {
		return map[string][]StatsRow{}
	}

	byDate := make(map[string][]StatsRow)
	writtenAtUTC := writtenAt.UTC().Format(time.RFC3339Nano)

	for key, bucket := range state {
		if len(bucket.delays) == 0 {
			continue
		}

		delays := append([]int64(nil), bucket.delays...)
		sort.Slice(delays, func(i, j int) bool { return delays[i] < delays[j] })

		sampleCount := int64(len(delays))
		avg := average(delays)
		p95 := nearestRank(delays, 0.95)
		p99 := nearestRank(delays, 0.99)
		onTimePct := (float64(bucket.onTimeCount) / float64(sampleCount)) * 100

		row := StatsRow{
			Date:            key.Date,
			HourStartUTC:    key.HourStartUTC,
			Route:           key.Route,
			SampleCount:     sampleCount,
			AvgDelaySeconds: avg,
			P95DelaySeconds: p95,
			P99DelaySeconds: p99,
			OnTimePercent:   onTimePct,
			WrittenAt:       writtenAtUTC,
		}

		byDate[key.Date] = append(byDate[key.Date], row)
	}

	return byDate
}

func parseTimestampUTC(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("empty timestamp")
	}

	t, err := time.Parse(time.RFC3339Nano, value)
	if err == nil {
		return t.UTC(), nil
	}

	t, err = time.Parse(time.RFC3339, value)
	if err == nil {
		return t.UTC(), nil
	}

	return time.Time{}, err
}

func average(values []int64) float64 {
	if len(values) == 0 {
		return 0
	}

	var sum int64
	for _, value := range values {
		sum += value
	}

	return float64(sum) / float64(len(values))
}

func nearestRank(sortedValues []int64, p float64) int64 {
	if len(sortedValues) == 0 {
		return 0
	}

	if p <= 0 {
		return sortedValues[0]
	}
	if p >= 1 {
		return sortedValues[len(sortedValues)-1]
	}

	rank := int(math.Ceil(p*float64(len(sortedValues)))) - 1
	if rank < 0 {
		rank = 0
	}
	if rank >= len(sortedValues) {
		rank = len(sortedValues) - 1
	}

	return sortedValues[rank]
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
