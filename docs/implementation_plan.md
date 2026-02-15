# arRIval — Implementation Plan

Each step below is self-contained with a clear deliverable. Complete one before moving to the next. Each step will get its own detailed plan when you start it.

---

## Step 0: Project Skeleton & Local Infrastructure

Set up the Go module, directory layout (`cmd/`, `internal/`, `data/`), and a `docker-compose.yml` that runs Redpanda. Verify you can produce and consume a test message from a Redpanda topic using a throwaway Go script.

**Deliverable:** `go.mod` exists, `docker-compose up` starts Redpanda, and a test program round-trips a message through a topic.

**Status:** Completed
---

## Step 1: API Client & Authentication

Write a Go package that handles the Autotrolej API: login, token refresh, and fetching `/autobusi`. Load credentials from `.env`. Write a small `main` that authenticates and prints one live snapshot to stdout.

**Deliverable:** Running `go run cmd/apiclient/main.go` prints a JSON array of live bus positions.

**Status:** Completed
---

## Step 2: Static Data Loader

Write a Go package that reads the static JSON files (stations, lines, schedule) into in-memory structs. Use `voznired_dnevni` as the timetable source. Expose lookup functions: station by ID, line-variant stops, scheduled departures by PolazakId.

**Deliverable:** A test program loads all static data and prints summary counts (e.g. "1186 stations, 155 line variants, 20581 departures loaded").

**Status:** Completed
---

## Step 3: Ingester Service

Build `cmd/ingester` — a long-running service that polls `/autobusi` every 30 seconds and publishes each raw JSON response to the Redpanda topic `bus-positions-raw`. Add basic logging (poll count, errors).

**Deliverable:** Start Redpanda + ingester, watch `bus-positions-raw` topic fill up with messages using `rpk topic consume`.

**Status**: Completed
---

## Step 4: Processor Service — Bronze Layer

Build the first half of `cmd/processor` — consume from `bus-positions-raw` and write raw events as-is to daily Parquet files under `data/bronze/YYYY-MM-DD/positions.parquet`. One row per bus per poll. Append to the day's file.

**Deliverable:** After running ingester + processor for a few minutes, `data/bronze/` contains a Parquet file you can read back with a test script or Python.

**Status**: Completed
---

## Step 5: Processor Service — Silver Layer (Delay Calculation)

Extend the processor with the enrichment logic:
1. For each live position, find the nearest station (haversine, threshold < 100m).
2. If near a station, check the schedule — is a bus expected at that station around now?
3. If yes, calculate `delay = actual_time - scheduled_time`.
4. Write enriched rows to `data/silver/YYYY-MM-DD/delays.parquet`.
5. Publish delay events to the `bus-delays` Redpanda topic.

**Deliverable:** Silver Parquet files contain rows with fields like `bus_id, station_id, line, scheduled_time, actual_time, delay_seconds`. Consume `bus-delays` topic to verify.

---

## Step 6: Aggregator Service — Gold Layer

Build `cmd/aggregator` — consume from `bus-delays`, compute hourly stats per route (average delay, p95, p99, on-time percentage), and write to `data/gold/YYYY-MM-DD/stats.parquet`.

**Deliverable:** Gold Parquet files contain hourly route-level aggregations you can query.

---

## Step 7: Prometheus Metrics

Add a `/metrics` HTTP endpoint to each service exposing Prometheus counters/histograms:
- **Ingester:** poll count, API latency, errors.
- **Processor:** messages processed, processing lag, delay distribution histogram.
- **Aggregator:** aggregations computed.

Update `docker-compose.yml` to include Prometheus, scraping all three services.

**Deliverable:** Open Prometheus UI, see metrics from all services updating in real time.

---

## Step 8: Grafana Dashboards

Add Grafana to Docker Compose with Prometheus as a data source. Build a single dashboard with panels for: live poll health, delay distribution, busiest/worst routes, and system lag.

**Deliverable:** `docker-compose up` gives you a working Grafana dashboard showing bus delay data.

---

## Step 9: Polish & Run End-to-End

Wire everything together. One `docker-compose up` starts all services. Let it run, verify data flows from API through Bronze/Silver/Gold and into Grafana. Write a short README section on how to run the full stack.

**Deliverable:** Complete working system. Clone, add `.env`, `docker-compose up`, see dashboards.
