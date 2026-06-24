import { test } from "node:test";
import assert from "node:assert/strict";
import {
  ALL_ROUTE,
  parseHash,
  formatHash,
  tagHash,
  viewHash,
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
});
