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
  /**
   * F104: an optional render-pipeline lens kind (blocked / overdue / today /
   * week / nodue) captured alongside the serializable filter. A lens is NOT a
   * ViewFilter facet — it's time-relative / cross-task, so it can't live inside
   * `filter` — but the lens+facet COMBO (F81/F97) is a recurring drill worth
   * naming. Stored as a plain string (validated against LENS_ORDER on the
   * main.ts side at recall time, via parseLens) so views.ts stays decoupled
   * from the lens module. Absent for a plain filter-only view (the common case),
   * keeping older stored views byte-identical.
   */
  lens?: string;
  /**
   * F133: an optional CHOKEPOINT source id captured for a "cohort view" — a
   * saved bookmark of "the tasks waiting on #N". A cohort's id-set is a momentary
   * snapshot (meaningless once those tasks complete), so it is NOT serialized;
   * only the stable chokepoint id is, and the id-set is RE-DERIVED on recall via
   * cohort.buildCohort against the live graph (exactly how setCohort works). A
   * cohort view has an EMPTY filter and NO lens — it's a third kind of saved view
   * beside filter views and pure-lens views (F110). Stored as a positive integer;
   * absent for the common filter / lens views, keeping older stored views
   * byte-identical. The main.ts recall path re-runs the cohort live (degrading
   * gracefully to "nothing waits on #N" if the chokepoint has since cleared).
   */
  cohort?: number;
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
    const view: SavedView = {
      id: typeof o.id === "string" && o.id !== "" ? o.id : makeId(),
      name: o.name.trim(),
      filter: normalizeFilter({
        query: typeof f.query === "string" ? f.query : "",
        priorities: Array.isArray(f.priorities) ? (f.priorities.filter(isPriority) as Priority[]) : [],
        tags: Array.isArray(f.tags) ? f.tags.filter((t): t is string => typeof t === "string") : [],
        hideDone: f.hideDone === true,
      }),
    };
    // F104: carry a captured lens kind through round-trips. Kept as a plain
    // non-empty string here (the main.ts recall path validates it against the
    // real LENS_ORDER via parseLens, so a stale/garbage kind degrades to "no
    // lens" rather than wedging the board). A non-string/empty lens is dropped.
    if (typeof o.lens === "string" && o.lens !== "") view.lens = o.lens;
    // F133: carry a captured chokepoint id for a cohort view. A cohort view is a
    // re-derivable bookmark ("tasks waiting on #N"), so only the stable source id
    // is stored; the id-set is rebuilt on recall. Accept a positive integer only
    // (a non-number / non-positive value is dropped, degrading to a plain view).
    if (typeof o.cohort === "number" && Number.isInteger(o.cohort) && o.cohort > 0) {
      view.cohort = o.cohort;
    }
    out.push(view);
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
 *
 * F104: an optional `lens` captures a render-pipeline lens kind alongside the
 * filter (the lens+facet combo). When a lens is supplied the view is savable
 * even if the plain filter is otherwise empty (a pure-lens view like "blocked"
 * is meaningful), so the empty-filter rejection only applies when there's ALSO
 * no lens. A captured lens is stored on the view; omitting it (or passing
 * undefined) keeps the filter-only behaviour byte-identical.
 */
export function addView(views: SavedView[], name: string, filter: ViewFilter, lens?: string): SavedView[] {
  const trimmed = name.trim();
  if (trimmed === "" || (filterIsEmpty(filter) && !lens)) return views;
  const norm = normalizeFilter(filter);
  const existing = views.find((v) => v.name.toLowerCase() === trimmed.toLowerCase());
  if (existing) {
    return views.map((v) =>
      v.id === existing.id ? withLens({ ...v, name: trimmed, filter: norm }, lens) : v,
    );
  }
  return [...views, withLens({ id: makeId(), name: trimmed, filter: norm }, lens)];
}

/** F104: attach (or strip) the optional lens on a view, keeping it absent when none. */
function withLens(view: SavedView, lens?: string): SavedView {
  if (lens) return { ...view, lens };
  const { lens: _drop, ...rest } = view;
  return rest;
}

/** Remove a view by id. Returns a NEW array. */
export function removeView(views: SavedView[], id: string): SavedView[] {
  return views.filter((v) => v.id !== id);
}

/**
 * Overwrite an existing view's filter with the given one (F32 "update this view
 * to the current filter"). Keeps the view's id + name; only the captured filter
 * changes. An empty filter is rejected (returns the list unchanged) so a view
 * never silently degrades into "match everything". A no-op when the id is
 * unknown. Returns a NEW array.
 *
 * F104: an optional `lens` re-captures the render-pipeline lens alongside the
 * filter, so updating a recalled lens+facet view to the live state keeps the
 * lens half in sync (toggling the lens off updates the view to filter-only).
 * When a lens is supplied the empty-filter rejection is relaxed (a pure-lens
 * view is valid), matching addView. Omitting `lens` strips any stored lens.
 */
export function updateView(views: SavedView[], id: string, filter: ViewFilter, lens?: string): SavedView[] {
  if (filterIsEmpty(filter) && !lens) return views;
  const norm = normalizeFilter(filter);
  return views.map((v) => (v.id === id ? withLens({ ...v, filter: norm }, lens) : v));
}

/**
 * Reorder the views list (F32 drag-to-reorder chips): move `movedId` to sit
 * immediately before `beforeId`. A null/unknown `beforeId` moves it to the end.
 * Dropping a view onto itself, or in a spot that doesn't change the order, is a
 * no-op (returns the SAME array reference so callers can skip a redundant
 * save). Otherwise returns a NEW array. Mirrors the store.Move(before) contract
 * the drag-reorder of tasks already uses.
 */
