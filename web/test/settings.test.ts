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
  });
});

test("normalizeSettings: showIds defaults true, only explicit false hides", () => {
  assert.equal(normalizeSettings({}).showIds, true);
  assert.equal(normalizeSettings({ showIds: false }).showIds, false);
  assert.equal(normalizeSettings({ showIds: "yes" }).showIds, true);
});

test("parseSettings round-trips a serialized object", () => {
  const s: Settings = { density: "compact", motion: "reduced", hideDone: true, showIds: false };
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
  });
  assert.deepEqual(
    settingsAttributes({ density: "compact", motion: "reduced", hideDone: false, showIds: false }),
    {
      "data-density": "compact",
      "data-motion": "reduced",
      "data-show-ids": "off",
    },
  );
});

test("renderSettings shows density + motion segmented controls and toggles", () => {
  const html = renderSettings(defaultSettings());
  assert.match(html, /data-set="density"/);
  assert.match(html, /data-set="motion"/);
  assert.match(html, /data-toggle-setting="hideDone"/);
  assert.match(html, /data-toggle-setting="showIds"/);
  assert.match(html, /data-settings-close/);
});

test("renderSettings reflects active states", () => {
  const html = renderSettings({
    density: "compact",
    motion: "reduced",
    hideDone: true,
    showIds: false,
  });
  // The compact density seg button is active.
  assert.match(html, /data-value="compact" aria-pressed="true"/);
  // The hideDone switch is on.
  assert.match(html, /data-toggle-setting="hideDone" role="switch" aria-checked="true"/);
  // The showIds switch is off.
  assert.match(html, /data-toggle-setting="showIds" role="switch" aria-checked="false"/);
});
