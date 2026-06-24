/**
 * Filter model (F11) — a pure, testable layer that narrows the visible task
 * list by free-text search, priority, and tag, without touching the DOM.
 *
 * main.ts owns the filter-bar widgets and the render pipeline; this module
 * owns the data: what a FilterState is, how a task is matched against it, and
 * how to collect the tag facets to offer. Keeping it pure means the matching
 * rules (fuzzy search, priority OR, tag OR) are unit-tested with zero browser.
 *
 * Semantics (documented so the UI copy can match):
 *   - search: every whitespace-delimited token must fuzzy-subsequence-match the
 *     task's title OR one of its tags (so "buy mlk" finds "buy milk #grocery").
 *   - priority: OR within the facet — selecting High + Urgent shows both.
 *   - tags: OR within the facet — selecting #work + #home shows either.
 *   - hideDone: drop completed tasks entirely.
 * Facets combine with AND across each other; an empty facet imposes no constraint.
 */

export type Priority = "low" | "medium" | "high" | "urgent";

export interface FilterState {
  /** Free-text fuzzy query over title + tags. */
  query: string;
  /** Selected priorities; empty = any. */
  priorities: Priority[];
  /** Selected tags (lower-cased); empty = any. */
  tags: string[];
  /** When true, completed tasks are hidden. */
  hideDone: boolean;
}

/** The minimal shape a task needs to be filtered. */
export interface FilterableTask {
  title: string;
  priority: string;
  tags: string[];
  done: boolean;
}

export interface TagCount {
  tag: string;
  count: number;
}

/** A fresh, no-op filter (matches everything). */
export function emptyFilter(): FilterState {
  return { query: "", priorities: [], tags: [], hideDone: false };
}

/** True when any facet would actually narrow the list. */
export function isFilterActive(state: FilterState): boolean {
  return (
    state.query.trim() !== "" ||
    state.priorities.length > 0 ||
    state.tags.length > 0 ||
    state.hideDone
  );
}

/** Case-insensitive subsequence test: do all chars of `needle` appear, in order, in `hay`? */
export function isSubsequence(needle: string, hay: string): boolean {
  if (needle === "") return true;
  let ni = 0;
  for (let hi = 0; hi < hay.length && ni < needle.length; hi++) {
    if (hay[hi] === needle[ni]) ni++;
  }
  return ni === needle.length;
}

/**
 * Fuzzy match a query against text. The query is split on whitespace; every
 * token must be a subsequence of the (lower-cased) text. An empty/blank query
 * matches anything.
 */
export function fuzzyMatch(query: string, text: string): boolean {
  const tokens = query.toLowerCase().split(/\s+/).filter(Boolean);
  if (tokens.length === 0) return true;
  const hay = text.toLowerCase();
  return tokens.every((tok) => isSubsequence(tok, hay));
}

/** Does a single task satisfy the filter? */
export function matchesFilter(task: FilterableTask, state: FilterState): boolean {
  if (state.hideDone && task.done) return false;
  if (state.priorities.length > 0 && !state.priorities.includes(task.priority as Priority)) {
    return false;
  }
  if (state.tags.length > 0 && !state.tags.some((tag) => task.tags.includes(tag))) {
    return false;
  }
  if (state.query.trim() !== "") {
    const haystack = task.title + " " + task.tags.join(" ");
    if (!fuzzyMatch(state.query, haystack)) return false;
  }
  return true;
}

/** Apply the filter to a list, preserving input order. */
export function applyFilter<T extends FilterableTask>(tasks: T[], state: FilterState): T[] {
  return tasks.filter((t) => matchesFilter(t, state));
}

/**
 * Collect the distinct tags across tasks with usage counts, sorted by count
 * descending then name ascending — the order the filter-bar chips render in.
 */
export function collectTags(tasks: FilterableTask[]): TagCount[] {
  const counts = new Map<string, number>();
  for (const t of tasks) {
    for (const tag of t.tags) {
      counts.set(tag, (counts.get(tag) ?? 0) + 1);
    }
  }
  const out: TagCount[] = [];
  for (const [tag, count] of counts) out.push({ tag, count });
  out.sort((a, b) => (b.count !== a.count ? b.count - a.count : a.tag < b.tag ? -1 : 1));
  return out;
}

/** Toggle a value's membership in an array, returning a new array. */
export function toggleMember<T>(arr: T[], value: T): T[] {
  return arr.includes(value) ? arr.filter((v) => v !== value) : [...arr, value];
}

/** The four priorities in display order (urgent first), for rendering pills. */
export const PRIORITIES_DESC: ReadonlyArray<Priority> = ["urgent", "high", "medium", "low"];

/** One-letter glyph for a priority, matching the list's chips. */
export function priorityGlyph(p: Priority): string {
  return { urgent: "U", high: "H", medium: "M", low: "L" }[p];
}

/** Escape strings before injecting into innerHTML. Local copy keeps this module dependency-free. */
function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}

/**
 * Render the priority pills row. Pure → unit-tested. Active pills carry
 * `is-active`; each pill carries `data-prio` so a delegated listener can
 * toggle it.
 */
export function renderPriorityPills(state: FilterState): string {
  return PRIORITIES_DESC.map((p) => {
    const active = state.priorities.includes(p) ? " is-active" : "";
    return `<button type="button" class="fpill prio-${p}${active}" data-prio="${p}" aria-pressed="${state.priorities.includes(p)}" title="${p} priority">${priorityGlyph(p)}</button>`;
  }).join("");
}

/**
 * Render the tag chips row from the available tags + current selection. Pure →
 * unit-tested. Returns "" when there are no tags so the row can collapse.
 */
export function renderTagChips(tags: TagCount[], state: FilterState): string {
  if (tags.length === 0) return "";
  return tags
    .map(({ tag, count }) => {
      const active = state.tags.includes(tag) ? " is-active" : "";
      return `<button type="button" class="fchip${active}" data-tag="${escapeHTML(tag)}" aria-pressed="${state.tags.includes(tag)}">#${escapeHTML(tag)}<span class="fchip-n">${count}</span></button>`;
    })
    .join("");
}

/** Human summary of how many of N tasks are visible under the active filter. */
export function filterSummary(visible: number, total: number): string {
  if (visible === total) return `${total} task${total === 1 ? "" : "s"}`;
  return `${visible} of ${total} shown`;
}
