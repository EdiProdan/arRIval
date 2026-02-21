package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/EdiProdan/arRIval/internal/autotrolej"
	"github.com/EdiProdan/arRIval/internal/contracts"
	"github.com/EdiProdan/arRIval/internal/processorlogic"
	"github.com/EdiProdan/arRIval/internal/staticdata"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func TestProcessRecordV2Fanout(t *testing.T) {
	metrics := testProcessorMetrics()
	bw := &bronzeWriterStub{}
	ow := &observedWriterStub{}
	pw := &predictedWriterStub{}
	publisher := &publishStub{}

	tracker := &trackerStub{
		outputs: []processorlogic.V2TrackOutput{
			{
				Observed: []contracts.ObservedDelayV2{
					{
						TripID:         "1001",
						VoznjaBusID:    1001,
						LinVarID:       "lv-1",
						BrojLinije:     "2A",
						StationID:      1,
						StationName:    "Stop 1",
						StationSeq:     1,
						ScheduledTime:  "2026-02-21T12:00:00Z",
						ObservedTime:   "2026-02-21T12:01:00Z",
						DelaySeconds:   60,
						DistanceM:      7.5,
						TrackerVersion: "v2",
					},
				},
				Predicted: []contracts.PredictedDelayV2{
					{
						TripID:                "1001",
						VoznjaBusID:           1001,
						LinVarID:              "lv-1",
						BrojLinije:            "2A",
						StationID:             2,
						StationName:           "Stop 2",
						StationSeq:            2,
						ScheduledTime:         "2026-02-21T12:05:00Z",
						PredictedTime:         "2026-02-21T12:06:00Z",
						PredictedDelaySeconds: 60,
						GeneratedAt:           "2026-02-21T12:01:00Z",
						TrackerVersion:        "v2",
					},
					{
						TripID:                "1001",
						VoznjaBusID:           1001,
						LinVarID:              "lv-1",
						BrojLinije:            "2A",
						StationID:             3,
						StationName:           "Stop 3",
						StationSeq:            3,
						ScheduledTime:         "2026-02-21T12:10:00Z",
						PredictedTime:         "2026-02-21T12:11:00Z",
						PredictedDelaySeconds: 60,
						GeneratedAt:           "2026-02-21T12:01:00Z",
						TrackerVersion:        "v2",
					},
				},
			},
		},
	}

	rec := autobusiRecord(t, []autotrolej.LiveBus{{
		VoznjaBusID: intPtr(1001),
		Lon:         float64Ptr(14.44),
		Lat:         float64Ptr(45.33),
	}}, time.Date(2026, 2, 21, 12, 1, 0, 0, time.UTC))

	commit, published, err := processRecord(
		context.Background(),
		bw,
		ow,
		pw,
		tracker,
		publisher.publish,
		contracts.TopicBusDelayObservedV2,
		contracts.TopicBusDelayPredictedV2,
		metrics,
		rec,
	)
	if err != nil {
		t.Fatalf("processRecord: %v", err)
	}
	if !commit {
		t.Fatalf("commit = false, want true")
	}
	if published != 3 {
		t.Fatalf("published = %d, want 3", published)
	}
	if len(bw.rows) != 1 {
		t.Fatalf("bronze rows = %d, want 1", len(bw.rows))
	}
	if len(ow.rows) != 1 {
		t.Fatalf("observed rows = %d, want 1", len(ow.rows))
	}
	if len(pw.rows) != 2 {
		t.Fatalf("predicted rows = %d, want 2", len(pw.rows))
	}
	if len(publisher.records) != 3 {
		t.Fatalf("published records = %d, want 3", len(publisher.records))
	}
	if publisher.records[0].Topic != contracts.TopicBusDelayObservedV2 {
		t.Fatalf("first topic = %q, want %q", publisher.records[0].Topic, contracts.TopicBusDelayObservedV2)
	}
	if publisher.records[1].Topic != contracts.TopicBusDelayPredictedV2 || publisher.records[2].Topic != contracts.TopicBusDelayPredictedV2 {
		t.Fatalf("predicted topics mismatch: got %q, %q", publisher.records[1].Topic, publisher.records[2].Topic)
	}
	if got := counterValue(t, metrics.observedPublished); got != 1 {
		t.Fatalf("observedPublished = %f, want 1", got)
	}
	if got := counterValue(t, metrics.predictedPublished); got != 2 {
		t.Fatalf("predictedPublished = %f, want 2", got)
	}
}