export function moveView(views: SavedView[], movedId: string, beforeId: string | null): SavedView[] {
  const from = views.findIndex((v) => v.id === movedId);
  if (from < 0 || movedId === beforeId) return views;
  const without = views.filter((v) => v.id !== movedId);
  let insertAt: number;
  if (beforeId === null) {
    insertAt = without.length;
  } else {
    insertAt = without.findIndex((v) => v.id === beforeId);
    if (insertAt < 0) insertAt = without.length;
  }
  const next = [...without.slice(0, insertAt), views[from], ...without.slice(insertAt)];
  // No-op guard: same order means nothing moved.
  if (next.every((v, i) => v.id === views[i].id)) return views;
  return next;
}

/** Find the view whose filter matches the live one, or null. */
export function activeView(views: SavedView[], filter: ViewFilter): SavedView | null {
  if (filterIsEmpty(filter)) return null;
  return views.find((v) => filtersEqual(v.filter, filter)) ?? null;
}

/**
 * F104: does a saved view match the live filter AND lens? The lens-aware sister
 * of activeView: a view that captured a lens is "active" only when the live
 * lens equals the captured one (so "Urgent (overdue)" doesn't light up while
 * you're on the same facet but a DIFFERENT lens, or no lens). A view with no
 * captured lens (`view.lens` absent) requires NO live lens to match, so a plain
 * filter-only view stays distinct from its lensed sibling. `liveLens` is the
 * active lens kind, or null when none. Unlike activeView this also matches a
 * pure-lens view (empty filter + a lens), which is a meaningful saved drill.
 * Pure → unit-tested.
 */
export function viewMatches(view: SavedView, filter: ViewFilter, liveLens: string | null): boolean {
  const lensOk = (view.lens ?? null) === liveLens;
  if (!lensOk) return false;
  // A pure-lens view (empty filter) matches on the lens alone; otherwise the
  // serializable filter must match too.
  if (filterIsEmpty(view.filter)) return filterIsEmpty(filter);
  return filtersEqual(view.filter, filter);
}

/**
 * F104: the lens-aware active view — the view (if any) matching BOTH the live
 * filter and the live lens. Used by main.ts to drive the dup-detection +
 * active-chip highlight so a lens+facet view only reads as "active" when the
 * whole combo is in effect. Returns null when nothing matches. Unlike
 * activeView, an empty plain filter is fine here as long as a lens-bearing view
 * matches the live lens. Pure → unit-tested.
 */
export function activeViewWithLens(
  views: SavedView[],
  filter: ViewFilter,
  liveLens: string | null,
): SavedView | null {
  return views.find((v) => viewMatches(v, filter, liveLens)) ?? null;
}

/**
 * F109: the provenance of the active render-pipeline lens — the NAME of the
 * recalled view that re-applied it, or null when the live lens didn't come from
 * a view. A lens can be set three ways: a digit key, a stat tile, or recalling a
 * lensed view (F104). Only the third has a "source view" to name, so when you
 * recall "Sprint (overdue)" and later wonder "why is the overdue lens on?", this
 * answers it. Returns the recalled view's name ONLY when that view captured a
 * lens AND it equals the live lens (so a digit-key lens, or a lens changed after
 * recall, reports no provenance — the view no longer explains it). Pure →
 * unit-tested; main.ts renders the returned name into a small readout beside the
 * active-lens chip.
 */
export function lensProvenanceNote(recalled: SavedView | null, liveLens: string | null): string | null {
  if (!recalled || liveLens === null) return null;
  if ((recalled.lens ?? null) !== liveLens) return null;
  return recalled.name;
}

/**
 * F110: the default name for a one-click "pin this lens" quick view — the human
 * lens label as-is (e.g. "overdue", "due this week"). Kept tiny + pure so the
 * pin path and the dup-check agree on the name without the caller re-deriving
 * it. main.ts passes lensMeta(kind).label so the saved chip reads exactly like
 * the lens it pins.
 */
export function pureLensViewName(lensLabel: string): string {
  return lensLabel;
}

/**
 * F110: find the existing PURE-lens view for a lens kind — a saved view whose
 * filter is empty and whose captured lens equals `lens`. Returns it (so the pin
 * affordance can read as "already pinned" + recall instead of re-saving) or null
 * when this lens isn't pinned yet. Distinct from activeViewWithLens, which also
 * requires the live FILTER to match: a pin is a pure-lens bookmark, so only the
 * lens + an empty filter define it. Pure → unit-tested.
 */
export function findPureLensView(views: SavedView[], lens: string): SavedView | null {
  return views.find((v) => (v.lens ?? null) === lens && filterIsEmpty(v.filter)) ?? null;
}

/**
 * F112: is this view a PURE-lens bookmark — a captured lens with no serializable
 * filter narrowing it? F110's one-click pin saves exactly this shape (empty
 * filter + a lens), so in the Views chip row such a chip is a "lens bookmark"
 * (recall the overdue lens) rather than a "filter bookmark" (recall #work) or a
 * lens+facet drill (F104's "Urgent (overdue)"). renderViewChips uses it to give
 * pure-lens chips the lens's own glyph so the two read differently at a glance.
 * A view with a lens AND a filter is a drill, not a pure pin, so it returns
 * false. Pure → unit-tested.
 */
export function isPureLensView(view: SavedView): boolean {
  return Boolean(view.lens) && filterIsEmpty(view.filter);
}

/**
 * F133: is this view a COHORT bookmark — a captured chokepoint id with no filter
 * and no lens? F133's panel pin saves exactly this shape (a `cohort` source id,
 * empty filter, no lens), a re-derivable "tasks waiting on #N" bookmark. The
 * Views chip row uses it to give a cohort chip the ↑ glyph + an `is-cohort-pin`
 * class so it reads distinct from filter / lens bookmarks, and the recall path
 * uses it to route through setCohort instead of applying a filter. A view that
 * also carries a filter or a lens is not a pure cohort bookmark (returns false).
 * Pure → unit-tested.
 */
export function isCohortView(view: SavedView): boolean {
  return (
    typeof view.cohort === "number" &&
    view.cohort > 0 &&
    !view.lens &&
    filterIsEmpty(view.filter)
  );
}

