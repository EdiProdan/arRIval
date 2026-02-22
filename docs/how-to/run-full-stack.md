# How-to: Run the Full Stack

Use this guide to start, monitor, and stop the full compose runtime.

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
docker compose exec redpanda rpk topic consume bus-delay-observed -n 1
docker compose exec redpanda rpk topic consume bus-delay-predicted -n 1
```

Realtime API:
- UI: <http://localhost:8080/>
- Snapshot: <http://localhost:8080/v1/snapshot>
- Health: <http://localhost:8080/healthz>
- Readiness: <http://localhost:8080/readyz>

Observability:
- Prometheus: <http://localhost:9090/targets>
- Grafana: <http://localhost:3000>

Storage:
- Bronze: `data/bronze/YYYY-MM-DD/positions.parquet`
- Silver observed: `data/silver/YYYY-MM-DD/observed_delays.parquet`
- Silver predicted: `data/silver/YYYY-MM-DD/predicted_delays.parquet`
- Gold: `data/gold/YYYY-MM-DD/stats.parquet`

## Demo path

1. Confirm both delay topics produce messages.

```bash
docker compose exec redpanda rpk topic consume bus-delay-observed -n 1
docker compose exec redpanda rpk topic consume bus-delay-predicted -n 1
```

2. Confirm snapshot fields:
- `observed_delays`
- `predicted_delays`
- `meta.observed_delays_count`
- `meta.predicted_delays_count`

3. Confirm UI split boards:
- open <http://localhost:8080/>
- verify observed and predicted sections update

4. Confirm Grafana panels:
- `Prediction Volume`
- `Tracker Lock Quality`

5. Validate lock-quality reason labels used by PromQL:

```bash
curl -sS "http://localhost:9090/api/v1/query?query=sum%20by%20(reason)%20(rate(arrival_processor_tracker_skips_total%5B5m%5D))"
```

## One-time migration cleanup

If old artifacts still exist from previous naming:

```bash
for topic in bus-delay-observed bus-delay-predicted; do
  docker compose exec redpanda rpk topic delete "${topic}-v2" || true
done
find data/silver -type f -name '*_delays_v2.parquet' -delete
```

## Stop

```bash
docker compose down
```
