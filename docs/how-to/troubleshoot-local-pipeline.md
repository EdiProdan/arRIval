# How-to: Troubleshoot Local Pipeline

Use this guide when the compose stack is up but data, topics, or metrics are missing.

## Delay flow reminder

Current delay topics:
- observed: `bus-delay-observed`
- predicted: `bus-delay-predicted`

`aggregator` consumes observed only; `realtime` consumes observed + predicted.

## API authentication failures

Symptoms:
- `ingester` logs auth errors
- no messages in `bus-positions-raw`

Checks:

```bash
cat .env | grep ARRIVAL_API_
docker compose logs --tail=200 ingester
```

Fixes:
- verify `ARRIVAL_API_BASE_URL`, `ARRIVAL_API_USERNAME`, `ARRIVAL_API_PASSWORD`
- recreate services after `.env` updates

## Redpanda connectivity issues

Checks:

```bash
docker compose ps
docker compose logs --tail=200 redpanda
docker compose exec redpanda rpk cluster info
```

Fixes:
- restart broker and dependents
- verify compose network health

## No Bronze/Silver/Gold outputs

Checks:

```bash
docker compose logs --tail=200 processor aggregator
ls -la data/bronze data/silver data/gold
```

Fixes:
- verify `ARRIVAL_BRONZE_DIR`, `ARRIVAL_SILVER_DIR`, `ARRIVAL_GOLD_DIR`
- verify bind mount `./data:/app/data`

## Missing observed or predicted UI updates

Checks:

```bash
docker compose exec redpanda rpk topic consume bus-delay-observed -n 1
docker compose exec redpanda rpk topic consume bus-delay-predicted -n 1
curl -sS http://localhost:8080/v1/snapshot
docker compose logs --tail=200 realtime processor
```

Fixes:
- confirm `ARRIVAL_KAFKA_DELAY_OBSERVED_TOPIC` and `ARRIVAL_KAFKA_DELAY_PREDICTED_TOPIC` match producer outputs
- inspect tracker skip reasons for progression/reset issues

## Lock quality panel looks wrong

Checks:

```bash
curl -sS "http://localhost:9090/api/v1/query?query=sum%20by%20(reason)%20(rate(arrival_processor_tracker_skips_total%5B5m%5D))"
```

Expected lock-related reasons:
- `no_trip_for_voznja_bus_id`
- `duplicate_station_seq`
- `backward_station_seq`
- `reset_after_invalid_progression`
- `reset_after_stale_state`

Fixes:
- ensure dashboard regex matches emitted reason labels exactly

## Missing metrics in Prometheus/Grafana

Checks:
- Prometheus targets: <http://localhost:9090/targets>

```bash
docker compose logs --tail=200 prometheus grafana
```

Fixes:
- verify `/metrics` endpoints on `:9101`, `:9102`, `:9103`, `:9104`
- restart observability services after config changes

## Cleanup old artifacts

```bash
for topic in bus-delay-observed bus-delay-predicted; do
  docker compose exec redpanda rpk topic delete "${topic}-v2" || true
done
find data/silver -type f -name '*_delays_v2.parquet' -delete
```
