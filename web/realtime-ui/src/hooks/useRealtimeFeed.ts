import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { resolveTransport, parseEnvelope } from "./transport";
import { applyEnvelope, fromSnapshot } from "../state/realtimeState";
import { isStale, nextReconnectDelayMs } from "../utils/reconnect";
import type { ConnectionState, DelayEvent, RealtimePosition } from "../types";

const STALE_AFTER_MS = Number(import.meta.env.VITE_STALE_AFTER_MS ?? "45000");
const MAX_RETRIES = 8;

interface FeedState {
  generatedAt: string;
  positionsByKey: Record<string, RealtimePosition>;
  delaysByKey: Record<string, DelayEvent>;
}

const initialState: FeedState = {
  generatedAt: "",
  positionsByKey: {},
  delaysByKey: {}
};

export interface RealtimeFeedModel {
  connection: ConnectionState;
  stale: boolean;
  loadingSnapshot: boolean;
  error: string;
  generatedAt: string;
  lastMessageAt: number | null;
  positions: RealtimePosition[];
  delays: DelayEvent[];
  refreshSnapshot: () => Promise<void>;
}

export function useRealtimeFeed(): RealtimeFeedModel {
  const transport = useMemo(() => resolveTransport(), []);
  const connectionRef = useRef<{ close: () => void } | null>(null);
  const retryTimerRef = useRef<number | null>(null);
  const retryCountRef = useRef(0);
  const closedByAppRef = useRef(false);

  const [state, setState] = useState<FeedState>(initialState);
  const [connection, setConnection] = useState<ConnectionState>("connecting");
  const [loadingSnapshot, setLoadingSnapshot] = useState(false);
  const [error, setError] = useState("");
  const [lastMessageAt, setLastMessageAt] = useState<number | null>(null);

  const clearRetryTimer = () => {
    if (retryTimerRef.current !== null) {
      window.clearTimeout(retryTimerRef.current);
      retryTimerRef.current = null;
    }
  };

  const loadSnapshot = useCallback(
    async (showLoading: boolean) => {
      if (showLoading) {
        setLoadingSnapshot(true);
      }

      try {
        const snapshot = await transport.fetchSnapshot();
        setState(fromSnapshot(snapshot));
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
          setConnection("live");

          if (attempt > 0) {
            void loadSnapshot(false);
          }
        },
        onClose: () => {
          if (closedByAppRef.current) {
            return;
          }

          if (retryCountRef.current >= MAX_RETRIES) {
            setConnection("offline");
            return;
          }

          const thisAttempt = retryCountRef.current;
          retryCountRef.current = thisAttempt + 1;
          const delay = nextReconnectDelayMs(thisAttempt);
          setConnection("reconnecting");

          clearRetryTimer();
          retryTimerRef.current = window.setTimeout(() => {
            connect(retryCountRef.current);
          }, delay);
        },
        onMessage: (raw) => {
          const envelope = parseEnvelope(raw);
          if (!envelope) {
            return;
          }

          const now = Date.now();
          setLastMessageAt(now);
          setConnection("live");

          if (envelope.type === "heartbeat") {
            return;
          }

          setState((previous) => applyEnvelope(previous, envelope));
        }
      });

      connectionRef.current = connectionHandle;
    },
    [loadSnapshot, transport]
  );

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
  }, [connect, loadSnapshot]);

  useEffect(() => {
    const ticker = window.setInterval(() => {
      setConnection((previous) => {
        if (previous === "offline" || previous === "connecting") {
          return previous;
        }

        if (isStale(lastMessageAt, Date.now(), STALE_AFTER_MS)) {
          return "stale";
        }

        return previous;
      });
    }, 1000);

    return () => {
      window.clearInterval(ticker);
    };
  }, [lastMessageAt]);

  const refreshSnapshot = useCallback(async () => {
    await loadSnapshot(true);
  }, [loadSnapshot]);

  const positions = useMemo(() => Object.values(state.positionsByKey), [state.positionsByKey]);
  const delays = useMemo(() => Object.values(state.delaysByKey), [state.delaysByKey]);

  return {
    connection,
    stale: connection === "stale",
    loadingSnapshot,
    error,
    generatedAt: state.generatedAt,
    lastMessageAt,
    positions,
    delays,
    refreshSnapshot
  };
}
