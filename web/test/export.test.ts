import { test } from "node:test";
import assert from "node:assert/strict";
import {
  EXPORT_OPTIONS,
  exportUrl,
  scopedExportUrl,
  exportScopeLabel,
  exportFilename,
  renderExportMenu,
  type ExportFormat,
} from "../src/export.ts";

test("EXPORT_OPTIONS lists json, csv, markdown in order", () => {
  assert.deepEqual(
    EXPORT_OPTIONS.map((o) => o.format),
    ["json", "csv", "markdown"],
  );
});

test("exportUrl builds the API path with the format param", () => {
  assert.equal(exportUrl("json"), "/api/export?format=json");
  assert.equal(exportUrl("csv"), "/api/export?format=csv");
  assert.equal(exportUrl("markdown"), "/api/export?format=markdown");
});

test("exportFilename maps each format to a sensible name", () => {
  assert.equal(exportFilename("json"), "tasks.json");
  assert.equal(exportFilename("csv"), "tasks.csv");
  assert.equal(exportFilename("markdown"), "tasks.md");
});

test("renderExportMenu emits a dispatchable item per format", () => {
  const html = renderExportMenu();
  for (const o of EXPORT_OPTIONS) {
    assert.ok(
      html.includes(`data-export-format="${o.format}"`),
      `menu missing format ${o.format}`,
    );
  }
});

test("renderExportMenu shows labels and extensions", () => {
  const html = renderExportMenu();
  assert.match(html, /JSON/);
  assert.match(html, /\.csv/);
  assert.match(html, /Markdown/);
  assert.match(html, /\.md/);
});

test("every option's format is a valid ExportFormat literal", () => {
  const valid: ExportFormat[] = ["json", "csv", "markdown"];
  for (const o of EXPORT_OPTIONS) {
    assert.ok(valid.includes(o.format));
  }
});

// --- F75: scoped "export what you see" -------------------------------------

test("scopedExportUrl appends ids when a subset is given", () => {
  assert.equal(scopedExportUrl("json", [1, 2, 3]), "/api/export?format=json&ids=1%2C2%2C3");
  assert.equal(scopedExportUrl("csv", [7]), "/api/export?format=csv&ids=7");
});

test("scopedExportUrl with no ids is the whole-store url", () => {
  assert.equal(scopedExportUrl("markdown", []), exportUrl("markdown"));
});

test("scopedExportUrl preserves the given id order", () => {
  // store order matters: the server filters in store order, but the client
  // passes whatever order it has; the join must not re-sort.
  assert.equal(scopedExportUrl("json", [3, 1, 2]), "/api/export?format=json&ids=3%2C1%2C2");
});

test("exportScopeLabel reads 'Export N shown' when scoped, 'Export' otherwise", () => {
  assert.equal(exportScopeLabel(null), "Export");
  assert.equal(exportScopeLabel(4), "Export 4 shown");
  assert.equal(exportScopeLabel(0), "Export 0 shown");
});

test("renderExportMenu adds a scope header only when scopedCount is a number", () => {
  assert.doesNotMatch(renderExportMenu(), /export-scope/);
  assert.doesNotMatch(renderExportMenu(null), /export-scope/);
  const scoped = renderExportMenu(4);
  assert.match(scoped, /export-scope/);
  assert.match(scoped, /Export 4 shown/);
  // the items still render alongside the header
  for (const o of EXPORT_OPTIONS) {
    assert.ok(scoped.includes(`data-export-format="${o.format}"`));
  }
});
