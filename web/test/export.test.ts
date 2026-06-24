import { test } from "node:test";
import assert from "node:assert/strict";
import {
  EXPORT_OPTIONS,
  exportUrl,
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
