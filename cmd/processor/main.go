package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/EdiProdan/arRIval/internal/autotrolej"
	"github.com/EdiProdan/arRIval/internal/contracts"
	"github.com/EdiProdan/arRIval/internal/envutil"
	"github.com/EdiProdan/arRIval/internal/metrics"
	"github.com/EdiProdan/arRIval/internal/processorlogic"
	"github.com/EdiProdan/arRIval/internal/staticdata"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/parquet"
	"github.com/xitongsys/parquet-go/writer"
)

const (
	defaultBrokers              = "localhost:19092"
	defaultInputTopic           = "bus-positions-raw"
	defaultObservedOutputTopic  = contracts.TopicBusDelayObserved
	defaultPredictedOutputTopic = contracts.TopicBusDelayPredicted
	defaultConsumerGroup        = "arrival-processor-bronze"
	defaultBronzeDir            = "data/bronze"
	defaultSilverDir            = "data/silver"
	defaultStaticDir            = "data"
	defaultMetricsAddr          = ":9102"
	stationMatchMeters          = 100.0
)

const (
	observedSilverFilename  = "observed_delays.parquet"
	predictedSilverFilename = "predicted_delays.parquet"
	createTopicTimeout      = 15 * time.Second
	createTopicPartitions   = 1
	createTopicReplicas     = 1
)

var serviceLocation = func() *time.Location {
	loc, err := time.LoadLocation("Europe/Zagreb")
	if err != nil {
		panic("failed to load Europe/Zagreb timezone: " + err.Error())
	}
	return loc
}()

type processorMetrics struct {
	messagesProcessed     prometheus.Counter
	processingLagSec      prometheus.Histogram
	observedDelaySeconds  prometheus.Histogram
	predictedDelaySeconds prometheus.Histogram
	observedPublished     prometheus.Counter
	predictedPublished    prometheus.Counter
	trackerSkips          *prometheus.CounterVec
}

type bronzePositionRow struct {
	IngestedAt     string   `parquet:"name=ingested_at, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	IngestedDate   string   `parquet:"name=ingested_date, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	KafkaTopic     string   `parquet:"name=kafka_topic, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	KafkaPartition int32    `parquet:"name=kafka_partition, type=INT32"`
	KafkaOffset    int64    `parquet:"name=kafka_offset, type=INT64"`
	Msg            string   `parquet:"name=msg, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	Err            bool     `parquet:"name=err, type=BOOLEAN"`
	GBR            *int64   `parquet:"name=gbr, type=INT64, repetitiontype=OPTIONAL"`
	Lon            *float64 `parquet:"name=lon, type=DOUBLE, repetitiontype=OPTIONAL"`
	Lat            *float64 `parquet:"name=lat, type=DOUBLE, repetitiontype=OPTIONAL"`
	VoznjaID       *int64   `parquet:"name=voznja_id, type=INT64, repetitiontype=OPTIONAL"`
	VoznjaBusID    *int64   `parquet:"name=voznja_bus_id, type=INT64, repetitiontype=OPTIONAL"`
}

type bronzeDailyWriter struct {
	baseDir     string
	currentDate string
	currentPath string
	pw          *writer.ParquetWriter
	rowsWritten int64
}

type silverObservedDelayRow struct {
	IngestedAt     string  `parquet:"name=ingested_at, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	IngestedDate   string  `parquet:"name=ingested_date, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	TripID         string  `parquet:"name=trip_id, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	VoznjaBusID    int64   `parquet:"name=voznja_bus_id, type=INT64"`
	GBR            *int64  `parquet:"name=gbr, type=INT64, repetitiontype=OPTIONAL"`
	LinVarID       string  `parquet:"name=lin_var_id, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	BrojLinije     string  `parquet:"name=broj_linije, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	StationID      int64   `parquet:"name=station_id, type=INT64"`
	StationName    string  `parquet:"name=station_name, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	StationSeq     int64   `parquet:"name=station_seq, type=INT64"`
	ScheduledTime  string  `parquet:"name=scheduled_time, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	ObservedTime   string  `parquet:"name=observed_time, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	DelaySeconds   int64   `parquet:"name=delay_seconds, type=INT64"`
	DistanceM      float64 `parquet:"name=distance_m, type=DOUBLE"`
	TrackerVersion string  `parquet:"name=tracker_version, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	KafkaTopic     string  `parquet:"name=kafka_topic, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	KafkaPartition int32   `parquet:"name=kafka_partition, type=INT32"`
	KafkaOffset    int64   `parquet:"name=kafka_offset, type=INT64"`
}

