# Reference: Delay V2 Contract

This page defines the active V2 delay contracts after cutover.

Status on **February 21, 2026**:
- V2 contracts are finalized in docs and `internal/contracts`.
- Runtime services use V2 observed/predicted topics and V2 realtime payloads.

## Kafka Topics

### Active V2 topics (new)

| Topic | Purpose |
|---|---|
| `bus-delay-observed-v2` | Observed stop-level delay events |
| `bus-delay-predicted-v2` | Predicted stop-level delay/ETA events |

### Legacy topic (deprecated)

| Topic | Status |
|---|---|
| `bus-delays` | V1 only. Deprecated and scheduled for removal. |

## Event Contracts

### `ObservedDelayV2`

Go type: `internal/contracts/delay_v2.go` (`ObservedDelayV2`)

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
| `tracker_version` | string | yes | Tracker version marker (for example `v2`) |

### `PredictedDelayV2`

Go type: `internal/contracts/delay_v2.go` (`PredictedDelayV2`)

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
| `tracker_version` | string | yes | Tracker version marker (for example `v2`) |

## Realtime API V2 Payload Contract

V2 keeps `/v1/snapshot` and `/v1/ws` paths, but changes delay payload shape.

### Snapshot payload (`/v1/snapshot`)

Go type: `internal/contracts/realtime_v2.go` (`RealtimeSnapshotV2`)

- `generated_at`
- `positions`
- `observed_delays`
- `predicted_delays`
- `meta.positions_count`
- `meta.observed_delays_count`
- `meta.predicted_delays_count`

Realtime state precedence rule:
- Observed delays are authoritative for progressed stops.
- On observed upsert, predicted entries for the same `trip_id` with `station_seq <= observed.station_seq` are removed from snapshot state.

### Websocket payloads (`/v1/ws`)

Envelope type values:
- `delay_observed_update`
- `delay_prediction_update`

Payload wrappers:
- `RealtimeObservedDelayUpdate` with key `observed_delay`
- `RealtimePredictedDelayUpdate` with key `predicted_delay`

## V1 Deprecation Notice

The following V1 contracts remain only as deprecated code artifacts until final cleanup:
- Topic constant `TopicBusDelaysV1`
- Event type `DelayEvent` (`internal/contracts/delay_event.go`)
- Legacy snapshot/ws contract types in `internal/contracts/realtime.go`
