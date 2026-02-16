# arRIval Data Schema Reference (Source of Truth)

This document defines the **practical data contract** used by this project.
It is intended for programming work: parsing, validation, joins, enrichment, and delay-related modeling.

## 1) Scope and Trust Model

This project uses a **dual-truth** schema model:

1. **Formal API contract** from `docs/live_bus_api.md` (OpenAPI preview)
2. **Observed payloads** from cached JSON and EDA workflows in `eda/data_raw/*` and notebooks

When they differ, implementation should prefer **observed runtime/static payloads** for ingestion logic, and keep API-model compatibility where possible.

## 2) Data Sources

### 2.1 Live API (authenticated)

Main endpoints from OpenAPI preview:

- `GET /api/open/v1/token/login` (headers: `Username`, `Password`) -> token string
- `GET /api/open/v1/token/refresh` (headers: `Username`, `token`) -> token string
- `GET /api/open/v1/voznired/autobusi` (header: `token`) -> live bus positions
- `GET /api/open/v1/voznired/autobus` (header: `token`, query: `gbr`) -> single bus status
- Static-like API endpoints also exist (`linije`, `stanice`, `polasci`, `polasciStanica`, `polasciLinija`)

### 2.2 Static cached OpenData files

Located in `eda/data_raw/`:

- `linije.json`
- `stanice.json`
- `voznired_dnevni.json`

Live snapshots are stored in `eda/data_raw/live/*.json`.

## 3) Transport/Envelope Contracts

### 3.1 Live/API envelope pattern

Live API responses are wrapped in:

```json
{
  "msg": "ok",
  "res": [...],
  "err": false
}
```

Common wrapper semantics:

- `msg`: informational message
- `err`: boolean error flag
- `res`: actual payload (array/object depending on endpoint)

### 3.2 Static OpenData contract

Static files in `eda/data_raw/*.json` are observed as **root-level arrays** of flat records (no `msg/res/err` wrapper).

## 4) Canonical Entities (Project-Level)

The project should reason about these entities regardless of source format.

---

### 4.1 `LiveVehicleStatus`

Represents a realtime bus position/status row (from `/voznired/autobusi`).

| Field | Type | Nullability | Meaning |
|---|---|---|---|
| `gbr` | integer | nullable | Vehicle/fleet identifier used in operations and filtering |
| `lon` | number | nullable | Longitude |
| `lat` | number | nullable | Latitude |
| `voznjaId` | integer | nullable | Runtime trip-instance identifier (often null for non-active buses) |
| `voznjaBusId` | integer | nullable | Runtime bus-trip linkage key (used for schedule enrichment) |

Notes:

- Coordinates can be null or zero-valued placeholders in some rows.
- `voznjaId == null` is common and should not be treated as parser failure.

---

### 4.2 `Station`

Represents a stop/station from static `stanice.json`.

| Field | Type | Nullability | Meaning |
|---|---|---|---|
| `StanicaId` | integer | expected non-null | Station key used across static datasets |
| `Naziv` | string | nullable | Full station name |
| `Kratki` | string | nullable | Short station label |
| `GpsX` | number | nullable | Longitude (see coordinate convention below) |
| `GpsY` | number | nullable | Latitude |

Notes:

- Coordinate convention in this project: **`GpsX = lon`, `GpsY = lat`**.
- Some stations can have missing coordinates.

---

### 4.3 `LinePathRow`

Represents one ordered stop on a line variant path from `linije.json`.

| Field | Type | Nullability | Meaning |
|---|---|---|---|
| `Id` | integer | varies | Row identifier |
| `LinVarId` | string | expected non-null | Line-variant key |
| `BrojLinije` | string | nullable | Public route/line number |
| `NazivVarijanteLinije` | string | nullable | Variant descriptive name |
| `Smjer` | string | nullable | Direction label |
| `StanicaId` | integer | expected non-null | Station foreign key |
| `RedniBrojStanice` | integer | expected non-null | Station order index within variant |
| `Varijanta` | string | nullable | Variant descriptor |

Notes:

- (`LinVarId`, `RedniBrojStanice`) defines progression along route geometry.

---

### 4.4 `TimetableStopRow`

Represents one stop-time row in the timetable dataset (`voznired_dnevni.json`).

| Field | Type | Nullability | Meaning |
|---|---|---|---|
| `Id` | integer | varies | Row identifier |
| `PolazakId` | string | expected non-null | Departure/trip key in static timetable |
| `StanicaId` | integer | expected non-null | Station foreign key |
| `LinVarId` | string | expected non-null | Line-variant key |
| `Polazak` | string (time-like) | expected non-null | Scheduled departure/arrival time token |
| `RedniBrojStanice` | integer | expected non-null | Stop order in trip progression |
| `BrojLinije` | string | nullable | Public line number |
| `Smjer` | string | nullable | Direction |
| `Varijanta` | string | nullable | Variant label |
| `NazivVarijanteLinije` | string | nullable | Variant name |
| `PodrucjePrometa` | string | nullable | Service area label |
| `GpsX` | number | nullable | Longitude |
| `GpsY` | number | nullable | Latitude |
| `Naziv` | string | nullable | Stop name copy |