type silverPredictedDelayRow struct {
	IngestedAt            string `parquet:"name=ingested_at, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	IngestedDate          string `parquet:"name=ingested_date, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	TripID                string `parquet:"name=trip_id, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	VoznjaBusID           int64  `parquet:"name=voznja_bus_id, type=INT64"`
	LinVarID              string `parquet:"name=lin_var_id, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	BrojLinije            string `parquet:"name=broj_linije, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	StationID             int64  `parquet:"name=station_id, type=INT64"`
	StationName           string `parquet:"name=station_name, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	StationSeq            int64  `parquet:"name=station_seq, type=INT64"`
	ScheduledTime         string `parquet:"name=scheduled_time, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	PredictedTime         string `parquet:"name=predicted_time, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	PredictedDelaySeconds int64  `parquet:"name=predicted_delay_seconds, type=INT64"`
	GeneratedAt           string `parquet:"name=generated_at, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	TrackerVersion        string `parquet:"name=tracker_version, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	KafkaTopic            string `parquet:"name=kafka_topic, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	KafkaPartition        int32  `parquet:"name=kafka_partition, type=INT32"`
	KafkaOffset           int64  `parquet:"name=kafka_offset, type=INT64"`
}

type silverObservedDailyWriter struct {
	baseDir     string
	currentDate string
	currentPath string
	pw          *writer.ParquetWriter
	rowsWritten int64
}

type silverPredictedDailyWriter struct {
	baseDir     string
	currentDate string
	currentPath string
	pw          *writer.ParquetWriter
	rowsWritten int64
}

type bronzeWriter interface {
	Write(row bronzePositionRow) error
}

type observedWriter interface {
	Write(row silverObservedDelayRow) error
}

type predictedWriter interface {
	Write(row silverPredictedDelayRow) error
}

type tracker interface {
	Track(input processorlogic.TrackInput) processorlogic.TrackOutput
}

type publishFunc func(ctx context.Context, record *kgo.Record) error

var requestCreateTopics = func(ctx context.Context, client *kgo.Client, req *kmsg.CreateTopicsRequest) (*kmsg.CreateTopicsResponse, error) {
	return req.RequestWith(ctx, client)
}

