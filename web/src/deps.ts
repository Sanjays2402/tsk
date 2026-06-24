/**
 * Dependency awareness (F26) — a pure, testable layer that answers "is this
 * task blocked, and by what?" without touching the DOM.
 *
 * The model already tracks `depends_on: number[]` per task (the same
 * `depends:` meta key the CLI's `tsk depend` writes). A task is BLOCKED when at
 * least one of the ids it depends on refers to a task that is not yet done.
 * Done blockers, and ids that no longer exist (deleted), don't block.
 *
 * main.ts owns the row chrome and the click handlers; this module owns the
 * data: the blocked predicate, which specific blockers are still open, and the
 * little "blocked by #N" badge markup. Keeping it pure means the graph logic is
 * unit-tested with zero browser.
 */

/** The minimal shape a task needs for dependency reasoning. */
export interface DepTask {
  id: number;
  done: boolean;
  depends_on?: number[];
}

/** Build a fast id -> done lookup over the whole task list. */
export function doneIndex(tasks: DepTask[]): Map<number, boolean> {
  const m = new Map<number, boolean>();
  for (const t of tasks) m.set(t.id, t.done);
  return m;
}

/**
 * The subset of a task's declared dependencies that are still OPEN blockers:
 * the dep id exists in the store AND that task is not done. Unknown ids
 * (deleted tasks) and completed deps are filtered out. Order matches the
 * declared `depends_on` order so the badge reads predictably.
 */
export function openBlockers(task: DepTask, done: Map<number, boolean>): number[] {
  const deps = task.depends_on;
  if (!deps || deps.length === 0) return [];
  const out: number[] = [];
  for (const dep of deps) {
    // A dep that isn't in the index anymore was deleted — it no longer blocks.
    if (!done.has(dep)) continue;
    if (!done.get(dep)) out.push(dep);
  }
  return out;
}

/**
 * Is this task blocked? True when it is not itself done and has at least one
 * open blocker. A done task is never "blocked" (you already finished it), and
 * neither is one whose blockers are all complete.
 */
export function isBlocked(task: DepTask, done: Map<number, boolean>): boolean {
  if (task.done) return false;
  return openBlockers(task, done).length > 0;
}

/** Escape strings before injecting into innerHTML. Local copy keeps this pure. */
function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}

/** Human label for a set of open blockers, e.g. "blocked by #3, #7". */
export function blockerLabel(blockers: number[]): string {
  if (blockers.length === 0) return "";
  return "blocked by " + blockers.map((id) => `#${id}`).join(", ");
}

/**
 * Render the "blocked by #N" badge for a row. Returns "" when the task has no
 * open blockers so the badge collapses. The badge is a button so a future
 * slice can wire a click to jump to / highlight the blocker; for now it carries
 * the data hook and a descriptive title.
 */
export function renderBlockedBadge(task: DepTask, done: Map<number, boolean>): string {
  const blockers = openBlockers(task, done);
  if (blockers.length === 0) return "";
  const label = blockerLabel(blockers);
  const first = blockers[0];
  return `<button type="button" class="dep-badge" data-dep-jump="${first}" title="${escapeHTML(label)} — click to jump to #${first}" aria-label="${escapeHTML(label)}">&#9211; ${escapeHTML(blockers.map((id) => `#${id}`).join(" "))}</button>`;
}

/**
 * The CSS flag a row carries when blocked, so the list can grey it out and
 * dim the checkbox. Returns "is-blocked" or "".
 */
export function blockedClass(task: DepTask, done: Map<number, boolean>): string {
  return isBlocked(task, done) ? "is-blocked" : "";
}
