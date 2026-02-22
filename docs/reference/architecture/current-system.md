# arRIval Current System Architecture

## 1. Scope

This page documents the runtime architecture that is currently deployed in local compose and reflected in source code.

Last verified: **February 22, 2026**.

## 2. Runtime topology

Core services:
- `ingester`: live API polling -> raw Kafka topic
- `processor`: raw topic -> Bronze/Silver + observed/predicted delay topics
- `aggregator`: observed delay topic -> Gold route-hour stats
- `realtime`: raw + observed + predicted topics -> HTTP snapshot and websocket
- `redpanda`: message broker
- `prometheus` and `grafana`: metrics and dashboarding

## 3. Kafka topics

| Topic | Producer | Consumer(s) | Payload |
|---|---|---|---|
| `bus-positions-raw` | `ingester` | `processor`, `realtime` | `autotrolej.AutobusiResponse` |
| `bus-delay-observed` | `processor` | `aggregator`, `realtime` | `contracts.ObservedDelay` |
| `bus-delay-predicted` | `processor` | `realtime` | `contracts.PredictedDelay` |

Go constants:
- `internal/contracts/topics.go`
- `TopicBusDelayObserved`
- `TopicBusDelayPredicted`

## 4. Storage layout

- Bronze: `data/bronze/YYYY-MM-DD/positions.parquet`
- Silver observed: `data/silver/YYYY-MM-DD/observed_delays.parquet`
- Silver predicted: `data/silver/YYYY-MM-DD/predicted_delays.parquet`
- Gold: `data/gold/YYYY-MM-DD/stats.parquet`

Static input cache:
- `data/linije.json`
- `data/stanice.json`
- `data/voznired_dnevni.json`

## 5. Executables

### 5.1 `cmd/ingester`

Responsibilities:
- authenticate and poll live endpoint `/autobusi`
- publish raw snapshots to `bus-positions-raw`
- expose scrape metrics

### 5.2 `cmd/processor`

Responsibilities:
- consume raw snapshots
- write Bronze rows
- run stateful tracker (`internal/processorlogic`)
- emit observed/predicted delay events
- write Silver observed/predicted parquet files
- expose processing and tracker metrics

Tracker contract surface:
- `TrackInput`
- `TrackOutput`
- `SkipReason*`
- `ObservedDelay`, `PredictedDelay`

### 5.3 `cmd/aggregator`

Responsibilities:
- consume `bus-delay-observed`
- aggregate per route/hour
- write Gold parquet

### 5.4 `cmd/realtime`

Responsibilities:
- consume raw + observed + predicted topics
- keep in-memory state with TTL pruning
- serve:
  - `GET /healthz`
  - `GET /readyz`
  - `GET /v1/snapshot`
  - `GET /v1/ws`

Websocket message types:
- `positions_batch`
- `delay_observed_update`
- `delay_prediction_update`
- `heartbeat`

## 6. Internal packages

### 6.1 `internal/contracts`

Defines active delay and realtime payload contracts:
- `delay.go`
- `realtime.go`
- `topics.go`

### 6.2 `internal/processorlogic`

Implements tracker state machine:
- trip lock and stop-order progression
- stale/reset behavior
- EMA smoothing for predictions

### 6.3 `internal/aggregatorlogic`

Implements route-hour statistics on observed delays:
- sample count
- average
- p95/p99
- on-time percentage

### 6.4 `internal/realtime`

Implements snapshot state store, websocket hub, and payload parsing.

## 7. Configuration contract

Primary environment variables:
- `ARRIVAL_KAFKA_BROKERS`
- `ARRIVAL_KAFKA_TOPIC` (raw topic)
- `ARRIVAL_KAFKA_DELAY_OBSERVED_TOPIC` (default `bus-delay-observed`)
- `ARRIVAL_KAFKA_DELAY_PREDICTED_TOPIC` (default `bus-delay-predicted`)
- `ARRIVAL_PROCESSOR_GROUP`
- `ARRIVAL_AGGREGATOR_GROUP`
- `ARRIVAL_REALTIME_GROUP`
- `ARRIVAL_BRONZE_DIR`
- `ARRIVAL_SILVER_DIR`
- `ARRIVAL_GOLD_DIR`
- `ARRIVAL_REALTIME_HTTP_ADDR`
- `ARRIVAL_REALTIME_POSITIONS_TTL`
- `ARRIVAL_REALTIME_DELAYS_TTL`

## 8. End-to-end runtime order

1. `staticsync` downloads static files.
2. `ingester` starts publishing to `bus-positions-raw`.
3. `processor` consumes raw events and publishes observed/predicted delay events.
4. `aggregator` consumes observed delay events and writes Gold.
5. `realtime` consumes all live streams and serves snapshot/websocket.

## 9. Migration cleanup notes

Old artifacts are not auto-removed by services.

Operational cleanup commands:

```bash
for topic in bus-delay-observed bus-delay-predicted; do
  docker compose exec redpanda rpk topic delete "${topic}-v2" || true
done

# old silver files
find data/silver -type f -name '*_delays_v2.parquet' -delete
```

## 10. Source files used

- `cmd/ingester/main.go`
- `cmd/processor/main.go`
- `cmd/aggregator/main.go`
- `cmd/realtime/main.go`
- `internal/contracts/*.go`
- `internal/processorlogic/*.go`
- `internal/aggregatorlogic/*.go`
- `internal/realtime/*.go`
- `docker-compose.yml`
