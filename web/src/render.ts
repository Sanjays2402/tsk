/**
 * Renderers — pure functions from data → DOM strings/elements.
 *
 * Kept separate from main.ts so future slices (search, filter, sort) can
 * test these in isolation.
 */

import type { Task } from "./api";
import { groupIntoSections, type Section } from "./sections";
import { renderNotesButton, renderNotesSnippet } from "./notes";
import { blockedClass, renderBlockedBadge, type DepTask } from "./deps";
import { highlightText, highlightTitle } from "./highlight";

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

/**
 * Per-render context threaded into row rendering. `done` is the id->done index
 * (F26) used to compute blocked state; `query` is the active filter text (F30)
 * used to highlight matched characters in titles. Absent context means "no
 * decoration" (keeps renderRow usable in isolation / tests).
 */
export interface RowContext {
  done?: Map<number, boolean>;
  query?: string;
}

/** Render a single task row as HTML. */
export function renderRow(t: Task, now: Date, ctx: RowContext = {}): string {
  const dueState = dueClassFor(t.due, t.done, now);
  const dep = ctx.done ? blockedClass(t as DepTask, ctx.done) : "";
  const pin = t.pinned ? "is-pinned" : "";
  const classes = ["row", t.done ? "is-done" : "", dueState, dep, pin]
    .filter(Boolean)
    .join(" ");
  const dueLabel = t.due ? formatDue(t.due, now) : null;
  // F43: when a search query is active, highlight the matched subsequence in
  // each tag pill too (not just the title), so a fuzzy match that landed on a
  // tag is visible. highlightText escapes the tag, so it's safe in innerHTML.
  const tagsHTML = t.tags
    .map((tag) => {
      const inner = ctx.query ? highlightText(tag, ctx.query) : escapeHTML(tag);
      return `<button class="tag" type="button" data-tagnav="${escapeHTML(tag)}" title="Open #${escapeHTML(tag)} page">${inner}</button>`;
    })
    .join("");
  const dueCell = dueLabel
    ? `<span class="due" data-due title="${escapeHTML(t.due ?? "")} — click to change (d)">${escapeHTML(dueLabel)}</span>`
    : `<button class="due-add" data-due type="button" aria-label="Set due date" title="Set due date (d)">+date</button>`;
  const depBadge = ctx.done ? renderBlockedBadge(t as DepTask, ctx.done) : "";
  const pinBtn = `<button class="pin-btn${t.pinned ? " is-on" : ""}" data-pin type="button" aria-pressed="${t.pinned ? "true" : "false"}" aria-label="${t.pinned ? "Unpin task" : "Pin task"}" title="${t.pinned ? "Unpin (p)" : "Pin to top (p)"}" tabindex="-1">${t.pinned ? "★" : "☆"}</button>`;
  // F30: when a search query is active, mark the matched characters in the
  // title. highlightTitle escapes the title itself, so it's safe in innerHTML.
  const titleHTML = ctx.query ? highlightTitle(t.title, ctx.query) : escapeHTML(t.title);
  // F31: a faded one-line notes preview under the title (empty when no notes)
  // so context is recallable without opening the editor. F43: when a search
  // query is active, highlight the matched subsequence inside the snippet too.
  const notesSnippet = renderNotesSnippet(
    t.notes,
    72,
    ctx.query ? (text) => highlightText(text, ctx.query!) : undefined,
  );
  return `
    <li class="${classes}" data-id="${t.id}" draggable="true">
      <button class="drag-handle" data-drag-handle type="button" aria-label="Drag to reorder" title="Drag to reorder" tabindex="-1">⠿</button>
      <input type="checkbox" class="check" data-toggle aria-label="Toggle done" ${t.done ? "checked" : ""}>
      <div class="title-wrap">
        <div class="title-line">
          ${pinBtn}
          <span class="title" title="${escapeHTML(t.title)}">${titleHTML}</span>
          <span class="id">#${t.id}</span>
        </div>
        ${notesSnippet}
      </div>
      <div class="meta">
        ${depBadge}
        ${tagsHTML ? `<span class="tags">${tagsHTML}</span>` : ""}
        ${dueCell}
        ${renderNotesButton(t.notes)}
        <button class="priority ${escapeHTML(t.priority)}" data-prio-cycle type="button" title="${escapeHTML(t.priority)} priority — click to raise, alt-click to lower" aria-label="Cycle priority (currently ${escapeHTML(t.priority)})">${priorityShort(t.priority)}</button>
        <button class="row-menu" data-row-menu type="button" aria-label="Task actions" aria-haspopup="menu" title="Actions (right-click)" tabindex="-1">&#8943;</button>
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
 * Render the full list view, grouped into Pinned / Overdue / Today / Upcoming
 * / No Due / Done sections (F9 + F27). Empty sections are omitted. Each section
 * carries a count so you can see the shape of your day at a glance.
 */
export function renderTasks(tasks: Task[], now: Date, ctx: RowContext = {}): string {
  return renderSections(groupIntoSections(tasks, now), now, ctx);
}

/**
 * Render pre-grouped sections. Split from renderTasks so callers (main.ts) can
 * group once and reuse the same Section[] for both the DOM and keyboard-nav
 * order, guaranteeing the two never drift.
 */
export function renderSections(
  sections: Section<Task>[],
  now: Date,
  ctx: RowContext = {},
): string {
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
      const rows = section.tasks.map((t) => renderRow(t, now, ctx)).join("");
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
