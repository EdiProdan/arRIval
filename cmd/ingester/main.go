package main

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
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
	defaultAPIBaseURL       = "https://www.rijekaplus.hr"
	defaultBrokers          = "localhost:19092"
	defaultTopic            = "bus-positions-raw"
	defaultMetricsAddr      = ":9101"
	defaultPollInterval     = 30 * time.Second
	defaultMaxBackoff       = 2 * time.Minute
	produceTimeout          = 15 * time.Second
	minBackoffMultiplier    = 0.8
	backoffJitterMultiplier = 0.4
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
	pollInterval, maxBackoff := loadIngesterTimingConfig()

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

	log.Printf("ingester started: topic=%s poll_interval=%s max_backoff=%s", topic, pollInterval, maxBackoff)

	pollNumber := 0
	consecutiveFailures := 0
	for {
		if ctx.Err() != nil {
			break
		}

		pollNumber++
		polledAt := time.Now().UTC()
		collector.pollTotal.Inc()
		pollFailed := false

		apiStart := time.Now()
		response, err := apiClient.GetAutobusi(ctx)
		collector.apiLatencySec.Observe(time.Since(apiStart).Seconds())
		if err != nil {
			collector.errorTotalByOp.WithLabelValues("fetch").Inc()
			log.Printf("poll=%d status=error stage=fetch err=%v", pollNumber, err)
			pollFailed = true
		} else {
			payload, marshalErr := json.Marshal(response)
			if marshalErr != nil {
				collector.errorTotalByOp.WithLabelValues("marshal").Inc()
				log.Printf("poll=%d status=error stage=marshal err=%v", pollNumber, marshalErr)
				pollFailed = true
			} else {
				record := &kgo.Record{Topic: topic, Value: payload}
				publishCtx, cancelPublish := context.WithTimeout(ctx, produceTimeout)
				produceErr := producer.ProduceSync(publishCtx, record).FirstErr()
				cancelPublish()

				if produceErr != nil {
					collector.errorTotalByOp.WithLabelValues("produce").Inc()
					log.Printf("poll=%d status=error stage=produce err=%v", pollNumber, produceErr)
					pollFailed = true
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

		if pollFailed {
			consecutiveFailures++
		} else {
			consecutiveFailures = 0
		}

		sleepDelay := nextPollDelay(pollInterval, maxBackoff, consecutiveFailures, rand.Float64)
		if !sleepOrDone(ctx, sleepDelay) {
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

func loadIngesterTimingConfig() (time.Duration, time.Duration) {
	pollInterval, invalidPollInterval := envutil.DurationEnv("ARRIVAL_INGESTER_POLL_INTERVAL", defaultPollInterval)
	if invalidPollInterval {
		log.Printf(
			"invalid duration for ARRIVAL_INGESTER_POLL_INTERVAL=%q, using fallback %s",
			os.Getenv("ARRIVAL_INGESTER_POLL_INTERVAL"),
			defaultPollInterval,
		)
	}

	maxBackoff, invalidMaxBackoff := envutil.DurationEnv("ARRIVAL_INGESTER_MAX_BACKOFF", defaultMaxBackoff)
	if invalidMaxBackoff {
		log.Printf(
			"invalid duration for ARRIVAL_INGESTER_MAX_BACKOFF=%q, using fallback %s",
			os.Getenv("ARRIVAL_INGESTER_MAX_BACKOFF"),
			defaultMaxBackoff,
		)
	}

	if maxBackoff < pollInterval {
		maxBackoff = pollInterval
	}

	return pollInterval, maxBackoff
}

func nextPollDelay(
	pollInterval time.Duration,
	maxBackoff time.Duration,
	consecutiveFailures int,
	randomFn func() float64,
) time.Duration {
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}
	if maxBackoff <= 0 {
		maxBackoff = defaultMaxBackoff
	}
	if maxBackoff < pollInterval {
		maxBackoff = pollInterval
	}
	if consecutiveFailures <= 0 {
		return pollInterval
	}

	backoff := pollInterval
	for step := 1; step < consecutiveFailures; step++ {
		if backoff >= maxBackoff {
			break
		}
		if backoff > maxBackoff/2 {
			backoff = maxBackoff
			break
		}
		backoff *= 2
	}
	if backoff > maxBackoff {
		backoff = maxBackoff
	}

	if randomFn == nil {
		randomFn = rand.Float64
	}
	random := randomFn()
	if random < 0 {
		random = 0
	}
	if random > 1 {
		random = 1
	}

	jitter := minBackoffMultiplier + random*backoffJitterMultiplier
	delay := time.Duration(float64(backoff) * jitter)
	if delay > maxBackoff {
		return maxBackoff
	}
	if delay <= 0 {
		return time.Millisecond
	}
	return delay
}
