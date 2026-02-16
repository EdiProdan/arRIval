# arRIval Current System Architecture

## 1. Document Scope

This document describes the currently implemented architecture in this repository as of 2026-02-16.
It includes only executable components, runtime dependencies, and data flows present in code and configuration.

## 2. System Context

arRIval is a Go-based data ingestion and processing system for Rijeka AUTOTROLEJ transit data.

Primary responsibilities currently implemented:
- Authenticate against AUTOTROLEJ Open API
- Poll live bus positions
- Publish raw payloads to Kafka-compatible broker (Redpanda)
- Consume raw topic and materialize Bronze+Silver Parquet output
- Consume delay topic and materialize Gold Parquet route-hour aggregates
- Download and load static OpenData reference datasets
- Provide a Kafka roundtrip connectivity smoke test
- Expose and visualize service metrics via Prometheus + Grafana (all services wired in Docker Compose)

## 3. Runtime Topology

### 3.1 Infrastructure Services

- Redpanda (single node), defined in `docker-compose.yml`
  - Image: `docker.redpanda.com/redpandadata/redpanda:v24.3.11`
  - External Kafka endpoint: `localhost:19092`
  - Pandaproxy endpoint: `localhost:18082`
  - Admin endpoint mapped: `localhost:19644`
  - Persistent volume: `redpanda_data`

- Prometheus, defined in `docker-compose.yml`
  - Image: `prom/prometheus:v2.55.1`
  - UI/API endpoint: `localhost:9090`
  - Config mount: `deploy/prometheus/prometheus.yml`
  - Scrape targets:
    - `ingester:9101`
    - `processor:9102`
    - `aggregator:9103`

- Grafana, defined in `docker-compose.yml`
  - Image: `grafana/grafana:11.6.0`
  - UI endpoint: `localhost:3000`
  - Provisioning mount: `deploy/grafana/provisioning`
  - Dashboard mount: `deploy/grafana/dashboards`
  - Default local credentials: `admin` / `admin`
  - Provisioned dashboard: `arRIval - Minimal Operations`
    - Includes live poll health, delay quantiles, system lag, and busiest/worst proxy panel

- Application services, defined in `docker-compose.yml`
  - `staticsync` (one-shot startup sync of static JSON files into `data/`)
  - `ingester` (continuous API poller -> `bus-positions-raw`)
  - `processor` (raw topic -> Bronze/Silver + `bus-delays`)
  - `aggregator` (`bus-delays` -> Gold hourly route stats)
  - Shared bind mount: `./data:/app/data`

### 3.2 External Systems

- AUTOTROLEJ API host
  - Default base URL: `https://www.rijekaplus.hr`
  - Live endpoint used by runtime: `/api/open/v1/voznired/autobusi`
  - Token endpoints used by client package:
    - `/api/open/v1/token/login`
    - `/api/open/v1/token/refresh`

- Rijeka OpenData static endpoints consumed by static sync:
  - `http://e-usluge2.rijeka.hr/OpenData/ATlinije.json`
  - `http://e-usluge2.rijeka.hr/OpenData/ATstanice.json`
  - `http://e-usluge2.rijeka.hr/OpenData/ATvoznired.json`

## 4. Build and Module Baseline

- Language: Go
- Module: `github.com/EdiProdan/arRIval`
- Go version: `1.24.0`
- Key runtime libraries:
  - `github.com/twmb/franz-go` (Kafka client)
  - `github.com/xitongsys/parquet-go`
  - `github.com/xitongsys/parquet-go-source`
  - `github.com/prometheus/client_golang`

## 5. Executable Components (Binaries)

Implemented entrypoints under `cmd/`:

### 5.1 `cmd/roundtrip`

Purpose:
- Kafka connectivity validation by producing and consuming one message on the configured topic.

Behavior:
- Uses brokers from `ARRIVAL_KAFKA_BROKERS` (default `localhost:19092`)
- Uses topic from `ARRIVAL_KAFKA_TOPIC` (default `bus-positions-raw`)
- Produces one JSON message with generated `message_id`
- Consumes exact partition/offset and prints `ROUNDTRIP_OK`

### 5.2 `cmd/apiclient`

