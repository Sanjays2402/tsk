import { test } from "node:test";
import assert from "node:assert/strict";
import {
  EXPORT_OPTIONS,
  exportUrl,
  scopedExportUrl,
  exportScopeLabel,
  exportFilename,
  renderExportMenu,
  buildExportCommands,
  exportCommandTarget,
  EXPORT_SCOPE_KEY,
  parseExportScope,
  serializeExportScope,
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

test("renderExportMenu adds a scope toggle only when scopedCount is a number", () => {
  // No scoping -> no toggle header; items default to the 'all' scope.
  assert.doesNotMatch(renderExportMenu(), /export-scope-toggle/);
  assert.doesNotMatch(renderExportMenu(null), /export-scope-toggle/);
  assert.match(renderExportMenu(), /data-export-scope="all"/);
  // Scoping active -> the all/shown segmented toggle appears.
  const scoped = renderExportMenu(4);
  assert.match(scoped, /export-scope-toggle/);
  assert.match(scoped, /data-export-scope-toggle="all"/);
  assert.match(scoped, /data-export-scope-toggle="shown"/);
  assert.match(scoped, /4 shown/);
  // the items still render alongside the header
  for (const o of EXPORT_OPTIONS) {
    assert.ok(scoped.includes(`data-export-format="${o.format}"`));
  }
});

// --- F84: scoped export parity for the menu button -------------------------

test("renderExportMenu defaults the 'shown' segment on + scopes items when scoped", () => {
  const html = renderExportMenu(4); // scopeShown defaults to true
  // the shown segment is the active one
  assert.match(html, /data-export-scope-toggle="shown"[^>]*aria-pressed="true"/);
  assert.match(html, /data-export-scope-toggle="all"[^>]*aria-pressed="false"/);
  // and the format items advertise the 'shown' scope
  assert.match(html, /data-export-format="json" data-export-scope="shown"/);
});

test("renderExportMenu with scopeShown=false flips to the 'all' segment + scope", () => {
  const html = renderExportMenu(4, false);
  assert.match(html, /data-export-scope-toggle="all"[^>]*aria-pressed="true"/);
  assert.match(html, /data-export-scope-toggle="shown"[^>]*aria-pressed="false"/);
  // items now advertise the whole-store scope
  assert.match(html, /data-export-format="csv" data-export-scope="all"/);
  assert.doesNotMatch(html, /data-export-scope="shown"/);
});

test("renderExportMenu unscoped board keeps items on 'all' regardless of scopeShown", () => {
  // With nothing narrowing the board there's only one target.
  for (const shown of [true, false]) {
    const html = renderExportMenu(null, shown);
    assert.doesNotMatch(html, /export-scope-toggle/);
    assert.match(html, /data-export-format="markdown" data-export-scope="all"/);
  }
});

test("renderExportMenu 'is-on' marks exactly the active segment", () => {
  const shownMenu = renderExportMenu(3, true);
  assert.match(shownMenu, /export-scope-seg is-on" data-export-scope-toggle="shown"/);
  const allMenu = renderExportMenu(3, false);
  assert.match(allMenu, /export-scope-seg is-on" data-export-scope-toggle="all"/);
});

// --- F78: scoped export commands in the palette ----------------------------

test("buildExportCommands: only the three whole-store commands when unscoped", () => {
  const cmds = buildExportCommands(null);
  assert.equal(cmds.length, 3);
  assert.deepEqual(
    cmds.map((c) => c.id),
    ["export-json", "export-csv", "export-markdown"],
  );
  // unscoped titles read plainly ("Export tasks as JSON")
  assert.match(cmds[0].title, /Export tasks as JSON/);
});

test("buildExportCommands: prepends the scoped trio when a count is given", () => {
  const cmds = buildExportCommands(4);
  assert.equal(cmds.length, 6);
  // scoped first, then whole-store
  assert.deepEqual(
    cmds.map((c) => c.id),
    [
      "export-scoped-json",
      "export-scoped-csv",
      "export-scoped-markdown",
      "export-json",
      "export-csv",
      "export-markdown",
    ],
  );
});

test("buildExportCommands: scoped titles carry the count, whole-store say 'all'", () => {
  const cmds = buildExportCommands(4);
  const scopedCsv = cmds.find((c) => c.id === "export-scoped-csv")!;
  assert.match(scopedCsv.title, /Export 4 shown as CSV/);
  const allCsv = cmds.find((c) => c.id === "export-csv")!;
  assert.match(allCsv.title, /Export all tasks as CSV/);
});

test("buildExportCommands: every command is in the Export group", () => {
  for (const c of buildExportCommands(2)) {
    assert.equal(c.group, "Export");
  }
});

test("exportCommandTarget decodes whole-store ids (not scoped)", () => {
  assert.deepEqual(exportCommandTarget("export-json"), { format: "json", scoped: false });
  assert.deepEqual(exportCommandTarget("export-csv"), { format: "csv", scoped: false });
  assert.deepEqual(exportCommandTarget("export-markdown"), { format: "markdown", scoped: false });
});

test("exportCommandTarget decodes scoped ids", () => {
  assert.deepEqual(exportCommandTarget("export-scoped-json"), { format: "json", scoped: true });
  assert.deepEqual(exportCommandTarget("export-scoped-csv"), { format: "csv", scoped: true });
  assert.deepEqual(exportCommandTarget("export-scoped-markdown"), { format: "markdown", scoped: true });
});

test("exportCommandTarget returns null for non-export / malformed ids", () => {
  assert.equal(exportCommandTarget("toggle"), null);
  assert.equal(exportCommandTarget("export-pdf"), null); // unknown format
  assert.equal(exportCommandTarget("export-scoped-pdf"), null);
  assert.equal(exportCommandTarget(""), null);
  assert.equal(exportCommandTarget("view:abc"), null);
});

test("exportCommandTarget round-trips every buildExportCommands id", () => {
  for (const c of buildExportCommands(3)) {
    const t = exportCommandTarget(c.id);
    assert.notEqual(t, null);
    // scoped ids decode to scoped:true, whole-store to scoped:false
    assert.equal(t!.scoped, c.id.startsWith("export-scoped-"));
  }
});

// F88 — persisted All/shown scope choice.
test("EXPORT_SCOPE_KEY is a stable sessionStorage key", () => {
  assert.equal(EXPORT_SCOPE_KEY, "tsk.export.scopeShown");
});

test("parseExportScope defaults to shown (true) for unset / unknown values", () => {
  assert.equal(parseExportScope(null), true); // never set
  assert.equal(parseExportScope("1"), true); // shown
  assert.equal(parseExportScope("garbage"), true); // corrupt -> safe default
});

test("parseExportScope decodes the explicit 'all' choice", () => {
  assert.equal(parseExportScope("0"), false);
});

test("serializeExportScope round-trips through parseExportScope", () => {
  for (const v of [true, false]) {
    assert.equal(parseExportScope(serializeExportScope(v)), v);
  }
  assert.equal(serializeExportScope(true), "1");
  assert.equal(serializeExportScope(false), "0");
});
