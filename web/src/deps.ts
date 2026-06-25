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

/**
 * F64: narrow a task list to only the tasks that are currently BLOCKED (an
 * undone task with at least one open blocker), using the supplied done-index.
 * Generic so it preserves the caller's concrete task type (it's applied to the
 * live `Task[]` in the render pipeline). Pure → unit-tested. Input order is
 * preserved. The done-index should be built over the WHOLE live list so a
 * blocker hidden by another filter still counts.
 */
export function filterBlocked<T extends DepTask>(tasks: T[], done: Map<number, boolean>): T[] {
  return tasks.filter((t) => isBlocked(t, done));
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

/**
 * F56: the actual longest chain of OPEN blockers as an ordered list of ids,
 * from the most-downstream blocked task to its deepest root blocker
 * (#blocked -> ... -> #root). This is the path whose length is computeDepStats'
 * `longestChain`, materialized so the UI can offer a "walk to the root blocker"
 * jump-list. Returns [] when the graph is flat (no open blockers). Ties pick the
 * first task in list order, then the first open blocker at each step, so the
 * result is deterministic. A cycle is bounded by a visited-on-path guard.
 */
export function longestChainPath(tasks: DepStatsTask[]): number[] {
  const done = doneIndex(tasks);
  const byId = new Map<number, DepStatsTask>();
  for (const t of tasks) byId.set(t.id, t);

  // Walk from a start id, always stepping into the open blocker whose own
  // sub-chain is longest, recording the path. memo holds best depth per id.
  const depth = new Map<number, number>();
  const onPath = new Set<number>();
  const subDepth = (id: number): number => {
    const cached = depth.get(id);
    if (cached !== undefined) return cached;
    if (onPath.has(id)) return 0;
    const task = byId.get(id);
    if (!task) return 0;
    const open = openBlockers(task, done);
    if (open.length === 0) {
      depth.set(id, 0);
      return 0;
    }
    onPath.add(id);
    let best = 0;
    for (const dep of open) {
      const d = subDepth(dep) + 1;
      if (d > best) best = d;
    }
    onPath.delete(id);
    depth.set(id, best);
    return best;
  };

  // Find the undone task with the deepest chain (the head of the longest path).
  let head = -1;
  let headDepth = 0;
  for (const t of tasks) {
    if (t.done) continue;
    const d = subDepth(t.id);
    if (d > headDepth) {
      headDepth = d;
      head = t.id;
    }
  }
  if (head < 0 || headDepth === 0) return [];

  // Materialize the path: at each node, step into the open blocker with the
  // greatest sub-depth (ties -> first in declared order).
  const path: number[] = [];
  const seen = new Set<number>();
  let cur: number | undefined = head;
  while (cur !== undefined && !seen.has(cur)) {
    path.push(cur);
    seen.add(cur);
    const task = byId.get(cur);
    if (!task) break;
    const open = openBlockers(task, done);
    if (open.length === 0) break;
    let next: number | undefined;
    let bestD = -1;
    for (const dep of open) {
      const d = subDepth(dep);
      if (d > bestD) {
        bestD = d;
        next = dep;
      }
    }
    cur = next;
  }
  return path;
}

/** F56: a node in the chain drill-down jump-list: id + display title. */
export interface ChainNode {
  id: number;
  title: string;
}

/** Escape strings before injecting into innerHTML. Local copy keeps this pure. */
function escapeChainHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}

/**
 * F56: render the longest-chain drill-down as an ordered jump-list. Each step
 * is a button carrying `data-chain-jump="<id>"` so a delegated click selects +
 * scrolls to that task; an arrow separates the steps to read the chain
 * direction (downstream blocked task first, deepest root blocker last). The
 * last node is tagged `is-root` so the UI can label "root blocker". Returns ""
 * for an empty chain so the caller can skip opening an empty popover.
 */
export function renderChainDrill(nodes: ChainNode[]): string {
  if (nodes.length === 0) return "";
  const items = nodes
    .map((n, i) => {
      const root = i === nodes.length - 1 ? " is-root" : "";
      const arrow =
        i < nodes.length - 1
          ? `<span class="chain-arrow" aria-hidden="true">&#8595;</span>`
          : "";
      return `<li class="chain-step${root}">
        <button type="button" class="chain-jump" data-chain-jump="${n.id}" title="Jump to #${n.id}">
          <span class="chain-id">#${n.id}</span>
          <span class="chain-title">${escapeChainHTML(n.title)}</span>
        </button>
        ${arrow}
      </li>`;
    })
    .join("");
  return `<ul class="chain-list" role="menu" aria-label="Blocker chain">${items}</ul>`;
}