func main() {
	if err := envutil.LoadDotEnv(".env"); err != nil {
		log.Fatalf("load .env: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	brokers := envutil.CSVEnv("ARRIVAL_KAFKA_BROKERS", defaultBrokers)
	inputTopic := envutil.StringEnv("ARRIVAL_KAFKA_TOPIC", defaultInputTopic)
	observedTopic := envutil.StringEnv("ARRIVAL_KAFKA_DELAY_OBSERVED_TOPIC", defaultObservedOutputTopic)
	predictedTopic := envutil.StringEnv("ARRIVAL_KAFKA_DELAY_PREDICTED_TOPIC", defaultPredictedOutputTopic)
	consumerGroup := envutil.StringEnv("ARRIVAL_PROCESSOR_GROUP", defaultConsumerGroup)
	bronzeDir := envutil.StringEnv("ARRIVAL_BRONZE_DIR", defaultBronzeDir)
	silverDir := envutil.StringEnv("ARRIVAL_SILVER_DIR", defaultSilverDir)
	staticDir := envutil.StringEnv("ARRIVAL_STATIC_DIR", defaultStaticDir)
	metricsAddr := envutil.StringEnv("ARRIVAL_PROCESSOR_METRICS_ADDR", defaultMetricsAddr)

	collector := newProcessorMetrics()
	metrics.StartServer(ctx, metricsAddr)

	store, err := staticdata.LoadFromDir(staticDir)
	if err != nil {
		log.Fatalf("load static data: %v", err)
	}

	statefulTracker := processorlogic.NewTracker(store, processorlogic.TrackerConfig{
		StationMatchMeters: stationMatchMeters,
		ServiceLocation:    serviceLocation,
	})

	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(consumerGroup),
		kgo.ConsumeTopics(inputTopic),
	)
	if err != nil {
		log.Fatalf("create consumer client: %v", err)
	}
	defer consumer.Close()

	producer, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		log.Fatalf("create producer client: %v", err)
	}
	defer producer.Close()

	createCtx, cancelCreate := context.WithTimeout(ctx, createTopicTimeout)
	if err := ensureTopics(createCtx, producer, observedTopic, predictedTopic); err != nil {
		cancelCreate()
		log.Fatalf("ensure delay topics: %v", err)
	}
	cancelCreate()

	publish := func(ctx context.Context, record *kgo.Record) error {
		return producer.ProduceSync(ctx, record).FirstErr()
	}

	bw := &bronzeDailyWriter{baseDir: bronzeDir}
	defer func() {
		if err := bw.Close(); err != nil {
			log.Printf("close bronze writer: %v", err)
		}
	}()

	ow := &silverObservedDailyWriter{baseDir: silverDir}
	defer func() {
		if err := ow.Close(); err != nil {
			log.Printf("close observed silver writer: %v", err)
		}
	}()

	pw := &silverPredictedDailyWriter{baseDir: silverDir}
	defer func() {
		if err := pw.Close(); err != nil {
			log.Printf("close predicted silver writer: %v", err)
		}
	}()

	log.Printf(
		"processor started: input_topic=%s observed_topic=%s predicted_topic=%s group=%s bronze_dir=%s silver_dir=%s",
		inputTopic,
		observedTopic,
		predictedTopic,
		consumerGroup,
		bronzeDir,
		silverDir,
	)

	var messageCount int64
	var publishedCount int64
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
			if !rec.Timestamp.IsZero() {
				collector.processingLagSec.Observe(time.Since(rec.Timestamp.UTC()).Seconds())
			}

			commit, published, writeErr := processRecord(ctx, bw, ow, pw, statefulTracker, publish, observedTopic, predictedTopic, collector, rec)
			if writeErr != nil {
				log.Printf("status=error stage=process partition=%d offset=%d err=%v", rec.Partition, rec.Offset, writeErr)
				return
			}

			messageCount++
			collector.messagesProcessed.Inc()
			publishedCount += published
			if commit {
				commitRecords = append(commitRecords, rec)
			}
		})

		if len(commitRecords) > 0 {
			consumer.MarkCommitRecords(commitRecords...)
		}
	}

	log.Printf(
		"processor stopped: messages=%d bronze_rows=%d observed_silver_rows=%d predicted_silver_rows=%d published=%d",
		messageCount,
		bw.rowsWritten,
		ow.rowsWritten,
		pw.rowsWritten,
		publishedCount,
	)
}

