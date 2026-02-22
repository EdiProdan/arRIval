import { useEffect, useMemo, useRef, useState } from "react";
import L from "leaflet";

import { formatZagrebTime } from "../utils/time";
import type { ObservedDelay, PredictedDelay, RealtimePosition } from "../types";

const RIJEKA_CENTER: L.LatLngTuple = [45.3271, 14.4422];

interface StationRow {
  StanicaId?: number;
  Naziv?: string;
  Kratki?: string;
  GpsX?: number | null;
  GpsY?: number | null;
}

interface MapPanelProps {
  positions: RealtimePosition[];
  observedDelays: ObservedDelay[];
  predictedDelays: PredictedDelay[];
  stale: boolean;
}

interface BusMeta {
  brojLinije: string;
  linVarID: string;
}

interface BusTimelineRow {
  phase: "Visited" | "Future";
  stationName: string;
  stationSeq: number;
  scheduledTime: string;
  delaySeconds: number;
}

interface BusTimeline {
  rows: BusTimelineRow[];
}

function collectBusMeta(observedDelays: ObservedDelay[], predictedDelays: PredictedDelay[]): Map<number, BusMeta> {
  const byBusID = new Map<number, BusMeta>();

  for (const delay of predictedDelays) {
    byBusID.set(delay.voznja_bus_id, {
      brojLinije: delay.broj_linije,
      linVarID: delay.lin_var_id
    });
  }

  for (const delay of observedDelays) {
    byBusID.set(delay.voznja_bus_id, {
      brojLinije: delay.broj_linije,
      linVarID: delay.lin_var_id
    });
  }

  return byBusID;
}

function parseTimestamp(value: string): number {
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? parsed : 0;
}

function collectBusTimelines(observedDelays: ObservedDelay[], predictedDelays: PredictedDelay[]): Map<number, BusTimeline> {
  const byBusID = new Map<number, Map<string, { observed: ObservedDelay[]; predicted: PredictedDelay[] }>>();

  for (const observed of observedDelays) {
    const tripMap = byBusID.get(observed.voznja_bus_id) ?? new Map<string, { observed: ObservedDelay[]; predicted: PredictedDelay[] }>();
    const timeline = tripMap.get(observed.trip_id) ?? { observed: [], predicted: [] };
    timeline.observed.push(observed);
    tripMap.set(observed.trip_id, timeline);
    byBusID.set(observed.voznja_bus_id, tripMap);
  }

  for (const predicted of predictedDelays) {
    const tripMap = byBusID.get(predicted.voznja_bus_id) ?? new Map<string, { observed: ObservedDelay[]; predicted: PredictedDelay[] }>();
    const timeline = tripMap.get(predicted.trip_id) ?? { observed: [], predicted: [] };
    timeline.predicted.push(predicted);
    tripMap.set(predicted.trip_id, timeline);
    byBusID.set(predicted.voznja_bus_id, tripMap);
  }

  const timelineByBusID = new Map<number, BusTimeline>();
  for (const [busID, tripMap] of byBusID.entries()) {
    let selected: { observed: ObservedDelay[]; predicted: PredictedDelay[] } | undefined;
    let selectedLatest = -1;
    let selectedPredictedCount = -1;
    let selectedRowCount = -1;

    for (const timeline of tripMap.values()) {
      const predictedCount = timeline.predicted.length;
      const latest = Math.max(
        ...timeline.observed.map((row) => parseTimestamp(row.observed_time)),
        ...timeline.predicted.map((row) => parseTimestamp(row.predicted_time)),
        0
      );
      const rowCount = timeline.observed.length + timeline.predicted.length;
      if (latest > selectedLatest) {
        selected = timeline;
        selectedLatest = latest;
        selectedPredictedCount = predictedCount;
        selectedRowCount = rowCount;
        continue;
      }
      if (latest === selectedLatest && predictedCount > selectedPredictedCount) {
        selected = timeline;
        selectedPredictedCount = predictedCount;
        selectedRowCount = rowCount;
        continue;
      }
      if (latest === selectedLatest && predictedCount === selectedPredictedCount && rowCount > selectedRowCount) {
        selected = timeline;
        selectedRowCount = rowCount;
      }
    }

    if (!selected) {
      continue;
    }

    const visitedRows = [...selected.observed]
      .sort((left, right) => left.station_seq - right.station_seq)
      .map((row) => ({
        phase: "Visited" as const,
        stationName: row.station_name,
        stationSeq: row.station_seq,
        scheduledTime: row.scheduled_time,
        delaySeconds: row.delay_seconds
      }));
    const futureRows = [...selected.predicted]
      .sort((left, right) => left.station_seq - right.station_seq)
      .map((row) => ({
        phase: "Future" as const,
        stationName: row.station_name,
        stationSeq: row.station_seq,
        scheduledTime: row.scheduled_time,
        delaySeconds: row.predicted_delay_seconds
      }));

    timelineByBusID.set(busID, {
      rows: [...visitedRows, ...futureRows]
    });
  }

  return timelineByBusID;
}

