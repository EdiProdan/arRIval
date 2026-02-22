import type {
  RealtimeEnvelope,
  ObservedDelay,
  PredictedDelay,
  RealtimeObservedDelayUpdate,
  RealtimePosition,
  RealtimePredictedDelayUpdate,
  RealtimePositionsBatch,
  RealtimeSnapshot
} from "../types";

export interface RealtimeCollections {
  generatedAt: string;
  positionsByKey: Record<string, RealtimePosition>;
  observedByKey: Record<string, ObservedDelay>;
  predictedByKey: Record<string, PredictedDelay>;
}

export function delayKey(tripID: string, stationID: number): string {
  return `${tripID}:${stationID}`;
}

function observedDelayKey(delay: ObservedDelay): string {
  return delayKey(delay.trip_id, delay.station_id);
}

function predictedDelayKey(delay: PredictedDelay): string {
  return delayKey(delay.trip_id, delay.station_id);
}

function observedSeqByTrip(observedByKey: Record<string, ObservedDelay>): Record<string, number> {
  const seqByTrip: Record<string, number> = {};
  for (const delay of Object.values(observedByKey)) {
    const existing = seqByTrip[delay.trip_id];
    if (existing === undefined || delay.station_seq > existing) {
      seqByTrip[delay.trip_id] = delay.station_seq;
    }
  }
  return seqByTrip;
}

function isPredictedProgressed(predicted: PredictedDelay, observedByTrip: Record<string, number>): boolean {
  const seq = observedByTrip[predicted.trip_id];
  if (seq === undefined) {
    return false;
  }
  return predicted.station_seq <= seq;
}

export function fromSnapshot(snapshot: RealtimeSnapshot): RealtimeCollections {
  const positionsByKey: Record<string, RealtimePosition> = {};
  for (const position of snapshot.positions) {
    positionsByKey[position.key] = position;
  }

  const observedByKey: Record<string, ObservedDelay> = {};
  for (const observed of snapshot.observed_delays) {
    observedByKey[observedDelayKey(observed)] = observed;
  }

  const seqByTrip = observedSeqByTrip(observedByKey);
  const predictedByKey: Record<string, PredictedDelay> = {};
  for (const predicted of snapshot.predicted_delays) {
    if (isPredictedProgressed(predicted, seqByTrip)) {
      continue;
    }
    predictedByKey[predictedDelayKey(predicted)] = predicted;
  }

  return {
    generatedAt: snapshot.generated_at,
    positionsByKey,
    observedByKey,
    predictedByKey
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
  update: RealtimeObservedDelayUpdate,
  ts: string
): RealtimeCollections {
  const observed = update.observed_delay;
  const observedByKey = {
    ...previous.observedByKey,
    [observedDelayKey(observed)]: observed
  };

  const predictedByKey: Record<string, PredictedDelay> = {};
  for (const [key, predicted] of Object.entries(previous.predictedByKey)) {
    if (predicted.trip_id === observed.trip_id && predicted.station_seq <= observed.station_seq) {
      continue;
    }
    predictedByKey[key] = predicted;
  }

  return {
    ...previous,
    generatedAt: ts,
    observedByKey,
    predictedByKey
  };
}

export function applyPredictedDelayUpdate(
  previous: RealtimeCollections,
  update: RealtimePredictedDelayUpdate,
  ts: string
): RealtimeCollections {
  const predicted = update.predicted_delay;
  const seqByTrip = observedSeqByTrip(previous.observedByKey);
  const predictedByKey = { ...previous.predictedByKey };
  const key = predictedDelayKey(predicted);

  if (isPredictedProgressed(predicted, seqByTrip)) {
    delete predictedByKey[key];
    return {
      ...previous,
      generatedAt: ts,
      predictedByKey
    };
  }

  predictedByKey[key] = predicted;
  return {
    ...previous,
    generatedAt: ts,
    predictedByKey
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

  if (envelope.type === "delay_observed_update") {
    const payload = envelope.data as RealtimeObservedDelayUpdate;
    if (!payload || !payload.observed_delay) {
      return previous;
    }
    return applyDelayUpdate(previous, payload, envelope.ts);
  }

  if (envelope.type === "delay_prediction_update") {
    const payload = envelope.data as RealtimePredictedDelayUpdate;
    if (!payload || !payload.predicted_delay) {
      return previous;
    }
    return applyPredictedDelayUpdate(previous, payload, envelope.ts);
  }

  return previous;
}
