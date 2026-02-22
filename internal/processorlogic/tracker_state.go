package processorlogic

import (
	"math"
	"time"
)

type vehicleState struct {
	TripID                         string
	LastObservedSeq                int64
	HasProgress                    bool
	EmittedSeq                     map[int64]struct{}
	SmoothedDelaySeconds           float64
	HasSmoothedDelay               bool
	LastProgressAt                 time.Time
	ConsecutiveBackwardProgression int
}

func newVehicleState(tripID string) *vehicleState {
	return &vehicleState{
		TripID:     tripID,
		EmittedSeq: make(map[int64]struct{}),
	}
}

func (s *vehicleState) updateSmoothedDelay(delaySeconds int64, alpha float64) int64 {
	delay := float64(delaySeconds)
	if !s.HasSmoothedDelay {
		s.SmoothedDelaySeconds = delay
		s.HasSmoothedDelay = true
		return delaySeconds
	}

	s.SmoothedDelaySeconds = alpha*delay + (1-alpha)*s.SmoothedDelaySeconds
	return int64(math.Round(s.SmoothedDelaySeconds))
}

func (t *Tracker) resetState(voznjaBusID int64) {
	delete(t.stateByVoznjaBusID, voznjaBusID)
}

func (t *Tracker) stateForVoznjaBusID(voznjaBusID int64) *vehicleState {
	state, ok := t.stateByVoznjaBusID[voznjaBusID]
	if ok {
		return state
	}

	tripID := t.tripIDResolver(voznjaBusID)
	state = newVehicleState(tripID)
	t.stateByVoznjaBusID[voznjaBusID] = state
	return state
}
