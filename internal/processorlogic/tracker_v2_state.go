package processorlogic

import (
	"math"
	"time"
)

type v2VehicleState struct {
	TripID                         string
	LastObservedSeq                int64
	HasProgress                    bool
	EmittedSeq                     map[int64]struct{}
	SmoothedDelaySeconds           float64
	HasSmoothedDelay               bool
	LastProgressAt                 time.Time
	ConsecutiveBackwardProgression int
}

func newV2VehicleState(tripID string) *v2VehicleState {
	return &v2VehicleState{
		TripID:     tripID,
		EmittedSeq: make(map[int64]struct{}),
	}
}

func (s *v2VehicleState) updateSmoothedDelay(delaySeconds int64, alpha float64) int64 {
	delay := float64(delaySeconds)
	if !s.HasSmoothedDelay {
		s.SmoothedDelaySeconds = delay
		s.HasSmoothedDelay = true
		return delaySeconds
	}

	s.SmoothedDelaySeconds = alpha*delay + (1-alpha)*s.SmoothedDelaySeconds
	return int64(math.Round(s.SmoothedDelaySeconds))
}

func (t *V2Tracker) resetState(voznjaBusID int64) {
	delete(t.stateByVoznjaBusID, voznjaBusID)
}

func (t *V2Tracker) stateForVoznjaBusID(voznjaBusID int64) *v2VehicleState {
	state, ok := t.stateByVoznjaBusID[voznjaBusID]
	if ok {
		return state
	}

	tripID := t.tripIDResolver(voznjaBusID)
	state = newV2VehicleState(tripID)
	t.stateByVoznjaBusID[voznjaBusID] = state
	return state
}
