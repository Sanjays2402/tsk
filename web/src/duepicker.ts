/**
 * Due-date picker logic (F12) — pure helpers for the natural-language due
 * editor. The server owns the authoritative parse (via /api/parse-date over
 * the dateparse package); this module owns the surrounding UX decisions:
 * quick-pick presets, the live-preview view-model, and the commit value.
 *
 * Keeping it pure means the preset list, debounce-free preview shaping, and
 * the "what do we PATCH" decision are unit-tested without a browser or a
 * server. main.ts wires the popover DOM + the fetch.
 */

/** The server's response shape for GET /api/parse-date?q=... */
export interface ParseDateResult {
  ok: boolean;
  input: string;
  date?: string; // YYYY-MM-DD when ok
  weekday?: string; // "Mon"
  pretty?: string; // "Sat, Jul 4 2026"
  relative?: string; // "in 10d"
  error?: string;
}

/** A one-tap preset offered in the picker. `value` is sent verbatim to the server. */
export interface DuePreset {
  label: string;
  value: string;
}

/**
 * Quick presets, in display order. Values are the same natural-language tokens
 * the CLI accepts, so the server's dateparse resolves them identically.
 */
export const DUE_PRESETS: ReadonlyArray<DuePreset> = [
  { label: "Today", value: "today" },
  { label: "Tomorrow", value: "tomorrow" },
  { label: "This Fri", value: "fri" },
  { label: "Next week", value: "1w" },
  { label: "End of month", value: "eom" },
];

/** The view-model the preview line renders from. */
export interface DuePreviewVM {
  state: "empty" | "valid" | "invalid";
  /** Human text for the preview line. */
  text: string;
  /** Resolved YYYY-MM-DD when state==="valid", else null. */
  date: string | null;
}

/**
 * Shape a parse result (or the lack of one) into the preview view-model.
 *   - blank input            -> empty ("clear the due date")
 *   - server says ok         -> valid, e.g. "Sat, Jul 4 2026  ·  in 10d"
 *   - server says not ok     -> invalid, surface a hint
 *   - result for stale input -> treated as empty (caller passes null)
 */
export function previewVM(raw: string, result: ParseDateResult | null): DuePreviewVM {
  if (raw.trim() === "") {
    return { state: "empty", text: "Clears the due date", date: null };
  }
  if (!result || !result.ok) {
    const why = result?.error ? `Unrecognized date` : "Parsing…";
    return { state: result ? "invalid" : "empty", text: why, date: null };
  }
  const rel = result.relative ? `  \u00b7  ${result.relative}` : "";
  return { state: "valid", text: `${result.pretty ?? result.date}${rel}`, date: result.date ?? null };
}

/**
 * Decide what (if anything) to PATCH when the picker commits.
 *   - blank          -> { due: "" } clears the date (server treats "" as clear)
 *   - non-blank      -> { due: raw } the server re-parses + validates
 * Returns null when there is nothing to do (no change vs the current value).
 */
export function resolveDueCommit(
  raw: string,
  currentDue: string | undefined,
): { due: string } | null {
  const trimmed = raw.trim();
  const current = (currentDue ?? "").trim();
  if (trimmed === "" && current === "") return null; // already no due date
  if (trimmed === current) return null; // typed exactly the stored YYYY-MM-DD
  return { due: trimmed };
}

/** Render the preset buttons row. Pure → unit-tested. */
export function renderPresets(): string {
  return DUE_PRESETS.map(
    (p) =>
      `<button type="button" class="due-preset" data-due-preset="${escapeAttr(p.value)}">${escapeHTML(p.label)}</button>`,
  ).join("");
}

/** Render the live preview line from a view-model. Pure → unit-tested. */
export function renderDuePreview(vm: DuePreviewVM): string {
  return `<span class="due-preview is-${vm.state}">${escapeHTML(vm.text)}</span>`;
}

function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}
function escapeAttr(s: string): string {
  return escapeHTML(s);
}
