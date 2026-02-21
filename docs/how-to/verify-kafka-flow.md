# How-to: Verify Kafka Flow

Use this guide to confirm live ingestion and Phase 3 V2 delay events are flowing through Redpanda.

## Phase 3 transition note

As of **February 21, 2026**, `processor` publishes only V2 delay topics.  
`aggregator` and `realtime` still consume legacy `bus-delays` until Phase 4 cutover.

## Preconditions

- Compose stack is running
- `ingester` and `processor` are healthy

## 1) Confirm topics exist

```bash
docker compose exec redpanda rpk topic list
```

Expected topics:

- `bus-positions-raw`
- `bus-delay-observed-v2`
- `bus-delay-predicted-v2`
- `bus-delays` (legacy downstream topic, Phase 4 removal)

## 2) Consume one raw message

```bash
docker compose exec redpanda rpk topic consume bus-positions-raw -n 1
```

Expected shape:

- envelope with `msg`, `res`, `err`

## 3) Consume one observed V2 delay message

```bash
docker compose exec redpanda rpk topic consume bus-delay-observed-v2 -n 1
```

Expected shape:

- `ObservedDelayV2` fields (`trip_id`, `station_seq`, `observed_time`, `delay_seconds`, `tracker_version`, ...)

## 4) Consume one predicted V2 delay message

```bash
docker compose exec redpanda rpk topic consume bus-delay-predicted-v2 -n 1
```

Expected shape:

- `PredictedDelayV2` fields (`trip_id`, `station_seq`, `predicted_time`, `predicted_delay_seconds`, `generated_at`, ...)

## 5) Cross-check logs

```bash
docker compose logs --tail=100 ingester processor
```

Confirm:

- ingester publishes to `bus-positions-raw`
- processor consumes raw input and publishes to both V2 delay topics