Purpose:
- One-shot authenticated fetch of live `/autobusi` payload to stdout.

Behavior:
- Loads `.env` if present
- Requires credentials via:
  - `ARRIVAL_API_USERNAME` or `ARRIVAL_API_USER`
  - `ARRIVAL_API_PASSWORD` or `ARRIVAL_API_PASS`
- Constructs `internal/autotrolej` client and calls `GetAutobusi`
- Writes full response JSON to stdout

### 5.3 `cmd/ingester`

Purpose:
- Continuous live ingestion loop from AUTOTROLEJ API to Kafka.

Behavior:
- Loads `.env` if present
- Poll interval: `30s` (constant)
- Fetches `/autobusi` using `internal/autotrolej.Client`
- Marshals response and produces to configured Kafka topic
- Produce timeout: `15s`
- Logs poll status, bus count, topic, partition, offset, poll timestamp
- Exposes Prometheus metrics endpoint on `/metrics`
  - Default bind address: `:9101`
- Metrics emitted:
  - poll count
  - API latency histogram
  - error counter by stage
- Graceful shutdown on SIGINT/SIGTERM

### 5.4 `cmd/processor`

Purpose:
- Consume raw live topic, write Bronze and Silver Parquet datasets, and publish delay events.

Behavior:
- Loads `.env` if present
- Creates Kafka consumer group client
- Consumes topic (default `bus-positions-raw`)
- Creates Kafka producer client for delay event publishing
- Loads static reference data from local data directory for matching/enrichment
- Unmarshals messages into `autotrolej.AutobusiResponse`
- Emits one Parquet row per bus item from `res`
- Partitions output by UTC date under Bronze directory
- Performs nearest-station matching and schedule-window checks for delay enrichment
- Writes matched delay rows into date-partitioned Silver Parquet files
- Publishes matched delay events to output topic (default `bus-delays`)
- Default output path pattern:
  - `data/bronze/YYYY-MM-DD/positions.parquet`
- Silver output path pattern:
  - `data/silver/YYYY-MM-DD/delays.parquet`
- Uses Snappy compression
- Marks records for commit after successful processing path
- On malformed JSON payload: logs error and still allows commit path for that record
- If a day file already exists and writer is reopened, file is overwritten (warning logged)
- Exposes Prometheus metrics endpoint on `/metrics`
  - Default bind address: `:9102`
- Metrics emitted:
  - processed message counter
  - processing lag histogram
  - delay-seconds histogram

Parquet row schema (`bronzePositionRow`):
- `ingested_at` (string)
- `ingested_date` (string)
- `kafka_topic` (string)
- `kafka_partition` (int32)
- `kafka_offset` (int64)
- `msg` (string)
- `err` (bool)
- `gbr` (optional int64)
- `lon` (optional float64)
- `lat` (optional float64)
- `voznja_id` (optional int64)
- `voznja_bus_id` (optional int64)

### 5.5 `cmd/staticsync`

Purpose:
- One-shot download of static OpenData JSON files to local data directory.

Behavior:
- Flag: `-data-dir` (default `data`)
- Downloads three static datasets (`linije`, `stanice`, `voznired_dnevni`)
- Validates HTTP status is 2xx
- Writes files:
  - `linije.json`
  - `stanice.json`
  - `voznired_dnevni.json`

### 5.6 `cmd/staticloader`

Purpose:
- Load static data through internal loader and print summary counts.

Behavior:
- Flag: `-data-dir` (default `data`)
- Calls `staticdata.LoadFromDir`
- Prints counts for stations, line variants map, and timetable departures map

### 5.7 `cmd/staticstatus`

- Directory exists but currently contains no source files.
- No executable is currently implemented for this component.

### 5.8 `cmd/aggregator`

Purpose:
- Consume delay events and materialize Gold route-hour aggregations.

Behavior:
- Loads `.env` if present
- Consumes delay topic in consumer group mode (default topic `bus-delays`)
- Buckets events by `actual_time` hour (UTC) and route (`broj_linije`)
- Computes per-bucket metrics: average delay, p95, p99, on-time percentage
- On-time rule default: absolute delay <= 300 seconds
- Writes date-partitioned Gold Parquet output with Snappy compression
- Gold output path pattern:
  - `data/gold/YYYY-MM-DD/stats.parquet`