Notes:

- `Polazak` is observed as **time-of-day string**, not full ISO datetime in static files.
- Same `PolazakId` appears on multiple rows (one per stop in sequence).

## 5) Keys and Relationships

## 5.1 Primary join keys

- `StanicaId`:
  - `stanice` <-> `linije`
  - `stanice` <-> `voznired_dnevni`
- `LinVarId`:
  - `linije` <-> `voznired_dnevni`
- `PolazakId`:
  - groups ordered stop rows within `voznired_dnevni`

## 5.2 Live-to-static enrichment key

Observed EDA mapping strategy:

- `LiveVehicleStatus.voznjaBusId` <-> `TimetableStopRow.PolazakId`

This join works for a substantial subset of live rows, but not always 100%. Always handle unmatched rows.

## 5.3 Route progression

For distance/sequence calculations, use ordered pairs in each variant:

- sort by `LinVarId`, then `RedniBrojStanice`
- build consecutive stop segments
- optionally filter zero/near-zero and same-station transitions

## 6) Coordinate and Time Conventions

### 6.1 Coordinates

Project-wide geographic convention:

- `lon`/`GpsX` = longitude
- `lat`/`GpsY` = latitude

Recommended validation bounds:

- latitude in `[-90, 90]`
- longitude in `[-180, 180]`

Treat `(0, 0)` as invalid/no-position unless explicitly needed.

### 6.2 Time values

- Static timetable `Polazak` is typically parsed as time-of-day (fractional seconds may appear).
- Static `Polazak` values are interpreted in local service timezone (`Europe/Zagreb`) when aligning with live events.
- Live snapshot timestamps are captured by the client/runtime (UTC in current EDA workflow).
- Processor matching uses local-date anchoring (Europe/Zagreb) for schedule candidates while keeping event timestamps in UTC.

## 7) Known Data Quality Patterns

Observed in EDA and snapshots:

- Live rows can contain null `voznjaId`.
- Live coordinates can be null or zeroed in some rows.
- Static stations can have missing coordinates.
- Consecutive route-stop artifacts can include duplicate same-stop adjacency or near-zero segment distances.

Implementation guidance:

- Keep parser tolerant (nullable numeric fields).
- Separate ingestion validation (schema) from business filtering (active, mappable, geolocated).

## 8) OpenAPI vs Observed Payload Drift

The project must account for structural differences between OpenAPI preview and cached OpenData/live samples.

### 8.1 Main mismatches

1. **Wrapper vs array roots**
   - OpenAPI endpoints use wrapper objects (`msg/res/err`).
   - Static cache files are plain arrays.

2. **Model shape differences in static-like endpoints**
   - OpenAPI includes nested response models/dictionaries for `linije/stanice/polasci`.
   - Cached static files are flat denormalized rows.

3. **Field naming/casing differences**
   - OpenAPI examples: `gpsX/gpsY`, lowerCamelCase.
   - Static files: `GpsX/GpsY`, PascalCase.

4. **Time type differences**
   - OpenAPI timetable-related models may indicate `date-time`.
   - Static `Polazak` behaves as time-of-day string.

### 8.2 How to handle drift in code

- Build source-specific adapters:
  - `Live API adapter` (unwrap envelope)
  - `Static file adapter` (direct array normalize)
- Normalize into project-level entities listed in Section 4.
- Log/monitor schema changes instead of hard-failing on unknown optional fields.

## 9) Minimal Validation Rules (Recommended)

At ingestion time:

1. Live envelope contains keys `msg`, `err`, `res`.
2. `res` is list-like for `/autobusi`.
3. Coordinates, if present, are numeric and globally bounded.
4. Static datasets include core join keys:
   - stations: `StanicaId`
   - route-path: `LinVarId`, `StanicaId`, `RedniBrojStanice`
   - timetable: `PolazakId`, `StanicaId`, `LinVarId`, `Polazak`

At enrichment time:

5. Track join coverage for `voznjaBusId -> PolazakId`.
6. Keep unmatched records and label them explicitly.

## 10) Developer Usage Rules

- Treat this file as the schema reference for implementation.
- If a new payload field appears, update this file and note whether it is API-only, static-only, or normalized.
- If OpenAPI and observed payloads diverge further, append to Section 8 before refactoring parsers.

---

## Appendix A: Current Source Files Used to Derive This Reference

- `docs/live_bus_api.md`
- `eda/autotrolej_live_eda.ipynb`
- `eda/autotrolej_static_eda.ipynb`
- `eda/data_raw/linije.json`
- `eda/data_raw/stanice.json`
- `eda/data_raw/voznired_dnevni.json`
- `eda/data_raw/live/*.json`
