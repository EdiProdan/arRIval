# arRIval V1 Implementation Plan

This plan is the high-level implementation sequence after MVP. Each phase is intentionally scoped at a planning level so it can be detailed independently before coding.

## Planning principles

- Follow phases in order to reduce risk and rework.
- Each phase starts only when the previous phase exit criteria are met.
- Each phase must produce one concrete deliverable.
- Keep V1 cost-aware and MVP-oriented.

## Phase 1 — Testing and CI foundation

**Goal**: establish a safety net before feature expansion.

**Deliverable**: automated CI pipeline that runs lint, tests, and builds on push/PR.

**Exit criteria**:

- CI is green on `main`.
- Pull requests are blocked on failing checks.
- Baseline test coverage report is generated.

**Status:** Completed

## Phase 2 — Shared contracts and refactor

**Goal**: make core logic reusable and testable across services.

**Deliverable**: shared internal packages for common helpers, domain types, and extracted business logic used by ingestion, processing, aggregation, and serving boundaries.

**Exit criteria**:

- Duplicated helper code across command entrypoints is removed.
- Shared event/data types are centralized and imported where needed.
- Core logic is testable outside `package main`.

**Status:** Completed

## Phase 3 — Realtime backend (WebSocket service)

**Goal**: provide a stable stream for live clients.

**Deliverable**: a minimal websocket-serving backend that consumes `bus-positions-raw` + `bus-delays`, provides an in-memory `GET /v1/snapshot` endpoint, and broadcasts incremental `positions_batch` / `delay_update` events.

**Scope notes (minimal)**:

- Public read-only API (no auth, no replay, no persistence).
- In-memory latest-state keyed by `voznja_bus_id`/`gbr` for positions and `voznja_bus_id + station_id` for delays.
- Health/readiness + Prometheus connection/stream/state metrics.
- Docker Compose + docs wiring for local end-to-end verification.

**Exit criteria**:

- Local end-to-end flow from pipeline to connected clients works reliably.
- Connection lifecycle behavior is defined (connect, disconnect, reconnect).
- Service health and connection metrics are exposed.

**Status:** In Progress

## Phase 4 — Realtime UI (public read-only MVP)

**Goal**: expose live operations data in a minimal, usable interface.

**Deliverable**: a realtime UI with map + delay board consuming snapshot + websocket updates.

**Exit criteria**:

- UI reflects live updates without manual refresh.
- Reconnect and stale-data states are handled visibly.
- MVP UX works against the local full stack.

**Status:** In Progress

## Phase 5 — Low-cost always-on AWS deployment

**Goal**: run the stack continuously in a budget-friendly cloud setup.

**Deliverable**: repeatable deployment for pipeline + realtime backend + UI on AWS with minimal monthly cost.

**Exit criteria**:

- Public endpoint is reachable and stable.
- Restart and recovery behavior is validated.
- Monthly cost target and operating assumptions are documented.

## Phase 6 — Post-launch hardening

**Goal**: improve reliability after first real usage.

**Deliverable**: operational hardening package (alerts, runbooks, backup/restore checks, scaling triggers).

**Exit criteria**:

- Core alerts are active and actionable.
- Incident response and rollback steps are documented.
- Capacity upgrade triggers are defined from observed metrics.

## Suggested tracking template per phase

Use this mini-template when planning each phase in detail:

- Scope
- In-scope tasks
- Out-of-scope tasks
- Risks and assumptions
- Deliverable
- Exit criteria
- Demo checklist
- Status (`Planned`, `In Progress`, `Blocked`, `Completed`)
