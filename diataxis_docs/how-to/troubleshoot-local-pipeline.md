# How-to: Troubleshoot Local Pipeline

Use this guide when the compose stack is up but data, topics, or metrics are missing.

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

## Missing metrics in Prometheus/Grafana

Checks:

- Prometheus targets: <http://localhost:9090/targets>
- Grafana datasources/dashboard provisioning logs:

```bash
docker compose logs --tail=200 prometheus grafana
```

Fixes:

- confirm services expose `/metrics` on `:9101`, `:9102`, `:9103`
- restart observability services after config changes
