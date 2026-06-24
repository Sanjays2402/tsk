/**
 * PWA helpers (F22) — the request-classification logic the service worker uses,
 * extracted as a pure function so the caching rules are unit-tested under
 * `node --test` without a ServiceWorker runtime. The actual SW (public/sw.js)
 * mirrors classifyRequest; this module is the documented source of truth for
 * the rules and also owns the (browser-only) registration call.
 */

export type CacheStrategy =
  | "passthrough" // never touch the cache (mutations, /api/*, cross-origin)
  | "network-first" // navigations: fresh when online, shell when offline
  | "cache-first"; // hashed assets, icons, manifest

export interface RequestInfoLike {
  /** HTTP method, e.g. "GET". */
  method: string;
  /** Absolute URL of the request. */
  url: string;
  /** Fetch request mode; "navigate" marks a top-level navigation. */
  mode?: string;
}

/**
 * Decide how the service worker should handle a request, given the origin it
 * is serving. Rules (in order):
 *
 *   1. non-GET            -> passthrough (writes always hit the network)
 *   2. cross-origin       -> passthrough
 *   3. /api/* path        -> passthrough (live data + SSE stream)
 *   4. navigation         -> network-first (fresh index, offline shell fallback)
 *   5. anything else      -> cache-first (the hashed JS/CSS, icons, manifest)
 */
export function classifyRequest(req: RequestInfoLike, origin: string): CacheStrategy {
  if (req.method.toUpperCase() !== "GET") return "passthrough";
  let url: URL;
  try {
    url = new URL(req.url);
  } catch {
    return "passthrough";
  }
  if (url.origin !== origin) return "passthrough";
  if (url.pathname.startsWith("/api/")) return "passthrough";
  if (req.mode === "navigate") return "network-first";
  return "cache-first";
}

/**
 * Register the service worker once the page has loaded. No-ops when the SW API
 * is unavailable (older browsers, insecure non-localhost origins) so callers
 * don't need to guard. Returns the registration, or null if unsupported/failed
 * — failure is non-fatal: the app works fine without offline support.
 */
export async function registerServiceWorker(
  swUrl = "./sw.js",
): Promise<ServiceWorkerRegistration | null> {
  if (typeof navigator === "undefined" || !("serviceWorker" in navigator)) {
    return null;
  }
  try {
    return await navigator.serviceWorker.register(swUrl, { scope: "./" });
  } catch {
    return null;
  }
}

// --- F35: install prompt + offline state ------------------------------------

/**
 * Connectivity state derived from two independent signals (F35):
 *   - the SSE live stream status (live / connecting / offline / paused)
 *   - the browser's navigator.onLine flag
 *
 * We distinguish three situations so the banner copy is honest:
 *   - "online":    everything's fine (stream live or just paused).
 *   - "server":    the network is up but the stream is down — the `tsk serve`
 *                  process is probably restarting or stopped. The page still
 *                  works against its last load + the cache.
 *   - "offline":   the device itself is offline (navigator.onLine === false).
 */
export type Connectivity = "online" | "server" | "offline";

/**
 * Classify connectivity from the live-stream status + the online flag. A
 * stream that is "live" or "paused" is fine. When the stream is down, a false
 * onLine flag means the device is offline; otherwise it's the server that's
 * unreachable.
 */
export function classifyConnectivity(streamOffline: boolean, online: boolean): Connectivity {
  if (!streamOffline) return "online";
  return online ? "server" : "offline";
}

/** Whether a banner should show for a given connectivity (only the bad states). */
export function shouldShowOfflineBanner(c: Connectivity): boolean {
  return c === "server" || c === "offline";
}

/** Human banner copy for a connectivity state. "" for the healthy state. */
export function connectivityMessage(c: Connectivity): string {
  switch (c) {
    case "server":
      return "Can't reach tsk serve — it may be restarting. Showing the last loaded tasks.";
    case "offline":
      return "You're offline. Showing cached tasks; changes will fail until you reconnect.";
    case "online":
      return "";
  }
}

/**
 * Render the offline/server banner. Returns "" for the healthy state so the
 * caller can hide the element. Carries a data-connectivity hook for styling.
 */
export function renderOfflineBanner(c: Connectivity): string {
  if (!shouldShowOfflineBanner(c)) return "";
  const icon = c === "offline" ? "&#9888;" : "&#8635;"; // warning / restart
  return `<span class="offline-ico" aria-hidden="true">${icon}</span><span class="offline-msg">${connectivityMessage(c)}</span>`;
}

/**
 * The minimal shape of the beforeinstallprompt event we depend on (it isn't in
 * the standard lib types). Captured so the settings "Install app" button can
 * trigger it on demand.
 */
export interface InstallPromptEvent extends Event {
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: "accepted" | "dismissed" }>;
}

/**
 * Decide whether to show the "Install app" affordance: only when we captured a
 * deferred prompt AND the app isn't already running as an installed PWA.
 */
export function canInstall(prompt: InstallPromptEvent | null, standalone: boolean): boolean {
  return prompt !== null && !standalone;
}

/**
 * True when the page is running as an installed/standalone PWA. Checks both the
 * display-mode media query and the iOS-only navigator.standalone flag. Guarded
 * so it's safe to call without a browser (returns false).
 */
export function isStandalone(): boolean {
  if (typeof window === "undefined") return false;
  const mm = window.matchMedia?.("(display-mode: standalone)").matches ?? false;
  const ios = (navigator as unknown as { standalone?: boolean }).standalone === true;
  return mm || ios;
}
