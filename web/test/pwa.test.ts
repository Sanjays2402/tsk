import { test } from "node:test";
import assert from "node:assert/strict";
import { classifyRequest, type RequestInfoLike } from "../src/pwa.ts";

const ORIGIN = "http://127.0.0.1:7878";

function req(partial: Partial<RequestInfoLike> & { url: string }): RequestInfoLike {
  return { method: "GET", ...partial };
}

test("non-GET requests always passthrough", () => {
  for (const method of ["POST", "PATCH", "DELETE", "PUT"]) {
    assert.equal(
      classifyRequest(req({ method, url: `${ORIGIN}/api/tasks` }), ORIGIN),
      "passthrough",
    );
  }
});

test("/api/* GETs passthrough (live data + SSE)", () => {
  assert.equal(classifyRequest(req({ url: `${ORIGIN}/api/tasks` }), ORIGIN), "passthrough");
  assert.equal(classifyRequest(req({ url: `${ORIGIN}/api/events` }), ORIGIN), "passthrough");
  assert.equal(classifyRequest(req({ url: `${ORIGIN}/api/stats` }), ORIGIN), "passthrough");
});

test("cross-origin GETs passthrough", () => {
  assert.equal(
    classifyRequest(req({ url: "https://cdn.example.com/x.js" }), ORIGIN),
    "passthrough",
  );
});

test("navigations are network-first", () => {
  assert.equal(
    classifyRequest(req({ url: `${ORIGIN}/`, mode: "navigate" }), ORIGIN),
    "network-first",
  );
  assert.equal(
    classifyRequest(req({ url: `${ORIGIN}/index.html`, mode: "navigate" }), ORIGIN),
    "network-first",
  );
});

test("hashed assets, icons, manifest are cache-first", () => {
  assert.equal(
    classifyRequest(req({ url: `${ORIGIN}/assets/app-abc123.js` }), ORIGIN),
    "cache-first",
  );
  assert.equal(
    classifyRequest(req({ url: `${ORIGIN}/assets/style-def456.css` }), ORIGIN),
    "cache-first",
  );
  assert.equal(classifyRequest(req({ url: `${ORIGIN}/icon.svg` }), ORIGIN), "cache-first");
  assert.equal(
    classifyRequest(req({ url: `${ORIGIN}/manifest.webmanifest` }), ORIGIN),
    "cache-first",
  );
});

test("a non-navigate GET to / (e.g. shell prefetch) is cache-first", () => {
  assert.equal(classifyRequest(req({ url: `${ORIGIN}/` }), ORIGIN), "cache-first");
});

test("malformed URLs passthrough rather than throw", () => {
  assert.equal(classifyRequest(req({ url: "::::not a url" }), ORIGIN), "passthrough");
});
