package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	defaultBrokers = "localhost:19092"
	defaultTopic   = "bus-positions-raw"
)

func main() {
	brokers := splitCSV(getenv("ARRIVAL_KAFKA_BROKERS", defaultBrokers))
	topic := getenv("ARRIVAL_KAFKA_TOPIC", defaultTopic)
	messageID := fmt.Sprintf("step0-%d", time.Now().UTC().UnixNano())

	producer, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
	)
	if err != nil {
		log.Fatalf("create producer client: %v", err)
	}
	defer producer.Close()

	payload := map[string]any{
		"message_id":  messageID,
		"emitted_at":  time.Now().UTC().Format(time.RFC3339Nano),
		"description": "step0 redpanda roundtrip validation",
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		log.Fatalf("marshal payload: %v", err)
	}

	publishCtx, cancelPublish := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelPublish()

	record := &kgo.Record{
		Topic: topic,
		Key:   []byte(messageID),
		Value: payloadBytes,
	}

	if err := producer.ProduceSync(publishCtx, record).FirstErr(); err != nil {
		log.Fatalf("produce message: %v", err)
	}

	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumePartitions(map[string]map[int32]kgo.Offset{
			topic: {
				record.Partition: kgo.NewOffset().At(record.Offset),
			},
		}),
	)
	if err != nil {
		log.Fatalf("create consumer client: %v", err)
	}
	defer consumer.Close()

	consumeCtx, cancelConsume := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancelConsume()

	for {
		if err := consumeCtx.Err(); err != nil {
			log.Fatalf("consume timeout waiting for roundtrip message id %q", messageID)
		}

		fetches := consumer.PollFetches(consumeCtx)
		if errs := fetches.Errors(); len(errs) > 0 {
			log.Fatalf("consume error: %v", errs[0].Err)
		}

		found := false
		fetches.EachRecord(func(rec *kgo.Record) {
			if rec.Topic == topic && string(rec.Key) == messageID {
				found = true
				fmt.Printf("ROUNDTRIP_OK topic=%s partition=%d offset=%d key=%s\n", rec.Topic, rec.Partition, rec.Offset, rec.Key)
				fmt.Printf("payload=%s\n", string(rec.Value))
			}
		})

		if found {
			return
		}
	}
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
