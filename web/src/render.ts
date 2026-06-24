/**
 * Renderers — pure functions from data → DOM strings/elements.
 *
 * Kept separate from main.ts so future slices (search, filter, sort) can
 * test these in isolation.
 */

import type { Task } from "./api";

/** Escape strings before injecting into innerHTML. Cheap, no deps. */
export function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}

/** Pretty-print a due date relative to a reference day, e.g. "today" or "in 3d". */
export function formatDue(due: string, now: Date): string | null {
  if (!due) return null;
  // due is YYYY-MM-DD in the server's TZ. Parse it as local midnight to compare apples-to-apples.
  const [y, m, d] = due.split("-").map((n) => parseInt(n, 10));
  if (!y || !m || !d) return due;
  const due0 = new Date(y, m - 1, d);
  const today0 = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const days = Math.round((due0.getTime() - today0.getTime()) / 86_400_000);
  if (days === 0) return "today";
  if (days === 1) return "tomorrow";
  if (days === -1) return "yesterday";
  if (days < 0) return `${-days}d ago`;
  if (days < 7) return `in ${days}d`;
  if (days < 14) return "next week";
  // Fall back to a compact month-day (e.g. "Jul 4")
  return due0.toLocaleDateString(undefined, { month: "short", day: "numeric" });
}

/** Returns CSS class flags for due-date state on a row. */
export function dueClassFor(due: string | undefined, done: boolean, now: Date): string {
  if (!due || done) return "";
  const [y, m, d] = due.split("-").map((n) => parseInt(n, 10));
  if (!y || !m || !d) return "";
  const due0 = new Date(y, m - 1, d).getTime();
  const today0 = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  if (due0 < today0) return "is-overdue";
  if (due0 === today0) return "is-today";
  return "";
}

/** Render a single task row as HTML. */
export function renderRow(t: Task, now: Date): string {
  const dueState = dueClassFor(t.due, t.done, now);
  const classes = ["row", t.done ? "is-done" : "", dueState].filter(Boolean).join(" ");
  const dueLabel = t.due ? formatDue(t.due, now) : null;
  const tagsHTML = t.tags
    .map((tag) => `<span class="tag">${escapeHTML(tag)}</span>`)
    .join("");
  return `
    <li class="${classes}" data-id="${t.id}">
      <input type="checkbox" class="check" data-toggle aria-label="Toggle done" ${t.done ? "checked" : ""}>
      <div class="title-wrap">
        <span class="title" title="${escapeHTML(t.title)}">${escapeHTML(t.title)}</span>
        <span class="id">#${t.id}</span>
      </div>
      <div class="meta">
        ${tagsHTML ? `<span class="tags">${tagsHTML}</span>` : ""}
        ${dueLabel ? `<span class="due" title="${escapeHTML(t.due ?? "")}">${escapeHTML(dueLabel)}</span>` : ""}
        <span class="priority ${escapeHTML(t.priority)}" title="${escapeHTML(t.priority)} priority">${priorityShort(t.priority)}</span>
      </div>
    </li>`;
}

/** One-letter glyph matching the TUI's Priority.Short(). */
export function priorityShort(p: string): string {
  switch (p) {
    case "urgent":
      return "U";
    case "high":
      return "H";
    case "low":
      return "L";
    default:
      return "M";
  }
}

/** Render the full list view (header summary + list + empty state). */
export function renderTasks(tasks: Task[], now: Date): string {
  if (tasks.length === 0) {
    return `
      <div class="empty">
        <div class="glyph">✓</div>
        <div>No tasks yet.</div>
        <div class="hint">Add one from the CLI: <code>tsk add "buy milk"</code></div>
      </div>`;
  }
  // Stable order: undone first (sorted by priority desc, then id),
  // then done (by id). Mirrors the TUI's mental model.
  const sorted = [...tasks].sort((a, b) => {
    if (a.done !== b.done) return a.done ? 1 : -1;
    const order = { urgent: 0, high: 1, medium: 2, low: 3 } as const;
    const pa = order[a.priority] ?? 9;
    const pb = order[b.priority] ?? 9;
    if (pa !== pb) return pa - pb;
    return a.id - b.id;
  });
  const rows = sorted.map((t) => renderRow(t, now)).join("");
  return `<ul class="list">${rows}</ul>`;
}

/** Compute the "12 undone / 5 done" statusline summary. */
export function summarize(tasks: Task[]): string {
  let done = 0;
  let undone = 0;
  for (const t of tasks) {
    if (t.done) done++;
    else undone++;
  }
  return `<strong>${undone}</strong> undone &middot; <strong>${done}</strong> done &middot; <strong>${tasks.length}</strong> total`;
}
