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

Output is a JSON object from `/api/open/v1/voznired/autobusi` with fields `msg`, `res`, and `err`.

## Optional: notebooks

- Static EDA: `eda/autotrolej_static_eda.ipynb`
- Live EDA: `eda/autotrolej_live_eda.ipynb`
- Cached inputs: `eda/data_raw/`

## Key folders

- `cmd/apiclient` - login + snapshot fetch CLI
- `cmd/roundtrip` - Kafka produce/consume smoke test
- `cmd/staticsync` - one-shot static OpenData downloader into `data/`
- `cmd/staticloader` - static dataset loader + summary counts
- `internal/autotrolej` - AUTOTROLEJ API client package
- `internal/staticdata` - static JSON structs, loader, and lookup maps
- `docs` - architecture/context, schema, implementation notes
- `eda` - notebooks and raw cache used for analysis
- `data` - workspace for downstream data artifacts

## Documentation

- `docs/overview.md`
- `docs/data_schema.md`
- `docs/implementation_plan.md`
- `docs/autotrolej_static_eda.html`
- `docs/autotrolej_live_eda.html`