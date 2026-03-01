import { useEffect, useMemo, useRef, useState } from "react";
import L from "leaflet";

import { formatZagrebClock } from "../utils/time";
import type { ObservedDelay, PredictedDelay, RealtimePosition, StationTimetableResponse, StationTimetableRow } from "../types";

const RIJEKA_CENTER: L.LatLngTuple = [45.3271, 14.4422];
const STATION_TIMETABLE_WINDOW_MINUTES = 60;
const STATION_TIMETABLE_CACHE_TTL_MS = 20_000;
const MIN_MARKER_ANIMATION_MS = 400;
const DEFAULT_MARKER_ANIMATION_MS = 1200;
const MAX_MARKER_ANIMATION_MS = 5000;

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

interface StationTimetableCacheEntry {
  fetchedAt: number;
  data?: StationTimetableResponse;
  error?: string;
  inFlight?: Promise<StationTimetableResponse>;
}

interface MarkerEntry {
  marker: L.Marker;
  observedAtMs: number;
  animationFrame: number | null;
  animationToken: number;
  lineLabel: string;
}

function collectBusMeta(observedDelays: ObservedDelay[], predictedDelays: PredictedDelay[]): Map<number, BusMeta> {
  const byBusID = new Map<number, BusMeta>();

  for (const delay of predictedDelays) {
    byBusID.set(delay.voznja_bus_id, {
      brojLinije: delay.broj_linije
    });
  }

  for (const delay of observedDelays) {
    byBusID.set(delay.voznja_bus_id, {
      brojLinije: delay.broj_linije
    });
  }

  return byBusID;
}

function parseTimestamp(value: string): number {
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? parsed : 0;
}

function normalizeLineLabel(value?: string): string | null {
  if (!value) {
    return null;
  }
  const trimmed = value.trim();
  if (!trimmed) {
    return null;
  }
  const withoutPrefix = trimmed.replace(/^line\s+/i, "").trim();
  return withoutPrefix || null;
}

function resolveLineLabel(meta: BusMeta | undefined): string | null {
  return normalizeLineLabel(meta?.brojLinije);
}

function escapeHtml(value: string): string {
  return value.replace(/[&<>"']/g, (match) => {
    switch (match) {
      case "&":
        return "&amp;";
      case "<":
        return "&lt;";
      case ">":
        return "&gt;";
      case "\"":
        return "&quot;";
      case "'":
        return "&#39;";
      default:
        return match;
    }
  });
}

function stationLabel(station: StationRow): string {
  if (typeof station.Naziv === "string" && station.Naziv.trim() !== "") {
    return station.Naziv;
  }
  if (typeof station.Kratki === "string" && station.Kratki.trim() !== "") {
    return station.Kratki;
  }
  return `Station ${station.StanicaId ?? ""}`.trim();
}

function replaceChildren(root: HTMLElement, child: HTMLElement): void {
  while (root.firstChild) {
    root.removeChild(root.firstChild);
  }
  root.appendChild(child);
}

function stationPopupLoadingNode(): HTMLElement {
  const loading = document.createElement("p");
  loading.className = "station-popup__loading";
  loading.textContent = "Loading arrivals...";
  return loading;
}

function stationPopupErrorNode(message: string): HTMLElement {
  const error = document.createElement("p");
  error.className = "station-popup__error";
  error.textContent = message;
  return error;
}

function sortStationRows(rows: StationTimetableRow[]): StationTimetableRow[] {
  return [...rows].sort((left, right) => {
    const byEta = parseTimestamp(left.eta_time) - parseTimestamp(right.eta_time);
    if (byEta !== 0) {
      return byEta;
    }
    const byScheduled = parseTimestamp(left.scheduled_time) - parseTimestamp(right.scheduled_time);
    if (byScheduled !== 0) {
      return byScheduled;
    }
    return left.line.localeCompare(right.line, undefined, { numeric: true, sensitivity: "base" });
  });
}

function formatStationDelay(delaySeconds: number | null, status: "live" | "scheduled"): string {
  if (typeof delaySeconds !== "number" || Number.isNaN(delaySeconds) || Math.abs(delaySeconds) < 30) {
    return status === "live" ? "on time" : "-";
  }
  const minutes = Math.max(1, Math.round(Math.abs(delaySeconds) / 60));
  return delaySeconds > 0 ? `+${minutes}m` : `-${minutes}m`;
}