/**
 * F133: find the saved COHORT view for a chokepoint id — a cohort bookmark whose
 * `cohort` equals `sourceId`. Returns it (so the panel pin star can read as
 * "already pinned" + recall instead of re-saving) or null when this chokepoint
 * isn't pinned yet. The cohort sibling of findPureLensView (F110). Pure →
 * unit-tested.
 */
export function findCohortView(views: SavedView[], sourceId: number): SavedView | null {
  return views.find((v) => isCohortView(v) && v.cohort === sourceId) ?? null;
}

/**
 * F138: is a cohort bookmark STALE — i.e. its chokepoint no longer has a live
 * cohort? F133's cohort views re-derive their id-set on recall (setCohort), but
 * once the chokepoint completes (or all its waiters finish) recall no-ops with
 * "nothing waits on #N" and the chip just lingers, a dead bookmark with no
 * obvious tell. This answers "would recalling this view land on nothing?" so the
 * Views row can mark such a chip and offer a self-clean.
 *
 * `hasLiveCohort` is injected (the cohort lives in cohort.ts; keeping views.ts
 * decoupled mirrors the lensGlyph / cohortGlyph injection pattern) — main.ts
 * passes a predicate backed by cohort.buildCohort over the live graph. A view
 * that isn't a cohort bookmark is never stale by this measure (returns false);
 * a cohort bookmark is stale exactly when its chokepoint has no live cohort.
 * Pure → unit-tested.
 */
export function isStaleCohortView(
  view: SavedView,
  hasLiveCohort: (sourceId: number) => boolean,
): boolean {
  if (!isCohortView(view)) return false;
  return !hasLiveCohort(view.cohort!);
}

/**
 * F144: the ids of EVERY stale cohort bookmark — the bulk-sweep sister of F138's
 * one-at-a-time self-clean. F138 drops a dead cohort chip only when you recall
 * it; after a big external edit (the CLI / TUI / hand completing a batch of
 * chokepoints) several cohort bookmarks can go dead at once, and clearing them
 * one recall at a time is tedious. This returns the id list of all currently-
 * stale cohort views so a "forget all stale" affordance (a Cmd-K command + a
 * Views-row button) can drop them in one go.
 *
 * `hasLiveCohort` is injected exactly as isStaleCohortView takes it (main.ts
 * backs it with buildCohort over the live graph), so the two can't disagree on
 * what "stale" means — this is just isStaleCohortView mapped over the list. A
 * non-cohort view is never stale, so the result is a subset of the cohort
 * bookmarks. Order follows the views list (stable). Pure → unit-tested.
 */
export function staleCohortViewIds(
  views: SavedView[],
  hasLiveCohort: (sourceId: number) => boolean,
): string[] {
  return views.filter((v) => isStaleCohortView(v, hasLiveCohort)).map((v) => v.id);
}

/**
 * F151: snapshot a set of views about to be removed — the data half of the
 * stale-sweep undo. F144's "forget all stale" drops every dead cohort bookmark
 * in one go; a misfire (a chokepoint that was momentarily unreachable, a hand
 * that fat-fingered the button) would silently lose those bookmarks. This
 * captures the to-be-swept views (a deep-ish copy via the serialize round-trip,
 * so the held snapshot can't be mutated by later edits to the live list) so an
 * undo toast can put them back, mirroring F8's delete-undo.
 *
 * Returns the views whose ids are in `ids`, in the SAME order they appear in
 * `views` (stable), as fresh objects detached from the live list. An empty id
 * set (or no matches) returns []. Pure → unit-tested; main.ts holds the result
 * for the toast window and feeds it to restoreSweptViews on undo.
 */
export function snapshotViews(views: SavedView[], ids: readonly string[]): SavedView[] {
  const want = new Set(ids);
  return parseViews(serializeViews(views.filter((v) => want.has(v.id))));
}

/**
 * F151: restore a snapshot of swept views back into the live list — the action
 * half of the stale-sweep undo. Re-inserts every snapshot view that isn't
 * already present (matched by id), so an undo after F144's bulk sweep puts the
 * forgotten cohort bookmarks back. Idempotent on id: a view already in `current`
 * (e.g. re-pinned between sweep and undo) is left as-is rather than duplicated.
 * Restored views are appended after the current list (their original positions
 * aren't tracked — order within the Views row isn't load-bearing, and appending
 * keeps the undo simple + predictable). Returns a NEW array; an empty snapshot
 * is a no-op (returns the same reference). Pure → unit-tested.
 */
export function restoreSweptViews(current: SavedView[], snapshot: readonly SavedView[]): SavedView[] {
  if (snapshot.length === 0) return current;
  const have = new Set(current.map((v) => v.id));
  const missing = snapshot.filter((v) => !have.has(v.id));
  if (missing.length === 0) return current;
  return [...current, ...missing.map((v) => ({ ...v }))];
}

/**
 * F133: add a COHORT bookmark capturing `sourceId` (or recall semantics via the
 * caller when one already exists — this just keeps the store clean by
 * overwriting any same-name OR same-chokepoint cohort view rather than
 * duplicating). A blank name or a non-positive id is rejected (returns the list
 * unchanged). The saved view has an empty filter, no lens, and the captured
 * cohort id. The cohort sibling of addView's pure-lens path. Returns a NEW array.
 */
export function addCohortView(views: SavedView[], name: string, sourceId: number): SavedView[] {
  const trimmed = name.trim();
  if (trimmed === "" || !Number.isInteger(sourceId) || sourceId <= 0) return views;
  // Overwrite an existing cohort view for the SAME chokepoint (re-pinning #N
  // shouldn't make a second chip) or the same name, whichever matches first.
  const existing =
    findCohortView(views, sourceId) ??
    views.find((v) => v.name.toLowerCase() === trimmed.toLowerCase()) ??
    null;
  const made: SavedView = { id: existing?.id ?? makeId(), name: trimmed, filter: normalizeFilter(emptyViewFilter()), cohort: sourceId };
  if (existing) return views.map((v) => (v.id === existing.id ? made : v));
  return [...views, made];
}

