import { test } from "node:test";
import assert from "node:assert/strict";
import {
  buildConfig,
  serializeConfig,
  parseConfig,
  configFilename,
  resetBundle,
  CONFIG_KIND,
  CONFIG_VERSION,
} from "../src/config.ts";
import { defaultSettings, type Settings } from "../src/settings.ts";
import { addView, type ViewFilter } from "../src/views.ts";

const EMPTY: ViewFilter = { query: "", priorities: [], tags: [], hideDone: false };
const COMPACT: Settings = {
  density: "compact",
  motion: "reduced",
  hideDone: true,
  showIds: false,
  hideMeta: true,
};

test("buildConfig captures normalized settings + views + theme", () => {
  const views = addView([], "Work", { ...EMPTY, tags: ["work"] });
  const bundle = buildConfig(COMPACT, views, "dark");
  assert.equal(bundle.kind, CONFIG_KIND);
  assert.equal(bundle.version, CONFIG_VERSION);
  assert.deepEqual(bundle.settings, COMPACT);
  assert.equal(bundle.views.length, 1);
  assert.equal(bundle.theme, "dark");
});

test("buildConfig drops an invalid theme", () => {
  const bundle = buildConfig(defaultSettings(), [], "neon");
  assert.equal(bundle.theme, undefined);
});

test("serialize/parse round-trips a config bundle", () => {
  const views = addView([], "Urgent", { ...EMPTY, priorities: ["urgent"] });
  const bundle = buildConfig(COMPACT, views, "light");
  const result = parseConfig(serializeConfig(bundle));
  assert.ok(result.ok);
  if (result.ok) {
    assert.deepEqual(result.bundle.settings, COMPACT);
    assert.equal(result.bundle.views[0].name, "Urgent");
    assert.equal(result.bundle.theme, "light");
  }
});

test("parseConfig rejects non-JSON", () => {
  const r = parseConfig("not json{");
  assert.equal(r.ok, false);
  if (!r.ok) assert.match(r.error, /JSON/i);
});

test("parseConfig rejects a non-object / array", () => {
  assert.equal(parseConfig("[1,2,3]").ok, false);
  assert.equal(parseConfig("42").ok, false);
  assert.equal(parseConfig("null").ok, false);
});

test("parseConfig rejects a foreign object lacking the kind marker", () => {
  const r = parseConfig(JSON.stringify({ settings: {}, views: [] }));
  assert.equal(r.ok, false);
  if (!r.ok) assert.match(r.error, /tsk web config/i);
});

test("parseConfig normalizes junk settings + drops bad views", () => {
  const blob = JSON.stringify({
    kind: CONFIG_KIND,
    settings: { density: "huge" }, // junk -> defaults
    views: [{ name: "ok", filter: { tags: ["x"] } }, "garbage", { name: "" }],
  });
  const r = parseConfig(blob);
  assert.ok(r.ok);
  if (r.ok) {
    assert.deepEqual(r.bundle.settings, defaultSettings());
    assert.equal(r.bundle.views.length, 1);
    assert.equal(r.bundle.views[0].name, "ok");
  }
});

test("parseConfig assumes v1 when version is missing", () => {
  const r = parseConfig(JSON.stringify({ kind: CONFIG_KIND }));
  assert.ok(r.ok);
  if (r.ok) assert.equal(r.bundle.version, CONFIG_VERSION);
});

test("configFilename is a dated json name", () => {
  const name = configFilename(new Date(2026, 5, 24));
  assert.equal(name, "tsk-config-2026-06-24.json");
});

test("resetBundle is factory settings, no views, auto theme", () => {
  const b = resetBundle();
  assert.deepEqual(b.settings, defaultSettings());
  assert.deepEqual(b.views, []);
  assert.equal(b.theme, "auto");
});
