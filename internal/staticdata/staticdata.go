package staticdata

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Station struct {
	StanicaID int      `json:"StanicaId"`
	Naziv     string   `json:"Naziv"`
	Kratki    string   `json:"Kratki"`
	GpsX      *float64 `json:"GpsX"`
	GpsY      *float64 `json:"GpsY"`
}

type LinePathRow struct {
	ID                   int    `json:"Id"`
	LinVarID             string `json:"LinVarId"`
	BrojLinije           string `json:"BrojLinije"`
	NazivVarijanteLinije string `json:"NazivVarijanteLinije"`
	Smjer                string `json:"Smjer"`
	StanicaID            int    `json:"StanicaId"`
	RedniBrojStanice     int    `json:"RedniBrojStanice"`
	Varijanta            string `json:"Varijanta"`
}

type TimetableStopRow struct {
	ID                   int      `json:"Id"`
	PolazakID            string   `json:"PolazakId"`
	StanicaID            int      `json:"StanicaId"`
	LinVarID             string   `json:"LinVarId"`
	Polazak              string   `json:"Polazak"`
	RedniBrojStanice     int      `json:"RedniBrojStanice"`
	BrojLinije           string   `json:"BrojLinije"`
	Smjer                string   `json:"Smjer"`
	Varijanta            string   `json:"Varijanta"`
	NazivVarijanteLinije string   `json:"NazivVarijanteLinije"`
	PodrucjePrometa      string   `json:"PodrucjePrometa"`
	GpsX                 *float64 `json:"GpsX"`
	GpsY                 *float64 `json:"GpsY"`
	Naziv                string   `json:"Naziv"`
}

type stationLineKey struct {
	StanicaID  int
	BrojLinije string
}

type Store struct {
	Stations              []Station
	LinePaths             []LinePathRow
	TimetableStops        []TimetableStopRow
	StationsByID          map[int]Station
	LinePathsByLinVar      map[string][]LinePathRow
	TimetableByPolazak    map[string][]TimetableStopRow
	TimetableByStationLine map[stationLineKey][]TimetableStopRow
}

func LoadFromDir(dir string) (*Store, error) {
	stationsPath := filepath.Join(dir, "stanice.json")
	linePathsPath := filepath.Join(dir, "linije.json")
	timetablePath := filepath.Join(dir, "voznired_dnevni.json")

	var stations []Station
	if err := readJSONFile(stationsPath, &stations); err != nil {
		return nil, err
	}

	var linePaths []LinePathRow
	if err := readJSONFile(linePathsPath, &linePaths); err != nil {
		return nil, err
	}

	var timetableStops []TimetableStopRow
	if err := readJSONFile(timetablePath, &timetableStops); err != nil {
		return nil, err
	}

	store := &Store{
		Stations:              stations,
		LinePaths:             linePaths,
		TimetableStops:        timetableStops,
		StationsByID:          make(map[int]Station, len(stations)),
		LinePathsByLinVar:      make(map[string][]LinePathRow),
		TimetableByPolazak:    make(map[string][]TimetableStopRow),
		TimetableByStationLine: make(map[stationLineKey][]TimetableStopRow),
	}

	for _, station := range stations {
		store.StationsByID[station.StanicaID] = station
	}

	for _, row := range linePaths {
		store.LinePathsByLinVar[row.LinVarID] = append(store.LinePathsByLinVar[row.LinVarID], row)
	}

	for _, row := range timetableStops {
		store.TimetableByPolazak[row.PolazakID] = append(store.TimetableByPolazak[row.PolazakID], row)
		slKey := stationLineKey{StanicaID: row.StanicaID, BrojLinije: row.BrojLinije}
		store.TimetableByStationLine[slKey] = append(store.TimetableByStationLine[slKey], row)
	}

	return store, nil
}

func (s *Store) StationByID(id int) (Station, bool) {
	station, ok := s.StationsByID[id]
	return station, ok
}

func (s *Store) StopsByLineVariant(linVarID string) []LinePathRow {
	return s.LinePathsByLinVar[linVarID]
}

func (s *Store) DeparturesByPolazakID(polazakID string) []TimetableStopRow {
	return s.TimetableByPolazak[polazakID]
}

func (s *Store) DeparturesByStationLine(stationID int, brojLinije string) []TimetableStopRow {
	return s.TimetableByStationLine[stationLineKey{StanicaID: stationID, BrojLinije: brojLinije}]
}

func readJSONFile(path string, target any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("unmarshal %s: %w", path, err)
	}

	return nil
}
