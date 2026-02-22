package realtime

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/EdiProdan/arRIval/internal/autotrolej"
	"github.com/EdiProdan/arRIval/internal/contracts"
)

func ParsePositionsRecord(payload []byte, observedAt time.Time) ([]contracts.RealtimePosition, int, error) {
	var response autotrolej.AutobusiResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, 0, fmt.Errorf("unmarshal positions payload: %w", err)
	}

	observedAt = observedAt.UTC()
	positions := make([]contracts.RealtimePosition, 0, len(response.Res))
	invalid := 0

	for _, bus := range response.Res {
		if bus.Lon == nil || bus.Lat == nil {
			invalid++
			continue
		}
		if bus.VoznjaBusID == nil && bus.GBR == nil {
			invalid++
			continue
		}

		position := contracts.RealtimePosition{
			Lon:        *bus.Lon,
			Lat:        *bus.Lat,
			ObservedAt: observedAt.Format(time.RFC3339Nano),
		}

		if bus.VoznjaBusID != nil {
			voznjaBusID := int64(*bus.VoznjaBusID)
			position.VoznjaBusID = &voznjaBusID
			position.Key = positionKey(position.VoznjaBusID, nil)
		}

		if bus.GBR != nil {
			gbr := int64(*bus.GBR)
			position.GBR = &gbr
			if position.Key == "" {
				position.Key = positionKey(nil, position.GBR)
			}
		}

		positions = append(positions, position)
	}

	return positions, invalid, nil
}

func ParseObservedDelayRecord(payload []byte) (contracts.ObservedDelay, error) {
	var event contracts.ObservedDelay
	if err := json.Unmarshal(payload, &event); err != nil {
		return contracts.ObservedDelay{}, fmt.Errorf("unmarshal observed delay payload: %w", err)
	}
	return event, nil
}

func ParsePredictedDelayRecord(payload []byte) (contracts.PredictedDelay, error) {
	var event contracts.PredictedDelay
	if err := json.Unmarshal(payload, &event); err != nil {
		return contracts.PredictedDelay{}, fmt.Errorf("unmarshal predicted delay payload: %w", err)
	}
	return event, nil
}
