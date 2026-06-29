import { test } from "node:test";
import assert from "node:assert/strict";
import {
  ALL_ROUTE,
  parseHash,
  formatHash,
  tagHash,
  viewHash,
  sharedViewHash,
  routesEqual,
  type Route,
} from "../src/router.ts";

test("parseHash: empty / bare hash -> all", () => {
  assert.deepEqual(parseHash(""), ALL_ROUTE);
  assert.deepEqual(parseHash("#"), ALL_ROUTE);
  assert.deepEqual(parseHash("#/"), ALL_ROUTE);
});

test("parseHash: tag route", () => {
  assert.deepEqual(parseHash("#tag/dev"), { kind: "tag", tag: "dev" });
  assert.deepEqual(parseHash("tag/dev"), { kind: "tag", tag: "dev" });
  assert.deepEqual(parseHash("#/tag/dev"), { kind: "tag", tag: "dev" });
});

test("parseHash: lower-cases the tag to match the store", () => {
  assert.deepEqual(parseHash("#tag/Work"), { kind: "tag", tag: "work" });
});

test("parseHash: percent-encoded tag round-trips", () => {
  assert.deepEqual(parseHash("#tag/two%20words"), { kind: "tag", tag: "two words" });
});

test("parseHash: empty tag falls back to all", () => {
  assert.deepEqual(parseHash("#tag/"), ALL_ROUTE);
  assert.deepEqual(parseHash("#tag/%20"), ALL_ROUTE);
});

test("parseHash: malformed percent-encoding doesn't throw", () => {
  // %ZZ is invalid; safeDecode keeps the raw text rather than throwing.
  const r = parseHash("#tag/%ZZ");
  assert.equal(r.kind, "tag");
});

test("parseHash: unknown route -> all", () => {
  assert.deepEqual(parseHash("#settings"), ALL_ROUTE);
  assert.deepEqual(parseHash("#foo/bar"), ALL_ROUTE);
});

test("parseHash: view route (F32)", () => {
  assert.deepEqual(parseHash("#view/vabc123"), { kind: "view", id: "vabc123" });
  assert.deepEqual(parseHash("#/view/vabc123"), { kind: "view", id: "vabc123" });
  // empty id falls back to all
  assert.deepEqual(parseHash("#view/"), ALL_ROUTE);
});

test("parseHash: view ids are NOT lower-cased (opaque tokens)", () => {
  assert.deepEqual(parseHash("#view/vAbC"), { kind: "view", id: "vAbC" });
});

test("formatHash: round-trips with parseHash", () => {
  const routes: Route[] = [ALL_ROUTE, { kind: "tag", tag: "dev" }, { kind: "tag", tag: "two words" }, { kind: "view", id: "vabc123" }];
  for (const r of routes) {
    assert.deepEqual(parseHash(formatHash(r)), r);
  }
});

test("formatHash: all -> '#'", () => {
  assert.equal(formatHash(ALL_ROUTE), "#");
});

test("viewHash builds an encoded view hash (F32)", () => {
  assert.equal(viewHash("vabc123"), "#view/vabc123");
  assert.deepEqual(parseHash(viewHash("v x")), { kind: "view", id: "v x" });
});

test("tagHash builds an encoded tag hash, lower-cased", () => {
  assert.equal(tagHash("Dev"), "#tag/dev");
  assert.equal(tagHash("two words"), "#tag/two%20words");
});

test("routesEqual compares kind + tag", () => {
  assert.ok(routesEqual(ALL_ROUTE, { kind: "all" }));
  assert.ok(routesEqual({ kind: "tag", tag: "x" }, { kind: "tag", tag: "x" }));
  assert.ok(!routesEqual({ kind: "tag", tag: "x" }, { kind: "tag", tag: "y" }));
  assert.ok(!routesEqual(ALL_ROUTE, { kind: "tag", tag: "x" }));
  // F32: view routes compare by id
  assert.ok(routesEqual({ kind: "view", id: "v1" }, { kind: "view", id: "v1" }));
  assert.ok(!routesEqual({ kind: "view", id: "v1" }, { kind: "view", id: "v2" }));
  assert.ok(!routesEqual({ kind: "view", id: "v1" }, { kind: "tag", tag: "v1" }));
  // F203: shared routes compare by doc
  assert.ok(routesEqual({ kind: "shared", doc: "x" }, { kind: "shared", doc: "x" }));
  assert.ok(!routesEqual({ kind: "shared", doc: "x" }, { kind: "shared", doc: "y" }));
});

// F203: shared-view LINK route (#view=<base64url doc>) round-trips a portable
// views doc through the URL hash so a bookmark travels in a link.
test("sharedViewHash round-trips a doc through parseHash", () => {
  const doc = JSON.stringify({ tsk: "tsk.views", v: 1, views: [{ id: "v1", name: "Work" }] });
  const hash = sharedViewHash(doc);
  assert.ok(hash.startsWith("#view="));
  const r = parseHash(hash);
  assert.equal(r.kind, "shared");
  if (r.kind === "shared") assert.equal(r.doc, doc);
});

test("sharedViewHash uses base64url (no '/', '+', or '=' in the fragment)", () => {
  // a doc engineered to produce '+' and '/' in standard base64 (so the URL-safe
  // swap is actually exercised) — many bytes near 0xfb/0xff.
  const doc = JSON.stringify({ tsk: "tsk.views", v: 1, note: "\u00ff\u00fe\u00fd\u00fc?>?>" });
  const hash = sharedViewHash(doc);
  const frag = hash.slice("#view=".length);
  assert.doesNotMatch(frag, /[/+=]/);
  // still decodes back to the exact doc
  const r = parseHash(hash);
  assert.equal(r.kind, "shared");
  if (r.kind === "shared") assert.equal(r.doc, doc);
});

test("parseHash: tolerates a leading-slash shared link and unicode payload", () => {
  const doc = JSON.stringify({ tsk: "tsk.views", v: 1, views: [{ id: "v1", name: "caf\u00e9 \u2615" }] });
  const r = parseHash(`#/${sharedViewHash(doc).slice(1)}`);
  assert.equal(r.kind, "shared");
  if (r.kind === "shared") assert.equal(r.doc, doc);
});

test("parseHash: a mangled #view= blob falls back to all (never throws)", () => {
  // an empty payload decodes to "" → all-tasks
  assert.deepEqual(parseHash("#view="), ALL_ROUTE);
});

test("formatHash: shared route round-trips with parseHash", () => {
  const doc = JSON.stringify({ tsk: "tsk.views", v: 1, views: [] });
  const r: Route = { kind: "shared", doc };
  const back = parseHash(formatHash(r));
  assert.deepEqual(back, r);
});
