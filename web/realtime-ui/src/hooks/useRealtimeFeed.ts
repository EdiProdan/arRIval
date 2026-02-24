import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { resolveTransport, parseEnvelope } from "./transport";
import { applyEnvelope, fromSnapshot, type RealtimeCollections } from "../state/realtimeState";
import { isStale, nextReconnectDelayMs } from "../utils/reconnect";
import type { ConnectionState, ObservedDelay, PredictedDelay, RealtimePosition } from "../types";

const DEFAULT_SOURCE_INTERVAL_MS = 30_000;
const DEFAULT_HEARTBEAT_INTERVAL_MS = 20_000;
const MIN_STALE_THRESHOLD_MS = 15_000;
const UI_TICK_MS = 250;

interface FeedState extends RealtimeCollections {
  sourceIntervalMs: number;
  heartbeatIntervalMs: number;
}

const initialState: FeedState = {
  generatedAt: "",
  positionsByKey: {},
  observedByKey: {},
  predictedByKey: {},
  sourceIntervalMs: DEFAULT_SOURCE_INTERVAL_MS,
  heartbeatIntervalMs: DEFAULT_HEARTBEAT_INTERVAL_MS
};

export interface RealtimeFeedModel {
  connection: ConnectionState;
  stale: boolean;
  dataStale: boolean;
  connectionStale: boolean;
  loadingSnapshot: boolean;
  error: string;
  generatedAt: string;
  lastMessageAt: number | null;
  lastDataAt: number | null;
  serverLagMs: number | null;
  reconnectAttempt: number;
  positions: RealtimePosition[];
  observedDelays: ObservedDelay[];
  predictedDelays: PredictedDelay[];
  refreshSnapshot: () => Promise<void>;
}

function parseTimestampMs(value: string): number | null {
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? parsed : null;
}

function normalizeIntervalMs(value: number | undefined, fallback: number): number {
  if (typeof value !== "number" || !Number.isFinite(value) || value <= 0) {
    return fallback;
  }
  return Math.floor(value);
}

function connectionStaleAfterMs(heartbeatIntervalMs: number): number {
  return Math.max(2 * heartbeatIntervalMs + 5000, MIN_STALE_THRESHOLD_MS);
}

function dataStaleAfterMs(sourceIntervalMs: number): number {
  return Math.max(3 * sourceIntervalMs, MIN_STALE_THRESHOLD_MS);
}

