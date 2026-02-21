package contracts

// ObservedDelayV2 is emitted when a vehicle has reached a scheduled stop and
// a real observed delay can be calculated.
type ObservedDelayV2 struct {
	TripID         string  `json:"trip_id"`
	VoznjaBusID    int64   `json:"voznja_bus_id"`
	GBR            *int64  `json:"gbr,omitempty"`
	LinVarID       string  `json:"lin_var_id"`
	BrojLinije     string  `json:"broj_linije"`
	StationID      int64   `json:"station_id"`
	StationName    string  `json:"station_name"`
	StationSeq     int64   `json:"station_seq"`
	ScheduledTime  string  `json:"scheduled_time"`
	ObservedTime   string  `json:"observed_time"`
	DelaySeconds   int64   `json:"delay_seconds"`
	DistanceM      float64 `json:"distance_m"`
	TrackerVersion string  `json:"tracker_version"`
}

// PredictedDelayV2 is emitted for future stops on the currently tracked trip.
type PredictedDelayV2 struct {
	TripID                string `json:"trip_id"`
	VoznjaBusID           int64  `json:"voznja_bus_id"`
	LinVarID              string `json:"lin_var_id"`
	BrojLinije            string `json:"broj_linije"`
	StationID             int64  `json:"station_id"`
	StationName           string `json:"station_name"`
	StationSeq            int64  `json:"station_seq"`
	ScheduledTime         string `json:"scheduled_time"`
	PredictedTime         string `json:"predicted_time"`
	PredictedDelaySeconds int64  `json:"predicted_delay_seconds"`
	GeneratedAt           string `json:"generated_at"`
	TrackerVersion        string `json:"tracker_version"`
}
