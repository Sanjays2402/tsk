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

import { highlightText } from "./highlight.ts";

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
 * F60: the bulk sibling of F45's single-task guard. Given the ids about to be
 * bulk-toggled and the current task list, return the ids that would be
 * COMPLETED while still BLOCKED — i.e. each id whose task is currently undone
 * (a toggle completes it) AND has at least one open blocker. Re-opening a done
 * task, or completing an unblocked one, is never flagged. Ids not in the list
 * (deleted) are skipped. Order follows the input `ids` so the confirm reads
 * predictably. Pure → unit-tested.
 */
export function blockedInBulkToggle(ids: number[], tasks: DepTask[]): number[] {
  const done = doneIndex(tasks);
  const byId = new Map<number, DepTask>();
  for (const t of tasks) byId.set(t.id, t);
  const out: number[] = [];
  for (const id of ids) {
    const task = byId.get(id);
    if (!task) continue;
    if (needsBlockedConfirm(task, done)) out.push(id);
  }
  return out;
}

/**
 * F60: the confirmation message for a bulk toggle that would complete one or
 * more blocked tasks. Names how many of the selection are blocked out of the
 * total being toggled, so the user can decide before overriding the dependency
 * gate. Returns "" when none are blocked (no confirm needed). `blocked` is the
 * count from blockedInBulkToggle; `total` is the whole selection size.
 */
