package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/EdiProdan/arRIval/internal/aggregatorlogic"
	"github.com/EdiProdan/arRIval/internal/contracts"
	"github.com/EdiProdan/arRIval/internal/envutil"
	"github.com/EdiProdan/arRIval/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/parquet"
	"github.com/xitongsys/parquet-go/writer"
)

const (
	defaultBrokers       = "localhost:19092"
	defaultInputTopic    = contracts.TopicBusDelayObservedV2
	defaultConsumerGroup = "arrival-aggregator"
	defaultGoldDir       = "data/gold"
	defaultMetricsAddr   = ":9103"
	defaultOnTimeSeconds = 300
	flushInterval        = 30 * time.Second
)

type aggregatorMetrics struct {
	aggregationsComputed prometheus.Counter
}

func main() {
	if err := envutil.LoadDotEnv(".env"); err != nil {
		log.Fatalf("load .env: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	brokers := envutil.CSVEnv("ARRIVAL_KAFKA_BROKERS", defaultBrokers)
	inputTopic := envutil.StringEnv("ARRIVAL_KAFKA_DELAY_OBSERVED_TOPIC", defaultInputTopic)
	consumerGroup := envutil.StringEnv("ARRIVAL_AGGREGATOR_GROUP", defaultConsumerGroup)
	goldDir := envutil.StringEnv("ARRIVAL_GOLD_DIR", defaultGoldDir)
	onTimeSeconds := envutil.IntEnv("ARRIVAL_ON_TIME_SECONDS", defaultOnTimeSeconds)
	metricsAddr := envutil.StringEnv("ARRIVAL_AGGREGATOR_METRICS_ADDR", defaultMetricsAddr)

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

	state := make(map[aggregatorlogic.AggregateKey]*aggregatorlogic.AggregateBucket)
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

func consumeDelayRecord(state map[aggregatorlogic.AggregateKey]*aggregatorlogic.AggregateBucket, rec *kgo.Record, onTimeSeconds int) error {
	var event contracts.ObservedDelayV2
	if err := json.Unmarshal(rec.Value, &event); err != nil {
		return fmt.Errorf("unmarshal observed delay event: %w", err)
	}

	if err := aggregatorlogic.ConsumeEvent(state, event, onTimeSeconds); err != nil {
		return err
	}

	return nil
}

func flushGold(goldDir string, state map[aggregatorlogic.AggregateKey]*aggregatorlogic.AggregateBucket) (int64, error) {
	if len(state) == 0 {
		return 0, nil
	}

	byDate := aggregatorlogic.ComputeRowsByDate(state, time.Now().UTC())
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

func writeGoldDay(baseDir, date string, rows []aggregatorlogic.StatsRow) error {
	dayDir := filepath.Join(baseDir, date)
	if err := os.MkdirAll(dayDir, 0o755); err != nil {
		return fmt.Errorf("create gold day dir %s: %w", dayDir, err)
	}

	filePath := filepath.Join(dayDir, "stats.parquet")
	fw, err := local.NewLocalFileWriter(filePath)
	if err != nil {
		return fmt.Errorf("open gold parquet file writer %s: %w", filePath, err)
	}

	pw, err := writer.NewParquetWriter(fw, new(aggregatorlogic.StatsRow), 1)
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