func TestProcessRecordSkipIncrementsReasonMetric(t *testing.T) {
	metrics := testProcessorMetrics()
	bw := &bronzeWriterStub{}
	ow := &observedWriterStub{}
	pw := &predictedWriterStub{}
	publisher := &publishStub{}

	tracker := &trackerStub{
		outputs: []processorlogic.V2TrackOutput{
			{SkipReason: processorlogic.V2SkipReasonMissingCoordinates},
		},
	}

	rec := autobusiRecord(t, []autotrolej.LiveBus{{
		VoznjaBusID: intPtr(1001),
	}}, time.Date(2026, 2, 21, 12, 1, 0, 0, time.UTC))

	commit, published, err := processRecord(
		context.Background(),
		bw,
		ow,
		pw,
		tracker,
		publisher.publish,
		contracts.TopicBusDelayObservedV2,
		contracts.TopicBusDelayPredictedV2,
		metrics,
		rec,
	)
	if err != nil {
		t.Fatalf("processRecord: %v", err)
	}
	if !commit {
		t.Fatalf("commit = false, want true")
	}
	if published != 0 {
		t.Fatalf("published = %d, want 0", published)
	}
	if len(ow.rows) != 0 || len(pw.rows) != 0 || len(publisher.records) != 0 {
		t.Fatalf("unexpected outputs: observed=%d predicted=%d published=%d", len(ow.rows), len(pw.rows), len(publisher.records))
	}
	reason := string(processorlogic.V2SkipReasonMissingCoordinates)
	if got := counterValue(t, metrics.trackerSkips.WithLabelValues(reason)); got != 1 {
		t.Fatalf("trackerSkips[%q] = %f, want 1", reason, got)
	}
}

func TestProcessRecordDuplicateStationProgressionDoesNotRepublish(t *testing.T) {
	store := buildProcessorTestStore(t)
	realTracker := processorlogic.NewV2Tracker(store, processorlogic.V2TrackerConfig{
		ServiceLocation: time.UTC,
	})

	metrics := testProcessorMetrics()
	bw := &bronzeWriterStub{}
	ow := &observedWriterStub{}
	pw := &predictedWriterStub{}
	publisher := &publishStub{}

	first := autobusiRecord(t, []autotrolej.LiveBus{{
		VoznjaBusID: intPtr(1001),
		Lon:         float64Ptr(14.4400),
		Lat:         float64Ptr(45.3300),
	}}, time.Date(2026, 2, 21, 12, 1, 0, 0, time.UTC))
	second := autobusiRecord(t, []autotrolej.LiveBus{{
		VoznjaBusID: intPtr(1001),
		Lon:         float64Ptr(14.4400),
		Lat:         float64Ptr(45.3300),
	}}, time.Date(2026, 2, 21, 12, 1, 10, 0, time.UTC))

	_, firstPublished, err := processRecord(
		context.Background(),
		bw,
		ow,
		pw,
		realTracker,
		publisher.publish,
		contracts.TopicBusDelayObservedV2,
		contracts.TopicBusDelayPredictedV2,
		metrics,
		first,
	)
	if err != nil {
		t.Fatalf("first processRecord: %v", err)
	}
	if firstPublished == 0 {
		t.Fatalf("firstPublished = 0, want > 0")
	}

	_, secondPublished, err := processRecord(
		context.Background(),
		bw,
		ow,
		pw,
		realTracker,
		publisher.publish,
		contracts.TopicBusDelayObservedV2,
		contracts.TopicBusDelayPredictedV2,
		metrics,
		second,
	)
	if err != nil {
		t.Fatalf("second processRecord: %v", err)
	}
	if secondPublished != 0 {
		t.Fatalf("secondPublished = %d, want 0", secondPublished)
	}

	reason := string(processorlogic.V2SkipReasonDuplicateStationSeq)
	if got := counterValue(t, metrics.trackerSkips.WithLabelValues(reason)); got != 1 {
		t.Fatalf("trackerSkips[%q] = %f, want 1", reason, got)
	}
}

