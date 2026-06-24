import { test } from "node:test";
import assert from "node:assert/strict";
import {
  DUE_PRESETS,
  previewVM,
  resolveDueCommit,
  renderPresets,
  renderDuePreview,
  type ParseDateResult,
} from "../src/duepicker.ts";

const okResult = (over: Partial<ParseDateResult> = {}): ParseDateResult => ({
  ok: true,
  input: "jul 4",
  date: "2026-07-04",
  weekday: "Sat",
  pretty: "Sat, Jul 4 2026",
  relative: "in 10d",
  ...over,
});

test("presets are non-empty and carry natural-language values", () => {
  assert.ok(DUE_PRESETS.length >= 4);
  for (const p of DUE_PRESETS) {
    assert.ok(p.label.length > 0);
    assert.ok(p.value.length > 0);
  }
  assert.equal(DUE_PRESETS[0].value, "today");
});

test("previewVM: blank input means clear", () => {
  const vm = previewVM("", null);
  assert.equal(vm.state, "empty");
  assert.equal(vm.date, null);
  assert.match(vm.text, /[Cc]lear/);
});

test("previewVM: valid result shows pretty + relative", () => {
  const vm = previewVM("jul 4", okResult());
  assert.equal(vm.state, "valid");
  assert.equal(vm.date, "2026-07-04");
  assert.match(vm.text, /Sat, Jul 4 2026/);
  assert.match(vm.text, /in 10d/);
});

test("previewVM: valid result without relative still shows pretty", () => {
  const vm = previewVM("jul 4", okResult({ relative: undefined }));
  assert.equal(vm.state, "valid");
  assert.match(vm.text, /Sat, Jul 4 2026/);
});

test("previewVM: invalid result surfaces a hint", () => {
  const vm = previewVM("groundhog day", { ok: false, input: "groundhog day", error: "bad" });
  assert.equal(vm.state, "invalid");
  assert.equal(vm.date, null);
  assert.match(vm.text, /[Uu]nrecognized/);
});

test("previewVM: no result yet for non-blank input is a soft 'parsing' empty", () => {
  const vm = previewVM("jul", null);
  assert.equal(vm.state, "empty");
  assert.match(vm.text, /[Pp]arsing/);
});

test("resolveDueCommit: blank when already empty is a no-op", () => {
  assert.equal(resolveDueCommit("", undefined), null);
  assert.equal(resolveDueCommit("   ", ""), null);
});

test("resolveDueCommit: blank clears an existing due date", () => {
  assert.deepEqual(resolveDueCommit("", "2026-07-04"), { due: "" });
});

test("resolveDueCommit: typing the exact stored date is a no-op", () => {
  assert.equal(resolveDueCommit("2026-07-04", "2026-07-04"), null);
});

test("resolveDueCommit: a new value is sent verbatim for the server to parse", () => {
  assert.deepEqual(resolveDueCommit("tomorrow", "2026-07-04"), { due: "tomorrow" });
  assert.deepEqual(resolveDueCommit("fri", undefined), { due: "fri" });
});

test("renderPresets emits a button per preset with data-due-preset", () => {
  const html = renderPresets();
  for (const p of DUE_PRESETS) {
    assert.ok(html.includes(`data-due-preset="${p.value}"`), `missing ${p.value}`);
  }
});

test("renderDuePreview reflects the state class", () => {
  assert.match(renderDuePreview(previewVM("jul 4", okResult())), /is-valid/);
  assert.match(renderDuePreview(previewVM("", null)), /is-empty/);
  assert.match(
    renderDuePreview(previewVM("x", { ok: false, input: "x", error: "bad" })),
    /is-invalid/,
  );
});

test("renderDuePreview escapes text", () => {
  const html = renderDuePreview({ state: "invalid", text: "<script>", date: null });
  assert.match(html, /&lt;script&gt;/);
  assert.doesNotMatch(html, /<script>/);
});