/** F133: an empty ViewFilter — a cohort view carries no facet narrowing. */
function emptyViewFilter(): ViewFilter {
  return { query: "", priorities: [], tags: [], hideDone: false };
}

/**
 * F141: how many of the live `tasks` a saved view currently matches — the count
 * behind the Views-row chip badges, so the row doubles as an at-a-glance triage
 * dashboard ("#work ·12, blocked ·3, waiting on #1 ·5"). A saved view is one of
 * three kinds (filter / pure-lens / cohort), each with its own match semantics,
 * so the per-kind predicates are INJECTED to keep views.ts decoupled from the
 * filter / lens / cohort modules (mirroring the lensGlyph / staleCohort
 * injection pattern):
 *   - cohort view  -> the size of its live id-set (`cohortIds(sourceId)`), since
 *     a cohort isn't a per-task predicate but an explicit id set re-derived from
 *     the graph; an empty/dead cohort counts 0.
 *   - lens-bearing -> tasks passing BOTH the view's serializable filter AND its
 *     lens predicate (`matchesLens(task, lens)`), the lens+facet combo F104
 *     saves; a pure-lens view (empty filter) counts everything the lens passes.
 *   - plain filter -> tasks passing the view's filter (`matchesFilter`).
 * Pure → unit-tested; main.ts supplies the three predicates backed by the real
 * matchesFilter / applyLens / buildCohort over the same live pool the board
 * renders, so a badge can't drift from what a recall would actually show.
 */
export interface ViewMatchCounters<T> {
  /** Does this task pass the view's serializable filter? */
  matchesFilter: (task: T, filter: ViewFilter) => boolean;
  /** Does this task pass the given lens kind? (only called for lens views) */
  matchesLens: (task: T, lens: string) => boolean;
  /** The live id-set for a cohort's chokepoint (only called for cohort views). */
  cohortIds: (sourceId: number) => readonly number[];
}

export function countViewMatches<T>(
  view: SavedView,
  tasks: readonly T[],
  counters: ViewMatchCounters<T>,
): number {
  // A cohort view's "matches" are its live id-set — re-derived, not a predicate.
  if (isCohortView(view)) return counters.cohortIds(view.cohort!).length;
  const lens = view.lens ?? null;
  let n = 0;
  for (const task of tasks) {
    if (!counters.matchesFilter(task, view.filter)) continue;
    if (lens !== null && !counters.matchesLens(task, lens)) continue;
    n++;
  }
  return n;
}

/**
 * F145: the open/done split of a view's live match-count — the breakdown behind
 * F141's badge tooltip ("12 open · 3 done"). F141's badge shows ONE number
 * (countViewMatches), but a hide-done view and a show-all view read very
 * differently at the same total, and the badge alone can't say why. This splits
 * the SAME matched set into open vs done so the tooltip can explain the number.
 *
 * The split honours the view's own filter exactly as countViewMatches does, so a
 * hide-done view (its filter excludes completed tasks) naturally reports done=0 —
 * making "the badge respects hideDone" visible rather than implied. A show-all
 * view reports both halves. A cohort view's matches are its live id-set, which
 * is by construction the UNDONE dependents of a chokepoint (buildCohort →
 * openDependents), so a cohort is always {open: N, done: 0} without needing the
 * tasks back. `isDone` is injected (the done-state lives on the caller's concrete
 * task), mirroring the ViewMatchCounters injection so views.ts stays decoupled.
 * Pure → unit-tested; main.ts supplies isDone backed by the real task.done flag
 * over the same live pool countViewMatches reads, so the breakdown can't drift
 * from the badge number (open + done === the badge for a show-all view).
 */
export interface ViewMatchBreakdown {
  open: number;
  done: number;
}

export function countViewMatchesBreakdown<T>(
  view: SavedView,
  tasks: readonly T[],
  counters: ViewMatchCounters<T>,
  isDone: (task: T) => boolean,
): ViewMatchBreakdown {
  // A cohort's matches are its undone dependents — all open by construction.
  if (isCohortView(view)) return { open: counters.cohortIds(view.cohort!).length, done: 0 };
  const lens = view.lens ?? null;
  let open = 0;
  let done = 0;
  for (const task of tasks) {
    if (!counters.matchesFilter(task, view.filter)) continue;
    if (lens !== null && !counters.matchesLens(task, lens)) continue;
    if (isDone(task)) done++;
    else open++;
  }
  return { open, done };
}

/**
 * F145: render the human breakdown text for a view's match badge tooltip —
 * "12 open · 3 done", or just "12 open" when nothing's done (e.g. a hide-done
 * view, where done is filtered out by construction). An all-done view reads
 * "0 open · 3 done" so a fully-completed bucket is still legible. A view that
 * matches nothing returns "no matches" so the tooltip never reads a bare "0".
 * Kept pure + tiny so the badge tooltip can't drift from countViewMatchesBreakdown.
 */
export function describeViewMatchBreakdown(b: ViewMatchBreakdown): string {
  if (b.open === 0 && b.done === 0) return "no matches";
  if (b.done === 0) return `${b.open} open`;
  return `${b.open} open \u00b7 ${b.done} done`;
}

/**
 * F142: the id of the BUSIEST saved view — the one matching the most live tasks
 * (the densest bucket in F141's per-chip badges), so the Views row can mark it
 * and the user's eye jumps straight to where the work piled up. Reuses the SAME
 * match-count resolver F141's badge renders from (countViewMatches over the live
 * board) so the marked chip can't disagree with its own badge number.
 *
 * Returns the unique densest view's id, or null when there's no clear winner: an
 * empty list, every view matching nothing (max count 0 — nothing's "busy"), OR a
 * TIE for the top (two views both densest is ambiguous — better to mark neither
 * than mislead by picking one arbitrarily). A null/undefined count from the
 * resolver (a view whose count isn't meaningful) is treated as not-busy. Pure →
 * unit-tested; main.ts passes the resolver + sets a `busiestId` render opt.
 */
