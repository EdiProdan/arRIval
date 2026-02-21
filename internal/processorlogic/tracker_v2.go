package processorlogic

import (
	"strconv"
	"strings"
	"time"

	"github.com/EdiProdan/arRIval/internal/contracts"
	"github.com/EdiProdan/arRIval/internal/staticdata"
)

// V2Tracker is the Phase 2 stateful tracker core.
// It is intentionally not goroutine-safe; callers must serialize access.
type V2Tracker struct {
	store                        *staticdata.Store
	tripPlans                    map[string]*v2TripPlan
	stateByVoznjaBusID           map[int64]*v2VehicleState
	stationMatchMeters           float64
	staleAfter                   time.Duration
	invalidProgressionResetAfter int
	emaSmoothingAlpha            float64
	trackerVersion               string
	serviceLocation              *time.Location
	tripIDResolver               TripIDResolver
}

func NewV2Tracker(store *staticdata.Store, cfg V2TrackerConfig) *V2Tracker {
	stationMatchMeters := cfg.StationMatchMeters
	if stationMatchMeters <= 0 {
		stationMatchMeters = defaultStationMatchMeters
	}

	staleAfter := cfg.StaleAfter
	if staleAfter <= 0 {
		staleAfter = defaultV2StaleAfter
	}

	invalidProgressionResetAfter := cfg.InvalidProgressionResetAfter
	if invalidProgressionResetAfter <= 0 {
		invalidProgressionResetAfter = defaultV2InvalidProgressionResetAfter
	}

	emaSmoothingAlpha := cfg.EMASmoothingAlpha
	if emaSmoothingAlpha <= 0 || emaSmoothingAlpha >= 1 {
		emaSmoothingAlpha = defaultV2EMASmoothingAlpha
	}

	trackerVersion := strings.TrimSpace(cfg.TrackerVersion)
	if trackerVersion == "" {
		trackerVersion = defaultV2TrackerVersion
	}

	serviceLocation := cfg.ServiceLocation
	if serviceLocation == nil {
		serviceLocation = defaultV2ServiceLocation()
	}

	tripIDResolver := cfg.TripIDResolver
	if tripIDResolver == nil {
		tripIDResolver = defaultTripIDResolver
	}

	return &V2Tracker{
		store:                        store,
		tripPlans:                    buildV2TripPlans(store),
		stateByVoznjaBusID:           make(map[int64]*v2VehicleState),
		stationMatchMeters:           stationMatchMeters,
		staleAfter:                   staleAfter,
		invalidProgressionResetAfter: invalidProgressionResetAfter,
		emaSmoothingAlpha:            emaSmoothingAlpha,
		trackerVersion:               trackerVersion,
		serviceLocation:              serviceLocation,
		tripIDResolver:               tripIDResolver,
	}
}

