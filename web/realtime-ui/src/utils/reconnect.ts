const MAX_BACKOFF_MS = 20_000;

export function nextReconnectDelayMs(attempt: number, random: () => number = Math.random): number {
  const baseDelay = Math.min(1000 * 2 ** Math.max(0, attempt), MAX_BACKOFF_MS);
  const jitter = 0.8 + random() * 0.4;
  return Math.floor(baseDelay * jitter);
}

export function isStale(lastMessageAt: number | null, nowMs: number, thresholdMs: number): boolean {
  if (lastMessageAt === null) {
    return false;
  }
  return nowMs-lastMessageAt > thresholdMs;
}
