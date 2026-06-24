/*
 * tsk service worker (F22) — offline app-shell cache.
 *
 * Strategy, by request kind (mirrors classifyRequest() in src/pwa.ts, which is
 * the unit-tested source of truth for these rules — keep the two in sync):
 *
 *   - /api/*            passthrough  — live data + the SSE stream must always
 *                                      hit the network; never cached.
 *   - navigations       network-first with cached-shell fallback, so a fresh
 *                        load always gets the latest index (and the ?token=
 *                        cookie bootstrap still runs) but going offline still
 *                        opens the app shell.
 *   - same-origin GET   cache-first with background revalidate — the hashed
 *     assets (JS/CSS),  JS/CSS/SVG/manifest load instantly and update quietly.
 *     icons, manifest
 *   - everything else   passthrough.
 *
 * The cache name carries a version; activate() drops older versions so a new
 * build cleanly supersedes the last.
 */

const CACHE = "tsk-shell-v1";
const SHELL = ["./", "./index.html", "./manifest.webmanifest", "./icon.svg", "./icon-maskable.svg"];

self.addEventListener("install", (event) => {
  event.waitUntil(
    caches
      .open(CACHE)
      .then((cache) => cache.addAll(SHELL))
      // Don't fail the install if one optional shell URL 404s on first deploy.
      .catch(() => undefined)
      .then(() => self.skipWaiting()),
  );
});

self.addEventListener("activate", (event) => {
  event.waitUntil(
    caches
      .keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim()),
  );
});

self.addEventListener("fetch", (event) => {
  const req = event.request;
  if (req.method !== "GET") return; // mutations always go to the network

  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return; // cross-origin: passthrough
  if (url.pathname.startsWith("/api/")) return; // live data + SSE: passthrough

  // Navigations: network-first, fall back to the cached shell when offline.
  if (req.mode === "navigate") {
    event.respondWith(
      fetch(req)
        .then((res) => {
          // Tuck a fresh copy of the shell away for the next offline visit.
          const copy = res.clone();
          caches.open(CACHE).then((c) => c.put("./index.html", copy)).catch(() => undefined);
          return res;
        })
        .catch(() => caches.match("./index.html").then((m) => m || caches.match("./"))),
    );
    return;
  }

  // Static assets: cache-first, revalidating in the background.
  event.respondWith(
    caches.match(req).then((cached) => {
      const network = fetch(req)
        .then((res) => {
          if (res && res.status === 200 && res.type === "basic") {
            const copy = res.clone();
            caches.open(CACHE).then((c) => c.put(req, copy)).catch(() => undefined);
          }
          return res;
        })
        .catch(() => cached);
      return cached || network;
    }),
  );
});
