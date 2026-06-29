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
 *   #view=<b64>    -> { kind: "shared", doc } a shared saved-view link (F203)
 *
 * Tag names are percent-encoded in the hash so tags with spaces or reserved
 * characters survive a round-trip. Names are lower-cased to match the store's
 * tag normalization (the CLI/store lower-case tags). View ids are opaque
 * tokens (makeId output) and are NOT lower-cased.
 *
 * F203: a "shared" route carries a base64url-encoded portable views document
 * (exportViewsDoc / exportSingleViewDoc output) right in the URL, so a bookmark
 * travels in a link — open it and main.ts imports the view(s). Distinct from
 * the `#view/<id>` recall (that references a view you ALREADY have by id); a
 * shared link carries the view's whole definition so a fresh browser can adopt
 * it. The `=` separator (vs `/`) keeps the two unambiguous.
 */

export type Route =
  | { kind: "all" }
  | { kind: "tag"; tag: string }
  | { kind: "view"; id: string }
  | { kind: "shared"; doc: string };

const TAG_PREFIX = "tag/";
const VIEW_PREFIX = "view/";
const SHARED_PREFIX = "view=";

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
  // F203: a shared-view link carries the whole view doc base64url-encoded after
  // "view=". Decode it back to the portable JSON; an empty / undecodable blob
  // falls back to all-tasks so a mangled link never wedges the app.
  if (h.startsWith(SHARED_PREFIX)) {
    const raw = h.slice(SHARED_PREFIX.length).trim();
    const doc = b64urlDecode(raw);
    if (doc === "") return ALL_ROUTE;
    return { kind: "shared", doc };
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
  if (route.kind === "shared") {
    return `#${SHARED_PREFIX}${b64urlEncode(route.doc)}`;
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

/**
 * F203: the hash for a shared-view LINK — base64url-encodes a portable views
 * document (exportViewsDoc / exportSingleViewDoc output) into a `#view=<b64>`
 * fragment, so a bookmark travels in a copyable URL. main.ts prepends the page
 * origin to make the full shareable link. Round-trips through parseHash:
 * parseHash(sharedViewHash(doc)) yields { kind:"shared", doc }.
 */
export function sharedViewHash(doc: string): string {
  return formatHash({ kind: "shared", doc });
}

/** Two routes are equal when same kind (+ tag / id / doc). */
export function routesEqual(a: Route, b: Route): boolean {
  if (a.kind !== b.kind) return false;
  if (a.kind === "tag" && b.kind === "tag") return a.tag === b.tag;
  if (a.kind === "view" && b.kind === "view") return a.id === b.id;
  if (a.kind === "shared" && b.kind === "shared") return a.doc === b.doc;
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

/**
 * F203: encode a UTF-8 string to base64url (the URL-safe alphabet, '-' / '_',
 * no '=' padding) so a views doc rides in a hash fragment without needing extra
 * percent-encoding. TextEncoder gives the UTF-8 bytes; btoa over a binary
 * string emits base64 (views docs are tiny, so the fromCharCode spread is
 * safe). Falls back to Buffer in a non-DOM (node test) env. A failure returns
 * "" so a bad encode degrades to a no-op link rather than throwing.
 */
function b64urlEncode(s: string): string {
  try {
    let b64: string;
    if (typeof btoa === "function") {
      const bytes = new TextEncoder().encode(s);
      let bin = "";
      for (const b of bytes) bin += String.fromCharCode(b);
      b64 = btoa(bin);
    } else {
      b64 = Buffer.from(s, "utf-8").toString("base64");
    }
    return b64.replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
  } catch {
    return "";
  }
}

/**
 * F203: decode a base64url string (b64urlEncode's output) back to its UTF-8
 * string. Restores the standard alphabet + padding before atob, reads the bytes
 * back, and TextDecoder reassembles the UTF-8. A malformed / non-base64 input
 * returns "" so a hand-poked or truncated link degrades to the all-tasks route
 * rather than throwing.
 */
function b64urlDecode(s: string): string {
  try {
    let b64 = s.replace(/-/g, "+").replace(/_/g, "/");
    while (b64.length % 4 !== 0) b64 += "=";
    if (typeof atob === "function") {
      const bin = atob(b64);
      const bytes = new Uint8Array(bin.length);
      for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
      return new TextDecoder().decode(bytes);
    }
    return Buffer.from(b64, "base64").toString("utf-8");
  } catch {
    return "";
  }
}