export function useRealtimeFeed(): RealtimeFeedModel {
  const transport = useMemo(() => resolveTransport(), []);
  const connectionRef = useRef<{ close: () => void } | null>(null);
  const retryTimerRef = useRef<number | null>(null);
  const retryCountRef = useRef(0);
  const closedByAppRef = useRef(false);
  const connectionStateRef = useRef<ConnectionState>("connecting");

  const [state, setState] = useState<FeedState>(initialState);
  const [connection, setConnection] = useState<ConnectionState>("connecting");
  const [loadingSnapshot, setLoadingSnapshot] = useState(false);
  const [error, setError] = useState("");
  const [lastEnvelopeReceivedAt, setLastEnvelopeReceivedAt] = useState<number | null>(null);
  const [lastDataAt, setLastDataAt] = useState<number | null>(null);
  const [reconnectAttempt, setReconnectAttempt] = useState(0);
  const [nowMs, setNowMs] = useState(() => Date.now());

  const clearRetryTimer = useCallback(() => {
    if (retryTimerRef.current !== null) {
      window.clearTimeout(retryTimerRef.current);
      retryTimerRef.current = null;
    }
  }, []);

  const loadSnapshot = useCallback(
    async (showLoading: boolean) => {
      if (showLoading) {
        setLoadingSnapshot(true);
      }

      try {
        const snapshot = await transport.fetchSnapshot();
        const nextCollections = fromSnapshot(snapshot);
        setState({
          ...nextCollections,
          sourceIntervalMs: normalizeIntervalMs(snapshot.meta?.source_interval_ms, DEFAULT_SOURCE_INTERVAL_MS),
          heartbeatIntervalMs: normalizeIntervalMs(snapshot.meta?.heartbeat_interval_ms, DEFAULT_HEARTBEAT_INTERVAL_MS)
        });
        const generatedAtMs = parseTimestampMs(snapshot.generated_at);
        if (generatedAtMs !== null) {
          setLastDataAt(generatedAtMs);
        }
        setError("");
      } catch (fetchError) {
        const message = fetchError instanceof Error ? fetchError.message : "snapshot_fetch_failed";
        setError(message);
      } finally {
        if (showLoading) {
          setLoadingSnapshot(false);
        }
      }
    },
    [transport]
  );

  const connect = useCallback(
    (attempt: number) => {
      if (attempt === 0) {
        setConnection("connecting");
      }

      const connectionHandle = transport.connect({
        onOpen: () => {
          clearRetryTimer();
          retryCountRef.current = 0;
          setReconnectAttempt(0);
          setConnection("live");

          if (attempt > 0) {
            void loadSnapshot(false);
          }
        },
        onClose: () => {
          if (closedByAppRef.current) {
            return;
          }

          const online = window.navigator.onLine;
          const thisAttempt = retryCountRef.current;
          retryCountRef.current = thisAttempt + 1;
          setReconnectAttempt(retryCountRef.current);
          setConnection(online ? "reconnecting" : "offline");

          const delay = nextReconnectDelayMs(thisAttempt);
          clearRetryTimer();
          retryTimerRef.current = window.setTimeout(() => {
            if (closedByAppRef.current) {
              return;
            }
            connect(retryCountRef.current);
          }, delay);
        },
        onMessage: (raw) => {
          const envelope = parseEnvelope(raw);
          if (!envelope) {
            return;
          }

          const receivedAt = Date.now();
          setLastEnvelopeReceivedAt(receivedAt);
          setConnection("live");

          const serverTimestampMs = parseTimestampMs(envelope.ts);
          if (envelope.type === "heartbeat") {
            return;
          }

          if (serverTimestampMs !== null) {
            setLastDataAt(serverTimestampMs);
          }

          setState((previous) => {
            const nextCollections = applyEnvelope(previous, envelope);
            return {
              ...nextCollections,
              sourceIntervalMs: previous.sourceIntervalMs,
              heartbeatIntervalMs: previous.heartbeatIntervalMs
            };
          });
        }
      });

      connectionRef.current = connectionHandle;
    },
    [clearRetryTimer, loadSnapshot, transport]
  );

  useEffect(() => {
    connectionStateRef.current = connection;
  }, [connection]);

  useEffect(() => {
    closedByAppRef.current = false;

    void loadSnapshot(true).finally(() => {
      connect(0);
    });

    return () => {
      closedByAppRef.current = true;
      clearRetryTimer();
      if (connectionRef.current) {
        connectionRef.current.close();
      }
    };
  }, [clearRetryTimer, connect, loadSnapshot]);

  useEffect(() => {
    const ticker = window.setInterval(() => {
      setNowMs(Date.now());
    }, UI_TICK_MS);

    return () => {
      window.clearInterval(ticker);
    };
  }, []);

  useEffect(() => {
    const handleOnline = () => {
      if (closedByAppRef.current) {
        return;
      }
      const currentState = connectionStateRef.current;
      if (currentState === "live" || currentState === "connecting") {
        return;
      }
      clearRetryTimer();
      setConnection("reconnecting");
      connect(retryCountRef.current);
    };

    const handleOffline = () => {
      if (closedByAppRef.current) {
        return;
      }
      setConnection("offline");
    };

    window.addEventListener("online", handleOnline);
    window.addEventListener("offline", handleOffline);
    return () => {
      window.removeEventListener("online", handleOnline);
      window.removeEventListener("offline", handleOffline);
    };
  }, [clearRetryTimer, connect]);

  const connectionStale = useMemo(
    () => isStale(lastEnvelopeReceivedAt, nowMs, connectionStaleAfterMs(state.heartbeatIntervalMs)),
    [lastEnvelopeReceivedAt, nowMs, state.heartbeatIntervalMs]
  );
  const dataStale = useMemo(
    () => isStale(lastDataAt, nowMs, dataStaleAfterMs(state.sourceIntervalMs)),
    [lastDataAt, nowMs, state.sourceIntervalMs]
  );
  const serverLagMs = useMemo(() => {
    if (lastDataAt === null) {
      return null;
    }
    return Math.max(0, nowMs - lastDataAt);
  }, [lastDataAt, nowMs]);

  useEffect(() => {
    setConnection((previous) => {
      if (previous === "offline" || previous === "connecting" || previous === "reconnecting") {
        return previous;
      }
      if (connectionStale) {
        return "stale";
      }
      if (previous === "stale") {
        return "live";
      }
      return previous;
    });
  }, [connectionStale]);

  const refreshSnapshot = useCallback(async () => {
    await loadSnapshot(true);
  }, [loadSnapshot]);

  const positions = useMemo(() => Object.values(state.positionsByKey), [state.positionsByKey]);
  const observedDelays = useMemo(() => Object.values(state.observedByKey), [state.observedByKey]);
  const predictedDelays = useMemo(() => Object.values(state.predictedByKey), [state.predictedByKey]);

  return {
    connection,
    stale: dataStale,
    dataStale,
    connectionStale,
    loadingSnapshot,
    error,
    generatedAt: state.generatedAt,
    lastMessageAt: lastEnvelopeReceivedAt,
    lastDataAt,
    serverLagMs,
    reconnectAttempt,
    positions,
    observedDelays,
    predictedDelays,
    refreshSnapshot
  };
}
