import { test } from "node:test";
import assert from "node:assert/strict";
import {
  THEME_CYCLE,
  normalizeMode,
  nextMode,
  resolveTheme,
  themeAttr,
  modeLabel,
  modeGlyph,
  modeTitle,
} from "../src/theme.ts";

test("THEME_CYCLE is auto -> light -> dark", () => {
  assert.deepEqual(THEME_CYCLE, ["auto", "light", "dark"]);
});

test("normalizeMode accepts valid, defaults junk to auto", () => {
  assert.equal(normalizeMode("light"), "light");
  assert.equal(normalizeMode("dark"), "dark");
  assert.equal(normalizeMode("auto"), "auto");
  assert.equal(normalizeMode(null), "auto");
  assert.equal(normalizeMode(undefined), "auto");
  assert.equal(normalizeMode("sepia"), "auto");
  assert.equal(normalizeMode(""), "auto");
});

test("nextMode cycles and wraps", () => {
  assert.equal(nextMode("auto"), "light");
  assert.equal(nextMode("light"), "dark");
  assert.equal(nextMode("dark"), "auto");
});

test("resolveTheme: explicit modes win, auto follows system", () => {
  assert.equal(resolveTheme("light", true), "light");
  assert.equal(resolveTheme("light", false), "light");
  assert.equal(resolveTheme("dark", false), "dark");
  assert.equal(resolveTheme("dark", true), "dark");
  assert.equal(resolveTheme("auto", true), "dark");
  assert.equal(resolveTheme("auto", false), "light");
});

test("themeAttr: auto -> null, explicit -> own name", () => {
  assert.equal(themeAttr("auto"), null);
  assert.equal(themeAttr("light"), "light");
  assert.equal(themeAttr("dark"), "dark");
});

test("modeLabel echoes the mode", () => {
  assert.equal(modeLabel("auto"), "auto");
  assert.equal(modeLabel("light"), "light");
  assert.equal(modeLabel("dark"), "dark");
});

test("modeGlyph gives a distinct glyph per mode", () => {
  const glyphs = new Set([modeGlyph("auto"), modeGlyph("light"), modeGlyph("dark")]);
  assert.equal(glyphs.size, 3);
});

test("modeTitle announces the next action", () => {
  assert.match(modeTitle("auto"), /light/);
  assert.match(modeTitle("light"), /dark/);
  assert.match(modeTitle("dark"), /auto/);
});