export function busiestViewId(
  views: SavedView[],
  count: (view: SavedView) => number | null | undefined,
): string | null {
  let bestId: string | null = null;
  let bestCount = 0;
  let tied = false;
  for (const v of views) {
    const c = count(v);
    if (typeof c !== "number" || c <= 0) continue;
    if (c > bestCount) {
      bestCount = c;
      bestId = v.id;
      tied = false;
    } else if (c === bestCount) {
      tied = true;
    }
  }
  return tied ? null : bestId;
}

/**
 * F148: the at-a-glance coverage summary for the Views row head — "N views · M
 * tasks". With F141's per-chip counts + F142's busiest marker live, the row is a
 * triage dashboard; this adds a tiny leading readout that sums the whole row so
 * you see the totals without eyeballing every chip. The number of distinct saved
 * views (the chip count) plus the total matched tasks across them.
 *
 * The per-view count is the SAME resolver F141's badge + F142's marker read
 * (injected, backed by countViewMatches over the live board), so the summary
 * can't disagree with the chips it sits above. Tasks matched by multiple views
 * ARE counted once per view (the sum is "total chip coverage", not a de-duped
 * unique-task count) — that matches what the per-chip badges already show, so
 * summing them is consistent; a null/undefined/negative count contributes 0.
 * Returns { views, tasks }. An empty list is { views: 0, tasks: 0 }. Pure →
 * unit-tested; describeViewsRowSummary turns it into the readout text.
 */
export interface ViewsRowSummary {
  views: number;
  tasks: number;
}

export function viewsRowSummary(
  views: SavedView[],
  count: (view: SavedView) => number | null | undefined,
): ViewsRowSummary {
  let tasks = 0;
  for (const v of views) {
    const c = count(v);
    if (typeof c === "number" && c > 0) tasks += c;
  }
  return { views: views.length, tasks };
}

/**
 * F148: render the Views-row summary as "N views · M tasks" (singularizing both
 * nouns). With no views it returns "" so the readout stays hidden (the row is
 * itself hidden then anyway). When there ARE views but they currently match
 * nothing, the task half is dropped ("3 views") rather than reading "· 0 tasks",
 * keeping the readout quiet on an all-empty board. Pure + tiny so it can't drift
 * from viewsRowSummary.
 *
 * F154: when a `busiest` {name, count} is supplied (the F142 densest winner,
 * which busiestViewId resolves to null on a tie / empty board so there's never
 * an ambiguous headline), the readout gains a trailing "· busiest: <name> (K)"
 * so the row head doubles as the triage headline — "where's the pile-up?" is
 * answered without scanning the chips. Omitted / null busiest (no clear winner)
 * keeps the bare "N views · M tasks", so existing callers stay byte-identical.
 * The busiest clause is only appended when the summary already has a task half
 * (tasks > 0) — on an all-empty board there's no pile-up to name.
 */
export interface BusiestViewLabel {
  name: string;
  count: number;
}

export function describeViewsRowSummary(s: ViewsRowSummary, busiest?: BusiestViewLabel | null): string {
  if (s.views === 0) return "";
  const v = `${s.views} view${s.views === 1 ? "" : "s"}`;
  if (s.tasks === 0) return v;
  const base = `${v} \u00b7 ${s.tasks} task${s.tasks === 1 ? "" : "s"}`;
  if (busiest && busiest.name !== "") {
    return `${base} \u00b7 busiest: ${busiest.name} (${busiest.count})`;
  }
  return base;
}

/**
 * F163: append a quiet "· N stale" segment to the Views-row summary when stale
 * cohort bookmarks exist. F138 marks stale cohort chips (their chokepoint
 * cleared) and F144 sweeps them; this surfaces the sweep NEED at the row head so
 * it's legible without scanning for greyed chips — pairs with F144's sweep
 * button. A zero/negative/omitted stale count returns the summary unchanged, so
 * existing callers stay byte-identical and a clean board reads plainly. Empty
 * summary (no views) gains nothing. Pure → unit-tested; composes after
 * describeViewsRowSummary so it tacks onto whatever headline already rendered.
 */
export function appendStaleSegment(summary: string, staleCount: number): string {
  if (summary === "" || staleCount <= 0) return summary;
  return `${summary} \u00b7 ${staleCount} stale`;
}

/**
 * F146: the preview text for a "Peek view (<name>)" command — a view's live
 * match-count + a compact description of what it filters, shown in the palette's
 * preview slot WITHOUT recalling it, so you can compare saved views before
 * committing to one. The per-view recall commands (F25) jump straight in; this
 * lets you look first.
 *
 * Combines the live `count` (the SAME countViewMatches the F141 badge reads,
 * passed in so views.ts stays decoupled) with describeView's facet summary into
 * one line: "12 tasks · tags: #work" (or "no matches · lens: overdue" when the
 * count is 0, so an empty bucket reads honestly). A null/undefined count drops
 * the count half ("tags: #work") — used when a count isn't meaningful. The
 * description half is always present (describeView never returns ""). Pure →
 * unit-tested; main.ts renders the returned text into the preview slot via the
 * shared `.due-preview` style.
 */
export function peekViewLabel(view: SavedView, count: number | null | undefined): string {
  const desc = describeView(view);
  if (typeof count !== "number") return desc;
  const countText =
    count === 0 ? "no matches" : `${count} task${count === 1 ? "" : "s"}`;
  return `${countText} \u00b7 ${desc}`;
}

/**
 * F157: the title for a "Peek view (<name>)" command, with the live match-count
 * folded in as a quiet trailing "·N" — so scanning the palette list shows every
 * view's pile-up depth without highlighting each one to read F146's preview
 * slot. F146 reveals the count + facets in the preview on highlight; this puts
 * the bare count on the COMMAND TITLE itself, turning the whole Views group into
 * an at-a-glance count column.
 *
 * A null/undefined count (count not meaningful yet) keeps the plain
 * "Peek view (<name>)", so callers without a live count stay byte-identical. A
 * zero count still renders "·0" — an empty view reads honestly rather than
 * hiding. Pure → unit-tested; main.ts builds the peek commands' titles from it.
 */