function busTooltip(position: RealtimePosition, meta?: BusMeta, timeline?: BusTimeline): HTMLElement {
  const number = position.gbr ?? position.voznja_bus_id;
  const root = document.createElement("div");
  root.className = "bus-tooltip";

  const title = document.createElement("p");
  title.className = "bus-tooltip__title";
  title.textContent = `Bus ${number ?? "-"}`;
  root.appendChild(title);

  if (meta?.brojLinije) {
    const line = document.createElement("p");
    line.className = "bus-tooltip__meta";
    line.textContent = `Line ${meta.brojLinije}`;
    root.appendChild(line);
  }
  if (meta?.linVarID) {
    const route = document.createElement("p");
    route.className = "bus-tooltip__meta";
    route.textContent = `Route ${meta.linVarID}`;
    root.appendChild(route);
  }

  if (!timeline || timeline.rows.length === 0) {
    const empty = document.createElement("p");
    empty.className = "bus-tooltip__empty";
    empty.textContent = "No stop timeline available.";
    root.appendChild(empty);
    return root;
  }

  const wrap = document.createElement("div");
  wrap.className = "bus-tooltip__table-wrap";
  const table = document.createElement("table");
  table.className = "bus-tooltip__table";
  const head = document.createElement("thead");
  const headRow = document.createElement("tr");
  for (const label of ["Type", "Station", "Seq", "Scheduled", "Delay (s)"]) {
    const cell = document.createElement("th");
    cell.textContent = label;
    headRow.appendChild(cell);
  }
  head.appendChild(headRow);
  table.appendChild(head);

  const body = document.createElement("tbody");
  for (const row of timeline.rows) {
    const tableRow = document.createElement("tr");
    tableRow.className = row.phase === "Visited" ? "bus-tooltip__row--visited" : "bus-tooltip__row--future";

    const phaseCell = document.createElement("td");
    phaseCell.textContent = row.phase;
    tableRow.appendChild(phaseCell);

    const stationCell = document.createElement("td");
    stationCell.textContent = row.stationName || "-";
    tableRow.appendChild(stationCell);

    const seqCell = document.createElement("td");
    seqCell.textContent = String(row.stationSeq);
    tableRow.appendChild(seqCell);

    const scheduledCell = document.createElement("td");
    scheduledCell.textContent = formatZagrebTime(row.scheduledTime);
    tableRow.appendChild(scheduledCell);

    const delayCell = document.createElement("td");
    delayCell.textContent = String(row.delaySeconds);
    delayCell.className = row.delaySeconds > 0 ? "bus-tooltip__delay--positive" : "bus-tooltip__delay--early";
    tableRow.appendChild(delayCell);

    body.appendChild(tableRow);
  }

  table.appendChild(body);
  wrap.appendChild(table);
  root.appendChild(wrap);

  return root;
}

