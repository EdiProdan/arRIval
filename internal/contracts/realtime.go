package contracts

type RealtimePosition struct {
	Key         string  `json:"key"`
	VoznjaBusID *int64  `json:"voznja_bus_id,omitempty"`
	GBR         *int64  `json:"gbr,omitempty"`
	Lon         float64 `json:"lon"`
	Lat         float64 `json:"lat"`
	ObservedAt  string  `json:"observed_at"`
}

type RealtimeSnapshot struct {
	GeneratedAt string               `json:"generated_at"`
	Positions   []RealtimePosition   `json:"positions"`
	Delays      []DelayEvent         `json:"delays"`
	Meta        RealtimeSnapshotMeta `json:"meta"`
}

type RealtimeSnapshotMeta struct {
	PositionsCount int `json:"positions_count"`
	DelaysCount    int `json:"delays_count"`
}

type RealtimeEnvelope struct {
	Type string `json:"type"`
	TS   string `json:"ts"`
	Data any    `json:"data"`
}

type RealtimePositionsBatch struct {
	Positions []RealtimePosition `json:"positions"`
}

type RealtimeDelayUpdate struct {
	Delay DelayEvent `json:"delay"`
}

type RealtimeHeartbeat struct {
	Status string `json:"status"`
}
