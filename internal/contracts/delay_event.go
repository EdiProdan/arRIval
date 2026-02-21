package contracts

// Deprecated: DelayEvent is the V1 delay contract on topic "bus-delays".
// Use ObservedDelayV2 and PredictedDelayV2.
type DelayEvent struct {
	PolazakID     string  `json:"polazak_id"`
	VoznjaBusID   int64   `json:"voznja_bus_id"`
	GBR           *int64  `json:"gbr,omitempty"`
	StationID     int64   `json:"station_id"`
	StationName   string  `json:"station_name"`
	DistanceM     float64 `json:"distance_m"`
	LinVarID      string  `json:"lin_var_id"`
	BrojLinije    string  `json:"broj_linije"`
	ScheduledTime string  `json:"scheduled_time"`
	ActualTime    string  `json:"actual_time"`
	DelaySeconds  int64   `json:"delay_seconds"`
}
