# How-to: Verify Kafka Flow

Use this guide to confirm live ingestion and derived delay events are flowing through Redpanda.

## Preconditions

- Compose stack is running
- `ingester`, `processor`, and `aggregator` services are healthy

## 1) Confirm topics exist

```bash
docker compose exec redpanda rpk topic list
```

Expected topics:

- `bus-positions-raw`
- `bus-delays`

## 2) Consume one raw message

```bash
docker compose exec redpanda rpk topic consume bus-positions-raw -n 1
```

Expected shape:

- envelope with `msg`, `res`, `err`

## 3) Consume one delay message

```bash
docker compose exec redpanda rpk topic consume bus-delays -n 1
```

Expected shape:

- enriched delay event fields (route/station/scheduled vs actual timestamps)

## 4) Cross-check logs

```bash
docker compose logs --tail=100 ingester processor aggregator
```

Confirm:

- ingester publishes to raw topic
- processor consumes raw and publishes delay events
- aggregator consumes delay events
