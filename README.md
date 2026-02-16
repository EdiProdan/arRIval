# arRIval

Developer workspace for exploring and ingesting Rijeka AUTOTROLEJ transit data.

## Current scope

- Go tools for API snapshot fetch and Kafka round-trip verification
- EDA notebooks and cached raw datasets for static/live analysis
- Documentation for data schema and implementation planning

## Prerequisites

- Go `1.24+`
- Docker (for local Redpanda)
- Python/Jupyter (optional, only for notebooks)

## Quick start (Go tooling)

1. Start local broker:
	- `docker compose up -d redpanda`

2. Verify Kafka connectivity (produce + consume one test message):
	- `go run ./cmd/roundtrip`

3. Configure API credentials:
	- `cp .env.example .env`
	- edit `.env` with real values

4. Fetch one live bus snapshot:
	- `go run ./cmd/apiclient`

5. Sync static datasets into `data/`:
	- `go run ./cmd/staticsync`

6. Load static data and print summary counts:
	- `go run ./cmd/staticloader`

7. Start live ingestion loop (poll `/autobusi` every 30s and publish to Kafka):
	- `go run ./cmd/ingester`

8. Consume live messages from topic `bus-positions-raw`:
	- `rpk topic consume bus-positions-raw -X brokers=localhost:19092`
	- (optional) `export RPK_BROKERS=localhost:19092` to avoid repeating `-X brokers=...`

9. Start processor (consume raw topic, write Bronze+Silver Parquet, publish delays):
	- `go run ./cmd/processor`
	- Bronze output: `data/bronze/YYYY-MM-DD/positions.parquet` (UTC date)
	- Silver output: `data/silver/YYYY-MM-DD/delays.parquet` (UTC date)
	- Delay topic: `bus-delays`

10. Start aggregator (consume delay topic, compute hourly route stats, write Gold):
	- `go run ./cmd/aggregator`
	- Gold output: `data/gold/YYYY-MM-DD/stats.parquet` (UTC date)

11. Step 7 (minimal): expose metrics and run Prometheus:
	- default service metrics endpoints:
		- ingester: `http://localhost:9101/metrics`
		- processor: `http://localhost:9102/metrics`
		- aggregator: `http://localhost:9103/metrics`
	- start Prometheus:
		- `docker compose up -d prometheus`
	- verify targets:
		- open `http://localhost:9090/targets`
		- expected jobs: `ingester`, `processor`, `aggregator`

12. Step 8 (minimal): add Grafana with provisioned datasource + dashboard:
	- start Grafana (and Prometheus if not already running):
		- `docker compose up -d prometheus grafana`
	- open Grafana:
		- `http://localhost:3000` (default login `admin` / `admin`)
	- verify dashboard:
		- open dashboard `arRIval - Minimal Operations`
		- panels use current Step 7 metrics for poll health, delay distribution, throughput/worst-delay proxies, and processing lag

Output is a JSON object from `/api/open/v1/voznired/autobusi` with fields `msg`, `res`, and `err`.

## Optional: notebooks

- Static EDA: `eda/autotrolej_static_eda.ipynb`
- Live EDA: `eda/autotrolej_live_eda.ipynb`
- Cached inputs: `eda/data_raw/`

## Key folders

- `cmd/apiclient` - login + snapshot fetch CLI
- `cmd/ingester` - 30s poller that publishes live `/autobusi` payloads to Kafka
- `cmd/processor` - consume raw topic, write Bronze+Silver Parquet, publish delay events
- `cmd/aggregator` - consume delay topic and write Gold hourly route-level stats
- `cmd/roundtrip` - Kafka produce/consume smoke test
- `cmd/staticsync` - one-shot static OpenData downloader into `data/`
- `cmd/staticloader` - static dataset loader + summary counts
- `deploy/prometheus` - Prometheus scrape configuration for Step 7 metrics
- `deploy/grafana` - Grafana provisioning + minimal dashboard for Step 8
- `internal/autotrolej` - AUTOTROLEJ API client package
- `internal/metrics` - shared `/metrics` HTTP server helper
- `internal/staticdata` - static JSON structs, loader, and lookup maps
- `docs` - architecture/context, schema, implementation notes
- `eda` - notebooks and raw cache used for analysis
- `data` - workspace for downstream data artifacts

## Documentation

- `docs/overview.md`
- `docs/architecture_snapshot.md`
- `docs/data_schema.md`
- `docs/implementation_plan.md`
- `docs/autotrolej_static_eda.html`
- `docs/autotrolej_live_eda.html`