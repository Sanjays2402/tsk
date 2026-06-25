/**
 * Dependency editing (F39) — a pure, testable layer for the "blocked by" editor:
 * which ids a task can newly depend on, whether a proposed edge would create a
 * cycle, and how the editor's chips/markup read. The backend PATCH depends_on
 * already validates + persists (F26); this module does the CLIENT-side guards
 * so the UI can refuse a self-ref or a cycle before it ever hits the network,
 * and offer a sensible candidate list.
 *
 * main.ts owns the popover DOM, the input, and the PATCH; this module owns the
 * graph reasoning, unit-tested with zero browser.
 */

/** The minimal task shape needed to reason about the dependency graph. */
export interface DepGraphTask {
  id: number;
  title: string;
  done: boolean;
  depends_on?: number[];
}

/** Build an id -> depends_on adjacency map over the whole task list. */
export function buildAdjacency(tasks: DepGraphTask[]): Map<number, number[]> {
  const adj = new Map<number, number[]>();
  for (const t of tasks) adj.set(t.id, t.depends_on ? [...t.depends_on] : []);
  return adj;
}

/**
 * Can `from` reach `to` by following depends_on edges (transitively)? Used for
 * cycle detection: adding "a depends on b" makes a cycle iff b can already
 * reach a. Iterative DFS with a visited set; tolerates unknown ids and
 * self-loops in the data without infinite-looping.
 */
export function canReach(adj: Map<number, number[]>, from: number, to: number): boolean {
  if (from === to) return true;
  const seen = new Set<number>();
  const stack = [from];
  while (stack.length > 0) {
    const cur = stack.pop()!;
    if (cur === to) return true;
    if (seen.has(cur)) continue;
    seen.add(cur);
    for (const next of adj.get(cur) ?? []) {
      if (!seen.has(next)) stack.push(next);
    }
  }
  return false;
}

export type AddDepResult =
  | { ok: true }
  | { ok: false; reason: "self" | "missing" | "duplicate" | "cycle"; message: string };

/**
 * Validate adding `dep` as a blocker of `task`, client-side. Checks, in order:
 *   - self-reference (a task can't block itself)
 *   - the dep id must exist in the store
 *   - not already a declared blocker (no-op)
 *   - no cycle: dep must not already (transitively) depend on task
 * Returns ok, or a typed reason + a human message the UI can flash.
 */
export function validateAddDep(
  tasks: DepGraphTask[],
  taskId: number,
  dep: number,
): AddDepResult {
  if (dep === taskId) {
    return { ok: false, reason: "self", message: `#${taskId} can't depend on itself` };
  }
  const target = tasks.find((t) => t.id === dep);
  if (!target) {
    return { ok: false, reason: "missing", message: `no task #${dep}` };
  }
  const self = tasks.find((t) => t.id === taskId);
  if (self?.depends_on?.includes(dep)) {
    return { ok: false, reason: "duplicate", message: `already blocked by #${dep}` };
  }
  // Adding "task depends on dep" creates a cycle iff dep can already reach task.
  const adj = buildAdjacency(tasks);
  if (canReach(adj, dep, taskId)) {
    return {
      ok: false,
      reason: "cycle",
      message: `that would make a cycle (#${dep} already needs #${taskId})`,
    };
  }
  return { ok: true };
}

/** The current blocker ids of a task (empty when none / unknown). */
export function currentDeps(tasks: DepGraphTask[], taskId: number): number[] {
  const t = tasks.find((x) => x.id === taskId);
  return t?.depends_on ? [...t.depends_on] : [];
}

/** Add a dep to a task's blocker list (no validation; caller guards first). */
export function withDepAdded(deps: number[], dep: number): number[] {
  return deps.includes(dep) ? deps : [...deps, dep];
}

/** Remove a dep from a blocker list. */
export function withDepRemoved(deps: number[], dep: number): number[] {
  return deps.filter((d) => d !== dep);
}

/**
 * Candidate tasks to offer as new blockers for `taskId`: every OTHER task that
 * isn't already a blocker and wouldn't form a cycle. Optionally narrowed by a
 * query that matches the id (e.g. "3") or a case-insensitive title substring.
 * Open (undone) tasks rank above done ones, then by id ascending.
 */
