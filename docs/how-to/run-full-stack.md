# How-to: Run the Full Stack

Use this guide when you want to start, monitor, and stop the complete compose runtime.

## Phase 4 cutover note

As of **February 21, 2026**, the stack runs fully on V2 delay topics:
- `bus-delay-observed-v2`
- `bus-delay-predicted-v2`

`aggregator` consumes observed V2 events only; `realtime` consumes observed + predicted V2 events.

## Start

```bash
cp .env.example .env
docker compose up --build -d
```

Set real credentials in `.env` before startup.

## Monitor

```bash
docker compose logs -f staticsync ingester processor aggregator realtime
```

## Quick health checks

Kafka:

```bash
docker compose exec redpanda rpk topic consume bus-positions-raw -n 1
docker compose exec redpanda rpk topic consume bus-delay-observed-v2 -n 1
docker compose exec redpanda rpk topic consume bus-delay-predicted-v2 -n 1
```

Observability:

- Prometheus: <http://localhost:9090/targets>
- Grafana: <http://localhost:3000>

Realtime API:

- UI: <http://localhost:8080/>
- Snapshot: <http://localhost:8080/v1/snapshot>
- Health: <http://localhost:8080/healthz>
- Readiness: <http://localhost:8080/readyz>

Storage:

- Bronze: `data/bronze/YYYY-MM-DD/positions.parquet`
- Silver observed: `data/silver/YYYY-MM-DD/observed_delays_v2.parquet`
- Silver predicted: `data/silver/YYYY-MM-DD/predicted_delays_v2.parquet`
- Gold: `data/gold/YYYY-MM-DD/stats.parquet`

## Stop

```bash
docker compose down
```