export function MapPanel({ positions, observedDelays, predictedDelays, stale }: MapPanelProps): JSX.Element {
  const mapElementRef = useRef<HTMLDivElement | null>(null);
  const mapRef = useRef<L.Map | null>(null);
  const stationMarkersRef = useRef<L.LayerGroup | null>(null);
  const busMarkersRef = useRef<L.LayerGroup | null>(null);
  const [stations, setStations] = useState<StationRow[]>([]);
  const busMetaByID = useMemo(() => collectBusMeta(observedDelays, predictedDelays), [observedDelays, predictedDelays]);
  const busTimelineByID = useMemo(() => collectBusTimelines(observedDelays, predictedDelays), [observedDelays, predictedDelays]);

  useEffect(() => {
    if (!mapElementRef.current || mapRef.current) {
      return;
    }

    const map = L.map(mapElementRef.current, {
      zoomControl: true,
      attributionControl: true
    }).setView(RIJEKA_CENTER, 12);

    L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
      maxZoom: 19,
      attribution: "&copy; OpenStreetMap contributors"
    }).addTo(map);

    const stationMarkers = L.layerGroup().addTo(map);
    const busMarkers = L.layerGroup().addTo(map);

    mapRef.current = map;
    stationMarkersRef.current = stationMarkers;
    busMarkersRef.current = busMarkers;

    return () => {
      map.remove();
      mapRef.current = null;
      stationMarkersRef.current = null;
      busMarkersRef.current = null;
    };
  }, []);

  useEffect(() => {
    const busMarkers = busMarkersRef.current;
    if (!busMarkers) {
      return;
    }

    busMarkers.clearLayers();

    for (const position of positions) {
      const marker = L.circleMarker([position.lat, position.lon], {
        radius: 5,
        color: "#0f3b4c",
        fillColor: "#43c8b0",
        fillOpacity: 0.85,
        weight: 1
      });

      const meta = typeof position.voznja_bus_id === "number" ? busMetaByID.get(position.voznja_bus_id) : undefined;
      const timeline = typeof position.voznja_bus_id === "number" ? busTimelineByID.get(position.voznja_bus_id) : undefined;
      marker.bindPopup(busTooltip(position, meta, timeline), {
        className: "bus-popup-layer",
        maxWidth: 560
      });
      marker.addTo(busMarkers);
    }
  }, [positions, busMetaByID, busTimelineByID]);

  useEffect(() => {
    let mounted = true;

    async function loadStations(): Promise<void> {
      try {
        const response = await fetch("/v1/stations");
        if (!response.ok) {
          return;
        }
        const payload = (await response.json()) as unknown;
        if (!mounted || !Array.isArray(payload)) {
          return;
        }
        setStations(payload as StationRow[]);
      } catch {
        // Keep map functional even if station endpoint is unavailable.
      }
    }

    void loadStations();
    return () => {
      mounted = false;
    };
  }, []);

  useEffect(() => {
    const stationMarkers = stationMarkersRef.current;
    if (!stationMarkers) {
      return;
    }

    stationMarkers.clearLayers();

    for (const station of stations) {
      if (typeof station.GpsX !== "number" || typeof station.GpsY !== "number") {
        continue;
      }
      if (!Number.isFinite(station.GpsX) || !Number.isFinite(station.GpsY)) {
        continue;
      }

      const marker = L.circleMarker([station.GpsY, station.GpsX], {
        radius: 2.5,
        color: "#c96a00",
        fillColor: "#f59e0b",
        fillOpacity: 0.6,
        weight: 1
      });

      const label = typeof station.Naziv === "string" && station.Naziv.trim() !== ""
        ? station.Naziv
        : (typeof station.Kratki === "string" && station.Kratki.trim() !== "" ? station.Kratki : `Station ${station.StanicaId ?? ""}`.trim());
      marker.bindPopup(label, {
        className: "station-popup-layer",
        maxWidth: 280
      });

      marker.addTo(stationMarkers);
    }
  }, [stations]);

  return (
    <section className={`panel map-panel${stale ? " panel--stale" : ""}`}>
      <header className="panel-header">
        <h2>Live Map</h2>
        <p>{positions.length} active positions, {stations.length} stations</p>
      </header>
      <div className="map-frame" ref={mapElementRef} data-testid="map-frame" />
    </section>
  );
}
