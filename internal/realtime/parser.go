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

func ParseDelayRecord(payload []byte) (contracts.DelayEvent, error) {
	var event contracts.DelayEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return contracts.DelayEvent{}, fmt.Errorf("unmarshal delay payload: %w", err)
	}
	return event, nil
}
