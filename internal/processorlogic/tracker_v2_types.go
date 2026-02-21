package processorlogic

import (
	"time"

	"github.com/EdiProdan/arRIval/internal/autotrolej"
	"github.com/EdiProdan/arRIval/internal/contracts"
)

const (
	defaultV2StaleAfter                   = 15 * time.Minute
	defaultV2InvalidProgressionResetAfter = 3
	defaultV2EMASmoothingAlpha            = 0.35
	defaultV2TrackerVersion               = "v2"
)

// TripIDResolver maps a live voznja_bus_id to a timetable trip identifier.
// V2 currently assumes PolazakID == strconv.FormatInt(voznja_bus_id, 10).
type TripIDResolver func(voznjaBusID int64) string

type V2SkipReason string

const (
	V2SkipReasonNone                      V2SkipReason = ""
	V2SkipReasonInvalidObservedAt         V2SkipReason = "invalid_observed_at"
	V2SkipReasonMissingCoordinates        V2SkipReason = "missing_coordinates"
	V2SkipReasonMissingVoznjaBusID        V2SkipReason = "missing_voznja_bus_id"
	V2SkipReasonNoTripForVoznjaBusID      V2SkipReason = "no_trip_for_voznja_bus_id"
	V2SkipReasonStopMissingCoordinates    V2SkipReason = "stop_missing_coordinates"
	V2SkipReasonNoStopWithinMatchRadius   V2SkipReason = "no_stop_within_match_radius"
	V2SkipReasonDuplicateStationSeq       V2SkipReason = "duplicate_station_seq"
	V2SkipReasonBackwardStationSeq        V2SkipReason = "backward_station_seq"
	V2SkipReasonResetAfterInvalidProgress V2SkipReason = "reset_after_invalid_progression"
	V2SkipReasonResetAfterStaleState      V2SkipReason = "reset_after_stale_state"
	V2SkipReasonScheduleAlignmentFailed   V2SkipReason = "schedule_alignment_failed"
)

type V2TrackerConfig struct {
	StationMatchMeters           float64
	StaleAfter                   time.Duration
	InvalidProgressionResetAfter int
	EMASmoothingAlpha            float64
	TrackerVersion               string
	ServiceLocation              *time.Location
	TripIDResolver               TripIDResolver
}

type V2TrackInput struct {
	ObservedAt     time.Time
	KafkaTopic     string
	KafkaPartition int32
	KafkaOffset    int64
	Bus            autotrolej.LiveBus
}

type V2TrackOutput struct {
	Observed   []contracts.ObservedDelayV2
	Predicted  []contracts.PredictedDelayV2
	SkipReason V2SkipReason
}
