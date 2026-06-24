/**
 * Live-reload model (F21) — pure helpers for the SSE connection indicator and
 * the "should I refresh?" decision, kept browser-free so they unit-test under
 * `node --test`. The EventSource wiring + actual refresh live in main.ts.
 *
 * The server (GET /api/events) streams:
 *   event: ready   data: {"mtime":<ns>,"size":<bytes>}   once on connect
 *   event: change  data: {"mtime":<ns>,"size":<bytes>}   when .tsk.md moves
 *   : keep-alive                                          idle heartbeat
 *
 * A "change" means the file was edited by the CLI, TUI, another tab, or a hand
 * edit, so the client should re-fetch. We debounce bursts (an editor can write
 * several times in a second) by comparing fingerprints, not raw event counts.
 */

export type LiveStatus = "connecting" | "live" | "offline";

export interface FileFingerprint {
  mtime: number;
  size: number;
}

/** Parse an SSE data payload into a fingerprint, or null if it's malformed. */
export function parseFingerprint(data: string): FileFingerprint | null {
  try {
    const obj = JSON.parse(data) as Partial<FileFingerprint>;
    if (typeof obj.mtime !== "number" || typeof obj.size !== "number") return null;
    return { mtime: obj.mtime, size: obj.size };
  } catch {
    return null;
  }
}

/**
 * Decide whether a freshly received fingerprint should trigger a refresh,
 * given the last one we acted on. First sighting (prev === null) never
 * refreshes — it just establishes the baseline. After that, any field moving
 * is a real external edit worth pulling.
 */
export function shouldRefresh(
  prev: FileFingerprint | null,
  next: FileFingerprint,
): boolean {
  if (prev === null) return false;
  return prev.mtime !== next.mtime || prev.size !== next.size;
}

/** Short human label for the indicator dot, by connection status. */
export function liveLabel(status: LiveStatus): string {
  switch (status) {
    case "live":
      return "live";
    case "connecting":
      return "connecting";
    case "offline":
      return "offline";
  }
}

/** Tooltip text explaining what the indicator means + the underlying mechanic. */
export function liveTitle(status: LiveStatus): string {
  switch (status) {
    case "live":
      return "Live: auto-refreshing when .tsk.md changes on disk";
    case "connecting":
      return "Connecting to the live-update stream…";
    case "offline":
      return "Live updates offline — reconnecting. Press r to refresh manually.";
  }
}

/**
 * Render the live indicator pill. Carries `data-live` for state-driven CSS and
 * a status class so the dot color tracks the connection. Pure → unit-tested.
 */
export function renderLiveIndicator(status: LiveStatus): string {
  return `<span class="live-dot live-${status}" aria-hidden="true"></span><span class="live-label">${liveLabel(status)}</span>`;
}
