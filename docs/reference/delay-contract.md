# Reference: Delay Contract

This page defines the active delay contracts used by runtime services.

Status on **February 22, 2026**:
- `processor` emits observed and predicted delay events.
- `aggregator` consumes observed delay events.
- `realtime` consumes observed and predicted delay events.

## Kafka topics

| Topic | Purpose |
|---|---|
| `bus-delay-observed` | Observed stop-level delay events |
| `bus-delay-predicted` | Predicted stop-level delay/ETA events |

Go constants:
- `internal/contracts/topics.go`
- `TopicBusDelayObserved`
- `TopicBusDelayPredicted`

## Event contracts

### `ObservedDelay`

Go type: `internal/contracts/delay.go` (`ObservedDelay`)

| Field | Type | Required | Notes |
|---|---|---|---|
| `trip_id` | string | yes | Locked trip identity (`PolazakID`) |
| `voznja_bus_id` | int64 | yes | Live trip linkage key |
| `gbr` | int64 | no | Fleet identifier |
| `lin_var_id` | string | yes | Line variant |
| `broj_linije` | string | yes | Public route/line |
| `station_id` | int64 | yes | Station key |
| `station_name` | string | yes | Station label |
| `station_seq` | int64 | yes | Monotonic stop sequence |
| `scheduled_time` | string | yes | RFC3339 UTC scheduled timestamp |
| `observed_time` | string | yes | RFC3339 UTC observed timestamp |
| `delay_seconds` | int64 | yes | Signed observed delay |
| `distance_m` | float64 | yes | Vehicle-to-stop match distance |
| `tracker_version` | string | yes | Tracker build/version marker (default currently `current`) |

### `PredictedDelay`

Go type: `internal/contracts/delay.go` (`PredictedDelay`)

| Field | Type | Required | Notes |
|---|---|---|---|
| `trip_id` | string | yes | Locked trip identity (`PolazakID`) |
| `voznja_bus_id` | int64 | yes | Live trip linkage key |
| `lin_var_id` | string | yes | Line variant |
| `broj_linije` | string | yes | Public route/line |
| `station_id` | int64 | yes | Station key |
| `station_name` | string | yes | Station label |
| `station_seq` | int64 | yes | Planned stop sequence |
| `scheduled_time` | string | yes | RFC3339 UTC scheduled timestamp |
| `predicted_time` | string | yes | RFC3339 UTC predicted timestamp |
| `predicted_delay_seconds` | int64 | yes | Signed predicted delay |
| `generated_at` | string | yes | RFC3339 UTC prediction generation timestamp |
| `tracker_version` | string | yes | Tracker build/version marker (default currently `current`) |

## Realtime API payload contract

Realtime keeps `/v1/snapshot` and `/v1/ws` paths.

### Snapshot (`/v1/snapshot`)

Go type: `internal/contracts/realtime.go` (`RealtimeSnapshot`)

Top-level fields:
- `generated_at`
- `positions`
- `observed_delays`
- `predicted_delays`
- `meta.positions_count`
- `meta.observed_delays_count`
- `meta.predicted_delays_count`

State precedence:
- observed delay is authoritative for progressed stops
- when observed delay is upserted, predicted entries for the same `trip_id` and `station_seq <= observed.station_seq` are removed

### Websocket (`/v1/ws`)

Envelope type values:
- `delay_observed_update`
- `delay_prediction_update`

Payload wrappers:
- `RealtimeObservedDelayUpdate` with key `observed_delay`
- `RealtimePredictedDelayUpdate` with key `predicted_delay`
