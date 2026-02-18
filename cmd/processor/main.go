package main

import (
	"context"
	"encoding/json"
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
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/parquet"
	"github.com/xitongsys/parquet-go/writer"
)

const (
	defaultBrokers       = "localhost:19092"
	defaultInputTopic    = "bus-positions-raw"
	defaultOutputTopic   = "bus-delays"
	defaultConsumerGroup = "arrival-processor-bronze"
	defaultBronzeDir     = "data/bronze"
	defaultSilverDir     = "data/silver"
	defaultStaticDir     = "data"
	defaultMetricsAddr   = ":9102"
	stationMatchMeters   = 100.0
	scheduleWindow       = 15 * time.Minute
)

var serviceLocation = func() *time.Location {
	loc, err := time.LoadLocation("Europe/Zagreb")
	if err != nil {
		panic("failed to load Europe/Zagreb timezone: " + err.Error())
	}
	return loc
}()

type processorMetrics struct {
	messagesProcessed prometheus.Counter
	processingLagSec  prometheus.Histogram
	delaySeconds      prometheus.Histogram
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

type silverDelayRow struct {
	IngestedAt     string  `parquet:"name=ingested_at, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	IngestedDate   string  `parquet:"name=ingested_date, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	PolazakID      string  `parquet:"name=polazak_id, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	VoznjaBusID    int64   `parquet:"name=voznja_bus_id, type=INT64"`
	GBR            *int64  `parquet:"name=gbr, type=INT64, repetitiontype=OPTIONAL"`
	StationID      int64   `parquet:"name=station_id, type=INT64"`
	StationName    string  `parquet:"name=station_name, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	DistanceM      float64 `parquet:"name=distance_m, type=DOUBLE"`
	LinVarID       string  `parquet:"name=lin_var_id, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	BrojLinije     string  `parquet:"name=broj_linije, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	ScheduledTime  string  `parquet:"name=scheduled_time, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	ActualTime     string  `parquet:"name=actual_time, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	DelaySeconds   int64   `parquet:"name=delay_seconds, type=INT64"`
	KafkaTopic     string  `parquet:"name=kafka_topic, type=BYTE_ARRAY, convertedtype=UTF8, encoding=PLAIN_DICTIONARY"`
	KafkaPartition int32   `parquet:"name=kafka_partition, type=INT32"`
	KafkaOffset    int64   `parquet:"name=kafka_offset, type=INT64"`
}

type silverDailyWriter struct {
	baseDir     string
	currentDate string
	currentPath string
	pw          *writer.ParquetWriter
	rowsWritten int64
}

func main() {
	if err := envutil.LoadDotEnv(".env"); err != nil {
		log.Fatalf("load .env: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	brokers := envutil.CSVEnv("ARRIVAL_KAFKA_BROKERS", defaultBrokers)
	inputTopic := envutil.StringEnv("ARRIVAL_KAFKA_TOPIC", defaultInputTopic)
	outputTopic := envutil.StringEnv("ARRIVAL_KAFKA_DELAY_TOPIC", defaultOutputTopic)
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

	bw := &bronzeDailyWriter{baseDir: bronzeDir}
	defer func() {
		if err := bw.Close(); err != nil {
			log.Printf("close bronze writer: %v", err)
		}
	}()

	sw := &silverDailyWriter{baseDir: silverDir}
	defer func() {
		if err := sw.Close(); err != nil {
			log.Printf("close silver writer: %v", err)
		}
	}()

	log.Printf("processor started: input_topic=%s output_topic=%s group=%s bronze_dir=%s silver_dir=%s", inputTopic, outputTopic, consumerGroup, bronzeDir, silverDir)

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
				collector.processingLagSec.Observe(time.Since(rec.Timestamp).Seconds())
			}

			commit, published, writeErr := processRecord(ctx, bw, sw, producer, outputTopic, store, collector, rec)
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

	log.Printf("processor stopped: messages=%d bronze_rows=%d silver_rows=%d published=%d", messageCount, bw.rowsWritten, sw.rowsWritten, publishedCount)
}

func processRecord(ctx context.Context, bw *bronzeDailyWriter, sw *silverDailyWriter, producer *kgo.Client, outputTopic string, store *staticdata.Store, collector processorMetrics, rec *kgo.Record) (bool, int64, error) {
	var payload autotrolej.AutobusiResponse
	if err := json.Unmarshal(rec.Value, &payload); err != nil {
		log.Printf("status=error stage=unmarshal partition=%d offset=%d err=%v", rec.Partition, rec.Offset, err)
		return true, 0, nil
	}

	ingestedAt := time.Now().UTC()
	matchCfg := processorlogic.Config{
		StationMatchMeters: stationMatchMeters,
		ScheduleWindow:     scheduleWindow,
		ServiceLocation:    serviceLocation,
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

		match, ok, reason := processorlogic.BuildDelay(processorlogic.MatchInput{
			IngestedAt:     ingestedAt,
			KafkaTopic:     rec.Topic,
			KafkaPartition: rec.Partition,
			KafkaOffset:    rec.Offset,
			Bus:            bus,
		}, store, matchCfg)
		if !ok {
			log.Printf("status=skip stage=delay_match reason=%s partition=%d offset=%d voznja_bus_id=%v", reason, rec.Partition, rec.Offset, bus.VoznjaBusID)
			continue
		}

		silverRow := silverDelayRow{
			IngestedAt:     row.IngestedAt,
			IngestedDate:   row.IngestedDate,
			PolazakID:      match.PolazakID,
			VoznjaBusID:    match.VoznjaBusID,
			GBR:            match.GBR,
			StationID:      match.StationID,
			StationName:    match.StationName,
			DistanceM:      match.DistanceM,
			LinVarID:       match.LinVarID,
			BrojLinije:     match.BrojLinije,
			ScheduledTime:  match.ScheduledTime.Format(time.RFC3339Nano),
			ActualTime:     match.ActualTime.Format(time.RFC3339Nano),
			DelaySeconds:   match.DelaySeconds,
			KafkaTopic:     row.KafkaTopic,
			KafkaPartition: row.KafkaPartition,
			KafkaOffset:    row.KafkaOffset,
		}
		if err := sw.Write(silverRow); err != nil {
			return false, published, err
		}

		event := contracts.DelayEvent{
			PolazakID:     match.PolazakID,
			VoznjaBusID:   match.VoznjaBusID,
			GBR:           match.GBR,
			StationID:     match.StationID,
			StationName:   match.StationName,
			DistanceM:     match.DistanceM,
			LinVarID:      match.LinVarID,
			BrojLinije:    match.BrojLinije,
			ScheduledTime: match.ScheduledTime.Format(time.RFC3339Nano),
			ActualTime:    match.ActualTime.Format(time.RFC3339Nano),
			DelaySeconds:  match.DelaySeconds,
		}
		payloadBytes, err := json.Marshal(event)
		if err != nil {
			return false, published, fmt.Errorf("marshal delay event: %w", err)
		}

		key := fmt.Sprintf("%d:%d:%d", event.VoznjaBusID, rec.Partition, rec.Offset)
		if err := producer.ProduceSync(ctx, &kgo.Record{Topic: outputTopic, Key: []byte(key), Value: payloadBytes}).FirstErr(); err != nil {
			return false, published, fmt.Errorf("publish delay event: %w", err)
		}

		collector.delaySeconds.Observe(float64(event.DelaySeconds))
		published++
		log.Printf("status=ok stage=delay_publish topic=%s partition=%d offset=%d station_id=%d delay_seconds=%d", outputTopic, rec.Partition, rec.Offset, event.StationID, event.DelaySeconds)
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

	delaySeconds := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "arrival_processor_delay_seconds",
		Help:    "Distribution of computed bus delay values in seconds.",
		Buckets: []float64{-1800, -900, -300, -120, -60, -30, 0, 30, 60, 120, 300, 600, 900, 1800, 3600},
	})

	prometheus.MustRegister(messagesProcessed, processingLagSec, delaySeconds)

	return processorMetrics{
		messagesProcessed: messagesProcessed,
		processingLagSec:  processingLagSec,
		delaySeconds:      delaySeconds,
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

func (sw *silverDailyWriter) Write(row silverDelayRow) error {
	if err := sw.ensureDate(row.IngestedDate); err != nil {
		return err
	}

	if err := sw.pw.Write(row); err != nil {
		return fmt.Errorf("write silver parquet row: %w", err)
	}

	sw.rowsWritten++
	return nil
}

func (sw *silverDailyWriter) ensureDate(date string) error {
	if sw.pw != nil && sw.currentDate == date {
		return nil
	}

	if err := sw.Close(); err != nil {
		return err
	}

	dayDir := filepath.Join(sw.baseDir, date)
	if err := os.MkdirAll(dayDir, 0o755); err != nil {
		return fmt.Errorf("create silver day dir %s: %w", dayDir, err)
	}

	filePath := filepath.Join(dayDir, "delays.parquet")
	fw, err := local.NewLocalFileWriter(filePath)
	if err != nil {
		return fmt.Errorf("open silver parquet file writer %s: %w", filePath, err)
	}

	pw, err := writer.NewParquetWriter(fw, new(silverDelayRow), 1)
	if err != nil {
		_ = fw.Close()
		return fmt.Errorf("create silver parquet writer %s: %w", filePath, err)
	}
	pw.CompressionType = parquet.CompressionCodec_SNAPPY

	sw.currentDate = date
	sw.currentPath = filePath
	sw.pw = pw

	log.Printf("status=ok stage=silver_writer_open file=%s", filePath)
	return nil
}

func (sw *silverDailyWriter) Close() error {
	if sw.pw == nil {
		return nil
	}

	if err := sw.pw.WriteStop(); err != nil {
		return fmt.Errorf("close silver parquet writer %s: %w", sw.currentPath, err)
	}

	log.Printf("status=ok stage=silver_writer_close file=%s", sw.currentPath)
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