func processRecord(
	ctx context.Context,
	bw bronzeWriter,
	ow observedWriter,
	pw predictedWriter,
	track tracker,
	publish publishFunc,
	observedTopic string,
	predictedTopic string,
	collector processorMetrics,
	rec *kgo.Record,
) (bool, int64, error) {
	var payload autotrolej.AutobusiResponse
	if err := json.Unmarshal(rec.Value, &payload); err != nil {
		log.Printf("status=error stage=unmarshal partition=%d offset=%d err=%v", rec.Partition, rec.Offset, err)
		return true, 0, nil
	}

	ingestedAt := time.Now().UTC()
	observedAt := rec.Timestamp.UTC()
	if observedAt.IsZero() {
		observedAt = ingestedAt
	}

	var published int64
	for _, bus := range payload.Res {
		row := bronzePositionRow{
			IngestedAt:     ingestedAt.Format(time.RFC3339Nano),
			IngestedDate:   ingestedAt.Format("2006-01-02"),
			KafkaTopic:     rec.Topic,
			KafkaPartition: rec.Partition,
			KafkaOffset:    rec.Offset,
			Msg:            payload.Msg,
			Err:            payload.Err,
			GBR:            intPtrToInt64Ptr(bus.GBR),
			Lon:            bus.Lon,
			Lat:            bus.Lat,
			VoznjaID:       intPtrToInt64Ptr(bus.VoznjaID),
			VoznjaBusID:    intPtrToInt64Ptr(bus.VoznjaBusID),
		}
		if err := bw.Write(row); err != nil {
			return false, published, err
		}

		out := track.Track(processorlogic.TrackInput{
			ObservedAt:     observedAt,
			KafkaTopic:     rec.Topic,
			KafkaPartition: rec.Partition,
			KafkaOffset:    rec.Offset,
			Bus:            bus,
		})
		if out.SkipReason != processorlogic.SkipReasonNone {
			reason := string(out.SkipReason)
			collector.trackerSkips.WithLabelValues(reason).Inc()
			log.Printf(
				"status=skip stage=track reason=%s partition=%d offset=%d voznja_bus_id=%v",
				reason,
				rec.Partition,
				rec.Offset,
				bus.VoznjaBusID,
			)
			continue
		}

		for _, event := range out.Observed {
			silverRow := silverObservedDelayRow{
				IngestedAt:     row.IngestedAt,
				IngestedDate:   row.IngestedDate,
				TripID:         event.TripID,
				VoznjaBusID:    event.VoznjaBusID,
				GBR:            event.GBR,
				LinVarID:       event.LinVarID,
				BrojLinije:     event.BrojLinije,
				StationID:      event.StationID,
				StationName:    event.StationName,
				StationSeq:     event.StationSeq,
				ScheduledTime:  event.ScheduledTime,
				ObservedTime:   event.ObservedTime,
				DelaySeconds:   event.DelaySeconds,
				DistanceM:      event.DistanceM,
				TrackerVersion: event.TrackerVersion,
				KafkaTopic:     row.KafkaTopic,
				KafkaPartition: row.KafkaPartition,
				KafkaOffset:    row.KafkaOffset,
			}
			if err := ow.Write(silverRow); err != nil {
				return false, published, err
			}

			payloadBytes, err := json.Marshal(event)
			if err != nil {
				return false, published, fmt.Errorf("marshal observed delay event: %w", err)
			}

			key := fmt.Sprintf("%d:%d:%d:%d", event.VoznjaBusID, event.StationID, rec.Partition, rec.Offset)
			if err := publish(ctx, &kgo.Record{Topic: observedTopic, Key: []byte(key), Value: payloadBytes}); err != nil {
				return false, published, fmt.Errorf("publish observed delay event: %w", err)
			}

			collector.observedDelaySeconds.Observe(float64(event.DelaySeconds))
			collector.observedPublished.Inc()
			published++
			log.Printf(
				"status=ok stage=delay_observed_publish topic=%s partition=%d offset=%d station_id=%d delay_seconds=%d",
				observedTopic,
				rec.Partition,
				rec.Offset,
				event.StationID,
				event.DelaySeconds,
			)
		}

		for _, event := range out.Predicted {
			silverRow := silverPredictedDelayRow{
				IngestedAt:            row.IngestedAt,
				IngestedDate:          row.IngestedDate,
				TripID:                event.TripID,
				VoznjaBusID:           event.VoznjaBusID,
				LinVarID:              event.LinVarID,
				BrojLinije:            event.BrojLinije,
				StationID:             event.StationID,
				StationName:           event.StationName,
				StationSeq:            event.StationSeq,
				ScheduledTime:         event.ScheduledTime,
				PredictedTime:         event.PredictedTime,
				PredictedDelaySeconds: event.PredictedDelaySeconds,
				GeneratedAt:           event.GeneratedAt,
				TrackerVersion:        event.TrackerVersion,
				KafkaTopic:            row.KafkaTopic,
				KafkaPartition:        row.KafkaPartition,
				KafkaOffset:           row.KafkaOffset,
			}
			if err := pw.Write(silverRow); err != nil {
				return false, published, err
			}

			payloadBytes, err := json.Marshal(event)
			if err != nil {
				return false, published, fmt.Errorf("marshal predicted delay event: %w", err)
			}

			key := fmt.Sprintf("%d:%d:%d:%d", event.VoznjaBusID, event.StationID, rec.Partition, rec.Offset)
			if err := publish(ctx, &kgo.Record{Topic: predictedTopic, Key: []byte(key), Value: payloadBytes}); err != nil {
				return false, published, fmt.Errorf("publish predicted delay event: %w", err)
			}

			collector.predictedDelaySeconds.Observe(float64(event.PredictedDelaySeconds))
			collector.predictedPublished.Inc()
			published++
			log.Printf(
				"status=ok stage=delay_predicted_publish topic=%s partition=%d offset=%d station_id=%d predicted_delay_seconds=%d",
				predictedTopic,
				rec.Partition,
				rec.Offset,
				event.StationID,
				event.PredictedDelaySeconds,
			)
		}
	}

	log.Printf(
		"status=ok stage=process topic=%s partition=%d offset=%d buses=%d",
		rec.Topic,
		rec.Partition,
		rec.Offset,
		len(payload.Res),
	)

	return true, published, nil
}

