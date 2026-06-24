import { test } from "node:test";
import assert from "node:assert/strict";
import { renderComposerPreview } from "../src/composer.ts";
import { parseQuickAdd } from "../src/quickadd.ts";

test("empty / plain title produces no preview", () => {
  assert.equal(renderComposerPreview(parseQuickAdd("")), "");
  assert.equal(renderComposerPreview(parseQuickAdd("buy milk")), "");
});

test("priority pill carries the level class and glyph", () => {
  const html = renderComposerPreview(parseQuickAdd("x !urgent"));
  assert.match(html, /pill prio urgent/);
  assert.match(html, />U</);
});

test("due pill prefixes @", () => {
  const html = renderComposerPreview(parseQuickAdd("x @tomorrow"));
  assert.match(html, /pill due/);
  assert.match(html, /@tomorrow/);
});

test("tag pills prefix # and appear per tag", () => {
  const html = renderComposerPreview(parseQuickAdd("x #dev #ci"));
  assert.match(html, /#dev/);
  assert.match(html, /#ci/);
});

test("metadata-only line nudges for a title", () => {
  const html = renderComposerPreview(parseQuickAdd("!high #tag"));
  assert.match(html, /needs a title/);
});

test("escapes HTML in tag/due to prevent injection", () => {
  const html = renderComposerPreview(parseQuickAdd("x @<script> #<b>"));
  assert.doesNotMatch(html, /<script>/);
  assert.doesNotMatch(html, /<b>/);
  assert.match(html, /&lt;/);
});
