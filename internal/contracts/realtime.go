package contracts

type RealtimePosition struct {
	Key         string  `json:"key"`
	VoznjaBusID *int64  `json:"voznja_bus_id,omitempty"`
	GBR         *int64  `json:"gbr,omitempty"`
	Lon         float64 `json:"lon"`
	Lat         float64 `json:"lat"`
	ObservedAt  string  `json:"observed_at"`
}

const (
	RealtimeEventDelayObservedUpdate   = "delay_observed_update"
	RealtimeEventDelayPredictionUpdate = "delay_prediction_update"
)

// RealtimeSnapshot is the active snapshot payload contract served by realtime API.
type RealtimeSnapshot struct {
	GeneratedAt     string               `json:"generated_at"`
	Positions       []RealtimePosition   `json:"positions"`
	ObservedDelays  []ObservedDelay      `json:"observed_delays"`
	PredictedDelays []PredictedDelay     `json:"predicted_delays"`
	Meta            RealtimeSnapshotMeta `json:"meta"`
}

type RealtimeSnapshotMeta struct {
	PositionsCount       int `json:"positions_count"`
	ObservedDelaysCount  int `json:"observed_delays_count"`
	PredictedDelaysCount int `json:"predicted_delays_count"`
}

type RealtimeEnvelope struct {
	Type string `json:"type"`
	TS   string `json:"ts"`
	Data any    `json:"data"`
}

type RealtimePositionsBatch struct {
	Positions []RealtimePosition `json:"positions"`
}

type RealtimeObservedDelayUpdate struct {
	ObservedDelay ObservedDelay `json:"observed_delay"`
}

type RealtimePredictedDelayUpdate struct {
	PredictedDelay PredictedDelay `json:"predicted_delay"`
}

type RealtimeHeartbeat struct {
	Status string `json:"status"`
}