func ensureTopics(ctx context.Context, client *kgo.Client, topics ...string) error {
	for _, topic := range topics {
		if err := ensureTopic(ctx, client, topic); err != nil {
			return err
		}
	}
	return nil
}

func ensureTopic(ctx context.Context, client *kgo.Client, topic string) error {
	req := kmsg.NewPtrCreateTopicsRequest()
	requestTopic := kmsg.NewCreateTopicsRequestTopic()
	requestTopic.Topic = topic
	requestTopic.NumPartitions = createTopicPartitions
	requestTopic.ReplicationFactor = createTopicReplicas
	req.Topics = append(req.Topics, requestTopic)

	res, err := requestCreateTopics(ctx, client, req)
	if err != nil {
		return fmt.Errorf("request create topic %q: %w", topic, err)
	}
	if len(res.Topics) != 1 {
		return fmt.Errorf("create topic %q: expected one topic response, got %d", topic, len(res.Topics))
	}

	topicRes := res.Topics[0]
	if err := kerr.ErrorForCode(topicRes.ErrorCode); err != nil {
		if !errors.Is(err, kerr.TopicAlreadyExists) {
			return fmt.Errorf("create topic %q: %w", topic, err)
		}
		log.Printf("status=ok stage=topic_ensure topic=%s result=already_exists", topic)
		return nil
	}

	log.Printf("status=ok stage=topic_ensure topic=%s result=created_or_verified", topic)
	return nil
}