export function depCandidates(
  tasks: DepGraphTask[],
  taskId: number,
  query = "",
  limit = 8,
): DepGraphTask[] {
  const self = tasks.find((t) => t.id === taskId);
  const already = new Set(self?.depends_on ?? []);
  const adj = buildAdjacency(tasks);
  const q = query.trim().toLowerCase();
  const matches = tasks.filter((t) => {
    if (t.id === taskId || already.has(t.id)) return false;
    if (canReach(adj, t.id, taskId)) return false; // would cycle
    if (q === "") return true;
    return String(t.id) === q || String(t.id).startsWith(q) || t.title.toLowerCase().includes(q);
  });
  matches.sort((a, b) => {
    if (a.done !== b.done) return a.done ? 1 : -1;
    return a.id - b.id;
  });
  return matches.slice(0, limit);
}

/** Escape strings before injecting into innerHTML. Local copy keeps this pure. */
function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}

/** Short, escaped label for a task chip, e.g. "#3 buy milk" (title clamped). */
export function depChipLabel(t: DepGraphTask): string {
  const title = t.title.length > 28 ? t.title.slice(0, 27) + "…" : t.title;
  return `#${t.id} ${title}`;
}

/**
 * Render the dependency editor's body: the current blockers as removable chips,
 * an add input, and a hint. `tasks` resolves each blocker id to a title.
 *
 * F79: when `walkable` reports a CURRENT blocker has its own open-blocker chain,
 * a tiny "walk chain" button (data-dep-chip-walk) is added to that chip so you
 * can audit what an already-added blocker chains into — the sister of F74's
 * affordance on the candidate list. The set is computed by the caller (via
 * deps.hasWalkableChain over the live graph) and passed in, keeping this render
 * pure. A blocker that doesn't chain further carries no button.
 */
export function renderDepEditor(
  tasks: DepGraphTask[],
  taskId: number,
  walkable?: ReadonlySet<number>,
): string {
  const deps = currentDeps(tasks, taskId);
  const chips = deps
    .map((id) => {
      const t = tasks.find((x) => x.id === id);
      const label = t ? depChipLabel(t) : `#${id}`;
      const doneCls = t?.done ? " is-done" : "";
      const walk =
        walkable && walkable.has(id)
          ? `<button type="button" class="depedit-chip-walk" data-dep-chip-walk="${id}" title="Walk this blocker's chain" aria-label="Walk the chain from #${id}">&#8627;</button>`
          : "";
      return `<span class="depedit-chip${doneCls}" data-dep-chip="${id}">
        <span class="depedit-chip-label">${escapeHTML(label)}</span>
        ${walk}
        <button type="button" class="depedit-chip-x" data-dep-remove="${id}" aria-label="Remove blocker #${id}" title="Remove">&times;</button>
      </span>`;
    })
    .join("");
  const chipRow = deps.length
    ? `<div class="depedit-chips">${chips}</div>`
    : `<div class="depedit-empty">No blockers yet</div>`;
  return `
    ${chipRow}
    <div class="depedit-pop-row">
      <input class="depedit-input" data-dep-input type="text" spellcheck="false"
             placeholder="add a blocker by #id or title…" aria-label="Add a blocker">
    </div>
    <ul class="depedit-ac" data-dep-ac role="listbox" aria-label="Blocker candidates" hidden></ul>
    <div class="depedit-hint">Enter the top match &middot; click &times; to remove &middot; cycles are refused</div>`;
}

/**
 * Render the candidate dropdown for the add-blocker input.
 *
 * F74: when `walkable` reports a candidate id has its own open-blocker chain, a
 * tiny "walk chain" button (data-dep-walk) is appended so you can preview what
 * a prospective blocker would chain into BEFORE adding it. The set is computed
 * by the caller (via deps.hasWalkableChain) and passed in, keeping this render
 * pure. Candidates with no further chain carry no button.
 */
export function renderDepCandidates(
  candidates: DepGraphTask[],
  activeIndex: number,
  walkable?: ReadonlySet<number>,
): string {
  if (candidates.length === 0) return "";
  return candidates
    .map((t, i) => {
      const active = i === activeIndex ? " is-active" : "";
      const doneCls = t.done ? " is-done" : "";
      const walk =
        walkable && walkable.has(t.id)
          ? `<button type="button" class="depedit-ac-walk" data-dep-walk="${t.id}" title="Preview this blocker's chain" aria-label="Preview chain from #${t.id}">&#8627;</button>`
          : "";
      return `<li class="depedit-ac-item${active}${doneCls}" role="option" aria-selected="${i === activeIndex}" data-dep-cand="${t.id}">
        <span class="depedit-ac-label">${escapeHTML(depChipLabel(t))}</span>
        ${walk}
      </li>`;
    })
    .join("");
}
