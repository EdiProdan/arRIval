import { useEffect, useMemo, useRef, useState } from "react";
import L from "leaflet";

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

function busTooltip(position: RealtimePosition, meta?: BusMeta): string {
  const number = position.gbr ?? position.voznja_bus_id;
  const parts = [`Bus ${number ?? "-"}`];

  if (meta?.brojLinije) {
    parts.push(`Line ${meta.brojLinije}`);
  }
  if (meta?.linVarID) {
    parts.push(`Route ${meta.linVarID}`);
  }

  return parts.join(" | ");
}

export function MapPanel({ positions, observedDelays, predictedDelays, stale }: MapPanelProps): JSX.Element {
  const mapElementRef = useRef<HTMLDivElement | null>(null);
  const mapRef = useRef<L.Map | null>(null);
  const stationMarkersRef = useRef<L.LayerGroup | null>(null);
  const busMarkersRef = useRef<L.LayerGroup | null>(null);
  const [stations, setStations] = useState<StationRow[]>([]);
  const busMetaByID = useMemo(() => collectBusMeta(observedDelays, predictedDelays), [observedDelays, predictedDelays]);

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
      marker.bindTooltip(busTooltip(position, meta), { direction: "top", opacity: 0.95 });
      marker.addTo(busMarkers);
    }
  }, [positions, busMetaByID]);

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
      marker.bindTooltip(label, { direction: "top", opacity: 0.95 });

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
