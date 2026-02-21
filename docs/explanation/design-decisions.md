# Explanation: Design Decisions

This document captures why the current architecture is shaped the way it is.

## Minimal service split

The runtime is intentionally split into three long-running services:

- `ingester` (acquisition)
- `processor` (enrichment + delay event emission)
- `aggregator` (route-hour aggregation)

This keeps failure domains separate and allows independent scaling later.

## Broker-first decoupling

Redpanda topics separate acquisition from downstream processing.

Benefits:

- transient API issues do not directly block aggregation logic
- processing behavior is observable through topic inspection
- replay and backfill strategies remain possible later

## File-based medallion storage

Bronze/Silver/Gold outputs are persisted as local Parquet files.

Reasons:

- low operational overhead for MVP
- easy local inspection and notebook analysis
- clear incremental evolution path toward object storage or data lakehouse

## Static-vs-live dual truth model

Static route/schedule data and live bus snapshots do not perfectly align.

Design choice:

- preserve source-specific adapters
- normalize into project-level entities
- prefer tolerant parsing and explicit unmatched-record handling

## Compose-first operations

The default run mode is Docker Compose for reproducible local environments.

Direct host-run mode is retained as optional for development ergonomics.
