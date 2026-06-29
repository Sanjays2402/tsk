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

/**
 * F170: export the saved-view row as a PORTABLE document so the Views row can
 * travel between machines/browsers. Unlike serializeViews (the raw localStorage
 * array), this wraps the list in a tiny versioned envelope — {tsk:"tsk.views",
 * v:1, views:[...]} — so an importer can sniff the format and reject a
 * stranger's JSON blob. Pretty-printed (2-space) so a human can eyeball/diff it.
 * Pure -> unit-tested; main.ts hangs it off a "copy views" affordance.
 */
export const VIEWS_DOC_KIND = "tsk.views";
export const VIEWS_DOC_VERSION = 1;

export function exportViewsDoc(views: SavedView[]): string {
  return JSON.stringify({ tsk: VIEWS_DOC_KIND, v: VIEWS_DOC_VERSION, views }, null, 2);
}

/**
 * F196: export a SINGLE saved view as a portable document — the per-chip sister
 * of F181's whole-row exportViewsDoc, so you can copy just one bookmark to share
 * (a teammate wants your "#work overdue" view, not your entire row). Wraps the
 * one view in the SAME {tsk:"tsk.views",v:1,views:[v]} envelope exportViewsDoc
 * uses, so it round-trips through importViewsDoc / previewImportViews exactly
 * like a full-row paste (de-dup by name, fresh id on merge). Pretty-printed so a
 * human can eyeball it. Pure → unit-tested; main.ts hangs it off a per-chip copy
 * button.
 */
export function exportSingleViewDoc(view: SavedView): string {
  return JSON.stringify({ tsk: VIEWS_DOC_KIND, v: VIEWS_DOC_VERSION, views: [view] }, null, 2);
}

/**
 * F170: import a portable views document (exportViewsDoc's output) and MERGE it
 * into a current list, so pasting a row from another machine adds its bookmarks
 * without clobbering yours. De-dups by case-insensitive name: an incoming view
 * whose name already exists is dropped (yours wins -- import never silently
 * overwrites). Each genuinely-new view is normalized through parseViews (so its
 * filter/lens/cohort are validated exactly like a stored view), gets a FRESH id
 * (timestamp+random) so two machines' ids can't collide, and appends after the
 * current row. A non-views document, malformed JSON, or wrong kind/version
 * returns the current list UNCHANGED (no throw). Returns a NEW array. Pure ->
 * unit-tested; main.ts feeds it the clipboard text.
 */
export function importViewsDoc(current: SavedView[], doc: string): SavedView[] {
  let parsed: unknown;
  try {
    parsed = JSON.parse(doc);
  } catch {
    return current;
  }
  if (!parsed || typeof parsed !== "object") return current;
  const o = parsed as { tsk?: unknown; v?: unknown; views?: unknown };
  if (o.tsk !== VIEWS_DOC_KIND || o.v !== VIEWS_DOC_VERSION || !Array.isArray(o.views)) {
    return current;
  }
  const incoming = parseViews(JSON.stringify(o.views));
  const have = new Set(current.map((v) => v.name.toLowerCase()));
  const out = [...current];
  for (const v of incoming) {
    if (have.has(v.name.toLowerCase())) continue;
    have.add(v.name.toLowerCase());
    out.push({ ...v, id: makeId() });
  }
  return out;
}

/**
 * F199: import a portable views document like importViewsDoc, but INSERT the
 * genuinely-new views right AFTER a target chip instead of appending at the end
 * — the position-aware sister of importViewsDoc, so a single-view "paste after"
 * lands the bookmark beside the chip you aimed at (not orphaned at the tail of a
 * long row). De-dup, fresh-id, and garbage-rejection are byte-identical to
 * importViewsDoc (yours wins on a name collision; a non-views/malformed doc
 * returns the current list UNCHANGED). The ONLY difference is placement: the new
 * views slot in immediately after the view whose id === `afterId`. A null or
 * unknown `afterId` falls back to appending at the end, so importViewsDocAfter
 * with no anchor === importViewsDoc. Returns a NEW array. Pure -> unit-tested;
 * main.ts feeds it the clipboard text + the anchor chip's id.
 */
