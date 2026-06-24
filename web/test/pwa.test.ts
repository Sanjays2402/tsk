import { test } from "node:test";
import assert from "node:assert/strict";
import {
  classifyRequest,
  classifyConnectivity,
  shouldShowOfflineBanner,
  connectivityMessage,
  renderOfflineBanner,
  canInstall,
  type RequestInfoLike,
  type InstallPromptEvent,
} from "../src/pwa.ts";

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

// --- F35: connectivity classification ---------------------------------------

test("classifyConnectivity: a live stream is always online", () => {
  assert.equal(classifyConnectivity(false, true), "online");
  assert.equal(classifyConnectivity(false, false), "online"); // stream up beats onLine
});

test("classifyConnectivity: stream down + online flag distinguishes server vs offline", () => {
  assert.equal(classifyConnectivity(true, true), "server"); // network up, serve down
  assert.equal(classifyConnectivity(true, false), "offline"); // device offline
});

test("shouldShowOfflineBanner only for the bad states", () => {
  assert.equal(shouldShowOfflineBanner("online"), false);
  assert.equal(shouldShowOfflineBanner("server"), true);
  assert.equal(shouldShowOfflineBanner("offline"), true);
});

test("connectivityMessage: distinct honest copy per bad state, empty when healthy", () => {
  assert.equal(connectivityMessage("online"), "");
  assert.match(connectivityMessage("server"), /tsk serve|restart/i);
  assert.match(connectivityMessage("offline"), /offline/i);
  assert.notEqual(connectivityMessage("server"), connectivityMessage("offline"));
});

test("renderOfflineBanner: empty when online, carries the message otherwise", () => {
  assert.equal(renderOfflineBanner("online"), "");
  const server = renderOfflineBanner("server");
  assert.match(server, /offline-msg/);
  assert.match(server, /restart/i);
  const offline = renderOfflineBanner("offline");
  assert.match(offline, /offline-ico/);
});

// --- F35: install affordance ------------------------------------------------

test("canInstall requires a captured prompt and a non-standalone window", () => {
  const fakePrompt = {} as InstallPromptEvent;
  assert.equal(canInstall(fakePrompt, false), true); // have prompt, browser tab
  assert.equal(canInstall(fakePrompt, true), false); // already installed
  assert.equal(canInstall(null, false), false); // no prompt captured yet
  assert.equal(canInstall(null, true), false);
});