func (t *V2Tracker) Track(input V2TrackInput) V2TrackOutput {
	if input.ObservedAt.IsZero() {
		return V2TrackOutput{SkipReason: V2SkipReasonInvalidObservedAt}
	}
	if input.Bus.Lon == nil || input.Bus.Lat == nil {
		return V2TrackOutput{SkipReason: V2SkipReasonMissingCoordinates}
	}
	if input.Bus.VoznjaBusID == nil {
		return V2TrackOutput{SkipReason: V2SkipReasonMissingVoznjaBusID}
	}

	observedAt := input.ObservedAt.UTC()
	voznjaBusID := int64(*input.Bus.VoznjaBusID)

	state := t.stateForVoznjaBusID(voznjaBusID)
	if !state.LastProgressAt.IsZero() && observedAt.Sub(state.LastProgressAt) >= t.staleAfter {
		t.resetState(voznjaBusID)
		return V2TrackOutput{SkipReason: V2SkipReasonResetAfterStaleState}
	}

	plan, ok := t.tripPlans[state.TripID]
	if !ok {
		t.resetState(voznjaBusID)
		return V2TrackOutput{SkipReason: V2SkipReasonNoTripForVoznjaBusID}
	}

	match, hasCoordinateCandidate, found := t.matchTripStop(plan, *input.Bus.Lon, *input.Bus.Lat)
	if !hasCoordinateCandidate {
		return V2TrackOutput{SkipReason: V2SkipReasonStopMissingCoordinates}
	}
	if !found {
		return V2TrackOutput{SkipReason: V2SkipReasonNoStopWithinMatchRadius}
	}

	if state.HasProgress {
		if match.stop.StationSeq < state.LastObservedSeq {
			state.ConsecutiveBackwardProgression++
			if state.ConsecutiveBackwardProgression >= t.invalidProgressionResetAfter {
				t.resetState(voznjaBusID)
				return V2TrackOutput{SkipReason: V2SkipReasonResetAfterInvalidProgress}
			}
			return V2TrackOutput{SkipReason: V2SkipReasonBackwardStationSeq}
		}
		if match.stop.StationSeq == state.LastObservedSeq {
			state.ConsecutiveBackwardProgression = 0
			return V2TrackOutput{SkipReason: V2SkipReasonDuplicateStationSeq}
		}
	}

	if _, exists := state.EmittedSeq[match.stop.StationSeq]; exists {
		return V2TrackOutput{SkipReason: V2SkipReasonDuplicateStationSeq}
	}

	scheduledTimes, ok := t.scheduledTimelineFromMatch(plan, match.index, observedAt)
	if !ok {
		return V2TrackOutput{SkipReason: V2SkipReasonScheduleAlignmentFailed}
	}

	scheduledMatched := scheduledTimes[match.index]
	observedDelaySeconds := int64(observedAt.Sub(scheduledMatched) / time.Second)
	predictedDelaySeconds := state.updateSmoothedDelay(observedDelaySeconds, t.emaSmoothingAlpha)

	state.HasProgress = true
	state.LastObservedSeq = match.stop.StationSeq
	state.LastProgressAt = observedAt
	state.ConsecutiveBackwardProgression = 0
	state.EmittedSeq[match.stop.StationSeq] = struct{}{}

	observedEvent := contracts.ObservedDelayV2{
		TripID:         state.TripID,
		VoznjaBusID:    voznjaBusID,
		GBR:            intPtrToInt64Ptr(input.Bus.GBR),
		LinVarID:       match.stop.LinVarID,
		BrojLinije:     match.stop.BrojLinije,
		StationID:      match.stop.StationID,
		StationName:    match.stop.StationName,
		StationSeq:     match.stop.StationSeq,
		ScheduledTime:  scheduledMatched.Format(time.RFC3339Nano),
		ObservedTime:   observedAt.Format(time.RFC3339Nano),
		DelaySeconds:   observedDelaySeconds,
		DistanceM:      match.distanceM,
		TrackerVersion: t.trackerVersion,
	}

	output := V2TrackOutput{
		Observed: []contracts.ObservedDelayV2{observedEvent},
	}

	// Predictions intentionally fan out to all remaining stops (O(N^2) per trip)
	// so downstream consumers can apply last-write-wins per (trip_id, station_id).
	for i := match.index + 1; i < len(plan.Stops); i++ {
		stop := plan.Stops[i]
		scheduled := scheduledTimes[i]
		predicted := scheduled.Add(time.Duration(predictedDelaySeconds) * time.Second)

		output.Predicted = append(output.Predicted, contracts.PredictedDelayV2{
			TripID:                state.TripID,
			VoznjaBusID:           voznjaBusID,
			LinVarID:              stop.LinVarID,
			BrojLinije:            stop.BrojLinije,
			StationID:             stop.StationID,
			StationName:           stop.StationName,
			StationSeq:            stop.StationSeq,
			ScheduledTime:         scheduled.Format(time.RFC3339Nano),
			PredictedTime:         predicted.Format(time.RFC3339Nano),
			PredictedDelaySeconds: predictedDelaySeconds,
			GeneratedAt:           observedAt.Format(time.RFC3339Nano),
			TrackerVersion:        t.trackerVersion,
		})
	}

	return output
}

type v2TripStopMatch struct {
	stop      v2TripStop
	index     int
	distanceM float64
}

func (t *V2Tracker) matchTripStop(plan *v2TripPlan, lon, lat float64) (v2TripStopMatch, bool, bool) {
	var (
		match                   v2TripStopMatch
		hasCoordinateCandidates bool
		found                   bool
	)

	for i, stop := range plan.Stops {
		if !stop.HasCoordinates {
			continue
		}
		hasCoordinateCandidates = true

		distance := haversineMeters(lat, lon, stop.Lat, stop.Lon)
		if distance >= t.stationMatchMeters {
			continue
		}
		if !found || distance < match.distanceM {
			match = v2TripStopMatch{
				stop:      stop,
				index:     i,
				distanceM: distance,
			}
			found = true
		}
	}

	return match, hasCoordinateCandidates, found
}

func (t *V2Tracker) scheduledTimelineFromMatch(plan *v2TripPlan, matchedIndex int, observedAt time.Time) ([]time.Time, bool) {
	if matchedIndex < 0 || matchedIndex >= len(plan.Stops) {
		return nil, false
	}

	matchedStop := plan.Stops[matchedIndex]
	matchedSchedule, err := parseScheduleTimeLocalAligned(matchedStop.ScheduleToken, observedAt, t.serviceLocation)
	if err != nil {
		return nil, false
	}

	timeline := make([]time.Time, len(plan.Stops))
	matchedOffset := matchedStop.CumulativeScheduleSeconds
	for i := range plan.Stops {
		offset := plan.Stops[i].CumulativeScheduleSeconds - matchedOffset
		timeline[i] = matchedSchedule.Add(time.Duration(offset) * time.Second).UTC()
	}

	return timeline, true
}

func defaultTripIDResolver(voznjaBusID int64) string {
	// Current source-data assumption for Phase 2:
	// PolazakID == string(voznja_bus_id).
	return strconv.FormatInt(voznjaBusID, 10)
}

func defaultV2ServiceLocation() *time.Location {
	loc, err := time.LoadLocation("Europe/Zagreb")
	if err != nil {
		return time.UTC
	}
	return loc
}
