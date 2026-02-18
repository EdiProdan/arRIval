package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/EdiProdan/arRIval/internal/autotrolej"
	"github.com/EdiProdan/arRIval/internal/envutil"
	"github.com/EdiProdan/arRIval/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	defaultAPIBaseURL  = "https://www.rijekaplus.hr"
	defaultBrokers     = "localhost:19092"
	defaultTopic       = "bus-positions-raw"
	defaultMetricsAddr = ":9101"
	pollInterval       = 30 * time.Second
	produceTimeout     = 15 * time.Second
)

type ingesterMetrics struct {
	pollTotal      prometheus.Counter
	apiLatencySec  prometheus.Histogram
	errorTotalByOp *prometheus.CounterVec
}

func main() {
	if err := envutil.LoadDotEnv(".env"); err != nil {
		log.Fatalf("load .env: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	apiBaseURL := envutil.StringEnv("ARRIVAL_API_BASE_URL", defaultAPIBaseURL)
	username := envutil.FirstNonEmpty(os.Getenv("ARRIVAL_API_USER"), os.Getenv("ARRIVAL_API_USERNAME"))
	password := envutil.FirstNonEmpty(os.Getenv("ARRIVAL_API_PASS"), os.Getenv("ARRIVAL_API_PASSWORD"))
	brokers := envutil.CSVEnv("ARRIVAL_KAFKA_BROKERS", defaultBrokers)
	topic := envutil.StringEnv("ARRIVAL_KAFKA_TOPIC", defaultTopic)
	metricsAddr := envutil.StringEnv("ARRIVAL_INGESTER_METRICS_ADDR", defaultMetricsAddr)

	collector := newIngesterMetrics()
	metrics.StartServer(ctx, metricsAddr)

	apiClient, err := autotrolej.NewClient(autotrolej.Config{
		BaseURL:  apiBaseURL,
		Username: username,
		Password: password,
	})
	if err != nil {
		log.Fatalf("create API client: %v", err)
	}

	producer, err := kgo.NewClient(kgo.SeedBrokers(brokers...))
	if err != nil {
		log.Fatalf("create producer client: %v", err)
	}
	defer producer.Close()

	log.Printf("ingester started: topic=%s poll_interval=%s", topic, pollInterval)

	pollNumber := 0
	for {
		if ctx.Err() != nil {
			break
		}

		pollNumber++
		polledAt := time.Now().UTC()
		collector.pollTotal.Inc()

		apiStart := time.Now()
		response, err := apiClient.GetAutobusi(ctx)
		collector.apiLatencySec.Observe(time.Since(apiStart).Seconds())
		if err != nil {
			collector.errorTotalByOp.WithLabelValues("fetch").Inc()
			log.Printf("poll=%d status=error stage=fetch err=%v", pollNumber, err)
		} else {
			payload, marshalErr := json.Marshal(response)
			if marshalErr != nil {
				collector.errorTotalByOp.WithLabelValues("marshal").Inc()
				log.Printf("poll=%d status=error stage=marshal err=%v", pollNumber, marshalErr)
			} else {
				record := &kgo.Record{Topic: topic, Value: payload}
				publishCtx, cancelPublish := context.WithTimeout(ctx, produceTimeout)
				produceErr := producer.ProduceSync(publishCtx, record).FirstErr()
				cancelPublish()

				if produceErr != nil {
					collector.errorTotalByOp.WithLabelValues("produce").Inc()
					log.Printf("poll=%d status=error stage=produce err=%v", pollNumber, produceErr)
				} else {
					log.Printf(
						"poll=%d status=ok buses=%d produced_topic=%s partition=%d offset=%d polled_at=%s",
						pollNumber,
						len(response.Res),
						record.Topic,
						record.Partition,
						record.Offset,
						polledAt.Format(time.RFC3339Nano),
					)
				}
			}
		}

		if !sleepOrDone(ctx, pollInterval) {
			break
		}
	}

	log.Printf("ingester stopped")
}

func newIngesterMetrics() ingesterMetrics {
	pollTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "arrival_ingester_poll_total",
		Help: "Total number of ingester poll attempts.",
	})

	apiLatencySec := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "arrival_ingester_api_latency_seconds",
		Help:    "API request latency for /autobusi calls.",
		Buckets: prometheus.DefBuckets,
	})

	errorTotalByOp := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "arrival_ingester_errors_total",
		Help: "Total ingester errors by operation.",
	}, []string{"stage"})

	prometheus.MustRegister(pollTotal, apiLatencySec, errorTotalByOp)

	return ingesterMetrics{
		pollTotal:      pollTotal,
		apiLatencySec:  apiLatencySec,
		errorTotalByOp: errorTotalByOp,
	}
}

func sleepOrDone(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