- Periodically flushes and also flushes on graceful shutdown
- Exposes Prometheus metrics endpoint on `/metrics`
  - Default bind address: `:9103`
- Metrics emitted:
  - aggregations computed counter

## 6. Internal Packages and Responsibilities

### 6.1 `internal/autotrolej`

Primary type:
- `Client`

Public contract:
- `NewClient(Config) (*Client, error)`
- `Login(context.Context) error`
- `RefreshToken(context.Context) error`
- `GetAutobusi(context.Context) (AutobusiResponse, error)`

Data models:
- `LiveBus`:
  - `gbr`, `lon`, `lat`, `voznjaId`, `voznjaBusId` (nullable pointers)
- `AutobusiResponse` envelope:
  - `msg` (string), `res` ([]LiveBus), `err` (bool)

Token/auth behavior:
- Stores token and acquisition time in client state
- Refreshes token based on configured TTL and refresh margin
- On auth-related `/autobusi` failure, attempts refresh then login fallback before retrying fetch

Config defaults in package:
- HTTP timeout: `15s`
- Token TTL: `60m`
- Refresh margin: `5m`

### 6.2 `internal/staticdata`

Primary type:
- `Store`

Public contract:
- `LoadFromDir(dir string) (*Store, error)`
- `(*Store) StationByID(id int) (Station, bool)`
- `(*Store) StopsByLineVariant(linVarID string) []LinePathRow`
- `(*Store) DeparturesByPolazakID(polazakID string) []TimetableStopRow`

Storage model in memory:
- Raw slices:
  - `Stations []Station`
  - `LinePaths []LinePathRow`
  - `TimetableStops []TimetableStopRow`
- Lookup indexes:
  - `StationsByID map[int]Station`
  - `LinePathsByLinVar map[string][]LinePathRow`
  - `TimetableByPolazak map[string][]TimetableStopRow`

Expected file inputs in data directory:
- `stanice.json`
- `linije.json`
- `voznired_dnevni.json`

## 7. Configuration Contract

### 7.1 Environment Variables

Defined in `.env.example` and used by binaries:

- `ARRIVAL_API_BASE_URL`
  - Default in code: `https://www.rijekaplus.hr`
- `ARRIVAL_API_USERNAME` / alias `ARRIVAL_API_USER`
- `ARRIVAL_API_PASSWORD` / alias `ARRIVAL_API_PASS`
- `ARRIVAL_KAFKA_BROKERS`
  - Default: `localhost:19092`
  - CSV list supported by parsers
- `ARRIVAL_KAFKA_TOPIC`
  - Default: `bus-positions-raw`
- `ARRIVAL_PROCESSOR_GROUP`
  - Default: `arrival-processor-bronze`
- `ARRIVAL_BRONZE_DIR`
  - Default: `data/bronze`

Additional optional variables used by `cmd/apiclient` duration parsing:
- `ARRIVAL_API_TIMEOUT`
- `ARRIVAL_API_TOKEN_TTL`
- `ARRIVAL_API_REFRESH_MARGIN`

Additional processor variables:
- `ARRIVAL_KAFKA_DELAY_TOPIC`
  - Default: `bus-delays`
- `ARRIVAL_SILVER_DIR`
  - Default: `data/silver`
- `ARRIVAL_STATIC_DIR`
  - Default: `data`

Additional aggregator variables:
- `ARRIVAL_AGGREGATOR_GROUP`
  - Default: `arrival-aggregator`
- `ARRIVAL_GOLD_DIR`
  - Default: `data/gold`
- `ARRIVAL_ON_TIME_SECONDS`
  - Default: `300`

Metrics endpoint variables:
- `ARRIVAL_INGESTER_METRICS_ADDR`
  - Default: `:9101`
- `ARRIVAL_PROCESSOR_METRICS_ADDR`
  - Default: `:9102`
- `ARRIVAL_AGGREGATOR_METRICS_ADDR`
  - Default: `:9103`

### 7.2 Dotenv Loading Pattern