export function peekCommandTitle(name: string, count: number | null | undefined): string {
  const base = `Peek view (${name})`;
  return typeof count === "number" ? `${base} \u00b7${count}` : base;
}

/**
 * F124: is a chip horizontally clipped by its (overflow-scrolling) container —
 * i.e. would the just-flashed pin (F119) be off-screen when the highlight plays?
 * F119 flashes the freshly-pinned chip, but when the Views row has overflowed
 * horizontally the new chip can sit past the visible edge, so the spatial "it
 * landed here" confirmation is invisible. main.ts uses this to decide whether to
 * scrollIntoView the flashed chip before the animation runs.
 *
 * Pure geometry over two bounding rects (chip + container): true when the chip's
 * left edge is before the container's left, OR its right edge is past the
 * container's right (a small 1px epsilon absorbs sub-pixel rounding so a chip
 * flush with the edge isn't treated as clipped). Works on plain rect-shaped
 * objects so it's unit-testable with zero DOM. A chip fully inside the viewport
 * returns false (no scroll needed). Only the horizontal axis matters — the Views
 * row is a single horizontally-scrolling line.
 */
export interface RectLike {
  left: number;
  right: number;
}

export function chipClippedX(chip: RectLike, container: RectLike, epsilon = 1): boolean {
  return chip.left < container.left - epsilon || chip.right > container.right + epsilon;
}

/**
 * F129/F134: the one-shot exit-animation duration (ms) for a chip LEAVING the
 * Views row. F129 introduced it for the lens unpin (which removed the chip
 * instantly, no spatial feedback); F134 made it the duration for EVERY chip
 * removal (chip × delete, lens unpin, cohort unpin) routed through the shared
 * `animateChipExitThenRemove` helper. The inverse of F119's pin-flash: a brief
 * fade-out so the user sees WHICH chip left before it's removed. Exported as the
 * single source of truth so main.ts's deferred-removeView timer and the CSS
 * `.is-leaving` animation can't drift. Kept here (not main.ts) so it's importable
 * into tests without a DOM.
 */
export const UNPIN_EXIT_MS = 240;

/**
 * F129: should main.ts animate a chip's exit before removing its view? True only
 * when there's a real chip element that supports the class toggle — so the
 * jsdom-less/test env (chip is null, or a bare object with no classList), or a
 * chip that isn't currently in the Views row, falls through to a synchronous
 * remove and behaviour stays unchanged. Pure over a minimal element shape so the
 * decision is unit-testable with zero DOM, mirroring F124's chipClippedX seam.
 */
export function canAnimateChipExit(chip: { classList?: { add?: unknown } } | null): boolean {
  return Boolean(chip && chip.classList && typeof chip.classList.add === "function");
}

/** Escape strings before injecting into innerHTML. Local copy keeps this dependency-free. */
function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}

/** A compact human description of what a view filters, for the chip tooltip.
 *
 * F104: when the view captured a lens, prefix the description with the lens
 * kind ("lens: overdue") so the tooltip reveals the WHOLE drill, not just the
 * serializable facet half. The lens label is the raw kind string (main.ts could
 * map it to a prettier label, but the kind is already human-legible). */