func newProcessorMetrics() processorMetrics {
	messagesProcessed := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "arrival_processor_messages_processed_total",
		Help: "Total number of Kafka records processed by processor.",
	})

	processingLagSec := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "arrival_processor_processing_lag_seconds",
		Help:    "Lag between Kafka record timestamp and processor handling time.",
		Buckets: prometheus.DefBuckets,
	})

	observedDelaySeconds := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "arrival_processor_observed_delay_seconds",
		Help:    "Distribution of observed bus delay values in seconds.",
		Buckets: []float64{-1800, -900, -300, -120, -60, -30, 0, 30, 60, 120, 300, 600, 900, 1800, 3600},
	})

	predictedDelaySeconds := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "arrival_processor_predicted_delay_seconds",
		Help:    "Distribution of predicted bus delay values in seconds.",
		Buckets: []float64{-1800, -900, -300, -120, -60, -30, 0, 30, 60, 120, 300, 600, 900, 1800, 3600},
	})

	observedPublished := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "arrival_processor_observed_events_published_total",
		Help: "Total number of observed delay events published.",
	})

	predictedPublished := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "arrival_processor_predicted_events_published_total",
		Help: "Total number of predicted delay events published.",
	})

	trackerSkips := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "arrival_processor_tracker_skips_total",
		Help: "Total number of tracker skips grouped by skip reason.",
	}, []string{"reason"})

	prometheus.MustRegister(
		messagesProcessed,
		processingLagSec,
		observedDelaySeconds,
		predictedDelaySeconds,
		observedPublished,
		predictedPublished,
		trackerSkips,
	)

	return processorMetrics{
		messagesProcessed:     messagesProcessed,
		processingLagSec:      processingLagSec,
		observedDelaySeconds:  observedDelaySeconds,
		predictedDelaySeconds: predictedDelaySeconds,
		observedPublished:     observedPublished,
		predictedPublished:    predictedPublished,
		trackerSkips:          trackerSkips,
	}
}

func (bw *bronzeDailyWriter) Write(row bronzePositionRow) error {
	if err := bw.ensureDate(row.IngestedDate); err != nil {
		return err
	}

	if err := bw.pw.Write(row); err != nil {
		return fmt.Errorf("write parquet row: %w", err)
	}

	bw.rowsWritten++
	return nil
}

func (bw *bronzeDailyWriter) ensureDate(date string) error {
	if bw.pw != nil && bw.currentDate == date {
		return nil
	}

	if err := bw.Close(); err != nil {
		return err
	}

	dayDir := filepath.Join(bw.baseDir, date)
	if err := os.MkdirAll(dayDir, 0o755); err != nil {
		return fmt.Errorf("create bronze day dir %s: %w", dayDir, err)
	}

	filePath := filepath.Join(dayDir, "positions.parquet")
	if info, err := os.Stat(filePath); err == nil && info.Size() > 0 {
		log.Printf("status=warn stage=writer file=%s msg=existing file will be overwritten", filePath)
	}

	fw, err := local.NewLocalFileWriter(filePath)
	if err != nil {
		return fmt.Errorf("open parquet file writer %s: %w", filePath, err)
	}

	pw, err := writer.NewParquetWriter(fw, new(bronzePositionRow), 1)
	if err != nil {
		_ = fw.Close()
		return fmt.Errorf("create parquet writer %s: %w", filePath, err)
	}
	pw.CompressionType = parquet.CompressionCodec_SNAPPY

	bw.currentDate = date
	bw.currentPath = filePath
	bw.pw = pw

	log.Printf("status=ok stage=writer_open file=%s", filePath)
	return nil
}

func (bw *bronzeDailyWriter) Close() error {
	if bw.pw == nil {
		return nil
	}

	if err := bw.pw.WriteStop(); err != nil {
		return fmt.Errorf("close parquet writer %s: %w", bw.currentPath, err)
	}

	log.Printf("status=ok stage=writer_close file=%s", bw.currentPath)
	bw.pw = nil
	bw.currentDate = ""
	bw.currentPath = ""
	return nil
}

func (sw *silverObservedDailyWriter) Write(row silverObservedDelayRow) error {
	if err := sw.ensureDate(row.IngestedDate); err != nil {
		return err
	}

	if err := sw.pw.Write(row); err != nil {
		return fmt.Errorf("write observed silver parquet row: %w", err)
	}

	sw.rowsWritten++
	return nil
}

