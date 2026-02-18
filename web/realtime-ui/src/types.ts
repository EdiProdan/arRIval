export interface RealtimePosition {
  key: string;
  voznja_bus_id?: number;
  gbr?: number;
  lon: number;
  lat: number;
  observed_at: string;
}

export interface DelayEvent {
  polazak_id: string;
  voznja_bus_id: number;
  gbr?: number;
  station_id: number;
  station_name: string;
  distance_m: number;
  lin_var_id: string;
  broj_linije: string;
  scheduled_time: string;
  actual_time: string;
  delay_seconds: number;
}

export interface RealtimeSnapshotMeta {
  positions_count: number;
  delays_count: number;
}

export interface RealtimeSnapshot {
  generated_at: string;
  positions: RealtimePosition[];
  delays: DelayEvent[];
  meta: RealtimeSnapshotMeta;
}

export interface RealtimePositionsBatch {
  positions: RealtimePosition[];
}

export interface RealtimeDelayUpdate {
  delay: DelayEvent;
}

export interface RealtimeEnvelope {
  type: string;
  ts: string;
  data: unknown;
}

export type ConnectionState =
  | "connecting"
  | "live"
  | "reconnecting"
  | "stale"
  | "offline";

export interface TransportHandlers {
  onOpen: () => void;
  onClose: () => void;
  onMessage: (raw: string) => void;
}

export interface TransportConnection {
  close: () => void;
}

export interface RealtimeTransport {
  fetchSnapshot: () => Promise<RealtimeSnapshot>;
  connect: (handlers: TransportHandlers) => TransportConnection;
}

declare global {
  interface Window {
    __ARRIVAL_TEST_TRANSPORT__?: RealtimeTransport;
  }
}
