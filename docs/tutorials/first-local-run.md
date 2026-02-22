# Tutorial: First Local Run

Goal: run the full pipeline locally and verify end-to-end outputs.

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
docker compose logs -f staticsync ingester processor aggregator realtime
```

Expected:
- `staticsync` completes once
- `ingester` publishes raw snapshots
- `processor` writes Bronze/Silver and publishes delay topics
- `aggregator` writes Gold
- `realtime` serves API/UI

## 4) Verify Kafka topics

```bash
docker compose exec redpanda rpk topic consume bus-positions-raw -n 1
docker compose exec redpanda rpk topic consume bus-delay-observed -n 1
docker compose exec redpanda rpk topic consume bus-delay-predicted -n 1
```

## 5) Verify Parquet outputs

Check UTC partitions in `data/`:
- `data/bronze/YYYY-MM-DD/positions.parquet`
- `data/silver/YYYY-MM-DD/observed_delays.parquet`
- `data/silver/YYYY-MM-DD/predicted_delays.parquet`
- `data/gold/YYYY-MM-DD/stats.parquet`

## 6) Verify observability

- Prometheus: <http://localhost:9090/targets>
- Grafana: <http://localhost:3000> (`admin` / `admin`)
- Dashboard: `arRIval - Minimal Operations`

## 7) Stop the stack

```bash
docker compose down
```

## Next steps

- Full runtime operations: [`../how-to/run-full-stack.md`](../how-to/run-full-stack.md)
- Troubleshooting: [`../how-to/troubleshoot-local-pipeline.md`](../how-to/troubleshoot-local-pipeline.md)
- Contracts: [`../reference/delay-contract.md`](../reference/delay-contract.md)
