package contracts

// StationTimetableResponse is the popup payload for a single station timetable view.
type StationTimetableResponse struct {
	StationID     int64                 `json:"station_id"`
	StationName   string                `json:"station_name"`
	GeneratedAt   string                `json:"generated_at"`
	WindowMinutes int                   `json:"window_minutes"`
	Rows          []StationTimetableRow `json:"rows"`
}

// StationTimetableRow combines live ETA and static schedule rows in one list.
type StationTimetableRow struct {
	Status        string `json:"status"`
	TripID        string `json:"trip_id"`
	Line          string `json:"line"`
	StationID     int64  `json:"station_id"`
	StationSeq    int64  `json:"station_seq"`
	ScheduledTime string `json:"scheduled_time"`
	ETATime       string `json:"eta_time"`
	DelaySeconds  *int64 `json:"delay_seconds"`
}
