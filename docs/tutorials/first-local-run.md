# Tutorial: First Local Run

Goal: get the full data pipeline running locally and verify end-to-end output.

This tutorial is for first-time contributors. Follow it in order.

## Prerequisites

- Docker + Docker Compose installed
- AUTOTROLEJ API credentials

## 1) Configure credentials

```bash
cp .env.example .env
```

Edit `.env` and set:

- `ARRIVAL_API_BASE_URL`
- `ARRIVAL_API_USERNAME`
- `ARRIVAL_API_PASSWORD`

## 2) Start the stack

```bash
docker compose up --build -d
```

## 3) Watch runtime logs

```bash
docker compose logs -f staticsync ingester processor aggregator
```

What you should observe:

- `staticsync` finishes once
- `ingester` polls live data
- `processor` writes Bronze/Silver rows
- `aggregator` writes Gold aggregates

## 4) Verify Kafka topics

```bash
docker compose exec redpanda rpk topic consume bus-positions-raw -n 1
docker compose exec redpanda rpk topic consume bus-delays -n 1
```

## 5) Verify Parquet outputs

Check UTC date partitions in `data/`:

- `data/bronze/YYYY-MM-DD/positions.parquet`
- `data/silver/YYYY-MM-DD/delays.parquet`
- `data/gold/YYYY-MM-DD/stats.parquet`

## 6) Verify observability

- Prometheus targets: <http://localhost:9090/targets>
- Grafana: <http://localhost:3000> (`admin` / `admin`)
- Dashboard: `arRIval - Minimal Operations`

## 7) Stop the stack

```bash
docker compose down
```

## Next steps

- For repeat operations, use [`../how-to/run-full-stack.md`](../how-to/run-full-stack.md)
- For troubleshooting, use [`../how-to/troubleshoot-local-pipeline.md`](../how-to/troubleshoot-local-pipeline.md)
- For contracts and architecture facts, use [`../reference/`](../reference/)
