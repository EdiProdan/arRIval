package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/EdiProdan/arRIval/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/parquet"
	"github.com/xitongsys/parquet-go/writer"
)

const (
	defaultBrokers       = "localhost:19092"
	defaultInputTopic    = "bus-delays"
	defaultConsumerGroup = "arrival-aggregator"
	defaultGoldDir       = "data/gold"
	defaultMetricsAddr   = ":9103"
	defaultOnTimeSeconds = 300
	flushInterval        = 30 * time.Second
)

type aggregatorMetrics struct {
	aggregationsComputed prometheus.Counter
}

type delayEvent struct {
	PolazakID     string `json:"polazak_id"`
	VoznjaBusID   int64  `json:"voznja_bus_id"`
	LinVarID      string `json:"lin_var_id"`
	BrojLinije    string `json:"broj_linije"`
	ScheduledTime string `json:"scheduled_time"`
	ActualTime    string `json:"actual_time"`
	DelaySeconds  int64  `json:"delay_seconds"`
}

type goldStatsRow struct {
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

type aggregateKey struct {
	Date         string
	HourStartUTC string
	Route        string
}

type aggregateBucket struct {
	delays      []int64
	onTimeCount int64
}

func main() {
	if err := loadDotEnv(".env"); err != nil {
		log.Fatalf("load .env: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	brokers := splitCSV(getenv("ARRIVAL_KAFKA_BROKERS", defaultBrokers))
	inputTopic := getenv("ARRIVAL_KAFKA_DELAY_TOPIC", defaultInputTopic)
	consumerGroup := getenv("ARRIVAL_AGGREGATOR_GROUP", defaultConsumerGroup)
	goldDir := getenv("ARRIVAL_GOLD_DIR", defaultGoldDir)
	onTimeSeconds := parseInt(getenv("ARRIVAL_ON_TIME_SECONDS", fmt.Sprintf("%d", defaultOnTimeSeconds)), defaultOnTimeSeconds)
	metricsAddr := getenv("ARRIVAL_AGGREGATOR_METRICS_ADDR", defaultMetricsAddr)

	collector := newAggregatorMetrics()
	metrics.StartServer(ctx, metricsAddr)

	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(consumerGroup),
		kgo.ConsumeTopics(inputTopic),
	)
	if err != nil {
		log.Fatalf("create consumer client: %v", err)
	}
	defer consumer.Close()

	state := make(map[aggregateKey]*aggregateBucket)
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	log.Printf("aggregator started: topic=%s group=%s gold_dir=%s flush_interval=%s on_time_seconds=%d", inputTopic, consumerGroup, goldDir, flushInterval, onTimeSeconds)

	var consumed int64
	for {
		if ctx.Err() != nil {
			break
		}

		fetches := consumer.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, fetchErr := range errs {
				log.Printf("status=error stage=consume topic=%s partition=%d err=%v", fetchErr.Topic, fetchErr.Partition, fetchErr.Err)
			}
			continue
		}

		var commitRecords []*kgo.Record
		fetches.EachRecord(func(rec *kgo.Record) {
			if err := consumeDelayRecord(state, rec, onTimeSeconds); err != nil {
				log.Printf("status=error stage=process partition=%d offset=%d err=%v", rec.Partition, rec.Offset, err)
				return
			}

			consumed++
			commitRecords = append(commitRecords, rec)
		})

		if len(commitRecords) > 0 {
			consumer.MarkCommitRecords(commitRecords...)
		}

		select {
		case <-ctx.Done():
			continue
		case <-ticker.C:
			rows, err := flushGold(goldDir, state)
			if err != nil {
				log.Printf("status=error stage=flush err=%v", err)
			} else {
				collector.aggregationsComputed.Add(float64(rows))
			}
		default:
		}
	}

	rows, err := flushGold(goldDir, state)
	if err != nil {
		log.Printf("status=error stage=flush err=%v", err)
	} else {
		collector.aggregationsComputed.Add(float64(rows))
	}

	log.Printf("aggregator stopped: consumed=%d buckets=%d", consumed, len(state))
}

func newAggregatorMetrics() aggregatorMetrics {
	aggregationsComputed := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "arrival_aggregator_aggregations_computed_total",
		Help: "Total number of route-hour aggregation rows computed.",
	})

	prometheus.MustRegister(aggregationsComputed)

	return aggregatorMetrics{aggregationsComputed: aggregationsComputed}
}

