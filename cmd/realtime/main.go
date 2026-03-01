package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/EdiProdan/arRIval/internal/contracts"
	"github.com/EdiProdan/arRIval/internal/envutil"
	"github.com/EdiProdan/arRIval/internal/metrics"
	"github.com/EdiProdan/arRIval/internal/realtime"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/twmb/franz-go/pkg/kgo"
)

const (
	defaultBrokers              = "localhost:19092"
	defaultPositionsTopic       = "bus-positions-raw"
	defaultObservedDelaysTopic  = contracts.TopicBusDelayObserved
	defaultPredictedDelaysTopic = contracts.TopicBusDelayPredicted
	defaultConsumerGroup        = "arrival-realtime"
	defaultHTTPAddr             = ":8080"
	defaultMetricsAddr          = ":9104"
	defaultStaticDir            = "data"

	defaultPositionsTTL   = 5 * time.Minute
	defaultDelaysTTL      = 90 * time.Minute
	defaultPingInterval   = 20 * time.Second
	defaultSourceInterval = 30 * time.Second

	httpShutdownTimeout = 5 * time.Second
)

type realtimeMetrics struct {
	kafkaRecordsTotal     *prometheus.CounterVec
	wsConnectionsCurrent  prometheus.Gauge
	wsConnectionsTotal    prometheus.Counter
	wsDisconnectsTotal    *prometheus.CounterVec
	wsMessagesSentTotal   *prometheus.CounterVec
	wsDroppedTotal        *prometheus.CounterVec
	snapshotRequestsTotal prometheus.Counter
	statePositions        prometheus.Gauge
	stateObservedDelays   prometheus.Gauge
	statePredictedDelays  prometheus.Gauge
	invalidPositionsTotal prometheus.Counter
}

