import { MapPanel } from "./components/MapPanel";
import { useRealtimeFeed } from "./hooks/useRealtimeFeed";
import { formatZagrebTime } from "./utils/time";

function formatAgeLabel(valueMs: number | null): string {
  if (valueMs === null) {
    return "-";
  }
  const rounded = Math.max(0, Math.floor(valueMs / 1000));
  return `${rounded}s ago`;
}

export default function App(): JSX.Element {
  const {
    connection,
    dataStale,
    error,
    generatedAt,
    lastDataAt,
    positions,
    observedDelays,
    predictedDelays
  } = useRealtimeFeed();
  const isConnectionLive = connection === "live";

  return (
    <div className="app-shell">
      
      {dataStale ? (
        <aside className="stale-banner" data-testid="stale-banner">
          Data is stale. Last data timestamp: {lastDataAt ? formatZagrebTime(new Date(lastDataAt).toISOString()) : "-"}
        </aside>
      ) : null}

      {error ? <aside className="error-banner">Snapshot error: {error}</aside> : null}

      <main className="layout-grid layout-grid--single">
        <MapPanel
          positions={positions}
          observedDelays={observedDelays}
          predictedDelays={predictedDelays}
          stale={dataStale}
        />
      </main>

      <footer className="footer-meta">
        <span>Snapshot generated: {generatedAt ? formatZagrebTime(generatedAt) : "-"}</span>
        <span>Display timezone: Europe/Zagreb</span>
      </footer>
    </div>
  );
}
