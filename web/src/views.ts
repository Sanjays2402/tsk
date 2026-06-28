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
      const glyphSpan = glyph
        ? `<span class="view-chip-lens-glyph" aria-hidden="true">${escapeHTML(glyph)}</span> `
        : "";
      const update =
        opts.updatableId && opts.updatableId === v.id
          ? `<button type="button" class="view-chip-update" data-view-update="${escapeHTML(v.id)}" title="Update “${escapeHTML(v.name)}” to the current filter" aria-label="Update view ${escapeHTML(v.name)} to current filter">&#8635;</button>`
          : "";
      return `<span class="view-chip${active}${lensed}${pinClass}${flash}"${dragAttrs} data-view-id="${escapeHTML(v.id)}" title="${escapeHTML(describeView(v))}"><button type="button" class="view-chip-name" data-view-recall="${escapeHTML(v.id)}">${glyphSpan}${escapeHTML(v.name)}</button>${update}<button type="button" class="view-chip-del" data-view-del="${escapeHTML(v.id)}" aria-label="Delete view ${escapeHTML(v.name)}">&times;</button></span>`;
    })
    .join("");
}
