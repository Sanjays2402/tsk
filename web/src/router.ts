/**
 * Hash routing (F15) — pure parse/format helpers for the tag-page URLs. The
 * DOM side (listening to hashchange, pushing new hashes, reflecting the route
 * into the F11 filter) lives in main.ts; this module owns the grammar so the
 * encode/decode round-trip is unit-tested without a browser.
 *
 * Routes:
 *   #              -> { kind: "all" }        the full list
 *   #tag/<name>    -> { kind: "tag", tag }   a single-tag filtered page
 *   #view/<id>     -> { kind: "view", id }   a recalled saved view (F32)
 *
 * Tag names are percent-encoded in the hash so tags with spaces or reserved
 * characters survive a round-trip. Names are lower-cased to match the store's
 * tag normalization (the CLI/store lower-case tags). View ids are opaque
 * tokens (makeId output) and are NOT lower-cased.
 */

export type Route =
  | { kind: "all" }
  | { kind: "tag"; tag: string }
  | { kind: "view"; id: string };

const TAG_PREFIX = "tag/";
const VIEW_PREFIX = "view/";

/** The all-tasks route singleton. */
export const ALL_ROUTE: Route = { kind: "all" };

/**
 * Parse a location.hash string (with or without the leading '#') into a Route.
 * Unknown / malformed hashes fall back to the all-tasks route so a hand-typed
 * URL never breaks the app.
 */
export function parseHash(hash: string): Route {
  let h = hash.startsWith("#") ? hash.slice(1) : hash;
  if (h.startsWith("/")) h = h.slice(1); // tolerate "#/tag/x"
  if (h === "" || h === "/") return ALL_ROUTE;
  if (h.startsWith(TAG_PREFIX)) {
    const raw = h.slice(TAG_PREFIX.length);
    const tag = safeDecode(raw).trim().toLowerCase();
    if (tag === "") return ALL_ROUTE;
    return { kind: "tag", tag };
  }
  if (h.startsWith(VIEW_PREFIX)) {
    const id = safeDecode(h.slice(VIEW_PREFIX.length)).trim();
    if (id === "") return ALL_ROUTE;
    return { kind: "view", id };
  }
  return ALL_ROUTE;
}

/** Format a Route back into a hash string (including the leading '#'). */
export function formatHash(route: Route): string {
  if (route.kind === "tag") {
    return `#${TAG_PREFIX}${encodeURIComponent(route.tag)}`;
  }
  if (route.kind === "view") {
    return `#${VIEW_PREFIX}${encodeURIComponent(route.id)}`;
  }
  return "#";
}

/** Convenience: the hash for a given tag name. */
export function tagHash(tag: string): string {
  return formatHash({ kind: "tag", tag: tag.toLowerCase() });
}

/** Convenience: the hash for a given saved-view id (F32). */
export function viewHash(id: string): string {
  return formatHash({ kind: "view", id });
}

/** Two routes are equal when same kind (+ tag / id). */
export function routesEqual(a: Route, b: Route): boolean {
  if (a.kind !== b.kind) return false;
  if (a.kind === "tag" && b.kind === "tag") return a.tag === b.tag;
  if (a.kind === "view" && b.kind === "view") return a.id === b.id;
  return true;
}

/** decodeURIComponent that never throws on malformed input. */
function safeDecode(s: string): string {
  try {
    return decodeURIComponent(s);
  } catch {
    return s;
  }
}
