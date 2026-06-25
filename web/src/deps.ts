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
 * F45: build the confirmation message shown before completing a BLOCKED task,
 * mirroring the CLI's `done` dependency gate. Returns "" when no confirmation
 * is needed (the task isn't blocked). The message names the open blockers so
 * the user knows exactly what's still outstanding before they override it.
 */
export function blockedToggleConfirm(task: DepTask, done: Map<number, boolean>): string {
  const blockers = openBlockers(task, done);
  if (blockers.length === 0) return "";
  const list = blockers.map((id) => `#${id}`).join(", ");
  return `#${task.id} is blocked by ${list} — complete anyway?`;
}

/**
 * F45: should toggling this task require a blocked-confirm? Only when we are
 * COMPLETING it (it's currently not done) AND it has at least one open blocker.
 * Re-opening a done task, or completing an unblocked one, never prompts.
 */
export function needsBlockedConfirm(task: DepTask, done: Map<number, boolean>): boolean {
  if (task.done) return false; // re-opening is always fine
  return openBlockers(task, done).length > 0;
}

/**
 * F42: the ids of tasks that just became UNBLOCKED across a change in the
 * done-set. Given the task list BEFORE and AFTER a toggle, returns the ids of
 * tasks that were blocked before and are no longer blocked after AND are
 * themselves still undone (so an "#N is now unblocked — start it?" prompt is
 * actionable). Matching is by id across the two snapshots; a task missing from
 * either side (added/deleted) is ignored, and the task you just completed is
 * excluded because it's now done. Pure so the before/after diff is unit-tested.
 */
export function newlyUnblocked(before: DepTask[], after: DepTask[]): number[] {
  const beforeDone = doneIndex(before);
  const afterDone = doneIndex(after);
  const beforeById = new Map<number, DepTask>();
  for (const t of before) beforeById.set(t.id, t);
  const out: number[] = [];
  for (const t of after) {
    if (t.done) continue; // a now-done task isn't something to "start"
    const prev = beforeById.get(t.id);
    if (!prev) continue; // newly added — no "before" to compare against
    if (isBlocked(prev, beforeDone) && !isBlocked(t, afterDone)) out.push(t.id);
  }
  return out;
}

/**
 * F42: the message for the "just unblocked" toast. One id reads as an
 * invitation to start it; several are listed plainly. Returns "" for none.
 */
export function unblockedMessage(ids: number[]): string {
  if (ids.length === 0) return "";
  if (ids.length === 1) return `#${ids[0]} is now unblocked — start it?`;
  return `${ids.map((id) => `#${id}`).join(", ")} are now unblocked`;
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

/** F46: aggregate dependency stats for the sidebar. */
export interface DepStats {
  /** How many undone tasks are currently blocked by an open prereq. */
  blocked: number;
  /** How many tasks carry the sticky Pinned flag. */
  pinned: number;
  /**
   * The longest chain of blocker links among UNDONE tasks (the dependency
   * depth). A task with no open blockers has depth 0; one blocked by a task
   * that is itself blocked has depth 2, and so on. Cycles are bounded so a
   * malformed file can't spin forever.
   */
  longestChain: number;
}

/** Minimal shape for dep-stats: needs id, done, deps, and the pinned flag. */
export interface DepStatsTask extends DepTask {
  pinned?: boolean;
}

/**
 * F46: compute the blocked + pinned counts and the longest open-blocker chain
 * across the task list. Pure so the aggregation (including the depth DFS with
 * memo + cycle guard) is unit-tested with zero browser. The chain only follows
 * OPEN blockers (done/deleted prereqs don't extend depth), matching what the
 * "blocked" badge shows the user.
 */
export function computeDepStats(tasks: DepStatsTask[]): DepStats {
  const done = doneIndex(tasks);
  const byId = new Map<number, DepStatsTask>();
  for (const t of tasks) byId.set(t.id, t);

  let blocked = 0;
  let pinned = 0;
  for (const t of tasks) {
    if (t.pinned) pinned++;
    if (isBlocked(t, done)) blocked++;
  }

  // Longest open-blocker chain via memoized DFS with an on-stack cycle guard.
  const depth = new Map<number, number>();
  const onStack = new Set<number>();
  const visit = (id: number): number => {
    const cached = depth.get(id);
    if (cached !== undefined) return cached;
    if (onStack.has(id)) return 0; // cycle — stop counting here
    const task = byId.get(id);
    if (!task) return 0;
    const open = openBlockers(task, done);
    if (open.length === 0) {
      depth.set(id, 0);
      return 0;
    }
    onStack.add(id);
    let best = 0;
    for (const dep of open) {
      const d = visit(dep) + 1;
      if (d > best) best = d;
    }
    onStack.delete(id);
    depth.set(id, best);
    return best;
  };

  let longestChain = 0;
  for (const t of tasks) {
    if (t.done) continue; // depth is about what's still gating undone work
    const d = visit(t.id);
    if (d > longestChain) longestChain = d;
  }

  return { blocked, pinned, longestChain };
}
