import { test } from "node:test";
import assert from "node:assert/strict";
import { renderToast, deletedMessage } from "../src/toast.ts";

test("message-only toast has no action button or bar", () => {
  const html = renderToast({ message: "Saved" });
  assert.match(html, /Saved/);
  assert.doesNotMatch(html, /toast-action/);
  assert.doesNotMatch(html, /toast-bar/);
});

test("action toast carries the labelled button hook", () => {
  const html = renderToast({ message: "Deleted", actionLabel: "Undo" });
  assert.match(html, /data-toast-action/);
  assert.match(html, />Undo</);
});

test("seconds drives a progress bar with the right duration", () => {
  const html = renderToast({ message: "x", seconds: 5 });
  assert.match(html, /toast-bar/);
  assert.match(html, /animation-duration:5s/);
});

test("zero/absent seconds omits the bar", () => {
  assert.doesNotMatch(renderToast({ message: "x", seconds: 0 }), /toast-bar/);
  assert.doesNotMatch(renderToast({ message: "x" }), /toast-bar/);
});

test("escapes HTML in the message and action", () => {
  const html = renderToast({ message: '<img src=x>', actionLabel: "<b>" });
  assert.doesNotMatch(html, /<img/);
  assert.doesNotMatch(html, /<b>/);
  assert.match(html, /&lt;/);
});

test("deletedMessage quotes the title", () => {
  assert.equal(deletedMessage("buy milk"), 'Deleted "buy milk"');
});

test("deletedMessage truncates long titles with an ellipsis", () => {
  const long = "a".repeat(60);
  const msg = deletedMessage(long);
  assert.ok(msg.length < 60, "should be shortened");
  assert.match(msg, /\u2026"$/);
});
