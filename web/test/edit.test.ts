import { test } from "node:test";
import assert from "node:assert/strict";
import { resolveEdit } from "../src/edit.ts";

test("escape cancels regardless of draft", () => {
  assert.deepEqual(resolveEdit("a", "totally different", true), { kind: "cancel" });
});

test("unchanged draft is a noop", () => {
  assert.deepEqual(resolveEdit("buy milk", "buy milk", false), { kind: "noop" });
});

test("whitespace-only difference is a noop", () => {
  assert.deepEqual(resolveEdit("buy milk", "  buy milk  ", false), { kind: "noop" });
});

test("empty-after-trim is a noop (never clears a title)", () => {
  assert.deepEqual(resolveEdit("buy milk", "   ", false), { kind: "noop" });
  assert.deepEqual(resolveEdit("buy milk", "", false), { kind: "noop" });
});

test("a real change commits the trimmed title", () => {
  assert.deepEqual(resolveEdit("buy milk", "  buy oat milk  ", false), {
    kind: "commit",
    title: "buy oat milk",
  });
});
