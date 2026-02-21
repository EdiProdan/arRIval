# arRIval V2 Delay Tracker Implementation Plan

## Summary
Build a new stateful delay tracker that replaces V1 completely.  
V2 uses trip-locked progression (`voznja_bus_id -> PolazakID`) plus stop-order progression to produce:
- observed delay events for reached stops
- predicted delay/ETA events for upcoming stops

This plan is full-stack and no-backward-compat by design. V1 delay logic, contracts, topics, and consumers are removed by the end.

## Public Interfaces and Contract Changes

### Kafka topics (new)
- `bus-delay-observed-v2`
- `bus-delay-predicted-v2`

### Kafka topics (removed)
- `bus-delays`

### V2 event types (new)
- `ObservedDelayV2`
- `PredictedDelayV2`

### `ObservedDelayV2` fields
- `trip_id` (PolazakID)
- `voznja_bus_id`
- `gbr` (optional)
- `lin_var_id`
- `broj_linije`
- `station_id`
- `station_name`
- `station_seq`
- `scheduled_time`
- `observed_time`
- `delay_seconds`
- `distance_m`
- `tracker_version`

### `PredictedDelayV2` fields
- `trip_id` (PolazakID)
- `voznja_bus_id`
- `lin_var_id`
- `broj_linije`
- `station_id`
- `station_name`
- `station_seq`
- `scheduled_time`
- `predicted_time`
- `predicted_delay_seconds`
- `generated_at`
- `tracker_version`

### Realtime API changes
- Snapshot fields become `observed_delays` and `predicted_delays`
- WS event types become `delay_observed_update` and `delay_prediction_update`
- Remove V1 `delay_update` payload shape

---

## Phase 1 - V2 Domain and Contract Reset
Goal: Define V2 delay domain, event contracts, and service boundaries before code replacement.

Deliverable: Approved V2 contract spec in docs and Go contract types compiled in `internal/contracts`.

Exit criteria:
- V2 observed/predicted schemas finalized
- Topic names fixed (`*-v2`)
- Realtime payload shape finalized
- V1 contract marked for removal

Phase 1 artifacts (contract-first, wiring-neutral):
- `docs/reference/delay-v2-contract.md` (canonical V2 runtime contract reference)
- `internal/contracts/delay_v2.go` (`ObservedDelayV2`, `PredictedDelayV2`)
- `internal/contracts/realtime_v2.go` (`RealtimeSnapshotV2`, V2 websocket payload wrappers)
- `internal/contracts/topics.go` (V2 topic constants plus deprecated V1 topic constant)
- `internal/contracts/delay_event.go` + `internal/contracts/realtime.go` marked with `Deprecated:` comments for V1 delay contracts
- `internal/contracts/delay_v2_test.go` + `internal/contracts/realtime_v2_test.go` for JSON shape and round-trip coverage

Phase 1 completion checklist:
- No runtime behavior/topic cutover in processor, aggregator, realtime
- `go test ./...` passes with both V1 and V2 contracts present
- `go build ./...` passes with V2 contracts available for later phases

**Status:** Completed

## Phase 2 - Stateful Trip Tracker Engine
Goal: Build new tracker core that locks trip and tracks forward stop progression with smoothed delay offset.

Deliverable: `internal/processorlogic` V2 engine that outputs observed + predicted events from replayed input fixtures.

Exit criteria:
- Trip lock by `voznja_bus_id -> PolazakID`
- Monotonic stop progression (`RedniBrojStanice`) enforced
- Relock/reset rules implemented for bad lock or stale state
- Core unit tests passing for normal and failure paths

## Phase 3 - Processor Cutover to V2 Data Plane
Goal: Replace current delay matcher in processor and publish only V2 events.

Deliverable: Processor writes V2 silver datasets and publishes only `bus-delay-observed-v2` + `bus-delay-predicted-v2`.

Exit criteria:
- Old nearest-station/time-window delay path removed
- V2 topics receive valid events in local stack
- Silver schemas updated for observed/predicted outputs
- Processor metrics updated to V2 semantics

## Phase 4 - Aggregator and Realtime Service Cutover
Goal: Move downstream consumers fully to V2 contracts.

Deliverable: Aggregator consumes observed V2 events; realtime consumes observed+predicted V2 events and serves new API payloads.

Exit criteria:
- Aggregator no longer consumes V1 delay topic
- Realtime snapshot/ws reflect new observed/predicted split
- Gold aggregation logic uses observed delays only
- Service integration tests pass against V2 contracts

## Phase 5 - UI and Operational Visibility Update
Goal: Expose V2 semantics in UI and monitoring.

Deliverable: UI displays observed vs predicted delays clearly; dashboards and runbooks updated for V2 topics/events.

Exit criteria:
- UI differentiates observed vs predicted values
- Health checks and troubleshooting docs updated
- Metrics panels include tracker lock quality and prediction volume
- End-to-end demo path documented

## Phase 6 - V1 Removal and Hard Cleanup
Goal: Remove all V1 delay logic and references from code/docs/config.

Deliverable: Repository contains only V2 delay tracker and V2 interfaces.

Exit criteria:
- V1 delay code deleted (`processorlogic`, contracts, consumers, docs)
- `bus-delays` references removed from code and docs
- Legacy tests removed or replaced with V2 tests
- `docs/explanation/delay.md` rewritten for V2 behavior

---

## Test Cases and Scenarios

1. Trip lock and progression
- Lock on correct `PolazakID`
- Reject backward stop sequence
- Recover from wrong lock and relock

2. Time edge cases
- Midnight boundary alignment
- Early bus (`delay_seconds < 0`)
- Large late bus and stale state expiry

3. Data quality edge cases
- Missing coordinates / missing `voznja_bus_id`
- No station match
- No valid future stops for prediction

4. Event correctness
- Observed event emitted once per reached stop
- Predicted events include all remaining stops on route
- Event timestamps and delay math are consistent

5. Consumer compatibility inside V2
- Aggregator consumes only observed V2 topic
- Realtime API and WS validate against new payloads
- UI handles predicted updates without flicker or duplication

6. End-to-end
- Full stack run produces V2 topics, V2 snapshot, and V2 UI behavior
- No V1 delay artifacts emitted anywhere

---

## Assumptions and Defaults

- No backward compatibility is required at release.
- Full-stack cutover is in-scope (processor, aggregator, realtime, UI, docs).
- V2 uses new topics/schemas, not in-place schema mutation.
- Source of truth for trip identity is `PolazakID` lock via `voznja_bus_id`.
- Observed and predicted streams are separate; analytics use observed data only.
- Planned phase count is 6, each with one concrete deliverable.
