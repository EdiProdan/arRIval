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

## End-to-end demo path (Phase 5)

1. Confirm both V2 delay topics are producing messages:

```bash
docker compose exec redpanda rpk topic consume bus-delay-observed-v2 -n 1
docker compose exec redpanda rpk topic consume bus-delay-predicted-v2 -n 1
```

2. Confirm snapshot V2 shape:
- `observed_delays`
- `predicted_delays`
- `meta.observed_delays_count`
- `meta.predicted_delays_count`

3. Confirm UI split boards:
- open <http://localhost:8080/>
- verify "Observed Delays" and "Predicted Delays" sections are both present
- confirm counts update as realtime events arrive

4. Confirm Grafana Phase 5 panels:
- open <http://localhost:3000> dashboard `arRIval - Minimal Operations`
- check `Prediction Volume (V2)` panel
- check `Tracker Lock Quality (V2)` panel

5. Validate lock-quality reason regex before dashboard rollout:

```bash
curl -sS "http://localhost:9090/api/v1/query?query=sum%20by%20(reason)%20(rate(arrival_processor_tracker_skips_total%5B5m%5D))"
```

If the reasons in the lock panel regex do not exactly match live labels, quality score math is silently skewed.

## Stop

```bash
docker compose down
```
