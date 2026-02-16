# arRIval

Developer workspace for ingesting and analyzing Rijeka AUTOTROLEJ transit data.

## Prerequisites

- Docker + Docker Compose
- AUTOTROLEJ API credentials

## Step 9 quick start (full stack)

1. Configure credentials:
   - `cp .env.example .env`
   - edit `.env` with real `ARRIVAL_API_BASE_URL`, `ARRIVAL_API_USERNAME`, `ARRIVAL_API_PASSWORD`

2. Start everything:
   - `docker compose up --build -d`

3. Watch service logs:
   - `docker compose logs -f staticsync ingester processor aggregator`

4. Verify Kafka flow:
   - raw topic: `docker compose exec redpanda rpk topic consume bus-positions-raw -n 1`
   - delay topic: `docker compose exec redpanda rpk topic consume bus-delays -n 1`

5. Verify Parquet outputs (UTC date partition):
   - Bronze: `data/bronze/YYYY-MM-DD/positions.parquet`
   - Silver: `data/silver/YYYY-MM-DD/delays.parquet`
   - Gold: `data/gold/YYYY-MM-DD/stats.parquet`

6. Verify observability:
   - Prometheus targets: `http://localhost:9090/targets`
   - Grafana: `http://localhost:3000` (`admin` / `admin`)
   - Dashboard: `arRIval - Minimal Operations`

7. Stop stack:
   - `docker compose down`

## Notes

- `staticsync` runs once on startup and downloads static datasets into `data/`.
- Bronze/Silver/Gold are bind-mounted to local `./data` for direct inspection.
- `.env.example` defaults are Compose-first (`ARRIVAL_KAFKA_BROKERS=redpanda:9092`).

## Optional host-run mode (development)

If running binaries directly with `go run`, set `ARRIVAL_KAFKA_BROKERS=localhost:19092` and start only Redpanda with `docker compose up -d redpanda`.

## Key folders

- `cmd/ingester` - polls `/autobusi`, publishes raw snapshots
- `cmd/processor` - writes Bronze/Silver and publishes delay events
- `cmd/aggregator` - writes Gold route-hour aggregates
- `cmd/staticsync` - one-shot static data sync
- `deploy/prometheus` - scrape configuration
- `deploy/grafana` - provisioning and dashboard JSON
- `data` - Bronze/Silver/Gold outputs and static datasets