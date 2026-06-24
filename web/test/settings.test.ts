import { test } from "node:test";
import assert from "node:assert/strict";
import {
  defaultSettings,
  normalizeSettings,
  parseSettings,
  serializeSettings,
  settingsAttributes,
  renderSettings,
  type Settings,
} from "../src/settings.ts";

test("defaultSettings is comfortable/full/ids-on/show-done", () => {
  assert.deepEqual(defaultSettings(), {
    density: "comfortable",
    motion: "full",
    hideDone: false,
    showIds: true,
    hideMeta: false,
  });
});

test("normalizeSettings fills gaps and rejects junk values", () => {
  assert.deepEqual(normalizeSettings(null), defaultSettings());
  assert.deepEqual(normalizeSettings("nope"), defaultSettings());
  assert.deepEqual(normalizeSettings({ density: "huge" }), defaultSettings());
  assert.deepEqual(normalizeSettings({ density: "compact", motion: "reduced" }), {
    density: "compact",
    motion: "reduced",
    hideDone: false,
    showIds: true,
    hideMeta: false,
  });
});

test("normalizeSettings: showIds defaults true, only explicit false hides", () => {
  assert.equal(normalizeSettings({}).showIds, true);
  assert.equal(normalizeSettings({ showIds: false }).showIds, false);
  assert.equal(normalizeSettings({ showIds: "yes" }).showIds, true);
});

test("normalizeSettings: hideMeta defaults false, only explicit true enables", () => {
  assert.equal(normalizeSettings({}).hideMeta, false);
  assert.equal(normalizeSettings({ hideMeta: true }).hideMeta, true);
  assert.equal(normalizeSettings({ hideMeta: "x" }).hideMeta, false);
});

test("parseSettings round-trips a serialized object", () => {
  const s: Settings = { density: "compact", motion: "reduced", hideDone: true, showIds: false, hideMeta: true };
  assert.deepEqual(parseSettings(serializeSettings(s)), s);
});

test("parseSettings tolerates a null / malformed store", () => {
  assert.deepEqual(parseSettings(null), defaultSettings());
  assert.deepEqual(parseSettings("{bad json"), defaultSettings());
});

test("settingsAttributes maps non-default states to attrs, defaults to null", () => {
  assert.deepEqual(settingsAttributes(defaultSettings()), {
    "data-density": null,
    "data-motion": null,
    "data-show-ids": null,
    "data-hide-meta": null,
  });
  assert.deepEqual(
    settingsAttributes({ density: "compact", motion: "reduced", hideDone: false, showIds: false, hideMeta: true }),
    {
      "data-density": "compact",
      "data-motion": "reduced",
      "data-show-ids": "off",
      "data-hide-meta": "on",
    },
  );
});

test("renderSettings shows density + motion segmented controls and toggles", () => {
  const html = renderSettings(defaultSettings());
  assert.match(html, /data-set="density"/);
  assert.match(html, /data-set="motion"/);
  assert.match(html, /data-toggle-setting="hideDone"/);
  assert.match(html, /data-toggle-setting="showIds"/);
  assert.match(html, /data-toggle-setting="hideMeta"/);
  assert.match(html, /data-settings-close/);
});

test("renderSettings includes the F34 config actions", () => {
  const html = renderSettings(defaultSettings());
  assert.match(html, /data-config-export/);
  assert.match(html, /data-config-import/);
  assert.match(html, /data-config-reset/);
});

test("renderSettings reflects active states", () => {
  const html = renderSettings({
    density: "compact",
    motion: "reduced",
    hideDone: true,
    showIds: false,
    hideMeta: true,
  });
  // The compact density seg button is active.
  assert.match(html, /data-value="compact" aria-pressed="true"/);
  // The hideDone switch is on.
  assert.match(html, /data-toggle-setting="hideDone" role="switch" aria-checked="true"/);
  // The showIds switch is off.
  assert.match(html, /data-toggle-setting="showIds" role="switch" aria-checked="false"/);
  // The hideMeta switch is on.
  assert.match(html, /data-toggle-setting="hideMeta" role="switch" aria-checked="true"/);
});
