import { MapPanel } from "./components/MapPanel";
import { useRealtimeFeed } from "./hooks/useRealtimeFeed";
import { formatZagrebTime } from "./utils/time";

function statusLabel(status: string): string {
  if (status === "live") {
    return "Live";
  }
  if (status === "reconnecting") {
    return "Reconnecting";
  }
  if (status === "stale") {
    return "Stale";
  }
  if (status === "offline") {
    return "Offline";
  }
  return "Connecting";
}

export default function App(): JSX.Element {
  const {
    connection,
    stale,
    loadingSnapshot,
    error,
    generatedAt,
    lastMessageAt,
    positions,
    observedDelays,
    predictedDelays,
    refreshSnapshot
  } = useRealtimeFeed();

  return (
    <div className="app-shell">
      <header className="topbar">
        <div className="topbar-meta">
          <span className={`status-badge status-${connection}`} data-testid="status-badge">
            {statusLabel(connection)}
          </span>
          <span data-testid="positions-count">{positions.length} positions</span>
          <span>{observedDelays.length} observed delays</span>
          <span>{predictedDelays.length} predicted delays</span>
          <button onClick={() => void refreshSnapshot()} disabled={loadingSnapshot} data-testid="refresh-button">
            {loadingSnapshot ? "Refreshing..." : "Refresh snapshot"}
          </button>
        </div>
      </header>

      {stale ? (
        <aside className="stale-banner" data-testid="stale-banner">
          Data is stale. Last message: {lastMessageAt ? formatZagrebTime(new Date(lastMessageAt).toISOString()) : "-"}
        </aside>
      ) : null}

      {error ? <aside className="error-banner">Snapshot error: {error}</aside> : null}

      <main className="layout-grid layout-grid--single">
        <MapPanel
          positions={positions}
          observedDelays={observedDelays}
          predictedDelays={predictedDelays}
          stale={stale}
        />
      </main>

      <footer className="footer-meta">
        <span>Snapshot generated: {generatedAt ? formatZagrebTime(generatedAt) : "-"}</span>
        <span>Display timezone: Europe/Zagreb</span>
      </footer>
    </div>
  );
}
