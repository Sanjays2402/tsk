/**
 * Renderers — pure functions from data → DOM strings/elements.
 *
 * Kept separate from main.ts so future slices (search, filter, sort) can
 * test these in isolation.
 */

import type { Task } from "./api";
import { groupIntoSections, type Section } from "./sections";

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
    .map((tag) => `<button class="tag" type="button" data-tagnav="${escapeHTML(tag)}" title="Open #${escapeHTML(tag)} page">${escapeHTML(tag)}</button>`)
    .join("");
  const dueCell = dueLabel
    ? `<span class="due" data-due title="${escapeHTML(t.due ?? "")} — click to change (d)">${escapeHTML(dueLabel)}</span>`
    : `<button class="due-add" data-due type="button" aria-label="Set due date" title="Set due date (d)">+date</button>`;
  return `
    <li class="${classes}" data-id="${t.id}" draggable="true">
      <button class="drag-handle" data-drag-handle type="button" aria-label="Drag to reorder" title="Drag to reorder" tabindex="-1">⠿</button>
      <input type="checkbox" class="check" data-toggle aria-label="Toggle done" ${t.done ? "checked" : ""}>
      <div class="title-wrap">
        <span class="title" title="${escapeHTML(t.title)}">${escapeHTML(t.title)}</span>
        <span class="id">#${t.id}</span>
      </div>
      <div class="meta">
        ${tagsHTML ? `<span class="tags">${tagsHTML}</span>` : ""}
        ${dueCell}
        <span class="priority ${escapeHTML(t.priority)}" title="${escapeHTML(t.priority)} priority">${priorityShort(t.priority)}</span>
        <button class="row-del" data-del type="button" aria-label="Delete task" title="Delete (x)">&times;</button>
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

/**
 * Render the full list view, grouped into Overdue / Today / Upcoming / No Due
 * / Done sections (F9). Empty sections are omitted. Each section carries a
 * count so you can see the shape of your day at a glance.
 */
export function renderTasks(tasks: Task[], now: Date): string {
  return renderSections(groupIntoSections(tasks, now), now);
}

/**
 * Render pre-grouped sections. Split from renderTasks so callers (main.ts) can
 * group once and reuse the same Section[] for both the DOM and keyboard-nav
 * order, guaranteeing the two never drift.
 */
export function renderSections(sections: Section<Task>[], now: Date): string {
  if (sections.length === 0) {
    return `
      <div class="empty">
        <div class="glyph">✓</div>
        <div>No tasks yet.</div>
        <div class="hint">Add one above, or from the CLI: <code>tsk add "buy milk"</code></div>
      </div>`;
  }
  return sections
    .map((section) => {
      const rows = section.tasks.map((t) => renderRow(t, now)).join("");
      return `
      <section class="section section-${section.key}" data-section="${section.key}">
        <div class="section-head">
          <span class="section-label">${escapeHTML(section.label)}</span>
          <span class="section-count">${section.tasks.length}</span>
        </div>
        <ul class="list">${rows}</ul>
      </section>`;
    })
    .join("");
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
