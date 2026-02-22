# How-to: Troubleshoot Local Pipeline

Use this guide when the compose stack is up but data, topics, or metrics are missing.

## V2 delay flow quick reminder

As of **February 21, 2026**, delay flow is split:
- observed topic: `bus-delay-observed-v2`
- predicted topic: `bus-delay-predicted-v2`

`aggregator` consumes observed only; `realtime` consumes observed + predicted.

## API authentication failures

Symptoms:

- `ingester` logs authentication errors
- no new messages in `bus-positions-raw`

Checks:

```bash
cat .env | grep ARRIVAL_API_
docker compose logs --tail=200 ingester
```

Fixes:

- verify `ARRIVAL_API_BASE_URL`, `ARRIVAL_API_USERNAME`, `ARRIVAL_API_PASSWORD`
- recreate services after `.env` update

## Redpanda connectivity issues

Symptoms:

- producer/consumer connection errors

Checks:

```bash
docker compose ps
docker compose logs --tail=200 redpanda
docker compose exec redpanda rpk cluster info
```

Fixes:

- restart broker and dependent services
- ensure compose network is healthy

## No Bronze/Silver/Gold outputs

Symptoms:

- topics have messages but `data/*` remains empty

Checks:

```bash
docker compose logs --tail=200 processor aggregator
ls -la data/bronze data/silver data/gold
```

Fixes:

- verify `ARRIVAL_BRONZE_DIR`, `ARRIVAL_SILVER_DIR`, `ARRIVAL_GOLD_DIR`
- check bind mount `./data:/app/data`

## Missing observed or predicted UI updates

Symptoms:

- map updates but one delay board remains empty
- snapshot has only one delay collection populated

Checks:

```bash
docker compose exec redpanda rpk topic consume bus-delay-observed-v2 -n 1
docker compose exec redpanda rpk topic consume bus-delay-predicted-v2 -n 1
curl -sS http://localhost:8080/v1/snapshot
docker compose logs --tail=200 realtime processor
```

Fixes:

- confirm producer topics via `ARRIVAL_KAFKA_DELAY_OBSERVED_TOPIC` and `ARRIVAL_KAFKA_DELAY_PREDICTED_TOPIC`
- confirm realtime consumer topic env vars are identical to processor outputs
- if observed exists but predicted does not, inspect tracker skip reasons and trip progression resets in processor logs

## Lock quality panel looks wrong

Symptoms:

- `Tracker Lock Quality (V2)` panel remains flat/zero
- lock quality jumps unexpectedly while traffic exists

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

- verify dashboard regex exactly matches emitted reason labels
- any typo in the regex silently drops that reason from the quality-score denominator
- if labels changed in tracker constants, update dashboard PromQL immediately

## Missing metrics in Prometheus/Grafana

Checks:

- Prometheus targets: <http://localhost:9090/targets>
- Grafana datasources/dashboard provisioning logs:

```bash
docker compose logs --tail=200 prometheus grafana
```

Fixes:

- confirm services expose `/metrics` on `:9101`, `:9102`, `:9103`, `:9104`
- restart observability services after config changes
