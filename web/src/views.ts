/**
 * Saved views model (F25) — name a filter combination (search query + priority
 * + tag facets + hide-done) and recall it later from a chip row or the Cmd-K
 * palette. Persisted in localStorage as a per-client convenience; like F24
 * settings, saved views never touch .tsk.md.
 *
 * Pure + dependency-free (unit-tested under `node --test`). main.ts owns the
 * chip-row DOM, the save affordance, and the localStorage plumbing; this module
 * owns the data: the SavedView shape, CRUD over the list, (de)serialization,
 * the "does the live filter equal this view?" check that drives the active
 * highlight, and the chip markup.
 */

export type Priority = "low" | "medium" | "high" | "urgent";

/** The slice of FilterState a view captures. Mirrors filter.ts's FilterState. */
export interface ViewFilter {
  query: string;
  priorities: Priority[];
  tags: string[];
  hideDone: boolean;
}

export interface SavedView {
  /** Stable id (timestamp-ish) so renames don't lose recall wiring. */
  id: string;
  name: string;
  filter: ViewFilter;
}

export const STORAGE_KEY = "tsk.views";

/** Normalize a filter into the canonical view slice (sorted facets, trimmed). */
export function normalizeFilter(f: ViewFilter): ViewFilter {
  return {
    query: f.query.trim(),
    priorities: [...f.priorities].sort(),
    tags: [...f.tags].map((t) => t.toLowerCase()).sort(),
    hideDone: f.hideDone === true,
  };
}

/** True when a filter actually constrains anything (empty filters aren't worth saving). */
export function filterIsEmpty(f: ViewFilter): boolean {
  const n = normalizeFilter(f);
  return n.query === "" && n.priorities.length === 0 && n.tags.length === 0 && !n.hideDone;
}

/** Deep-equality on the normalized filter slice — drives the active-chip highlight. */
export function filtersEqual(a: ViewFilter, b: ViewFilter): boolean {
  const na = normalizeFilter(a);
  const nb = normalizeFilter(b);
  return (
    na.query === nb.query &&
    na.hideDone === nb.hideDone &&
    na.priorities.length === nb.priorities.length &&
    na.priorities.every((p, i) => p === nb.priorities[i]) &&
    na.tags.length === nb.tags.length &&
    na.tags.every((t, i) => t === nb.tags[i])
  );
}

/** Coerce unknown parsed JSON into a clean SavedView[], dropping junk entries. */
export function normalizeViews(raw: unknown): SavedView[] {
  if (!Array.isArray(raw)) return [];
  const out: SavedView[] = [];
  for (const item of raw) {
    if (typeof item !== "object" || item === null) continue;
    const o = item as Record<string, unknown>;
    if (typeof o.name !== "string" || o.name.trim() === "") continue;
    const f = (typeof o.filter === "object" && o.filter !== null ? o.filter : {}) as Record<string, unknown>;
    out.push({
      id: typeof o.id === "string" && o.id !== "" ? o.id : makeId(),
      name: o.name.trim(),
      filter: normalizeFilter({
        query: typeof f.query === "string" ? f.query : "",
        priorities: Array.isArray(f.priorities) ? (f.priorities.filter(isPriority) as Priority[]) : [],
        tags: Array.isArray(f.tags) ? f.tags.filter((t): t is string => typeof t === "string") : [],
        hideDone: f.hideDone === true,
      }),
    });
  }
  return out;
}

function isPriority(p: unknown): p is Priority {
  return p === "low" || p === "medium" || p === "high" || p === "urgent";
}

/** Parse the stored JSON into views, tolerating null / malformed stores. */
export function parseViews(stored: string | null): SavedView[] {
  if (stored === null) return [];
  try {
    return normalizeViews(JSON.parse(stored));
  } catch {
    return [];
  }
}

/** Serialize views for storage. */
export function serializeViews(views: SavedView[]): string {
  return JSON.stringify(views);
}

/** Generate a reasonably-unique id without a dependency. */
export function makeId(): string {
  return `v${Date.now().toString(36)}${Math.random().toString(36).slice(2, 6)}`;
}

/**
 * Add a view (or overwrite an existing one with the same case-insensitive name)
 * capturing the given filter. Returns a NEW array. A blank name or an empty
 * filter is rejected (returns the list unchanged) — the caller validates UX
 * messaging; this just guarantees the store stays clean.
 */
export function addView(views: SavedView[], name: string, filter: ViewFilter): SavedView[] {
  const trimmed = name.trim();
  if (trimmed === "" || filterIsEmpty(filter)) return views;
  const norm = normalizeFilter(filter);
  const existing = views.find((v) => v.name.toLowerCase() === trimmed.toLowerCase());
  if (existing) {
    return views.map((v) => (v.id === existing.id ? { ...v, name: trimmed, filter: norm } : v));
  }
  return [...views, { id: makeId(), name: trimmed, filter: norm }];
}

/** Remove a view by id. Returns a NEW array. */
export function removeView(views: SavedView[], id: string): SavedView[] {
  return views.filter((v) => v.id !== id);
}

/** Find the view whose filter matches the live one, or null. */
export function activeView(views: SavedView[], filter: ViewFilter): SavedView | null {
  if (filterIsEmpty(filter)) return null;
  return views.find((v) => filtersEqual(v.filter, filter)) ?? null;
}

/** Escape strings before injecting into innerHTML. Local copy keeps this dependency-free. */
function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}

/** A compact human description of what a view filters, for the chip tooltip. */
export function describeView(v: SavedView): string {
  const parts: string[] = [];
  if (v.filter.query) parts.push(`"${v.filter.query}"`);
  if (v.filter.priorities.length) parts.push(`priority: ${v.filter.priorities.join("/")}`);
  if (v.filter.tags.length) parts.push(`tags: ${v.filter.tags.map((t) => "#" + t).join(" ")}`);
  if (v.filter.hideDone) parts.push("hide done");
  return parts.length ? parts.join(" · ") : "all tasks";
}

/**
 * Render the saved-views chip row. Each chip carries `data-view-id` for recall
 * and a `data-view-del` button to forget it. The chip matching the live filter
 * gets `is-active`. Returns "" when there are no views so the row collapses.
 */
export function renderViewChips(views: SavedView[], filter: ViewFilter): string {
  if (views.length === 0) return "";
  return views
    .map((v) => {
      const active = filtersEqual(v.filter, filter) ? " is-active" : "";
      return `<span class="view-chip${active}" data-view-id="${escapeHTML(v.id)}" title="${escapeHTML(describeView(v))}"><button type="button" class="view-chip-name" data-view-recall="${escapeHTML(v.id)}">${escapeHTML(v.name)}</button><button type="button" class="view-chip-del" data-view-del="${escapeHTML(v.id)}" aria-label="Delete view ${escapeHTML(v.name)}">&times;</button></span>`;
    })
    .join("");
}