export function importViewsDocAfter(
  current: SavedView[],
  doc: string,
  afterId: string | null,
): SavedView[] {
  let parsed: unknown;
  try {
    parsed = JSON.parse(doc);
  } catch {
    return current;
  }
  if (!parsed || typeof parsed !== "object") return current;
  const o = parsed as { tsk?: unknown; v?: unknown; views?: unknown };
  if (o.tsk !== VIEWS_DOC_KIND || o.v !== VIEWS_DOC_VERSION || !Array.isArray(o.views)) {
    return current;
  }
  const incoming = parseViews(JSON.stringify(o.views));
  const have = new Set(current.map((v) => v.name.toLowerCase()));
  const fresh: SavedView[] = [];
  for (const v of incoming) {
    if (have.has(v.name.toLowerCase())) continue;
    have.add(v.name.toLowerCase());
    fresh.push({ ...v, id: makeId() });
  }
  if (fresh.length === 0) return current;
  // Place the fresh views right after the anchor; a null/unknown anchor appends.
  const at = afterId === null ? -1 : current.findIndex((v) => v.id === afterId);
  if (at < 0) return [...current, ...fresh];
  return [...current.slice(0, at + 1), ...fresh, ...current.slice(at + 1)];
}

/**
 * F187: preview a paste WITHOUT committing — how many genuinely-NEW views a doc
 * would add to the current row, so main.ts can confirm "+N views" before
 * merging. Mirrors importViewsDoc's de-dup logic exactly (case-insensitive name,
 * collisions skipped) but returns just the count: importViewsDoc(current,doc)
 * appends precisely this many. A non-views/garbage doc, or one whose names all
 * already exist, returns 0. Pure → unit-tested; main.ts shows the diff in the
 * paste prompt/toast so a no-op paste reads honestly. Counting on the de-dupped
 * incoming set means duplicate names INSIDE the doc are counted once.
 */
export function previewImportViews(current: SavedView[], doc: string): number {
  let parsed: unknown;
  try {
    parsed = JSON.parse(doc);
  } catch {
    return 0;
  }
  if (!parsed || typeof parsed !== "object") return 0;
  const o = parsed as { tsk?: unknown; v?: unknown; views?: unknown };
  if (o.tsk !== VIEWS_DOC_KIND || o.v !== VIEWS_DOC_VERSION || !Array.isArray(o.views)) {
    return 0;
  }
  const incoming = parseViews(JSON.stringify(o.views));
  const have = new Set(current.map((v) => v.name.toLowerCase()));
  let added = 0;
  for (const v of incoming) {
    if (have.has(v.name.toLowerCase())) continue;
    have.add(v.name.toLowerCase());
    added++;
  }
  return added;
}

/**
 * F176: cluster a saved-view list into groups keyed by each view's FIRST tag, so
 * a big Views row reads as labeled clusters ("#work: a, b · #home: c") instead
 * of one long undifferentiated chip strip. The first tag (filter.tags[0]) is the
 * group key; views with no tag fall into a trailing "" (untagged) bucket. Group
 * order = first-appearance order of each tag (stable, so the row doesn't shuffle
 * between renders); the untagged bucket always sorts last so labeled clusters
 * lead. Views inside a group keep their original list order. Returns [] for an
 * empty list. Pure → unit-tested; main.ts can label clusters when the row grows
 * past a threshold without disturbing the F17 drag order within a tag.
 */
export interface ViewGroup {
  tag: string;
  views: SavedView[];
}

export function groupViewsByTag(views: SavedView[]): ViewGroup[] {
  const order: string[] = [];
  const buckets = new Map<string, SavedView[]>();
  for (const v of views) {
    const tag = v.filter.tags.length ? v.filter.tags[0] : "";
    if (!buckets.has(tag)) {
      buckets.set(tag, []);
      order.push(tag);
    }
    buckets.get(tag)!.push(v);
  }
  // Untagged bucket sorts last so labeled clusters lead; tagged keep appearance order.
  order.sort((a, b) => (a === "" ? 1 : 0) - (b === "" ? 1 : 0));
  return order.map((tag) => ({ tag, views: buckets.get(tag)! }));
}