func consumeDelayRecord(state map[aggregateKey]*aggregateBucket, rec *kgo.Record, onTimeSeconds int) error {
	var event delayEvent
	if err := json.Unmarshal(rec.Value, &event); err != nil {
		return fmt.Errorf("unmarshal delay event: %w", err)
	}

	route := strings.TrimSpace(event.BrojLinije)
	if route == "" {
		route = "unknown"
	}

	actual, err := parseTimestampUTC(event.ActualTime)
	if err != nil {
		return fmt.Errorf("parse actual_time %q: %w", event.ActualTime, err)
	}

	hourStart := actual.UTC().Truncate(time.Hour)
	key := aggregateKey{
		Date:         hourStart.Format("2006-01-02"),
		HourStartUTC: hourStart.Format(time.RFC3339),
		Route:        route,
	}

	bucket, ok := state[key]
	if !ok {
		bucket = &aggregateBucket{}
		state[key] = bucket
	}

	bucket.delays = append(bucket.delays, event.DelaySeconds)
	if absInt64(event.DelaySeconds) <= int64(onTimeSeconds) {
		bucket.onTimeCount++
	}

	return nil
}

func flushGold(goldDir string, state map[aggregateKey]*aggregateBucket) (int64, error) {
	if len(state) == 0 {
		return 0, nil
	}

	now := time.Now().UTC().Format(time.RFC3339Nano)
	byDate := make(map[string][]goldStatsRow)

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

		row := goldStatsRow{
			Date:            key.Date,
			HourStartUTC:    key.HourStartUTC,
			Route:           key.Route,
			SampleCount:     sampleCount,
			AvgDelaySeconds: avg,
			P95DelaySeconds: p95,
			P99DelaySeconds: p99,
			OnTimePercent:   onTimePct,
			WrittenAt:       now,
		}

		byDate[key.Date] = append(byDate[key.Date], row)
	}

	for date, rows := range byDate {
		sort.Slice(rows, func(i, j int) bool {
			if rows[i].HourStartUTC != rows[j].HourStartUTC {
				return rows[i].HourStartUTC < rows[j].HourStartUTC
			}
			return rows[i].Route < rows[j].Route
		})

		if err := writeGoldDay(goldDir, date, rows); err != nil {
			return 0, err
		}
	}

	log.Printf("status=ok stage=flush dates=%d buckets=%d", len(byDate), len(state))

	var rowsComputed int64
	for _, rows := range byDate {
		rowsComputed += int64(len(rows))
	}

	return rowsComputed, nil
}

func writeGoldDay(baseDir, date string, rows []goldStatsRow) error {
	dayDir := filepath.Join(baseDir, date)
	if err := os.MkdirAll(dayDir, 0o755); err != nil {
		return fmt.Errorf("create gold day dir %s: %w", dayDir, err)
	}

	filePath := filepath.Join(dayDir, "stats.parquet")
	fw, err := local.NewLocalFileWriter(filePath)
	if err != nil {
		return fmt.Errorf("open gold parquet file writer %s: %w", filePath, err)
	}

	pw, err := writer.NewParquetWriter(fw, new(goldStatsRow), 1)
	if err != nil {
		_ = fw.Close()
		return fmt.Errorf("create gold parquet writer %s: %w", filePath, err)
	}
	pw.CompressionType = parquet.CompressionCodec_SNAPPY

	for _, row := range rows {
		if err := pw.Write(row); err != nil {
			return fmt.Errorf("write gold parquet row: %w", err)
		}
	}

	if err := pw.WriteStop(); err != nil {
		return fmt.Errorf("close gold parquet writer %s: %w", filePath, err)
	}

	log.Printf("status=ok stage=gold_writer file=%s rows=%d", filePath, len(rows))
	return nil
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

func parseInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		keyValue := strings.SplitN(line, "=", 2)
		if len(keyValue) != 2 {
			continue
		}

		key := strings.TrimSpace(keyValue[0])
		value := strings.TrimSpace(keyValue[1])
		value = strings.Trim(value, `"'`)
		if key == "" || value == "" {
			continue
		}

		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return fmt.Errorf("set %s from %s: %w", key, path, err)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan %s: %w", path, err)
	}

	return nil
}

func getenv(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	brokers := make([]string, 0, len(parts))
	for _, part := range parts {
		broker := strings.TrimSpace(part)
		if broker != "" {
			brokers = append(brokers, broker)
		}
	}
	if len(brokers) == 0 {
		return []string{defaultBrokers}
	}
	return brokers
}