function stationPopupListNode(payload: StationTimetableResponse): HTMLElement {
  if (!Array.isArray(payload.rows) || payload.rows.length === 0) {
    const empty = document.createElement("p");
    empty.className = "station-popup__empty";
    empty.textContent = `No arrivals in next ${payload.window_minutes || STATION_TIMETABLE_WINDOW_MINUTES} minutes.`;
    return empty;
  }

  const list = document.createElement("div");
  list.className = "station-popup__list";

  for (const row of sortStationRows(payload.rows)) {
    const isLive = row.status === "live";
    const item = document.createElement("div");
    item.className = `station-popup__item ${isLive ? "station-popup__item--live" : "station-popup__item--scheduled"}`;

    const linePill = document.createElement("span");
    linePill.className = "station-popup__line-pill";
    linePill.textContent = normalizeLineLabel(row.line) ?? "-";
    item.appendChild(linePill);

    const eta = document.createElement("div");
    eta.className = `station-popup__eta${isLive ? "" : " station-popup__eta--scheduled"}`;

    const etaMain = document.createElement("div");
    etaMain.className = "station-popup__eta-main";
    if (isLive) {
      const dot = document.createElement("span");
      dot.className = "station-popup__live-dot";
      dot.setAttribute("aria-label", "live");
      etaMain.appendChild(dot);
    }

    const etaValue = document.createElement("span");
    etaValue.textContent = formatZagrebClock(row.eta_time);
    etaMain.appendChild(etaValue);
    eta.appendChild(etaMain);

    item.appendChild(eta);

    const delayText = formatStationDelay(row.delay_seconds, row.status);
    const delay = document.createElement("span");
    delay.className = "station-popup__delay";
    if (delayText === "on time") {
      delay.className += " station-popup__delay--ontime";
    } else if (delayText === "-") {
      delay.className += " station-popup__delay--muted";
    } else if (delayText.startsWith("+")) {
      delay.className += " station-popup__delay--late";
    } else if (delayText.startsWith("-")) {
      delay.className += " station-popup__delay--early";
    } else {
      delay.className += " station-popup__delay--muted";
    }
    delay.textContent = delayText;
    item.appendChild(delay);

    list.appendChild(item);
  }

  return list;
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

function formatDelayMinutes(seconds: number): string {
  const minutes = Math.round(seconds / 60);
  if (minutes === 0) return "on time";
  return minutes > 0 ? `+${minutes} min` : `${minutes} min`;
}

function busTooltip(timeline?: BusTimeline): HTMLElement {
  const root = document.createElement("div");
  root.className = "bus-tooltip";

  if (!timeline || timeline.rows.length === 0) {
    const empty = document.createElement("p");
    empty.className = "bus-tooltip__empty";
    empty.textContent = "No stop timeline available.";
    root.appendChild(empty);
    return root;
  }

  const list = document.createElement("div");
  list.className = "bus-timeline";

  const lastVisitedIdx = timeline.rows.reduce(
    (acc, row, i) => (row.phase === "Visited" ? i : acc),
    -1
  );

  for (let i = 0; i < timeline.rows.length; i++) {
    const row = timeline.rows[i];
    const isVisited = row.phase === "Visited";
    const isLast = i === timeline.rows.length - 1;
    const isFirst = i === 0;
    const isTransition = i === lastVisitedIdx && lastVisitedIdx < timeline.rows.length - 1;

    const stop = document.createElement("div");
    let cls = `bus-timeline__stop ${isVisited ? "bus-timeline__stop--visited" : "bus-timeline__stop--future"}`;
    if (isFirst) cls += " bus-timeline__stop--first";
    if (isLast) cls += " bus-timeline__stop--last";
    if (isTransition) cls += " bus-timeline__stop--transition";
    stop.className = cls;

    // Dot
    const dot = document.createElement("div");
    dot.className = "bus-timeline__dot";
    stop.appendChild(dot);

    // Content
    const content = document.createElement("div");
    content.className = "bus-timeline__content";

    const name = document.createElement("span");
    name.className = "bus-timeline__station";
    name.textContent = row.stationName || "-";
    content.appendChild(name);

    const meta = document.createElement("div");
    meta.className = "bus-timeline__meta";

    const time = document.createElement("span");
    time.className = "bus-timeline__time";
    time.textContent = formatZagrebClock(row.scheduledTime);
    meta.appendChild(time);

    const delayText = formatDelayMinutes(row.delaySeconds);
    const delay = document.createElement("span");
    const delayMinutes = Math.round(row.delaySeconds / 60);
    delay.className = `bus-timeline__delay ${delayMinutes > 0 ? "bus-timeline__delay--late" : delayMinutes < 0 ? "bus-timeline__delay--early" : "bus-timeline__delay--ontime"}`;
    delay.textContent = delayText;
    meta.appendChild(delay);

    content.appendChild(meta);
    stop.appendChild(content);
    list.appendChild(stop);
  }

  root.appendChild(list);
  return root;
}

function createBusMarkerIcon(lineLabel: string): L.DivIcon {
  return L.divIcon({
    className: "bus-line-pill-marker",
    html: `<span class="bus-line-pill">${escapeHtml(lineLabel)}</span>`,
    iconSize: [34, 22],
    iconAnchor: [17, 11],
    popupAnchor: [0, -11]
  });
}

function resolveAnimationDurationMs(previousObservedAtMs: number, nextObservedAtMs: number): number {
  const deltaMs = nextObservedAtMs - previousObservedAtMs;
  if (!Number.isFinite(deltaMs) || deltaMs <= 0) {
    return DEFAULT_MARKER_ANIMATION_MS;
  }
  return Math.min(MAX_MARKER_ANIMATION_MS, Math.max(MIN_MARKER_ANIMATION_MS, deltaMs));
}

function updateBusMarkerPopup(marker: L.Marker, timeline: BusTimeline | undefined): void {
  if (marker.getPopup()) {
    marker.setPopupContent(busTooltip(timeline));
    return;
  }
  marker.bindPopup(busTooltip(timeline), {
    className: "bus-popup-layer",
    maxWidth: 560
  });
}

function animateMarkerTo(entry: MarkerEntry, target: L.LatLng, durationMs: number): void {
  if (entry.animationFrame !== null) {
    window.cancelAnimationFrame(entry.animationFrame);
    entry.animationFrame = null;
  }

  const start = entry.marker.getLatLng();
  const animationToken = entry.animationToken + 1;
  entry.animationToken = animationToken;
  const startedAt = performance.now();
  const safeDuration = Math.max(1, durationMs);

  const tick = (timestamp: number): void => {
    if (animationToken !== entry.animationToken) {
      return;
    }

    const progress = Math.min(1, (timestamp - startedAt) / safeDuration);
    const eased = progress * (2 - progress);
    const nextLat = start.lat + (target.lat - start.lat) * eased;
    const nextLon = start.lng + (target.lng - start.lng) * eased;
    entry.marker.setLatLng([nextLat, nextLon]);

    if (progress < 1) {
      entry.animationFrame = window.requestAnimationFrame(tick);
      return;
    }
    entry.animationFrame = null;
  };

  entry.animationFrame = window.requestAnimationFrame(tick);
}

export function MapPanel({ positions, observedDelays, predictedDelays, stale }: MapPanelProps): JSX.Element {
  const mapElementRef = useRef<HTMLDivElement | null>(null);
  const mapRef = useRef<L.Map | null>(null);
  const stationMarkersRef = useRef<L.LayerGroup | null>(null);
  const busMarkersRef = useRef<L.LayerGroup | null>(null);
  const markerEntriesRef = useRef<Map<string, MarkerEntry>>(new Map());
  const stationTimetableCacheRef = useRef<Map<number, StationTimetableCacheEntry>>(new Map());
  const [stations, setStations] = useState<StationRow[]>([]);
  const busMetaByID = useMemo(() => collectBusMeta(observedDelays, predictedDelays), [observedDelays, predictedDelays]);
  const busTimelineByID = useMemo(() => collectBusTimelines(observedDelays, predictedDelays), [observedDelays, predictedDelays]);

  async function fetchStationTimetable(stationID: number): Promise<StationTimetableResponse> {
    const response = await fetch(`/v1/station-arrivals?station_id=${stationID}&window_minutes=${STATION_TIMETABLE_WINDOW_MINUTES}`);
    if (!response.ok) {
      throw new Error(`station_arrivals_fetch_failed:${response.status}`);
    }

    const payload = (await response.json()) as StationTimetableResponse;
    if (!payload || typeof payload !== "object" || !Array.isArray(payload.rows)) {
      throw new Error("station_arrivals_invalid_payload");
    }
    return payload;
  }

  async function loadStationTimetable(stationID: number, container: HTMLElement): Promise<void> {
    const cache = stationTimetableCacheRef.current;
    const now = Date.now();
    const cached = cache.get(stationID);

    if (cached && cached.data && now-cached.fetchedAt < STATION_TIMETABLE_CACHE_TTL_MS) {
      replaceChildren(container, stationPopupListNode(cached.data));
      return;
    }
    if (cached && cached.error && now-cached.fetchedAt < STATION_TIMETABLE_CACHE_TTL_MS) {
      replaceChildren(container, stationPopupErrorNode("Timetable unavailable."));
      return;
    }

    replaceChildren(container, stationPopupLoadingNode());

    const inFlight = cached?.inFlight ?? fetchStationTimetable(stationID);
    cache.set(stationID, {
      fetchedAt: cached?.fetchedAt ?? 0,
      data: cached?.data,
      error: cached?.error,
      inFlight
    });

    try {
      const payload = await inFlight;
      cache.set(stationID, {
        fetchedAt: Date.now(),
        data: payload
      });
      replaceChildren(container, stationPopupListNode(payload));
    } catch {
      cache.set(stationID, {
        fetchedAt: Date.now(),
        error: "station_arrivals_fetch_failed"
      });
      replaceChildren(container, stationPopupErrorNode("Timetable unavailable."));
    }
  }

  useEffect(() => {
    if (!mapElementRef.current || mapRef.current) {
      return;
    }

    const map = L.map(mapElementRef.current, {
      zoomControl: true,
      attributionControl: true
    }).setView(RIJEKA_CENTER, 12);

    L.tileLayer("https://{s}.basemaps.cartocdn.com/light_all/{z}/{x}/{y}{r}.png", {
  maxZoom: 20,
  subdomains: "abcd",
  attribution: "&copy; OpenStreetMap contributors &copy; CARTO"
}).addTo(map);


    const stationMarkers = L.layerGroup().addTo(map);
    const busMarkers = L.layerGroup().addTo(map);

    mapRef.current = map;
    stationMarkersRef.current = stationMarkers;
    busMarkersRef.current = busMarkers;

    return () => {
      for (const entry of markerEntriesRef.current.values()) {
        entry.animationToken += 1;
        if (entry.animationFrame !== null) {
          window.cancelAnimationFrame(entry.animationFrame);
        }
      }
      markerEntriesRef.current.clear();
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

    const markerEntries = markerEntriesRef.current;
    const activeKeys = new Set<string>();

    for (const position of positions) {
      const markerKey = position.key;
      const busID = typeof position.voznja_bus_id === "number" ? position.voznja_bus_id : undefined;
      const meta = typeof busID === "number" ? busMetaByID.get(busID) : undefined;
      const timeline = typeof busID === "number" ? busTimelineByID.get(busID) : undefined;
      const lineLabel = resolveLineLabel(meta);
      if (!lineLabel) {
        const existingWithoutLabel = markerEntries.get(markerKey);
        if (existingWithoutLabel) {
          existingWithoutLabel.animationToken += 1;
          if (existingWithoutLabel.animationFrame !== null) {
            window.cancelAnimationFrame(existingWithoutLabel.animationFrame);
          }
          busMarkers.removeLayer(existingWithoutLabel.marker);
          markerEntries.delete(markerKey);
        }
        continue;
      }

      activeKeys.add(markerKey);
      const observedAtMs = parseTimestamp(position.observed_at);
      const target = L.latLng(position.lat, position.lon);
      const existing = markerEntries.get(markerKey);

      if (!existing) {
        const marker = L.marker(target, {
          icon: createBusMarkerIcon(lineLabel)
        });
        updateBusMarkerPopup(marker, timeline);
        marker.addTo(busMarkers);
        markerEntries.set(markerKey, {
          marker,
          observedAtMs,
          animationFrame: null,
          animationToken: 0,
          lineLabel
        });
        continue;
      }

      if (existing.lineLabel !== lineLabel) {
        existing.marker.setIcon(createBusMarkerIcon(lineLabel));
        existing.lineLabel = lineLabel;
      }
      updateBusMarkerPopup(existing.marker, timeline);

      if (observedAtMs < existing.observedAtMs) {
        continue;
      }

      const current = existing.marker.getLatLng();
      const hasPositionDelta = current.lat !== target.lat || current.lng !== target.lng;
      if (hasPositionDelta) {
        const durationMs = resolveAnimationDurationMs(existing.observedAtMs, observedAtMs);
        animateMarkerTo(existing, target, durationMs);
      }
      existing.observedAtMs = observedAtMs;
    }

    for (const [markerKey, existing] of markerEntries.entries()) {
      if (activeKeys.has(markerKey)) {
        continue;
      }
      existing.animationToken += 1;
      if (existing.animationFrame !== null) {
        window.cancelAnimationFrame(existing.animationFrame);
      }
      busMarkers.removeLayer(existing.marker);
      markerEntries.delete(markerKey);
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

      const label = stationLabel(station);
      const stationID = typeof station.StanicaId === "number" ? station.StanicaId : null;

      const popupRoot = document.createElement("div");
      popupRoot.className = "station-popup";
      const title = document.createElement("p");
      title.className = "station-popup__title";
      title.textContent = label;
      popupRoot.appendChild(title);

      const content = document.createElement("div");
      content.className = "station-popup__content";
      content.appendChild(stationPopupLoadingNode());
      popupRoot.appendChild(content);

      marker.bindPopup(popupRoot, {
        className: "station-popup-layer",
        maxWidth: 360
      });

      marker.on("popupopen", () => {
        if (stationID === null || stationID <= 0) {
          replaceChildren(content, stationPopupErrorNode("Station ID unavailable."));
          return;
        }
        void loadStationTimetable(stationID, content);
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