/**
 * F176: a one-line readout of the F176 tag clusters — "#work: 3 · #home: 2 ·
 * untagged: 1" — for the Views-chips hover tooltip so a big row's tag layout is
 * legible without scanning. Tagged clusters lead with a "#" prefix; the untagged
 * bucket reads "untagged". Returns "" for fewer than 2 groups (a single cluster
 * needs no breakdown). Pure → unit-tested.
 */
export function describeViewGroups(groups: ViewGroup[]): string {
  if (groups.length < 2) return "";
  return groups
    .map((g) => `${g.tag === "" ? "untagged" : "#" + g.tag}: ${g.views.length}`)
    .join(" \u00b7 ");
}

/**
 * F177: build the chip-row divider labels for the F176 tag clusters so a big
 * Views row reads as visibly-sectioned groups, not one long strip. F176 groups
 * the views; describeViewGroups tooltips the layout; this returns the inline
 * "#tag" divider label that main.ts renders BEFORE each cluster (untagged reads
 * "untagged"). Like describeViewGroups it returns [] for fewer than 2 groups
 * (one cluster needs no dividers). Each entry pairs the label with the view ids
 * it leads, so the renderer can place a divider span and then the group's chips.
 * Pure -> unit-tested; main.ts walks it to interleave divider spans with chips.
 */
export interface ViewGroupDivider {
  label: string;
  ids: string[];
}

export function viewGroupDividers(groups: ViewGroup[]): ViewGroupDivider[] {
  if (groups.length < 2) return [];
  return groups.map((g) => ({
    label: g.tag === "" ? "untagged" : "#" + g.tag,
    ids: g.views.map((v) => v.id),
  }));
}

/**
 * F183: a resolver that maps a view id to the divider label that should render
 * BEFORE its chip, or "" when no divider belongs there. Built from
 * viewGroupDividers: a label is keyed to the FIRST id of each cluster, so a
 * single thin "#tag" span leads each group and the rest of the cluster's chips
 * follow unmarked. Returns a function so renderViewChips can ask per-chip
 * without rebuilding the map; fewer than 2 groups yields a resolver that always
 * answers "" (one cluster needs no dividers). Pure → unit-tested; main.ts builds
 * it from groupViewsByTag(views) and the flattened group order so the chips
 * render in cluster order with their labels interleaved.
 */
export function viewDividerLabelBefore(groups: ViewGroup[]): (id: string) => string {
  const dividers = viewGroupDividers(groups);
  if (dividers.length === 0) return () => "";
  const first = new Map<string, string>();
  for (const d of dividers) {
    if (d.ids.length > 0) first.set(d.ids[0], d.label);
  }
  return (id: string) => first.get(id) ?? "";
}

/**
 * F201: the size-resolver sibling of viewDividerLabelBefore — maps a view id to
 * the COUNT of views in the cluster its divider leads, or null when no divider
 * belongs there. Keyed (like the label resolver) to the FIRST id of each
 * cluster, so renderViewChips can render "#work (3)" on the heading that leads a
 * 3-chip cluster. Returns a function so the chip renderer asks per-chip without
 * rebuilding the map; fewer than 2 groups yields a resolver that always answers
 * null (one cluster needs no dividers, hence no count). Pure → unit-tested;
 * main.ts pairs it with viewDividerLabelBefore over the same groupViewsByTag so
 * the label and its count can't disagree.
 */