func TestProcessRecordUsesKafkaTimestampForObservedAndGeneratedTimes(t *testing.T) {
	store := buildProcessorTestStore(t)
	realTracker := processorlogic.NewV2Tracker(store, processorlogic.V2TrackerConfig{
		ServiceLocation: time.UTC,
	})

	metrics := testProcessorMetrics()
	bw := &bronzeWriterStub{}
	ow := &observedWriterStub{}
	pw := &predictedWriterStub{}
	publisher := &publishStub{}

	kafkaTimestamp := time.Date(2026, 2, 21, 12, 1, 0, 123456789, time.UTC)
	rec := autobusiRecord(t, []autotrolej.LiveBus{{
		VoznjaBusID: intPtr(1001),
		Lon:         float64Ptr(14.4400),
		Lat:         float64Ptr(45.3300),
	}}, kafkaTimestamp)

	_, published, err := processRecord(
		context.Background(),
		bw,
		ow,
		pw,
		realTracker,
		publisher.publish,
		contracts.TopicBusDelayObservedV2,
		contracts.TopicBusDelayPredictedV2,
		metrics,
		rec,
	)
	if err != nil {
		t.Fatalf("processRecord: %v", err)
	}
	if published == 0 {
		t.Fatalf("published = 0, want > 0")
	}
	if len(ow.rows) != 1 {
		t.Fatalf("observed rows = %d, want 1", len(ow.rows))
	}
	if got, want := ow.rows[0].ObservedTime, kafkaTimestamp.Format(time.RFC3339Nano); got != want {
		t.Fatalf("observed time = %q, want %q", got, want)
	}
	if len(pw.rows) == 0 {
		t.Fatalf("predicted rows = 0, want >= 1")
	}
	if got, want := pw.rows[0].GeneratedAt, kafkaTimestamp.Format(time.RFC3339Nano); got != want {
		t.Fatalf("generated_at = %q, want %q", got, want)
	}
}

func TestSilverWritersUseV2Filenames(t *testing.T) {
	baseDir := t.TempDir()
	date := "2026-02-21"

	ow := &silverObservedDailyWriter{baseDir: baseDir}
	if err := ow.Write(silverObservedDelayV2Row{
		IngestedAt:   "2026-02-21T12:01:00Z",
		IngestedDate: date,
	}); err != nil {
		t.Fatalf("write observed row: %v", err)
	}
	if err := ow.Close(); err != nil {
		t.Fatalf("close observed writer: %v", err)
	}

	pw := &silverPredictedDailyWriter{baseDir: baseDir}
	if err := pw.Write(silverPredictedDelayV2Row{
		IngestedAt:   "2026-02-21T12:01:00Z",
		IngestedDate: date,
	}); err != nil {
		t.Fatalf("write predicted row: %v", err)
	}
	if err := pw.Close(); err != nil {
		t.Fatalf("close predicted writer: %v", err)
	}

	observedPath := filepath.Join(baseDir, date, observedSilverFilename)
	if _, err := os.Stat(observedPath); err != nil {
		t.Fatalf("observed file missing at %s: %v", observedPath, err)
	}

	predictedPath := filepath.Join(baseDir, date, predictedSilverFilename)
	if _, err := os.Stat(predictedPath); err != nil {
		t.Fatalf("predicted file missing at %s: %v", predictedPath, err)
	}
}

func TestEnsureTopicAlreadyExistsIsAllowed(t *testing.T) {
	previous := requestCreateTopics
	t.Cleanup(func() {
		requestCreateTopics = previous
	})

	requestCreateTopics = func(_ context.Context, _ *kgo.Client, req *kmsg.CreateTopicsRequest) (*kmsg.CreateTopicsResponse, error) {
		res := kmsg.NewPtrCreateTopicsResponse()
		topic := kmsg.NewCreateTopicsResponseTopic()
		topic.Topic = req.Topics[0].Topic
		topic.ErrorCode = kerr.TopicAlreadyExists.Code
		res.Topics = append(res.Topics, topic)
		return res, nil
	}

	if err := ensureTopic(context.Background(), nil, contracts.TopicBusDelayObservedV2); err != nil {
		t.Fatalf("ensureTopic should allow existing topic, got err=%v", err)
	}
}

