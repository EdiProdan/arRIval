package processorlogic

import (
	"time"

	"github.com/EdiProdan/arRIval/internal/autotrolej"
	"github.com/EdiProdan/arRIval/internal/contracts"
)

const (
	defaultStaleAfter                   = 15 * time.Minute
	defaultInvalidProgressionResetAfter = 3
	defaultEMASmoothingAlpha            = 0.35
	defaultTrackerVersion               = "current"
)

// TripIDResolver maps a live voznja_bus_id to a timetable trip identifier.
// Current tracker logic assumes PolazakID == strconv.FormatInt(voznja_bus_id, 10).
type TripIDResolver func(voznjaBusID int64) string

type SkipReason string

const (
	SkipReasonNone                      SkipReason = ""
	SkipReasonInvalidObservedAt         SkipReason = "invalid_observed_at"
	SkipReasonMissingCoordinates        SkipReason = "missing_coordinates"
	SkipReasonMissingVoznjaBusID        SkipReason = "missing_voznja_bus_id"
	SkipReasonNoTripForVoznjaBusID      SkipReason = "no_trip_for_voznja_bus_id"
	SkipReasonStopMissingCoordinates    SkipReason = "stop_missing_coordinates"
	SkipReasonNoStopWithinMatchRadius   SkipReason = "no_stop_within_match_radius"
	SkipReasonDuplicateStationSeq       SkipReason = "duplicate_station_seq"
	SkipReasonBackwardStationSeq        SkipReason = "backward_station_seq"
	SkipReasonResetAfterInvalidProgress SkipReason = "reset_after_invalid_progression"
	SkipReasonResetAfterStaleState      SkipReason = "reset_after_stale_state"
	SkipReasonScheduleAlignmentFailed   SkipReason = "schedule_alignment_failed"
)

type TrackerConfig struct {
	StationMatchMeters           float64
	StaleAfter                   time.Duration
	InvalidProgressionResetAfter int
	EMASmoothingAlpha            float64
	TrackerVersion               string
	ServiceLocation              *time.Location
	TripIDResolver               TripIDResolver
}

type TrackInput struct {
	ObservedAt     time.Time
	KafkaTopic     string
	KafkaPartition int32
	KafkaOffset    int64
	Bus            autotrolej.LiveBus
}

type TrackOutput struct {
	Observed   []contracts.ObservedDelay
	Predicted  []contracts.PredictedDelay
	SkipReason SkipReason
}
