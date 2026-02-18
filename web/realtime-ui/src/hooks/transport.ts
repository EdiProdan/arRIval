import type {
  RealtimeEnvelope,
  RealtimeSnapshot,
  RealtimeTransport,
  TransportConnection,
  TransportHandlers
} from "../types";

function toWSURL(path: string): string {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocol}//${window.location.host}${path}`;
}

export function parseEnvelope(raw: string): RealtimeEnvelope | null {
  try {
    const value = JSON.parse(raw) as RealtimeEnvelope;
    if (!value || typeof value.type !== "string" || typeof value.ts !== "string") {
      return null;
    }
    return value;
  } catch {
    return null;
  }
}

export function browserTransport(): RealtimeTransport {
  return {
    async fetchSnapshot(): Promise<RealtimeSnapshot> {
      const response = await fetch("/v1/snapshot");
      if (!response.ok) {
        throw new Error(`snapshot_fetch_failed:${response.status}`);
      }
      return (await response.json()) as RealtimeSnapshot;
    },

    connect(handlers: TransportHandlers): TransportConnection {
      const ws = new WebSocket(toWSURL("/v1/ws"));
      ws.onopen = () => handlers.onOpen();
      ws.onclose = () => handlers.onClose();
      ws.onmessage = (event) => {
        if (typeof event.data === "string") {
          handlers.onMessage(event.data);
        }
      };

      return {
        close: () => {
          ws.close();
        }
      };
    }
  };
}

export function resolveTransport(): RealtimeTransport {
  if (window.__ARRIVAL_TEST_TRANSPORT__) {
    return window.__ARRIVAL_TEST_TRANSPORT__;
  }
  return browserTransport();
}
