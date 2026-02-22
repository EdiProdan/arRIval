# How-to: Verify Kafka Flow

Use this guide to confirm live ingestion and delay events are flowing through Redpanda.

## Preconditions

- Compose stack is running
- `ingester` and `processor` are healthy

## 1) Confirm topics exist

```bash
docker compose exec redpanda rpk topic list
```

Expected topics:
- `bus-positions-raw`
- `bus-delay-observed`
- `bus-delay-predicted`

## 2) Consume one raw message

```bash
docker compose exec redpanda rpk topic consume bus-positions-raw -n 1
```

Expected envelope keys:
- `msg`
- `res`
- `err`

## 3) Consume one observed delay message

```bash
docker compose exec redpanda rpk topic consume bus-delay-observed -n 1
```

Expected fields include:
- `trip_id`
- `station_seq`
- `observed_time`
- `delay_seconds`
- `tracker_version`

## 4) Consume one predicted delay message

```bash
docker compose exec redpanda rpk topic consume bus-delay-predicted -n 1
```

Expected fields include:
- `trip_id`
- `station_seq`
- `predicted_time`
- `predicted_delay_seconds`
- `generated_at`

## 5) Cross-check logs

```bash
docker compose logs --tail=100 ingester processor
```

Confirm:
- ingester publishes to `bus-positions-raw`
- processor publishes to both delay topics