func TestEnsureTopicReturnsKafkaError(t *testing.T) {
	previous := requestCreateTopics
	t.Cleanup(func() {
		requestCreateTopics = previous
	})

	requestCreateTopics = func(_ context.Context, _ *kgo.Client, req *kmsg.CreateTopicsRequest) (*kmsg.CreateTopicsResponse, error) {
		res := kmsg.NewPtrCreateTopicsResponse()
		topic := kmsg.NewCreateTopicsResponseTopic()
		topic.Topic = req.Topics[0].Topic
		topic.ErrorCode = kerr.InvalidReplicationFactor.Code
		res.Topics = append(res.Topics, topic)
		return res, nil
	}

	err := ensureTopic(context.Background(), nil, contracts.TopicBusDelayPredictedV2)
	if err == nil {
		t.Fatalf("ensureTopic error = nil, want non-nil")
	}
	if !strings.Contains(err.Error(), "INVALID_REPLICATION_FACTOR") {
		t.Fatalf("ensureTopic error = %q, want INVALID_REPLICATION_FACTOR", err.Error())
	}
}

func TestEnsureTopicsCreatesAllConfiguredTopics(t *testing.T) {
	previous := requestCreateTopics
	t.Cleanup(func() {
		requestCreateTopics = previous
	})

	var requested []string
	requestCreateTopics = func(_ context.Context, _ *kgo.Client, req *kmsg.CreateTopicsRequest) (*kmsg.CreateTopicsResponse, error) {
		requested = append(requested, req.Topics[0].Topic)
		res := kmsg.NewPtrCreateTopicsResponse()
		topic := kmsg.NewCreateTopicsResponseTopic()
		topic.Topic = req.Topics[0].Topic
		res.Topics = append(res.Topics, topic)
		return res, nil
	}

	err := ensureTopics(context.Background(), nil, contracts.TopicBusDelayObservedV2, contracts.TopicBusDelayPredictedV2)
	if err != nil {
		t.Fatalf("ensureTopics: %v", err)
	}
	if len(requested) != 2 {
		t.Fatalf("requested topics len=%d, want 2", len(requested))
	}
	if requested[0] != contracts.TopicBusDelayObservedV2 || requested[1] != contracts.TopicBusDelayPredictedV2 {
		t.Fatalf("requested topics = %v, want [%s %s]", requested, contracts.TopicBusDelayObservedV2, contracts.TopicBusDelayPredictedV2)
	}
}

type bronzeWriterStub struct {
	rows []bronzePositionRow
}

func (s *bronzeWriterStub) Write(row bronzePositionRow) error {
	s.rows = append(s.rows, row)
	return nil
}

type observedWriterStub struct {
	rows []silverObservedDelayV2Row
}

func (s *observedWriterStub) Write(row silverObservedDelayV2Row) error {
	s.rows = append(s.rows, row)
	return nil
}

type predictedWriterStub struct {
	rows []silverPredictedDelayV2Row
}

func (s *predictedWriterStub) Write(row silverPredictedDelayV2Row) error {
	s.rows = append(s.rows, row)
	return nil
}

type trackerStub struct {
	outputs []processorlogic.V2TrackOutput
	calls   []processorlogic.V2TrackInput
}

func (s *trackerStub) Track(input processorlogic.V2TrackInput) processorlogic.V2TrackOutput {
	s.calls = append(s.calls, input)
	if len(s.outputs) == 0 {
		return processorlogic.V2TrackOutput{}
	}
	out := s.outputs[0]
	s.outputs = s.outputs[1:]
	return out
}

type publishStub struct {
	records []*kgo.Record
}

