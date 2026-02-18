import type {
  DelayEvent,
  RealtimeDelayUpdate,
  RealtimeEnvelope,
  RealtimePosition,
  RealtimePositionsBatch,
  RealtimeSnapshot
} from "../types";

export interface RealtimeCollections {
  generatedAt: string;
  positionsByKey: Record<string, RealtimePosition>;
  delaysByKey: Record<string, DelayEvent>;
}

export function delayKey(delay: DelayEvent): string {
  return `${delay.voznja_bus_id}:${delay.station_id}`;
}

export function fromSnapshot(snapshot: RealtimeSnapshot): RealtimeCollections {
  const positionsByKey: Record<string, RealtimePosition> = {};
  for (const position of snapshot.positions) {
    positionsByKey[position.key] = position;
  }

  const delaysByKey: Record<string, DelayEvent> = {};
  for (const delay of snapshot.delays) {
    delaysByKey[delayKey(delay)] = delay;
  }

  return {
    generatedAt: snapshot.generated_at,
    positionsByKey,
    delaysByKey
  };
}

export function applyPositionsBatch(
  previous: RealtimeCollections,
  batch: RealtimePositionsBatch,
  ts: string
): RealtimeCollections {
  const positionsByKey = { ...previous.positionsByKey };
  for (const position of batch.positions) {
    positionsByKey[position.key] = position;
  }

  return {
    ...previous,
    generatedAt: ts,
    positionsByKey
  };
}

export function applyDelayUpdate(
  previous: RealtimeCollections,
  update: RealtimeDelayUpdate,
  ts: string
): RealtimeCollections {
  const delaysByKey = {
    ...previous.delaysByKey,
    [delayKey(update.delay)]: update.delay
  };

  return {
    ...previous,
    generatedAt: ts,
    delaysByKey
  };
}

export function applyEnvelope(previous: RealtimeCollections, envelope: RealtimeEnvelope): RealtimeCollections {
  if (envelope.type === "positions_batch") {
    const payload = envelope.data as RealtimePositionsBatch;
    if (!payload || !Array.isArray(payload.positions)) {
      return previous;
    }
    return applyPositionsBatch(previous, payload, envelope.ts);
  }

  if (envelope.type === "delay_update") {
    const payload = envelope.data as RealtimeDelayUpdate;
    if (!payload || !payload.delay) {
      return previous;
    }
    return applyDelayUpdate(previous, payload, envelope.ts);
  }

  return previous;
}
