# arRIval

Data engineering and EDA workspace for Rijeka AUTOTROLEJ transit data.

## What this repository contains

- **Static EDA** notebook for schedule/station/line OpenData profiling
- **Live EDA** notebook for realtime `/voznired/autobusi` snapshot analysis
- **Raw data cache** (`eda/data_raw/`) used as local analysis input
- **Schema reference** documentation used as implementation source of truth

## Documentation map

- `docs/data_schema.md` -> canonical data schema reference for developers
- `docs/live_bus_api.md` -> OpenAPI preview schema and endpoint contracts
- `docs/intro.md` -> project framing and architecture notes
- `docs/implementation_plan.md` -> implementation roadmap and delivery phases
- `docs/autotrolej_static_eda.html` -> static EDA export
- `docs/autotrolej_live_eda.html` -> live EDA export

## EDA notebooks

### 1) Static data EDA

Notebook: `eda/autotrolej_static_eda.ipynb`

Purpose:
- loads AUTOTROLEJ OpenData static datasets
- profiles schema, nulls, duplicates, consistency and route/stop structure
- performs service-pattern and spatial checks for delay-readiness

Inputs / cache files (in `eda/data_raw/`):
- `linije.json`
- `stanice.json`
- `voznired_dnevni.json`
- `voznired_tjedan.json`
- `voznired_subota.json`
- `voznired_nedjelja.json`

Runtime note:
- set `REFRESH_CACHE = True` in notebook config to re-download source files

### 2) Live data EDA

Notebook: `eda/autotrolej_live_eda.ipynb`

Purpose:
- authenticates against the live API
- fetches one `/api/open/v1/voznired/autobusi` snapshot
- profiles payload shape and coordinate/data quality
- performs lightweight route enrichment against static schedules

Required environment variables:
- `ARRIVAL_API_BASE_URL`
- `ARRIVAL_API_USERNAME`
- `ARRIVAL_API_PASSWORD`

Live snapshots are saved to:
- `eda/data_raw/live/`

## Source-of-truth data contract

For implementation work, always start with:
- `docs/data_schema.md`

This file documents:
- canonical project entities (live vehicle status, stations, route path rows, timetable rows)
- join keys (`StanicaId`, `LinVarId`, `PolazakId`, `voznjaBusId`)
- coordinate/time conventions and validation rules
- known drift between OpenAPI models and observed JSON payloads

## Quick start

1. Open this folder in VS Code.
2. Open `eda/autotrolej_static_eda.ipynb` and run all cells.
3. (Optional) create `.env` in project root with live API credentials.
4. Open `eda/autotrolej_live_eda.ipynb` and run all cells.
5. Use `docs/data_schema.md` as reference when building parsers, joins, and downstream models.

## Step 0 local infrastructure

1. Start Redpanda:
	- `docker compose up -d redpanda`
2. Validate broker status:
	- `docker compose ps`
3. Run Go round-trip verifier (produce + consume one message on `bus-positions-raw`):
	- `go run ./cmd/roundtrip`

Optional environment overrides:
- `ARRIVAL_KAFKA_BROKERS` (default: `localhost:19092`)
- `ARRIVAL_KAFKA_TOPIC` (default: `bus-positions-raw`)

## Step 1 API client

1. Copy credentials template:
	- `cp .env.example .env`
2. Fill in `.env` with live API credentials.
3. Run API client snapshot fetch:
	- `go run ./cmd/apiclient`

Expected output:
- one JSON wrapper object from `/api/open/v1/voznired/autobusi` with fields `msg`, `res`, and `err`.

Optional environment overrides:
- `ARRIVAL_API_TIMEOUT` (duration, e.g. `15s`; or seconds as integer)
- `ARRIVAL_API_TOKEN_TTL` (default: `60m`)
- `ARRIVAL_API_REFRESH_MARGIN` (default: `5m`)