func (s *publishStub) publish(_ context.Context, record *kgo.Record) error {
	cloned := &kgo.Record{
		Topic:     record.Topic,
		Key:       append([]byte(nil), record.Key...),
		Value:     append([]byte(nil), record.Value...),
		Partition: record.Partition,
		Offset:    record.Offset,
		Timestamp: record.Timestamp,
	}
	s.records = append(s.records, cloned)
	return nil
}

func testProcessorMetrics() processorMetrics {
	return processorMetrics{
		messagesProcessed: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "test_arrival_processor_messages_processed_total",
			Help: "test",
		}),
		processingLagSec: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "test_arrival_processor_processing_lag_seconds",
			Help: "test",
		}),
		observedDelaySeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "test_arrival_processor_observed_delay_seconds",
			Help: "test",
		}),
		predictedDelaySeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "test_arrival_processor_predicted_delay_seconds",
			Help: "test",
		}),
		observedPublished: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "test_arrival_processor_observed_events_published_total",
			Help: "test",
		}),
		predictedPublished: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "test_arrival_processor_predicted_events_published_total",
			Help: "test",
		}),
		trackerSkips: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "test_arrival_processor_tracker_skips_total",
			Help: "test",
		}, []string{"reason"}),
	}
}

func autobusiRecord(t *testing.T, buses []autotrolej.LiveBus, timestamp time.Time) *kgo.Record {
	t.Helper()

	raw, err := json.Marshal(autotrolej.AutobusiResponse{
		Msg: "ok",
		Res: buses,
		Err: false,
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	return &kgo.Record{
		Topic:     "bus-positions-raw",
		Partition: 1,
		Offset:    42,
		Timestamp: timestamp,
		Value:     raw,
	}
}

func buildProcessorTestStore(t *testing.T) *staticdata.Store {
	t.Helper()

	dir := t.TempDir()
	stations := []staticdata.Station{
		{StanicaID: 1, Naziv: "Stop 1", GpsX: float64Ptr(14.4400), GpsY: float64Ptr(45.3300)},
		{StanicaID: 2, Naziv: "Stop 2", GpsX: float64Ptr(14.4410), GpsY: float64Ptr(45.3310)},
	}
	linePaths := []staticdata.LinePathRow{
		{ID: 1, LinVarID: "lv-1", BrojLinije: "2A", StanicaID: 1, RedniBrojStanice: 1},
		{ID: 2, LinVarID: "lv-1", BrojLinije: "2A", StanicaID: 2, RedniBrojStanice: 2},
	}
	timetable := []staticdata.TimetableStopRow{
		{ID: 1, PolazakID: "1001", StanicaID: 1, LinVarID: "lv-1", Polazak: "12:00:00", RedniBrojStanice: 1, BrojLinije: "2A", Naziv: "Stop 1", GpsX: float64Ptr(14.4400), GpsY: float64Ptr(45.3300)},
		{ID: 2, PolazakID: "1001", StanicaID: 2, LinVarID: "lv-1", Polazak: "12:05:00", RedniBrojStanice: 2, BrojLinije: "2A", Naziv: "Stop 2", GpsX: float64Ptr(14.4410), GpsY: float64Ptr(45.3310)},
	}

	writeJSONFile(t, filepath.Join(dir, "stanice.json"), stations)
	writeJSONFile(t, filepath.Join(dir, "linije.json"), linePaths)
	writeJSONFile(t, filepath.Join(dir, "voznired_dnevni.json"), timetable)

	store, err := staticdata.LoadFromDir(dir)
	if err != nil {
		t.Fatalf("LoadFromDir: %v", err)
	}
	return store
}

func writeJSONFile(t *testing.T, path string, value any) {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func intPtr(value int) *int {
	v := value
	return &v
}

func float64Ptr(value float64) *float64 {
	v := value
	return &v
}

func counterValue(t *testing.T, c prometheus.Counter) float64 {
	t.Helper()

	metric := &dto.Metric{}
	if err := c.Write(metric); err != nil {
		t.Fatalf("counter write: %v", err)
	}
	if metric.Counter == nil {
		t.Fatalf("counter metric is nil")
	}
	return metric.Counter.GetValue()
}
