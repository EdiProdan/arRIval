package processorlogic

import (
	"strconv"
	"strings"

	"github.com/EdiProdan/arRIval/internal/staticdata"
)

const secondsPerDay = int64(24 * 60 * 60)

type v2TripStop struct {
	TripID                    string
	StationID                 int64
	StationName               string
	StationSeq                int64
	LinVarID                  string
	BrojLinije                string
	ScheduleToken             string
	ScheduleSecondsOfDay      int64
	CumulativeScheduleSeconds int64
	Lon                       float64
	Lat                       float64
	HasCoordinates            bool
}

type v2TripPlan struct {
	TripID     string
	Stops      []v2TripStop
	indexBySeq map[int64]int
}

func buildV2TripPlans(store *staticdata.Store) map[string]*v2TripPlan {
	plans := make(map[string]*v2TripPlan)
	if store == nil {
		return plans
	}

	for tripID, rows := range store.TimetableByPolazak {
		tripID = strings.TrimSpace(tripID)
		if tripID == "" {
			continue
		}

		stopsBySeq := make(map[int64]v2TripStop)
		for _, row := range rows {
			seq := int64(row.RedniBrojStanice)
			if seq <= 0 {
				continue
			}

			secondsOfDay, ok := parseScheduleSecondsOfDay(row.Polazak)
			if !ok {
				continue
			}

			stop := v2TripStop{
				TripID:               tripID,
				StationID:            int64(row.StanicaID),
				StationName:          stationNameFromRow(row, store),
				StationSeq:           seq,
				LinVarID:             row.LinVarID,
				BrojLinije:           row.BrojLinije,
				ScheduleToken:        row.Polazak,
				ScheduleSecondsOfDay: secondsOfDay,
			}

			stop.Lon, stop.Lat, stop.HasCoordinates = resolveV2StopCoordinates(row, store)

			existing, exists := stopsBySeq[seq]
			if !exists || (!existing.HasCoordinates && stop.HasCoordinates) {
				stopsBySeq[seq] = stop
			}
		}

		if len(stopsBySeq) == 0 {
			continue
		}

		seqs := make([]int64, 0, len(stopsBySeq))
		for seq := range stopsBySeq {
			seqs = append(seqs, seq)
		}
		sortInt64s(seqs)

		stops := make([]v2TripStop, 0, len(seqs))
		var (
			dayOffset int64
			prevSec   int64
		)
		for i, seq := range seqs {
			stop := stopsBySeq[seq]
			if i > 0 && stop.ScheduleSecondsOfDay < prevSec {
				dayOffset += secondsPerDay
			}
			stop.CumulativeScheduleSeconds = stop.ScheduleSecondsOfDay + dayOffset
			prevSec = stop.ScheduleSecondsOfDay
			stops = append(stops, stop)
		}

		indexBySeq := make(map[int64]int, len(stops))
		for i := range stops {
			indexBySeq[stops[i].StationSeq] = i
		}

		plans[tripID] = &v2TripPlan{
			TripID:     tripID,
			Stops:      stops,
			indexBySeq: indexBySeq,
		}
	}

	return plans
}

func stationNameFromRow(row staticdata.TimetableStopRow, store *staticdata.Store) string {
	name := strings.TrimSpace(row.Naziv)
	if name != "" {
		return name
	}
	station, ok := store.StationByID(row.StanicaID)
	if !ok {
		return ""
	}
	return strings.TrimSpace(station.Naziv)
}

func resolveV2StopCoordinates(row staticdata.TimetableStopRow, store *staticdata.Store) (float64, float64, bool) {
	if row.GpsX != nil && row.GpsY != nil {
		return *row.GpsX, *row.GpsY, true
	}

	station, ok := store.StationByID(row.StanicaID)
	if !ok {
		return 0, 0, false
	}
	if station.GpsX == nil || station.GpsY == nil {
		return 0, 0, false
	}
	return *station.GpsX, *station.GpsY, true
}

func parseScheduleSecondsOfDay(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}

	parts := strings.Split(value, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, false
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, false
	}

	second := 0
	if len(parts) == 3 {
		secPart := parts[2]
		if dot := strings.Index(secPart, "."); dot >= 0 {
			secPart = secPart[:dot]
		}
		second, err = strconv.Atoi(secPart)
		if err != nil || second < 0 || second > 59 {
			return 0, false
		}
	}

	return int64(hour*3600 + minute*60 + second), true
}

func sortInt64s(values []int64) {
	for i := 1; i < len(values); i++ {
		j := i
		for j > 0 && values[j-1] > values[j] {
			values[j-1], values[j] = values[j], values[j-1]
			j--
		}
	}
}
