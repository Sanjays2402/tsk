/**
 * Export menu model (F19) — pure helpers for the "Export" affordance that
 * downloads the task list as JSON, CSV, or Markdown via /api/export.
 *
 * main.ts owns the menu DOM, open/close, and the actual download trigger; this
 * module owns the data: the available formats, the URL each maps to, the
 * download filename, and the menu markup. Keeping it pure means the URL
 * building + rendering are unit-tested with zero DOM.
 */

import type { Command } from "./palette.ts";

export type ExportFormat = "json" | "csv" | "markdown";

export interface ExportOption {
  format: ExportFormat;
  label: string;
  /** Short hint shown on the right, e.g. the file extension. */
  ext: string;
}

/** The export formats offered, in menu order. */
export const EXPORT_OPTIONS: ReadonlyArray<ExportOption> = [
  { format: "json", label: "JSON", ext: ".json" },
  { format: "csv", label: "CSV", ext: ".csv" },
  { format: "markdown", label: "Markdown", ext: ".md" },
];

/** Build the /api/export URL for a given format. */
export function exportUrl(format: ExportFormat): string {
  return `/api/export?format=${encodeURIComponent(format)}`;
}

/**
 * F75: build an export URL scoped to a specific set of task ids ("export what
 * you see"). When `ids` is non-empty, an `&ids=1,2,3` param is appended so the
 * server narrows the download to exactly that subset, in store order. When
 * `ids` is empty/undefined, this is identical to exportUrl (whole store) — the
 * caller only scopes when a lens/filter is actually active. Pure → unit-tested.
 */
export function scopedExportUrl(format: ExportFormat, ids: number[]): string {
  const base = exportUrl(format);
  if (!ids || ids.length === 0) return base;
  return `${base}&ids=${encodeURIComponent(ids.join(","))}`;
}

/**
 * F75: the label suffix for a scoped export (e.g. the export-menu header reads
 * "Export 4 shown" when a lens/filter is active, "Export" otherwise). Pure.
 */
export function exportScopeLabel(scopedCount: number | null): string {
  if (scopedCount === null) return "Export";
  return `Export ${scopedCount} shown`;
}

/** The download filename a format maps to (tasks.json / tasks.csv / tasks.md). */
export function exportFilename(format: ExportFormat): string {
  switch (format) {
    case "json":
      return "tasks.json";
    case "csv":
      return "tasks.csv";
    case "markdown":
      return "tasks.md";
  }
}

/** Escape strings before injecting into innerHTML. Local copy keeps this pure. */
function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}

/**
 * Render the export dropdown menu. Pure -> unit-tested. Each item carries
 * data-export-format so a delegated listener can trigger the download.
 *
 * F75: when `scopedCount` is a number, a header reads "Export N shown" so it's
 * clear the download will carry only the visible (lens/filter/tag) subset, not
 * the whole store. With null (nothing narrowing the board) the header is
 * omitted and the menu exports everything as before.
 *
 * F84: when scoping is active, the header is a two-button "all / shown" segmented
 * toggle (data-export-scope-toggle) so the menu reaches BOTH targets — mirroring
 * the palette's two-command split (F78). `scopeShown` selects which segment is
 * active: true -> the items download the visible subset, false -> the whole
 * store. The chosen segment wears `is-on`; the format items below carry
 * data-export-scope so the click handler knows which target to honour without
 * re-reading the toggle. With null scopedCount (nothing narrows the board) there
 * is only one possible target, so no toggle is shown.
 */
export function renderExportMenu(
  scopedCount: number | null = null,
  scopeShown = true,
): string {
  let header = "";
  if (scopedCount !== null) {
    const allOn = scopeShown ? "" : " is-on";
    const shownOn = scopeShown ? " is-on" : "";
    header = `<div class="export-scope-toggle" role="group" aria-label="Export scope">
      <button type="button" class="export-scope-seg${allOn}" data-export-scope-toggle="all" aria-pressed="${!scopeShown}">All</button>
      <button type="button" class="export-scope-seg${shownOn}" data-export-scope-toggle="shown" aria-pressed="${scopeShown}">${escapeHTML(exportScopeLabel(scopedCount).replace(/^Export /, ""))}</button>
    </div>`;
  }
  // F84: items advertise the active scope so the handler honours the toggle.
  // When nothing narrows the board (scopedCount null) the items are unscoped.
  const itemScope = scopedCount !== null && scopeShown ? "shown" : "all";
  const items = EXPORT_OPTIONS.map(
    (o) =>
      `<button type="button" class="export-item" role="menuitem" data-export-format="${o.format}" data-export-scope="${itemScope}">
        <span class="export-label">${escapeHTML(o.label)}</span>
        <span class="export-ext">${escapeHTML(o.ext)}</span>
      </button>`,
  ).join("");
  return header + items;
}

/**
 * F78: build the palette's export command group. Pure → unit-tested.
 *
 * The three base "Export tasks as <FMT>" commands always export the WHOLE store
 * (id `export-<fmt>`). When a lens/filter/tag is narrowing the board
 * (`scopedCount` is a number), three extra "Export N shown as <FMT>" commands
 * (id `export-scoped-<fmt>`) are prepended so the visible subset is reachable
 * keyboard-only — distinct from the whole-store commands, and only offered when
 * scoping is actually active so they never duplicate the base set on a plain
 * board. The `scopedCount` is woven into each scoped title via exportScopeLabel
 * so it reads "Export 4 shown as CSV".
 */
export function buildExportCommands(scopedCount: number | null = null): Command[] {
  const scoped: Command[] =
    scopedCount === null
      ? []
      : EXPORT_OPTIONS.map((o) => ({
          id: `export-scoped-${o.format}`,
          title: `${exportScopeLabel(scopedCount)} as ${o.label}`,
          group: "Export",
          keywords: ["download", "visible", "subset", "lens", "filter", "scoped", o.ext.replace(".", "")],
        }));
  const all: Command[] = EXPORT_OPTIONS.map((o) => ({
    id: `export-${o.format}`,
    title: scopedCount === null ? `Export tasks as ${o.label}` : `Export all tasks as ${o.label}`,
    group: "Export",
    keywords: ["download", "save", "all", "everything", o.ext.replace(".", "")],
  }));
  return [...scoped, ...all];
}

/**
 * F78: decode an export command id into its format + whether it's the scoped
 * ("export what you see") variant. Pure → unit-tested. Returns null for any
 * non-export command. Lets runCommand dispatch one switch over both the
 * whole-store (`export-csv`) and scoped (`export-scoped-csv`) ids.
 */
export function exportCommandTarget(
  id: string,
): { format: ExportFormat; scoped: boolean } | null {
  const scopedPrefix = "export-scoped-";
  const allPrefix = "export-";
  let scoped = false;
  let fmt = "";
  if (id.startsWith(scopedPrefix)) {
    scoped = true;
    fmt = id.slice(scopedPrefix.length);
  } else if (id.startsWith(allPrefix)) {
    fmt = id.slice(allPrefix.length);
  } else {
    return null;
  }
  if (fmt === "json" || fmt === "csv" || fmt === "markdown") {
    return { format: fmt, scoped };
  }
  return null;
}
