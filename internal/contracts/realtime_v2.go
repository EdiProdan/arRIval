package contracts

const (
	RealtimeEventDelayObservedUpdateV2   = "delay_observed_update"
	RealtimeEventDelayPredictionUpdateV2 = "delay_prediction_update"
)

// RealtimeSnapshotV2 is the V2 snapshot payload contract served by realtime API.
type RealtimeSnapshotV2 struct {
	GeneratedAt     string                 `json:"generated_at"`
	Positions       []RealtimePosition     `json:"positions"`
	ObservedDelays  []ObservedDelayV2      `json:"observed_delays"`
	PredictedDelays []PredictedDelayV2     `json:"predicted_delays"`
	Meta            RealtimeSnapshotMetaV2 `json:"meta"`
}

type RealtimeSnapshotMetaV2 struct {
	PositionsCount       int `json:"positions_count"`
	ObservedDelaysCount  int `json:"observed_delays_count"`
	PredictedDelaysCount int `json:"predicted_delays_count"`
}

type RealtimeObservedDelayUpdate struct {
	ObservedDelay ObservedDelayV2 `json:"observed_delay"`
}

type RealtimePredictedDelayUpdate struct {
	PredictedDelay PredictedDelayV2 `json:"predicted_delay"`
}
