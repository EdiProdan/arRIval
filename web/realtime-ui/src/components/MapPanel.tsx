import { useEffect, useRef } from "react";
import L from "leaflet";

import type { RealtimePosition } from "../types";

const RIJEKA_CENTER: L.LatLngTuple = [45.3271, 14.4422];

interface MapPanelProps {
  positions: RealtimePosition[];
  stale: boolean;
}

export function MapPanel({ positions, stale }: MapPanelProps): JSX.Element {
  const mapElementRef = useRef<HTMLDivElement | null>(null);
  const mapRef = useRef<L.Map | null>(null);
  const markersRef = useRef<L.LayerGroup | null>(null);

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

    const markers = L.layerGroup().addTo(map);

    mapRef.current = map;
    markersRef.current = markers;

    return () => {
      map.remove();
      mapRef.current = null;
      markersRef.current = null;
    };
  }, []);

  useEffect(() => {
    const markers = markersRef.current;
    if (!markers) {
      return;
    }

    markers.clearLayers();

    for (const position of positions) {
      const marker = L.circleMarker([position.lat, position.lon], {
        radius: 5,
        color: "#0f3b4c",
        fillColor: "#43c8b0",
        fillOpacity: 0.85,
        weight: 1
      });

      marker.bindTooltip(position.key, { direction: "top" });
      marker.addTo(markers);
    }
  }, [positions]);

  return (
    <section className={`panel map-panel${stale ? " panel--stale" : ""}`}>
      <header className="panel-header">
        <h2>Live Map</h2>
        <p>{positions.length} active positions</p>
      </header>
      <div className="map-frame" ref={mapElementRef} data-testid="map-frame" />
    </section>
  );
}