export function bulkBlockedConfirm(blocked: number[], total: number): string {
  if (blocked.length === 0) return "";
  const list = blocked.map((id) => `#${id}`).join(", ");
  const n = blocked.length;
  const noun = n === 1 ? "task is" : "tasks are";
  return `${n} of ${total} ${noun} blocked (${list}) — complete all anyway?`;
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
 * open blockers so the badge collapses. The badge is a group of two buttons:
 *   - the label button (data-dep-jump) jumps to the first open blocker (F26);
 *   - a small chain button (data-chain-from, F61) opens the chain-drill popover
 *     for THIS task's deepest blocker path, so you can walk #this -> ... -> root
 *     without leaving the row (vs. the F56 tile, which only walks the GLOBAL
 *     longest chain). The chain button carries this task's id; main.ts builds
 *     the path via deepestChainFrom on click.
 */
export function renderBlockedBadge(task: DepTask, done: Map<number, boolean>): string {
  const blockers = openBlockers(task, done);
  if (blockers.length === 0) return "";
  const label = blockerLabel(blockers);
  const first = blockers[0];
  const jumpBtn = `<button type="button" class="dep-badge" data-dep-jump="${first}" title="${escapeHTML(label)} — click to jump to #${first}" aria-label="${escapeHTML(label)}">&#9211; ${escapeHTML(blockers.map((id) => `#${id}`).join(" "))}</button>`;
  const chainBtn = `<button type="button" class="dep-chain-btn" data-chain-from="${task.id}" title="Walk the blocker chain from #${task.id}" aria-label="Walk the blocker chain from #${task.id}">&#8627;</button>`;
  return `<span class="dep-badge-group">${jumpBtn}${chainBtn}</span>`;
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

/**
 * F61: the deepest chain of OPEN blockers starting from a SPECIFIC task —
 * #start -> ... -> #root — so the "blocked by #N" row badge can open the chain
 * popover for THAT task's own blocker path, not just the global longest chain
 * (F56). Same greedy "step into the deepest sub-chain" walk and the same
 * determinism / cycle guard as longestChainPath; the only difference is the
 * head is fixed to `start` instead of searched for. Returns [] when `start`
 * isn't in the list, is done, or has no open blockers (nothing to walk).
 */
export function deepestChainFrom(tasks: DepStatsTask[], start: number): number[] {
  const done = doneIndex(tasks);
  const byId = new Map<number, DepStatsTask>();
  for (const t of tasks) byId.set(t.id, t);

  const head = byId.get(start);
  if (!head || head.done) return [];
  if (openBlockers(head, done).length === 0) return [];

  // memoized sub-chain depth per id (same shape as longestChainPath's helper).
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

  const path: number[] = [];
  const seen = new Set<number>();
  let cur: number | undefined = start;
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

/**
 * F74: does a task have a walkable open-blocker chain — i.e. is it worth
 * offering a "walk chain" affordance? True when deepestChainFrom yields at
 * least one STEP beyond the task itself (length >= 2), meaning there's a real
 * blocker path to preview. Pure → unit-tested. Used by the dep editor to show
 * the chain-preview button only on candidates that actually chain into
 * something, so a leaf blocker doesn't carry a dead button.
 */
export function hasWalkableChain(tasks: DepStatsTask[], start: number): boolean {
  return deepestChainFrom(tasks, start).length >= 2;
}

/**
 * F85: the OPEN dependents of a task — every UNDONE task that lists `target`
 * among its still-open blockers, i.e. the tasks currently WAITING on `target`
 * to be done. The mirror of openBlockers (which looks "down" at what blocks a
 * task); this looks "up" at what `target` blocks. Returns ids in task-list
 * order for determinism. A done task is never waiting (it's finished), and a
 * dependent whose dependency is already satisfied elsewhere still counts here
 * only if `target` itself is one of its OPEN blockers — which it is whenever
 * `target` is undone, since an undone blocker is always open.
 */
export function openDependents(
  tasks: DepStatsTask[],
  target: number,
  done: Map<number, boolean>,
): number[] {
  const out: number[] = [];
  for (const t of tasks) {
    if (t.done) continue;
    if (openBlockers(t, done).includes(target)) out.push(t.id);
  }
  return out;
}

/**
 * F85: the deepest chain of OPEN DEPENDENTS starting from a SPECIFIC task —
 * #start -> ... -> #most-downstream-waiter — so a "what waits on this?" popover
 * can walk the IMPACT of finishing `start` (the upstream mirror of F61's
 * downstream blocker walk). Same greedy "step into the deepest sub-chain" walk,
 * determinism (ties pick the first dependent in list order), and on-path cycle
 * guard as deepestChainFrom; the only difference is it follows reverse edges
 * (dependents) instead of forward edges (blockers). Returns [] when `start`
 * isn't in the list, is done, or nothing waits on it (nothing to walk).
 */
export function deepestDependentChainFrom(tasks: DepStatsTask[], start: number): number[] {
  const done = doneIndex(tasks);
  const byId = new Map<number, DepStatsTask>();
  for (const t of tasks) byId.set(t.id, t);

  const head = byId.get(start);
  if (!head || head.done) return [];
  if (openDependents(tasks, start, done).length === 0) return [];

  // memoized dependent-sub-chain depth per id (mirror of deepestChainFrom).
  const depth = new Map<number, number>();
  const onPath = new Set<number>();
  const subDepth = (id: number): number => {
    const cached = depth.get(id);
    if (cached !== undefined) return cached;
    if (onPath.has(id)) return 0;
    const deps = openDependents(tasks, id, done);
    if (deps.length === 0) {
      depth.set(id, 0);
      return 0;
    }
    onPath.add(id);
    let best = 0;
    for (const dep of deps) {
      const d = subDepth(dep) + 1;
      if (d > best) best = d;
    }
    onPath.delete(id);
    depth.set(id, best);
    return best;
  };

  const path: number[] = [];
  const seen = new Set<number>();
  let cur: number | undefined = start;
  while (cur !== undefined && !seen.has(cur)) {
    path.push(cur);
    seen.add(cur);
    const deps = openDependents(tasks, cur, done);
    if (deps.length === 0) break;
    let next: number | undefined;
    let bestD = -1;
    for (const dep of deps) {
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

/**
 * F85: does a task have a walkable DEPENDENT chain — i.e. is it worth offering a
 * "what waits on this?" affordance? True when deepestDependentChainFrom yields
 * at least one step beyond the task itself (length >= 2), meaning something real
 * waits on it. Pure → unit-tested. The upstream mirror of hasWalkableChain.
 */
export function hasWalkableDependents(tasks: DepStatsTask[], start: number): boolean {
  return deepestDependentChainFrom(tasks, start).length >= 2;
}

/**
 * F56: render the longest-chain drill-down as an ordered jump-list. Each step
 * is a button carrying `data-chain-jump="<id>"` so a delegated click selects +
 * scrolls to that task; an arrow separates the steps to read the chain
 * direction (downstream blocked task first, deepest root blocker last). The
 * last node is tagged `is-root` so the UI can label "root blocker". Returns ""
 * for an empty chain so the caller can skip opening an empty popover.
 *
 * F65: when a `query` is passed (the live search), the matched subsequence in
 * each node title is wrapped in <mark> via the generic highlightText engine, so
 * a search that landed on a blocker's text is visible as you walk the chain.
 * Without a query the title is plain-escaped (highlightText's empty-query path).
 */
export function renderChainDrill(nodes: ChainNode[], query = ""): string {
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
          <span class="chain-title">${highlightText(n.title, query)}</span>
        </button>
        ${arrow}
      </li>`;
    })
    .join("");
  return `<ul class="chain-list" role="menu" aria-label="Blocker chain">${items}</ul>`;
}

/**
 * F62: render the "which just-unblocked task do you want to start?" picker for
 * the plural unblock case. Each row is a button carrying `data-unblock-jump`
 * so a delegated click jumps to that task — reusing the same chain-jump chrome
 * so it reads consistently. Returns "" for an empty list so the caller skips
 * opening an empty popover. Pure → unit-tested.
 *
 * F65: an optional `query` highlights the matched subsequence in each title via
 * the generic highlightText engine, consistent with the chain drill.
 */
export function renderUnblockedPicker(nodes: ChainNode[], query = ""): string {
  if (nodes.length === 0) return "";
  const items = nodes
    .map(
      (n) => `<li class="chain-step">
        <button type="button" class="chain-jump" data-unblock-jump="${n.id}" title="Jump to #${n.id}">
          <span class="chain-id">#${n.id}</span>
          <span class="chain-title">${highlightText(n.title, query)}</span>
        </button>
      </li>`,
    )
    .join("");
  return `<ul class="chain-list" role="menu" aria-label="Newly unblocked tasks">${items}</ul>`;
}
