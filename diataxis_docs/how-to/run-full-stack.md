# How-to: Run the Full Stack

Use this guide when you want to start, monitor, and stop the complete compose runtime.

## Start

```bash
cp .env.example .env
docker compose up --build -d
```

Set real credentials in `.env` before startup.

## Monitor

```bash
docker compose logs -f staticsync ingester processor aggregator
```

## Quick health checks

Kafka:

```bash
docker compose exec redpanda rpk topic consume bus-positions-raw -n 1
docker compose exec redpanda rpk topic consume bus-delays -n 1
```

Observability:

- Prometheus: <http://localhost:9090/targets>
- Grafana: <http://localhost:3000>

Storage:

- Bronze: `data/bronze/YYYY-MM-DD/positions.parquet`
- Silver: `data/silver/YYYY-MM-DD/delays.parquet`
- Gold: `data/gold/YYYY-MM-DD/stats.parquet`

## Stop

```bash
docker compose down
```