func (sw *silverObservedDailyWriter) ensureDate(date string) error {
	if sw.pw != nil && sw.currentDate == date {
		return nil
	}

	if err := sw.Close(); err != nil {
		return err
	}

	dayDir := filepath.Join(sw.baseDir, date)
	if err := os.MkdirAll(dayDir, 0o755); err != nil {
		return fmt.Errorf("create observed silver day dir %s: %w", dayDir, err)
	}

	filePath := filepath.Join(dayDir, observedSilverFilename)
	fw, err := local.NewLocalFileWriter(filePath)
	if err != nil {
		return fmt.Errorf("open observed silver parquet file writer %s: %w", filePath, err)
	}

	pw, err := writer.NewParquetWriter(fw, new(silverObservedDelayRow), 1)
	if err != nil {
		_ = fw.Close()
		return fmt.Errorf("create observed silver parquet writer %s: %w", filePath, err)
	}
	pw.CompressionType = parquet.CompressionCodec_SNAPPY

	sw.currentDate = date
	sw.currentPath = filePath
	sw.pw = pw

	log.Printf("status=ok stage=observed_silver_writer_open file=%s", filePath)
	return nil
}

func (sw *silverObservedDailyWriter) Close() error {
	if sw.pw == nil {
		return nil
	}

	if err := sw.pw.WriteStop(); err != nil {
		return fmt.Errorf("close observed silver parquet writer %s: %w", sw.currentPath, err)
	}

	log.Printf("status=ok stage=observed_silver_writer_close file=%s", sw.currentPath)
	sw.pw = nil
	sw.currentDate = ""
	sw.currentPath = ""
	return nil
}

func (sw *silverPredictedDailyWriter) Write(row silverPredictedDelayRow) error {
	if err := sw.ensureDate(row.IngestedDate); err != nil {
		return err
	}

	if err := sw.pw.Write(row); err != nil {
		return fmt.Errorf("write predicted silver parquet row: %w", err)
	}

	sw.rowsWritten++
	return nil
}

func (sw *silverPredictedDailyWriter) ensureDate(date string) error {
	if sw.pw != nil && sw.currentDate == date {
		return nil
	}

	if err := sw.Close(); err != nil {
		return err
	}

	dayDir := filepath.Join(sw.baseDir, date)
	if err := os.MkdirAll(dayDir, 0o755); err != nil {
		return fmt.Errorf("create predicted silver day dir %s: %w", dayDir, err)
	}

	filePath := filepath.Join(dayDir, predictedSilverFilename)
	fw, err := local.NewLocalFileWriter(filePath)
	if err != nil {
		return fmt.Errorf("open predicted silver parquet file writer %s: %w", filePath, err)
	}

	pw, err := writer.NewParquetWriter(fw, new(silverPredictedDelayRow), 1)
	if err != nil {
		_ = fw.Close()
		return fmt.Errorf("create predicted silver parquet writer %s: %w", filePath, err)
	}
	pw.CompressionType = parquet.CompressionCodec_SNAPPY

	sw.currentDate = date
	sw.currentPath = filePath
	sw.pw = pw

	log.Printf("status=ok stage=predicted_silver_writer_open file=%s", filePath)
	return nil
}

func (sw *silverPredictedDailyWriter) Close() error {
	if sw.pw == nil {
		return nil
	}

	if err := sw.pw.WriteStop(); err != nil {
		return fmt.Errorf("close predicted silver parquet writer %s: %w", sw.currentPath, err)
	}

	log.Printf("status=ok stage=predicted_silver_writer_close file=%s", sw.currentPath)
	sw.pw = nil
	sw.currentDate = ""
	sw.currentPath = ""
	return nil
}

func intPtrToInt64Ptr(value *int) *int64 {
	if value == nil {
		return nil
	}
	v := int64(*value)
	return &v
}