func main() {
	if err := envutil.LoadDotEnv(".env"); err != nil {
		log.Fatalf("load .env: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	brokers := envutil.CSVEnv("ARRIVAL_KAFKA_BROKERS", defaultBrokers)
	positionsTopic := envutil.StringEnv("ARRIVAL_KAFKA_TOPIC", defaultPositionsTopic)
	observedDelaysTopic := envutil.StringEnv("ARRIVAL_KAFKA_DELAY_OBSERVED_TOPIC", defaultObservedDelaysTopic)
	predictedDelaysTopic := envutil.StringEnv("ARRIVAL_KAFKA_DELAY_PREDICTED_TOPIC", defaultPredictedDelaysTopic)
	consumerGroup := envutil.StringEnv("ARRIVAL_REALTIME_GROUP", defaultConsumerGroup)
	httpAddr := envutil.StringEnv("ARRIVAL_REALTIME_HTTP_ADDR", defaultHTTPAddr)
	metricsAddr := envutil.StringEnv("ARRIVAL_REALTIME_METRICS_ADDR", defaultMetricsAddr)
	staticDir := envutil.StringEnv("ARRIVAL_STATIC_DIR", defaultStaticDir)
	wsClientBuffer := envutil.IntEnv("ARRIVAL_REALTIME_WS_CLIENT_BUFFER", 128)

	positionsTTL, invalidPositionsTTL := envutil.DurationEnv("ARRIVAL_REALTIME_POSITIONS_TTL", defaultPositionsTTL)
	if invalidPositionsTTL {
		log.Printf("invalid duration for ARRIVAL_REALTIME_POSITIONS_TTL=%q, using fallback %s", os.Getenv("ARRIVAL_REALTIME_POSITIONS_TTL"), defaultPositionsTTL)
	}
	delaysTTL, invalidDelaysTTL := envutil.DurationEnv("ARRIVAL_REALTIME_DELAYS_TTL", defaultDelaysTTL)
	if invalidDelaysTTL {
		log.Printf("invalid duration for ARRIVAL_REALTIME_DELAYS_TTL=%q, using fallback %s", os.Getenv("ARRIVAL_REALTIME_DELAYS_TTL"), defaultDelaysTTL)
	}
	pingInterval, invalidPingInterval := envutil.DurationEnv("ARRIVAL_REALTIME_WS_PING_INTERVAL", defaultPingInterval)
	if invalidPingInterval {
		log.Printf("invalid duration for ARRIVAL_REALTIME_WS_PING_INTERVAL=%q, using fallback %s", os.Getenv("ARRIVAL_REALTIME_WS_PING_INTERVAL"), defaultPingInterval)
	}
	sourceInterval, invalidSourceInterval := envutil.DurationEnv("ARRIVAL_INGESTER_POLL_INTERVAL", defaultSourceInterval)
	if invalidSourceInterval {
		log.Printf("invalid duration for ARRIVAL_INGESTER_POLL_INTERVAL=%q, using fallback %s", os.Getenv("ARRIVAL_INGESTER_POLL_INTERVAL"), defaultSourceInterval)
	}

	collector := newRealtimeMetrics()
	metrics.StartServer(ctx, metricsAddr)

	store := realtime.NewStore(realtime.StoreConfig{
		PositionsTTL: positionsTTL,
		DelaysTTL:    delaysTTL,
	})

	hub := realtime.NewHub(realtime.HubConfig{
		PingInterval: pingInterval,
		ClientBuffer: wsClientBuffer,
		Callbacks: realtime.HubCallbacks{
			OnConnect: func() {
				collector.wsConnectionsCurrent.Inc()
				collector.wsConnectionsTotal.Inc()
			},
			OnDisconnect: func(reason string) {
				collector.wsConnectionsCurrent.Dec()
				collector.wsDisconnectsTotal.WithLabelValues(reason).Inc()
			},
			OnMessageSent: func(messageType string) {
				collector.wsMessagesSentTotal.WithLabelValues(messageType).Inc()
			},
			OnDropped: func(reason string) {
				collector.wsDroppedTotal.WithLabelValues(reason).Inc()
			},
		},
	})

	server := realtime.NewServer(realtime.ServerConfig{
		Store:             store,
		Hub:               hub,
		StationsPath:      filepath.Join(staticDir, "stanice.json"),
		LineMapPath:       filepath.Join(staticDir, "voznired_dnevni.json"),
		TimetablePath:     filepath.Join(staticDir, "voznired_dnevni.json"),
		SourceInterval:    sourceInterval,
		HeartbeatInterval: pingInterval,
		Callbacks: realtime.ServerCallbacks{
			OnKafkaRecord: func(topic string) {
				collector.kafkaRecordsTotal.WithLabelValues(topic).Inc()
			},
			OnInvalidPositions: func(count int) {
				collector.invalidPositionsTotal.Add(float64(count))
			},
			OnSnapshotRequest: func() {
				collector.snapshotRequestsTotal.Inc()
			},
			OnStateChange: func(positions, observed, predicted int) {
				collector.statePositions.Set(float64(positions))
				collector.stateObservedDelays.Set(float64(observed))
				collector.statePredictedDelays.Set(float64(predicted))
			},
		},
	})

	httpServer := &http.Server{
		Addr:    httpAddr,
		Handler: server.Routes(),
	}

	go func() {
		log.Printf("realtime HTTP server started addr=%s", httpAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("status=error stage=http_listen err=%v", err)
			stop()
		}
	}()

	consumer, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(consumerGroup),
		kgo.ConsumeTopics(positionsTopic, observedDelaysTopic, predictedDelaysTopic),
	)
	if err != nil {
		log.Fatalf("create consumer client: %v", err)
	}
	defer consumer.Close()

	server.SetReady(true)
	log.Printf(
		"realtime started: group=%s positions_topic=%s observed_delays_topic=%s predicted_delays_topic=%s positions_ttl=%s delays_ttl=%s ping_interval=%s source_interval=%s",
		consumerGroup,
		positionsTopic,
		observedDelaysTopic,
		predictedDelaysTopic,
		positionsTTL,
		delaysTTL,
		pingInterval,
		sourceInterval,
	)

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
			observedAt := rec.Timestamp.UTC()
			if observedAt.IsZero() {
				observedAt = time.Now().UTC()
			}

			var handleErr error
			switch rec.Topic {
			case positionsTopic:
				handleErr = server.HandlePositionsRecord(rec.Topic, rec.Value, observedAt)
			case observedDelaysTopic:
				handleErr = server.HandleObservedDelayRecord(rec.Topic, rec.Value, observedAt)
			case predictedDelaysTopic:
				handleErr = server.HandlePredictedDelayRecord(rec.Topic, rec.Value, observedAt)
			default:
				log.Printf("status=warn stage=consume topic=%s partition=%d offset=%d msg=unknown topic", rec.Topic, rec.Partition, rec.Offset)
			}

			if handleErr != nil {
				log.Printf("status=error stage=handle topic=%s partition=%d offset=%d err=%v", rec.Topic, rec.Partition, rec.Offset, handleErr)
			}

			consumed++
			commitRecords = append(commitRecords, rec)
		})

		if len(commitRecords) > 0 {
			consumer.MarkCommitRecords(commitRecords...)
		}
	}

	server.SetReady(false)
	server.Close()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), httpShutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("status=error stage=http_shutdown err=%v", err)
	}

	log.Printf("realtime stopped: consumed=%d", consumed)
}

