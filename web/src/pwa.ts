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
