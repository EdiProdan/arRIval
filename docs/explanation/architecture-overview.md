## Bus Delay Tracker Architecture Overview

### Stack
- Go services and CLIs
- Redpanda (single broker in local compose)
- Parquet medallion outputs on local filesystem
- Prometheus + Grafana observability

### Current data flow

```text
API Poller -> Redpanda -> Processor -> Parquet (Bronze/Silver)
                                |-> observed topic -> Aggregator -> Gold
                                |-> observed/predicted topics -> Realtime API/WS
```

### Services

1. `cmd/ingester`
- polls `/autobusi` every 30 seconds
- publishes raw envelopes to `bus-positions-raw`

2. `cmd/processor`
- consumes `bus-positions-raw`
- writes Bronze rows to `data/bronze/YYYY-MM-DD/positions.parquet`
- runs stateful tracker
- writes Silver rows to:
  - `data/silver/YYYY-MM-DD/observed_delays.parquet`
  - `data/silver/YYYY-MM-DD/predicted_delays.parquet`
- publishes delay events to:
  - `bus-delay-observed`
  - `bus-delay-predicted`

3. `cmd/aggregator`
- consumes `bus-delay-observed`
- computes hourly route stats
- writes `data/gold/YYYY-MM-DD/stats.parquet`

4. `cmd/realtime`
- consumes `bus-positions-raw`, `bus-delay-observed`, `bus-delay-predicted`
- serves `GET /v1/snapshot` and `GET /v1/ws`

### Operational notes
- static data is downloaded by `cmd/staticsync` into `data/*.json`
- `tracker_version` stays in payloads as runtime metadata (default `current`)
- old `*-v2` topics/files are cleaned up operationally, not automatically
