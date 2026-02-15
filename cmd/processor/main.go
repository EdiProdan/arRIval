package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/EdiProdan/arRIval/internal/autotrolej"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/xitongsys/parquet-go-source/local"
	"github.com/xitongsys/parquet-go/parquet"
	"github.com/xitongsys/parquet-go/writer"
)

const (
	defaultBrokers       = "localhost:19092"
	defaultInputTopic    = "bus-positions-raw"
	defaultConsumerGroup = "arrival-processor-bronze"
	defaultBronzeDir     = "data/bronze"
)

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

func main() {
	if err := loadDotEnv(".env"); err != nil {
		log.Fatalf("load .env: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	brokers := splitCSV(getenv("ARRIVAL_KAFKA_BROKERS", defaultBrokers))
	inputTopic := getenv("ARRIVAL_KAFKA_TOPIC", defaultInputTopic)
	consumerGroup := getenv("ARRIVAL_PROCESSOR_GROUP", defaultConsumerGroup)
	bronzeDir := getenv("ARRIVAL_BRONZE_DIR", defaultBronzeDir)

	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(consumerGroup),
		kgo.ConsumeTopics(inputTopic),
	)
	if err != nil {
		log.Fatalf("create consumer client: %v", err)
	}
	defer consumer.Close()

	bw := &bronzeDailyWriter{baseDir: bronzeDir}
	defer func() {
		if err := bw.Close(); err != nil {
			log.Printf("close bronze writer: %v", err)
		}
	}()

	log.Printf("processor started: input_topic=%s group=%s bronze_dir=%s", inputTopic, consumerGroup, bronzeDir)

	var messageCount int64
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
			commit, writeErr := processRecord(bw, rec)
			if writeErr != nil {
				log.Printf("status=error stage=process partition=%d offset=%d err=%v", rec.Partition, rec.Offset, writeErr)
				return
			}

			messageCount++
			if commit {
				commitRecords = append(commitRecords, rec)
			}
		})

		if len(commitRecords) > 0 {
			consumer.MarkCommitRecords(commitRecords...)
		}
	}

	log.Printf("processor stopped: messages=%d rows=%d", messageCount, bw.rowsWritten)
}

func processRecord(bw *bronzeDailyWriter, rec *kgo.Record) (bool, error) {
	var payload autotrolej.AutobusiResponse
	if err := json.Unmarshal(rec.Value, &payload); err != nil {
		log.Printf("status=error stage=unmarshal partition=%d offset=%d err=%v", rec.Partition, rec.Offset, err)
		return true, nil
	}

	ingestedAt := time.Now().UTC()
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
			return false, err
		}
	}

	log.Printf(
		"status=ok stage=process topic=%s partition=%d offset=%d buses=%d",
		rec.Topic,
		rec.Partition,
		rec.Offset,
		len(payload.Res),
	)

	return true, nil
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

func intPtrToInt64Ptr(value *int) *int64 {
	if value == nil {
		return nil
	}
	v := int64(*value)
	return &v
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