export function describeView(v: SavedView): string {
  const parts: string[] = [];
  // F133: a cohort bookmark describes itself by its chokepoint, since its filter
  // is empty (the id-set is re-derived on recall, not stored).
  if (typeof v.cohort === "number" && v.cohort > 0) parts.push(`waiting on #${v.cohort}`);
  if (v.lens) parts.push(`lens: ${v.lens}`);
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
 *
 * F32 options:
 *   - draggable: mark chips draggable + carry `data-view-drag` so the row can
 *     reorder them by dragging.
 *   - updatableId: the one view whose filter has diverged from the live filter
 *     (you recalled it, then tweaked) — that chip shows a `data-view-update`
 *     button to overwrite the saved view with the current filter.
 *
 * F104:
 *   - liveLens: the active render-pipeline lens kind (or null). When provided,
 *     the active-chip highlight uses the lens-aware viewMatches so a lens+facet
 *     view only lights up when BOTH its facet and its lens are in effect.
 *     Omitting it (undefined) falls back to the F32 filter-only match so
 *     existing callers/tests are unaffected.
 *   - a view that captured a lens wears a small "lens" marker class so it's
 *     visually distinct from a plain filter view.
 *
 * F112:
 *   - lensGlyph: an optional resolver (lens kind string -> a leading glyph) so a
 *     PURE-lens bookmark (empty filter + a lens, F110's one-click pin) wears the
 *     lens's OWN glyph and an `is-lens-pin` class — reading as a "lens bookmark"
 *     vs a "filter bookmark" at a glance. main.ts supplies it (parseLens +
 *     lensMeta(kind).glyph) so views.ts stays decoupled from the lens module; a
 *     resolver that returns "" (unknown/garbage lens) degrades to no glyph. Only
 *     pure-lens views get the glyph; a lens+facet drill stays a normal chip.
 *
 * F119:
 *   - flashId: the id of a view to flash once (a one-shot `is-flash` highlight
 *     class). F110/F115 pin a lens as a new pure-lens view but the only feedback
 *     is a status line — you can't see WHERE the pin landed in the row. Tagging
 *     the freshly-created chip with `is-flash` lets main.ts run a brief CSS
 *     highlight so the spatial "it went here" confirmation the status line can't
 *     give is obvious. main.ts clears the slot on the next render so the flash is
 *     a one-shot. An unknown/null id flashes nothing.
 */
export interface ViewChipOpts {
  draggable?: boolean;
  updatableId?: string | null;
  liveLens?: string | null;
  /** F112: lens kind -> leading glyph for pure-lens chips ("" = no glyph). */
  lensGlyph?: (lens: string) => string;
  /** F119: id of a view to flash once with an `is-flash` highlight class. */
  flashId?: string | null;
  /**
   * F133: the chokepoint id of the currently-focused cohort, or null. A cohort
   * chip (F133's panel pin) has an EMPTY filter, so the F32/F104 filter-equality
   * highlight would light EVERY cohort chip on an empty-filter board. Instead a
   * cohort chip is "active" only when `activeCohort` equals its captured cohort
   * id — so exactly the pinned chokepoint you're focused on lights up. Omitting
   * it leaves cohort chips un-highlighted (the safe default).
   */
  activeCohort?: number | null;
  /** F133: a fixed leading glyph for cohort chips (the ↑ chokepoint marker). */
  cohortGlyph?: string;
  /**
   * F138: a predicate marking a cohort chip STALE — its chokepoint no longer has
   * a live cohort, so recalling it would land on nothing. When supplied, a cohort
   * chip whose chokepoint is dead gets an `is-stale-cohort` class + a tooltip
   * note ("— stale, recall to clear") so the dead bookmark is visible and a recall
   * can self-clean it (main.ts). Backed by isStaleCohortView with an injected
   * live-cohort check so views.ts stays decoupled from the graph. Omitting it
   * leaves every chip un-marked (the safe default), keeping existing callers
   * byte-identical.
   */
  staleCohort?: (view: SavedView) => boolean;
  /**
   * F141: a resolver giving the live match-count for a view — when supplied,
   * each chip shows a quiet "·N" badge so the Views row doubles as an at-a-glance
   * triage dashboard. Returns the count (countViewMatches over the live board) or
   * null/undefined to suppress the badge for that chip (e.g. a count that isn't
   * meaningful). Opt-in via this resolver so existing callers/tests stay
   * byte-identical when it's omitted; main.ts supplies it backed by the real
   * filter / lens / cohort predicates.
   */
  matchCount?: (view: SavedView) => number | null | undefined;
  /**
   * F145: a resolver giving the open/done breakdown TEXT for a view's count
   * badge tooltip ("12 open · 3 done"). When supplied alongside matchCount, the
   * badge's title/aria reads the richer breakdown instead of the bare "·N
   * matching tasks", so a hide-done view (done filtered out) reads "12 open" and
   * a show-all view reads the split — making the badge respect hideDone visible.
   * Returns null/undefined (or omitted) to keep the F141 plain count tooltip, so
   * existing callers stay byte-identical. main.ts backs it with
   * describeViewMatchBreakdown(countViewMatchesBreakdown(...)).
   */
  matchTitle?: (view: SavedView) => string | null | undefined;
  /**
   * F152: a resolver flagging whether a view's count badge should be an
   * actionable "hide done" affordance (true) rather than a plain readout. When
   * it returns true for a view, that chip's badge renders as a clickable
   * `<button data-view-hide-done>` (recall + flip hideDone on click) instead of
   * the inert `<span>`, and its tooltip gains a "click to hide done" hint.
   * Backed by badgeHidesDone over the same open/done breakdown matchTitle reads,
   * so a badge is only actionable when it actually advertises done tasks to
   * hide. Omitted/false (or no matchCount) keeps the F141/F145 plain span, so
   * existing callers stay byte-identical.
   */
  hideDoneBadge?: (view: SavedView) => boolean;
  /**
   * F142: the id of the single BUSIEST view — the chip matching the most live
   * tasks (busiestViewId over the same match-count resolver F141's badge reads).
   * When supplied and a chip's id equals it, that chip gets an `is-busiest`
   * class so the densest bucket pops at a glance — the Views row's at-a-glance
   * triage marker. Omitting it (or null) marks nothing, keeping existing callers
   * byte-identical. A tie or an all-empty board resolves to null (no winner), so
   * the marker only appears when there's an unambiguous busiest view.
   */
  busiestId?: string | null;
}

/**
 * F152: should the F145 "·N" count badge act as a "hide these done ones"
 * affordance? On a SHOW-ALL view (its saved filter keeps done tasks) whose live
 * match set actually contains completed tasks, clicking the badge recalls the
 * view AND flips hideDone on — so the done count it advertises ("12 open · 3
 * done") becomes an actionable "land on just the open slice" jump. On a view
 * that already hides done, or matches no done tasks, there's nothing to hide so
 * the badge stays a plain readout.
 *
 * A COHORT bookmark (empty filter, a captured chokepoint) has no hideDone facet
 * of its own — it's recalled by re-focusing its chokepoint, not by applying a
 * filter — so the action never applies to it (returns false), keeping the badge
 * inert there. The breakdown is the SAME open/done split countViewMatchesBreakdown
 * gives the F145 tooltip, so the actionable state can't disagree with the number
 * the badge shows. Pure → unit-tested; main.ts opts the badge into a clickable
 * button only when this returns true, and the click runs recall + hideDone.
 */
export function badgeHidesDone(view: SavedView, breakdown: ViewMatchBreakdown): boolean {
  if (isCohortView(view)) return false; // no hideDone facet to flip
  if (view.filter.hideDone) return false; // already hiding done — nothing to do
  return breakdown.done > 0; // only actionable when there ARE done tasks to hide
}

/**
 * F158: the title for a per-view "Recall <name> (open only)" command — the
 * keyboard-reachable sibling of F152's actionable count badge. F152 lets the
 * mouse click a "·N" badge to recall a show-all view AND hide its done slice in
 * one go; this gives that same recall+hideDone jump a Cmd-K entry so the
 * keyboard reaches it too. Built only for views badgeHidesDone flags (non-cohort,
 * show-all, with live done tasks to hide), so the command appears exactly where
 * the badge would be actionable. Pure → unit-tested; main.ts maps the command id
 * "recall-open:<id>" through the same recallViewHideDone path the badge click uses.
 */
export function recallOpenOnlyTitle(name: string): string {
  return `Recall ${name} (open only)`;
}

export function renderViewChips(
  views: SavedView[],
  filter: ViewFilter,
  opts: ViewChipOpts = {},
): string {
  if (views.length === 0) return "";
  const dragAttrs = opts.draggable ? ` draggable="true" data-view-drag` : "";
  // F104: when the caller passes liveLens (even null), use the lens-aware match
  // so a lensed view's highlight reflects the whole combo; otherwise keep the
  // F32 filter-only equality.
  const lensAware = opts.liveLens !== undefined;
  return views
    .map((v) => {
      // F133: a cohort bookmark (empty filter, no lens, a captured chokepoint)
      // is "active" only when its chokepoint is the live focus — NOT via filter
      // equality (its empty filter would match every empty-filter board). For
      // non-cohort views the F104/F32 match below decides.
      const cohortPin = isCohortView(v);
      const isActive = cohortPin
        ? opts.activeCohort != null && v.cohort === opts.activeCohort
        : lensAware
          ? viewMatches(v, filter, opts.liveLens ?? null)
          : filtersEqual(v.filter, filter);
      const active = isActive ? " is-active" : "";
      const lensed = v.lens ? " is-lensed" : "";
      // F119: flash a freshly-pinned chip once so the user sees WHERE the pin
      // landed in the row. main.ts sets flashId after a pin and clears it on the
      // next render, so the class rides exactly one paint.
      const flash = opts.flashId && opts.flashId === v.id ? " is-flash" : "";
      // F112: a pure-lens bookmark (lens + empty filter) wears the lens's own
      // glyph + an is-lens-pin class so it reads as a "lens bookmark" distinct
      // from a filter bookmark. A lens+facet drill keeps the plain chip.
      // F133: a cohort bookmark wears a fixed ↑ chokepoint glyph + an
      // is-cohort-pin class so it reads distinct from both filter and lens chips.
      const pin = isPureLensView(v);
      let glyph = pin && opts.lensGlyph ? opts.lensGlyph(v.lens!) : "";
      let pinClass = pin ? " is-lens-pin" : "";
      if (cohortPin) {
        glyph = opts.cohortGlyph ?? "";
        pinClass = " is-cohort-pin";
      }
      // F138: mark a stale cohort chip (its chokepoint has no live cohort) so the
      // dead bookmark is visible + a recall can self-clean it. Only consulted for
      // cohort chips (the predicate already short-circuits non-cohort views).
      const stale = cohortPin && opts.staleCohort && opts.staleCohort(v) ? " is-stale-cohort" : "";
      const staleTitle = stale ? " \u2014 stale, recall to clear" : "";
      const glyphSpan = glyph
        ? `<span class="view-chip-lens-glyph" aria-hidden="true">${escapeHTML(glyph)}</span> `
        : "";
      const update =
        opts.updatableId && opts.updatableId === v.id
          ? `<button type="button" class="view-chip-update" data-view-update="${escapeHTML(v.id)}" title="Update “${escapeHTML(v.name)}” to the current filter" aria-label="Update view ${escapeHTML(v.name)} to current filter">&#8635;</button>`
          : "";
      // F141: a quiet "·N" match-count badge so the Views row reads as a triage
      // dashboard. Opt-in via the matchCount resolver; a null/undefined count (or
      // no resolver) renders nothing, keeping omitting-callers byte-identical.
      // F145: when a matchTitle resolver is supplied, the badge's title/aria
      // reads the richer open/done breakdown ("12 open · 3 done") instead of the
      // bare "·N matching tasks", so the badge respects hideDone visibly. A
      // null/undefined breakdown (or no resolver) falls back to the F141 text.
      const count = opts.matchCount ? opts.matchCount(v) : undefined;
      const breakdown = opts.matchTitle ? opts.matchTitle(v) : undefined;
      // F152: a show-all view whose match set holds done tasks turns its badge
      // into a "hide these done ones" button (recall + flip hideDone). The
      // resolver (badgeHidesDone) only flags non-cohort, show-all views with a
      // non-zero done count, so the action is offered exactly where it does
      // something. The tooltip gains a hint and the badge becomes a <button>.
      const actionable =
        typeof count === "number" && opts.hideDoneBadge ? opts.hideDoneBadge(v) : false;
      const baseTitle =
        typeof breakdown === "string" && breakdown !== ""
          ? breakdown
          : `${count} matching ${count === 1 ? "task" : "tasks"}`;
      const badgeTitle = actionable ? `${baseTitle} \u2014 click to hide done` : baseTitle;
      const badge =
        typeof count === "number"
          ? actionable
            ? `<button type="button" class="view-chip-count is-actionable" data-view-hide-done="${escapeHTML(v.id)}" aria-label="${escapeHTML(badgeTitle)}" title="${escapeHTML(badgeTitle)}">&middot;${count}</button>`
            : `<span class="view-chip-count" aria-label="${escapeHTML(badgeTitle)}" title="${escapeHTML(badgeTitle)}">&middot;${count}</span>`
          : "";
      // F142: mark the single busiest chip (the densest live bucket) so the eye
      // jumps to where the work piled up. Only the unambiguous winner gets it
      // (busiestViewId resolves a tie / empty board to null), so at most one chip
      // wears is-busiest; omitting busiestId marks nothing.
      const busiest = opts.busiestId && opts.busiestId === v.id ? " is-busiest" : "";
      return `<span class="view-chip${active}${lensed}${pinClass}${stale}${flash}${busiest}"${dragAttrs} data-view-id="${escapeHTML(v.id)}" title="${escapeHTML(describeView(v) + staleTitle)}"><button type="button" class="view-chip-name" data-view-recall="${escapeHTML(v.id)}">${glyphSpan}${escapeHTML(v.name)}</button>${badge}${update}<button type="button" class="view-chip-del" data-view-del="${escapeHTML(v.id)}" aria-label="Delete view ${escapeHTML(v.name)}">&times;</button></span>`;
    })
    .join("");
}