export function viewGroupSizeBefore(groups: ViewGroup[]): (id: string) => number | null {
  const dividers = viewGroupDividers(groups);
  if (dividers.length === 0) return () => null;
  const first = new Map<string, number>();
  for (const d of dividers) {
    if (d.ids.length > 0) first.set(d.ids[0], d.ids.length);
  }
  return (id: string) => first.get(id) ?? null;
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

export function describeViewsRowSummary(
  s: ViewsRowSummary,
  busiest?: BusiestViewLabel | null,
  done?: number | null,
): string {
  if (s.views === 0) return "";
  const v = `${s.views} view${s.views === 1 ? "" : "s"}`;
  if (s.tasks === 0) return v;
  let base = `${v} \u00b7 ${s.tasks} task${s.tasks === 1 ? "" : "s"}`;
  // F175: total completed across views, beside the task total — \"3 views · 18
  // tasks · 5 done\". The done count is summed the same way viewsRowSummary sums
  // tasks (chip coverage, counted once per view), so it pairs cleanly. Only
  // appended when > 0 so a board with nothing done reads plainly; null/undefined
  // / zero keeps existing callers byte-identical.
  if (typeof done === "number" && done > 0) base += ` \u00b7 ${done} done`;
  if (busiest && busiest.name !== "") {
    return `${base} \u00b7 busiest: ${busiest.name} (${busiest.count})`;
  }
  return base;
}

/**
 * F178: render the F175 "M done" segment as a CLICKABLE recall so the completed
 * total at the row head doubles as a one-click jump to everything done across
 * the Views row. F175 shows the count as plain text; this returns the same
 * "· M done" tail with the number wrapped in a `data-views-done` button so a
 * click recalls a hide-done-OFF, show-done view (all completions in coverage).
 * Reuses one delegated hook (no new dispatch). A zero/negative count returns ""
 * (nothing to recall), keeping a clean board byte-identical. Composes after the
 * task base like busiestHeadlineHTML. Pure -> unit-tested; main.ts appends it
 * between the numeric base and the busiest segment.
 */
export function doneSegmentHTML(done: number): string {
  if (done <= 0) return "";
  return ` \u00b7 <button type="button" class="views-summary-done" data-views-done title="Recall everything done across views">${done} done</button>`;
}

/**
 * F175: sum the DONE half of every view's coverage for the row-head summary —
 * the completed sibling of viewsRowSummary's task total. Each view's done count
 * is the SAME open/done split the F145 badge tooltip reads (countViewMatches-
 * Breakdown), summed across the row exactly as the task total is (counted once
 * per view, multi-matched tasks contribute per view). A null/undefined/negative
 * count contributes 0; an empty list is 0. Pure → unit-tested; describeViews-
 * RowSummary folds it onto the headline.
 */
export function viewsRowDoneCount(
  views: SavedView[],
  doneCount: (view: SavedView) => number | null | undefined,
): number {
  let done = 0;
  for (const v of views) {
    const c = doneCount(v);
    if (typeof c === "number" && c > 0) done += c;
  }
  return done;
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
 * F159: render the F154 busiest headline as a CLICKABLE recall affordance so the
 * triage answer doubles as a one-click jump to the pile-up. F154 names the
 * busiest view as plain text in describeViewsRowSummary; this returns the same
 * "· busiest: <name> (K)" tail but with the name wrapped in a `data-view-recall`
 * button so a click recalls that view — reusing the SAME hook every chip recall
 * uses (so no new dispatch). Returns the leading "·" separator + segment so it
 * composes after the escaped "N views · M tasks" base. An empty name / null
 * busiest returns "" (no winner to jump to), keeping a clean board's HTML the
 * bare base. Name is escaped for the markup. Pure → unit-tested; main.ts builds
 * the viewsSummary innerHTML from base + this + the stale segment.
 */
export function busiestHeadlineHTML(busiest: BusiestViewLabel | null | undefined, viewId: string): string {
  if (!busiest || busiest.name === "") return "";
  return ` \u00b7 busiest: <button type="button" class="views-summary-recall" data-view-recall="${escapeHTML(viewId)}" title="Recall ${escapeHTML(busiest.name)}">${escapeHTML(busiest.name)}</button> (${busiest.count})`;
}

/**
 * F164: render the F163 "· N stale" segment as a CLICKABLE sweep affordance so
 * the row-head readout of dead cohort bookmarks doubles as the F144 sweep
 * trigger. F163 tacks "· N stale" on as plain text; this returns the same tail
 * with the count wrapped in a `data-views-sweep` button so a click forgets every
 * stale cohort view (the SAME action the standalone "forget stale" button + the
 * F156 command run). A zero/negative count returns "" (nothing to sweep),
 * keeping a clean board byte-identical. Pure → unit-tested; main.ts appends it
 * after the busiest segment and the existing delegated sweep handler fires.
 */
export function staleSweepSegmentHTML(staleCount: number, names?: readonly string[], title?: string): string {
  if (staleCount <= 0) return "";
  const tip = title && title !== "" ? title : staleSweepTitle(names);
  return ` \u00b7 <button type="button" class="views-summary-stale" data-views-sweep title="${escapeHTML(tip)}">${staleCount} stale</button>`;
}

/**
 * F172: the hover tooltip for the F164 "N stale" sweep button — names the dead
 * cohort views the sweep would forget, so you can see WHICH bookmarks are stale
 * before clicking. F164's button title was a generic "Forget every stale cohort
 * view"; this turns it into "Forget: a, b, c" so hovering identifies the
 * casualties. An empty/omitted name list falls back to the generic phrase (the
 * count is still on the visible label). Names are joined in list order. Pure →
 * unit-tested; escaping happens at the HTML boundary in staleSweepSegmentHTML.
 */
export function staleSweepTitle(names?: readonly string[]): string {
  if (!names || names.length === 0) return "Forget every stale cohort view";
  return `Forget: ${names.join(", ")}`;
}

/**
 * F179: a richer stale-sweep tooltip that names EACH dead cohort view AND how
 * long it's been dead, so the F172 title gains an age cue ("Forget: a (dead 3d),
 * b (dead 1d)"). Each entry carries a name + days-stale; a view dead < 1 day
 * reads "dead today" (no "0d"), and a negative/missing age drops the age clause
 * (just the name) so a partial signal degrades gracefully. An empty list falls
 * back to the generic F172 phrase. Names stay in list order. Pure ->
 * unit-tested; main.ts pairs each stale id with a staleSince age before calling.
 */
export interface StaleViewAge {
  name: string;
  days: number;
}

export function staleSweepTitleAged(items?: readonly StaleViewAge[]): string {
  if (!items || items.length === 0) return "Forget every stale cohort view";
  const parts = items.map((it) => {
    if (it.days < 0) return it.name;
    const age = it.days === 0 ? "dead today" : `dead ${it.days}d`;
    return `${it.name} (${age})`;
  });
  return `Forget: ${parts.join(", ")}`;
}

/**
 * F190: the stale hint a cohort chip shows on its OWN title — F138 marks a dead
 * cohort chip "— stale, recall to clear"; this folds the age in ("— stale 3d,
 * recall to clear") so the chip reads how long it's been dead without hovering
 * the F179 sweep tooltip. A non-negative days reads "stale Nd" (0 = "stale
 * today"); a null/undefined/negative age degrades to the bare "stale" phrase so
 * a chip without a tracked clock stays byte-identical with F138. Pure ->
 * unit-tested; renderViewChips appends it to a stale chip's title.
 */
export function chipStaleTitle(days?: number | null): string {
  if (typeof days !== "number" || days < 0) return " \u2014 stale, recall to clear";
  const age = days === 0 ? "stale today" : `stale ${days}d`;
  return ` \u2014 ${age}, recall to clear`;
}

/**
 * F197: the VISIBLE stale-age badge for a dead cohort chip — F190 folds the age
 * into the chip's TITLE ("stale 3d, recall to clear"); this renders the bare age
 * ("3d") as a tiny suffix ON the chip face so the staleness is legible without
 * hovering. A non-negative day count reads "Nd" (0 = "0d" so a freshly-dead chip
 * still shows a badge rather than vanishing); a null/undefined/negative age
 * returns "" so a chip without a tracked clock shows no badge (the F138 dot
 * already marks it stale). Pure → unit-tested; renderViewChips renders it after
 * the chip name when an age resolver is supplied and the chip is stale.
 */
export function chipStaleBadge(days?: number | null): string {
  if (typeof days !== "number" || days < 0) return "";
  return `${days}d`;
}

/**
 * F167: the hover tooltip for the whole Views-row summary headline — a one-line
 * breakdown of the busiest pile-up + the stale-bookmark count, so resting on the
 * readout explains the row's state without clicking. The visible headline is
 * terse ("3 views · 17 tasks"); this expands to "busiest: #work (9) · 2 stale"
 * on hover. A null busiest drops its clause; zero stale drops the stale clause;
 * neither present returns "" (no title attr needed). Pure → unit-tested; main.ts
 * sets els.viewsSummary.title from it (escaping happens at the title boundary).
 */
export function viewsSummaryTooltip(
  busiest: BusiestViewLabel | null | undefined,
  staleCount: number,
): string {
  const parts: string[] = [];
  if (busiest && busiest.name !== "") parts.push(`busiest: ${busiest.name} (${busiest.count})`);
  if (staleCount > 0) parts.push(`${staleCount} stale`);
  return parts.join(" \u00b7 ");
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
 * F160: enrich F146's peek readout for a COHORT bookmark with its chokepoint
 * waiter depth, so "peek busiest" reads the bottleneck size — not just the match
 * count. A cohort view's count IS its waiter count, but peekViewLabel renders it
 * as a bare "N tasks · waiting on #M"; this appends "· N waiting" when the view
 * is a cohort and a waiter count is known, so the readout names the pile-up
 * depth explicitly ("4 tasks · waiting on #7 · 4 waiting"). A non-cohort view,
 * or a null/undefined waiter count, returns the label unchanged — non-cohort
 * peeks stay byte-identical. Pure → unit-tested; main.ts backs waiters with
 * buildCohort(...).ids.length over the live graph for the busiest cohort.
 */
export function enrichCohortPeek(
  view: SavedView,
  label: string,
  waiters: number | null | undefined,
): string {
  if (!isCohortView(view) || typeof waiters !== "number") return label;
  return `${label} \u00b7 ${waiters} waiting`;
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
  /**
   * F183: a resolver giving the divider label that should render BEFORE a chip
   * (a thin "#tag" span leading each tag cluster), or "" for no divider. Built
   * from viewDividerLabelBefore(groupViewsByTag(views)): only the FIRST chip of
   * each cluster gets a label, so a big Views row reads as labeled groups. The
   * caller renders chips in cluster order (groupViewsByTag flatten) so labels
   * land where the groups change. Omitting it renders no dividers, keeping
   * existing callers byte-identical.
   */
  dividerLabel?: (view: SavedView) => string;
  /**
   * F201: a resolver giving the COUNT of views in the cluster a chip's divider
   * leads, so a "#tag" heading can read "#work (3)" — the group size at the
   * divider, not just the F176 hover tooltip. Paired with dividerLabel (both
   * keyed to a cluster's FIRST chip via the same groupViewsByTag), so the label
   * and its count agree. Returns null/undefined (or omitted) to render the bare
   * label, keeping existing callers byte-identical. Only consulted when a
   * divider label is actually rendered for that chip.
   */
  dividerCount?: (view: SavedView) => number | null | undefined;
  /**
   * F190: a resolver giving the days-stale for a cohort chip's chokepoint, so a
   * dead bookmark's chip title shows "dead 3d" inline (not just the sweep
   * tooltip, F179). Returns days >= 0 (0 reads "dead today"), or null/undefined
   * for a live chip / unknown age. Only consulted for stale cohort chips. Omit
   * to keep the bare "— stale, recall to clear" hint, so existing callers stay
   * byte-identical. main.ts backs it with the same cohortStaleSince clock F184's
   * sweep tooltip uses, so the chip and the sweep age can't disagree.
   */
  staleCohortAge?: (view: SavedView) => number | null | undefined;
  /**
   * F196: a predicate enabling a per-chip "copy this view" button — when it
   * returns true for a view, that chip renders a small `data-view-copy` button
   * (alongside the × delete) that copies just that one view's portable doc
   * (exportSingleViewDoc) to the clipboard. The per-chip sister of F181's
   * whole-row copy. Omitting it (the default) renders no copy buttons, keeping
   * existing callers byte-identical. main.ts supplies `() => true` to offer it
   * on every chip and wires the click to a guarded clipboard write + toast.
   */
  copyable?: (view: SavedView) => boolean;
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
export function recallOpenOnlyTitle(name: string, openCount?: number | null): string {
  const base = `Recall ${name} (open only)`;
  return typeof openCount === "number" ? `${base} \u00b7${openCount}` : base;
}

/**
 * F165: the preview text for a "Peek open-only (<name>)" command — the look-don't-
 * touch sibling of F158's recall-open command. F158 jumps straight to a view's
 * OPEN slice (recall + hideDone); this lets you preview just that open count
 * first, the same way F146/F157 preview the full count. Renders "<open> open ·
 * <desc>" so you see how many tasks remain if you commit to the open-only jump
 * ("9 open · tags: #work"); a zero open count reads "all done" honestly. A
 * null/undefined open count drops the count half. Pure → unit-tested; main.ts
 * builds it from the same open/done breakdown F152's badge + F158 read.
 */
export function peekOpenOnlyLabel(view: SavedView, openCount: number | null | undefined): string {
  const desc = describeView(view);
  if (typeof openCount !== "number") return desc;
  const countText = openCount === 0 ? "all done" : `${openCount} open`;
  return `${countText} \u00b7 ${desc}`;
}

/**
 * F171: the title for a "Peek open-only (<name>)" command, with the live OPEN
 * count folded in as a quiet trailing "·N" — the open-slice sibling of F157's
 * peekCommandTitle (which folds the FULL count onto "Peek view"). The peek-open
 * group lists hide-done-able views; this lets you scan their open-slice depth in
 * the title without highlighting each to read F165's preview. A null/undefined
 * count keeps the bare "Peek open-only (<name>)" so callers without a live count
 * stay byte-identical; a zero count still renders "·0" so an all-done view reads
 * honestly. Pure → unit-tested; main.ts builds the peek-open command titles from it.
 */
export function peekOpenOnlyTitle(name: string, openCount: number | null | undefined): string {
  const base = `Peek open-only (${name})`;
  return typeof openCount === "number" ? `${base} \u00b7${openCount}` : base;
}

/**
 * F180: the title for a "Recall open-only (<name>)" Cmd-K command that COMMITS
 * the peek-open jump in one keystroke -- the keyboard sibling of F171's
 * preview-peek. F165/F171 let you PEEK a view's open slice (look, don't touch);
 * this lets the keyboard fire the actual recall+hideDone jump without reaching
 * for the badge. Folds the live open count as a quiet trailing "·N" like
 * peekOpenOnlyTitle so you scan depth before committing; a null/undefined count
 * keeps the bare title, a zero count still shows "·0" so an all-done view reads
 * honestly. Pure -> unit-tested; main.ts maps "recall-open-kbd:<id>" through the
 * same recallViewHideDone path the badge + F158 command use.
 */
export function peekOpenRecallTitle(name: string, openCount: number | null | undefined): string {
  const base = `Recall open-only (${name})`;
  return typeof openCount === "number" ? `${base} \u00b7${openCount}` : base;
}

/**
 * F189: render a Views-row cluster divider. F183 leads each tag cluster with a
 * thin "#tag" span; this turns the "#tag" labels into a CLICKABLE recall so
 * clicking the divider applies that tag as a filter (the whole cluster's common
 * tag), making the section heading a one-click "show me this tag" jump. A "#tag"
 * label becomes a `<button data-divider-tag>` carrying the bare tag (no #); the
 * "untagged" label stays an inert span (no tag to recall). Pure → unit-tested;
 * main.ts wires data-divider-tag through setFilter({tags:[tag]}). aria-hidden is
 * dropped for the actionable form so the button is reachable.
 */
export function renderViewDivider(label: string, count?: number): string {
  // F201: when a positive group size is supplied, append a quiet "(N)" count so
  // a big row reads its cluster sizes at the divider, not just the F176 tooltip.
  // A null/undefined/non-positive count renders the bare label (back-compat).
  const sizeSuffix =
    typeof count === "number" && count > 0
      ? `<span class="view-group-divider-count" aria-hidden="true"> (${count})</span>`
      : "";
  const countAria = typeof count === "number" && count > 0 ? ` (${count})` : "";
  if (label.startsWith("#") && label.length > 1) {
    const tag = label.slice(1);
    return `<button type="button" class="view-group-divider is-recall" data-divider-tag="${escapeHTML(tag)}" title="Filter to ${escapeHTML(label)}${countAria}" aria-label="Filter to ${escapeHTML(label)}${countAria}">${escapeHTML(label)}${sizeSuffix}</button>`;
  }
  return `<span class="view-group-divider" aria-hidden="true">${escapeHTML(label)}${sizeSuffix}</span>`;
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
      // F190: when stale, fold the chip's age into its own title ("— stale 3d,
      // recall to clear") so the dead bookmark reads how long it's been dead
      // inline; without an age resolver this is the bare F138 hint.
      const staleTitle = stale
        ? chipStaleTitle(opts.staleCohortAge ? opts.staleCohortAge(v) : null)
        : "";
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
      // F197: the VISIBLE stale-age badge — when a chip is stale and an age
      // resolver gives a day count, render a tiny "Nd" suffix ON the chip face
      // (not just the F190 title) so the staleness is legible at a glance. Shares
      // the staleCohortAge resolver F190's title reads, so the face badge and the
      // hover title can't show different ages. Only on stale chips with a known age.
      const staleAge = stale && opts.staleCohortAge ? opts.staleCohortAge(v) : null;
      const staleBadgeText = stale ? chipStaleBadge(staleAge) : "";
      const staleBadge = staleBadgeText
        ? `<span class="view-chip-stale-age" aria-hidden="true" title="${escapeHTML(`dead ${staleBadgeText}`)}">${escapeHTML(staleBadgeText)}</span>`
        : "";
      // F196: a per-chip "copy this view" button (the per-chip sister of F181's
      // whole-row copy) — when the copyable predicate flags a chip, render a small
      // data-view-copy button beside the × so just that one bookmark's portable
      // doc can be copied. Omitting the predicate renders no copy buttons.
      const copyBtn =
        opts.copyable && opts.copyable(v)
          ? `<button type="button" class="view-chip-copy" data-view-copy="${escapeHTML(v.id)}" title="Copy “${escapeHTML(v.name)}” to clipboard" aria-label="Copy view ${escapeHTML(v.name)}">&#9112;</button>`
          : "";
      // F183: a thin "#tag" divider span leads the first chip of each tag
      // cluster so a big Views row reads as labeled groups. Only the cluster's
      // first chip gets a label (resolver returns "" otherwise); omitting the
      // resolver renders no dividers, so existing callers stay byte-identical.
      const dl = opts.dividerLabel ? opts.dividerLabel(v) : "";
      // F201: when a divider renders, lead it with the cluster's size ("#work
      // (3)") if a count resolver supplies one — the group size read at the
      // heading, not just the F176 tooltip. Only consulted when a label exists.
      const dc = dl && opts.dividerCount ? opts.dividerCount(v) : null;
      const divider = dl ? renderViewDivider(dl, dc ?? undefined) : "";
      return `${divider}<span class="view-chip${active}${lensed}${pinClass}${stale}${flash}${busiest}"${dragAttrs} data-view-id="${escapeHTML(v.id)}" title="${escapeHTML(describeView(v) + staleTitle)}"><button type="button" class="view-chip-name" data-view-recall="${escapeHTML(v.id)}">${glyphSpan}${escapeHTML(v.name)}</button>${staleBadge}${badge}${update}${copyBtn}<button type="button" class="view-chip-del" data-view-del="${escapeHTML(v.id)}" aria-label="Delete view ${escapeHTML(v.name)}">&times;</button></span>`;
    })
    .join("");
}
