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
    connectionStale,
    loadingSnapshot,
    error,
    generatedAt,
    lastDataAt,
    serverLagMs,
    reconnectAttempt,
    positions,
    observedDelays,
    predictedDelays,
    refreshSnapshot
  } = useRealtimeFeed();
  const isConnectionLive = connection === "live";

  return (
    <div className="app-shell">
      <header className="topbar">
        <div>
          <h1>arRIval Realtime</h1>
          <p>Live transit map and stop delay feed</p>
        </div>
        <div className="topbar-meta">
          <span className="connection-indicator" aria-live="polite">
            {isConnectionLive ? (
              <span className="connection-indicator__dot" aria-label="live" title="live" />
            ) : (
              <span className={`connection-indicator__state connection-indicator__state--${connection}`}>{connection}</span>
            )}
          </span>
          <span>Last data: {formatAgeLabel(serverLagMs)}</span>
          {connectionStale ? <span>Feed heartbeat lagging</span> : null}
          {reconnectAttempt > 0 ? <span>Reconnect attempt: {reconnectAttempt}</span> : null}
          <button type="button" onClick={() => void refreshSnapshot()} disabled={loadingSnapshot}>
            {loadingSnapshot ? "Refreshing..." : "Refresh snapshot"}
          </button>
        </div>
      </header>

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