- `cmd/apiclient`, `cmd/ingester`, `cmd/processor`, and `cmd/aggregator` load `.env` from repository root when file exists.
- Existing process environment values are not overwritten by `.env` loader logic.

## 8. Data Flow Architecture

### 8.1 Live Ingestion and Bronze Materialization

Flow:
1. `cmd/ingester` authenticates and fetches live data via `internal/autotrolej.Client.GetAutobusi`
2. ingester publishes full JSON envelope to Kafka topic `bus-positions-raw` (or configured topic)
3. `cmd/processor` consumes topic in consumer group `arrival-processor-bronze` (or configured group)
4. processor expands each message into one row per bus element in `res`
5. processor writes rows into date-partitioned Bronze Parquet file
6. processor enriches matched events and writes date-partitioned Silver delay rows
7. processor publishes delay events to topic `bus-delays` (or configured delay topic)
8. `cmd/aggregator` consumes delay events and writes route-hour Gold aggregates

### 8.2 Static Reference Data Flow

Flow:
1. `cmd/staticsync` downloads static JSON files into local data directory
2. `cmd/staticloader` loads same files via `internal/staticdata.LoadFromDir`
3. static indexes are created for station and trip/path lookups in memory

### 8.3 Kafka Validation Flow

Flow:
1. `cmd/roundtrip` produces a test message with generated key
2. same binary consumes exact produced partition/offset
3. successful loop prints a deterministic success line

## 9. Storage Architecture

### 9.1 Repository Data Paths

- Static files (default):
  - `data/linije.json`
  - `data/stanice.json`
  - `data/voznired_dnevni.json`

- Bronze files (default):
  - `data/bronze/YYYY-MM-DD/positions.parquet`

- Silver files (default):
  - `data/silver/YYYY-MM-DD/delays.parquet`

- Gold files (default):
  - `data/gold/YYYY-MM-DD/stats.parquet`

### 9.2 Kafka Topic Usage

Current implemented topic usage:
- Primary topic: `bus-positions-raw`
  - Producer: `cmd/ingester`
  - Consumer: `cmd/processor`
  - Also used by `cmd/roundtrip` for smoke testing unless overridden
- Delay topic: `bus-delays`
  - Producer: `cmd/processor`
  - Consumer: `cmd/aggregator`

## 10. Runtime Execution Order (Operational)

Typical current sequence:
1. Start Redpanda via compose
2. Validate broker connectivity with `cmd/roundtrip`
3. Ensure `.env` credentials are configured
4. Run `cmd/staticsync` (optional static refresh)
5. Run `cmd/ingester`
6. Run `cmd/processor`
7. Run `cmd/aggregator`
8. Start Prometheus via compose and verify targets in UI/API
9. Start Grafana via compose and verify provisioned dashboard loads

## 11. Error Handling and Resilience Characteristics

- `internal/autotrolej` handles auth token lifecycle with refresh and login fallback.
- `cmd/ingester` is long-running and continues polling after transient fetch/produce errors.
- `cmd/processor` logs consume/process errors and continues loop.
- `cmd/processor` commits records marked during successful process path; malformed payloads are logged and skipped from row emission.
- All long-running binaries support graceful shutdown via OS signals.

## 12. Repository Artifacts Relevant to Architecture

- Root-level `ingester` file is a compiled Mach-O executable artifact.
- `cmd/staticstatus/` exists as an empty directory and is not an active component.

## 13. Source Files Used for This Documentation

- `go.mod`
- `docker-compose.yml`
- `.env.example`
- `README.md`
- `cmd/apiclient/main.go`
- `cmd/ingester/main.go`
- `cmd/processor/main.go`
- `cmd/aggregator/main.go`
- `cmd/roundtrip/main.go`
- `cmd/staticsync/main.go`
- `cmd/staticloader/main.go`
- `deploy/prometheus/prometheus.yml`
- `deploy/grafana/provisioning/datasources/prometheus.yml`
- `deploy/grafana/provisioning/dashboards/dashboards.yml`
- `deploy/grafana/dashboards/arrival-minimal.json`
- `internal/autotrolej/client.go`
- `internal/metrics/metrics.go`
- `internal/staticdata/staticdata.go`
