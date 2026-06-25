/**
 * Export menu model (F19) — pure helpers for the "Export" affordance that
 * downloads the task list as JSON, CSV, or Markdown via /api/export.
 *
 * main.ts owns the menu DOM, open/close, and the actual download trigger; this
 * module owns the data: the available formats, the URL each maps to, the
 * download filename, and the menu markup. Keeping it pure means the URL
 * building + rendering are unit-tested with zero DOM.
 */

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
 */
export function renderExportMenu(scopedCount: number | null = null): string {
  const header =
    scopedCount === null
      ? ""
      : `<div class="export-scope" role="presentation">${escapeHTML(exportScopeLabel(scopedCount))}</div>`;
  const items = EXPORT_OPTIONS.map(
    (o) =>
      `<button type="button" class="export-item" role="menuitem" data-export-format="${o.format}">
        <span class="export-label">${escapeHTML(o.label)}</span>
        <span class="export-ext">${escapeHTML(o.ext)}</span>
      </button>`,
  ).join("");
  return header + items;
}
