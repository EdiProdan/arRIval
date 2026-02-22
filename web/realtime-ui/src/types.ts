export interface RealtimePosition {
  key: string;
  voznja_bus_id?: number;
  gbr?: number;
  lon: number;
  lat: number;
  observed_at: string;
}

export interface ObservedDelay {
  trip_id: string;
  voznja_bus_id: number;
  gbr?: number;
  lin_var_id: string;
  broj_linije: string;
  station_id: number;
  station_name: string;
  station_seq: number;
  scheduled_time: string;
  observed_time: string;
  delay_seconds: number;
  distance_m: number;
  tracker_version: string;
}

export interface PredictedDelay {
  trip_id: string;
  voznja_bus_id: number;
  lin_var_id: string;
  broj_linije: string;
  station_id: number;
  station_name: string;
  station_seq: number;
  scheduled_time: string;
  predicted_time: string;
  predicted_delay_seconds: number;
  generated_at: string;
  tracker_version: string;
}

export interface RealtimeSnapshotMeta {
  positions_count: number;
  observed_delays_count: number;
  predicted_delays_count: number;
}

export interface RealtimeSnapshot {
  generated_at: string;
  positions: RealtimePosition[];
  observed_delays: ObservedDelay[];
  predicted_delays: PredictedDelay[];
  meta: RealtimeSnapshotMeta;
}

export interface RealtimePositionsBatch {
  positions: RealtimePosition[];
}

export interface RealtimeObservedDelayUpdate {
  observed_delay: ObservedDelay;
}

export interface RealtimePredictedDelayUpdate {
  predicted_delay: PredictedDelay;
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