func newRealtimeMetrics() realtimeMetrics {
	kafkaRecordsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "arrival_realtime_kafka_records_total",
		Help: "Total number of Kafka records consumed by realtime service, partitioned by topic.",
	}, []string{"topic"})

	wsConnectionsCurrent := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "arrival_realtime_ws_connections_current",
		Help: "Current number of active websocket connections.",
	})

	wsConnectionsTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "arrival_realtime_ws_connections_total",
		Help: "Total number of websocket connections opened.",
	})

	wsDisconnectsTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "arrival_realtime_ws_disconnects_total",
		Help: "Total number of websocket disconnects by reason.",
	}, []string{"reason"})

	wsMessagesSentTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "arrival_realtime_ws_messages_sent_total",
		Help: "Total number of websocket messages sent by message type.",
	}, []string{"type"})

	wsDroppedTotal := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "arrival_realtime_ws_dropped_total",
		Help: "Total number of websocket messages/clients dropped by reason.",
	}, []string{"reason"})

	snapshotRequestsTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "arrival_realtime_snapshot_requests_total",
		Help: "Total number of snapshot HTTP requests.",
	})

	statePositions := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "arrival_realtime_state_positions",
		Help: "Current number of latest positions kept in in-memory state.",
	})

	stateObservedDelays := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "arrival_realtime_state_observed_delays",
		Help: "Current number of latest observed delays kept in in-memory state.",
	})

	statePredictedDelays := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "arrival_realtime_state_predicted_delays",
		Help: "Current number of latest predicted delays kept in in-memory state.",
	})

	invalidPositionsTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "arrival_realtime_invalid_positions_total",
		Help: "Total number of ignored invalid positions.",
	})

	prometheus.MustRegister(
		kafkaRecordsTotal,
		wsConnectionsCurrent,
		wsConnectionsTotal,
		wsDisconnectsTotal,
		wsMessagesSentTotal,
		wsDroppedTotal,
		snapshotRequestsTotal,
		statePositions,
		stateObservedDelays,
		statePredictedDelays,
		invalidPositionsTotal,
	)

	return realtimeMetrics{
		kafkaRecordsTotal:     kafkaRecordsTotal,
		wsConnectionsCurrent:  wsConnectionsCurrent,
		wsConnectionsTotal:    wsConnectionsTotal,
		wsDisconnectsTotal:    wsDisconnectsTotal,
		wsMessagesSentTotal:   wsMessagesSentTotal,
		wsDroppedTotal:        wsDroppedTotal,
		snapshotRequestsTotal: snapshotRequestsTotal,
		statePositions:        statePositions,
		stateObservedDelays:   stateObservedDelays,
		statePredictedDelays:  statePredictedDelays,
		invalidPositionsTotal: invalidPositionsTotal,
	}
}
