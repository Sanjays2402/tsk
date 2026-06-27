/**
 * tsk web entry — vanilla DOM, framework-free.
 *
 * Wires the end-to-end vertical (slice F4): fetch /api/tasks, render the
 * styled list, refresh on visibility change so external CLI/TUI edits
 * show up when you switch back to the tab.
 *
 * Slice F5 layers click-to-toggle on top of this: optimistic flip, server
 * confirm, rollback on error. Round-trips through atomic store.Save so
 * the .tsk.md on disk matches what you see.
 */

import { api, ApiError, type Task } from "./api";
import { renderSections, summarize } from "./render";
import { doneIndex, needsBlockedConfirm, blockedToggleConfirm, computeDepStats, biggestChokepoint, topChokepoints, newlyUnblocked, unblockedMessage, longestChainPath, deepestChainFrom, hasWalkableChain, deepestDependentChainFrom, hasWalkableDependents, renderChainDrill, renderUnblockedPicker, blockedInBulkToggle, bulkBlockedConfirm, type DepTask, type DepStatsTask, type ChainNode, type Chokepoint } from "./deps";
import { nextPriority, prevPriority, floorPriority, ceilPriority, type Priority as CyclePriority } from "./priority";
import { renderPriorityPicker } from "./prioritypicker";
import {
  LONG_PRESS_MS,
  trackMove,
  type PressState,
} from "./touch";
import { parseQuickAdd, isSubmittable, splitPasteLines, isMultiLine } from "./quickadd";
import { renderComposerPreview } from "./composer";
import {
  activeToken,
  suggestFor,
  applySuggestion,
  renderAutocomplete,
  type Suggestion,
} from "./autocomplete";
import { groupIntoSections, flattenSections } from "./sections";
import { emptyNav, reconcile, move, select, type NavMove, type NavState } from "./keynav";
import { renderToast, deletedMessage } from "./toast";
import { resolveEdit } from "./edit";
import {
  emptyFilter,
  isFilterActive,
  applyFilter,
  collectTags,
  toggleMember,
  renderPriorityPills,
  renderTagChips,
  filterSummary,
  renderPriorityScopeNote,
  renderTagScopeNote,
  type FilterState,
  type Priority,
} from "./filter";
import {
  previewVM,
  resolveDueCommit,
  renderPresets,
  renderDuePreview,
  type DuePreviewVM,
} from "./duepicker";
import { renderStatsPanel, chokepointTrend } from "./stats";
import { computeScheduleStats } from "./schedule";
import {
  applyLens,
  renderLensChipBody,
  lensMeta,
  lensForDigit,
  renderActiveLensHelp,
  renderLensDigitMap,
  computeLensBreakdown,
  renderLensBreakdown,
  lensBreakdownPriority,
  parseLens,
  LENS_KEY,
  LENS_FACET_KEY,
  parseLensFacet,
  serializeLensFacet,
  type LensKind,
  type LensBreakdownTask,
} from "./lens";
import {
  buildCohort,
  applyCohort,
  renderCohortChipBody,
  cohortChipTitle,
  reconcileCohort,
  renderCohortFocusButton,
  cohortSummary,
  renderCohortHelp,
  pushCohortHistory,
  popCohortHistory,
  type CohortFocus,
} from "./cohort";
import { keyToPopNavAction, nextPopNavIndex } from "./popnav";
import {
  normalizeMode,
  nextMode,
  themeAttr,
  modeLabel,
  modeGlyph,
  modeTitle,
  type ThemeMode,
} from "./theme";
import { parseHash, tagHash, viewHash, type Route } from "./router";
import {
  emptyBulk,
  isBulkActive,
  isSelected as isBulkSelected,
  toggleOne,
  selectRange,
  clearBulk,
  reconcileBulk,
  selectedInOrder,
  renderBulkBar,
  type BulkState,
} from "./bulkselect";
import { computeReorder, computeSectionReorder, computePinToTop, dropPosForY, type DropPos } from "./reorder";
import {
  parseTagOps,
  isNoopTagOps,
  applyTagOps,
  renderBulkEditCluster,
  renderBulkPriorityMenu,
  renderBulkTagEditor,
  renderBulkDueEditor,
  renderBulkPinMenu,
  bulkReasonLine,
  bulkTagCommandReason,
  bulkDueClearDisabledReason,
  type BulkTaskLike,
  type Priority as BulkPriority,
} from "./bulkedit";
import {
  renderContextMenu,
  clampMenuPosition,
  type RowAction,
} from "./contextmenu";
import {
  validateAddDep,
  currentDeps,
  withDepAdded,
  withDepRemoved,
  depCandidates,
  renderDepEditor,
  renderDepCandidates,
  type DepGraphTask,
} from "./depedit";
import {
  filterCommands,
  moveIndex,
  clampIndex,
  renderPaletteList,
  buildPriorityCommands,
  buildDueCommands,
  dueTokenForCommandId,
  priorityForCommandId,
  priorityPreviewVM,
  renderPriorityPreview,
  commandDisabledReason,
  renderDisabledReason,
  clearLensCommand,
  pinLensCommand,
  clearCohortCommand,
  focusChokepointCommand,
  buildChokepointFocusCommands,
  focusShiftedChokepointCommand,
  type Command,
} from "./palette";
import {
  exportUrl,
  scopedExportUrl,
  exportFilename,
  renderExportMenu,
  buildExportCommands,
  exportCommandTarget,
  EXPORT_SCOPE_KEY,
  parseExportScope,
  serializeExportScope,
  type ExportFormat,
} from "./export";
import {
  parseFingerprint,
  shouldRefresh,
  liveTitle,
  liveChangeMessage,
  renderLiveIndicator,
  type FileFingerprint,
  type LiveStatus,
} from "./live";
import {
  registerServiceWorker,
  classifyConnectivity,
  shouldShowOfflineBanner,
  renderOfflineBanner,
  canInstall,
  isStandalone,
  type InstallPromptEvent,
} from "./pwa";
import { resolveNotes } from "./notes";
import {
  parseSettings,
  serializeSettings,
  settingsAttributes,
  renderSettings,
  defaultSettings,
  STORAGE_KEY as SETTINGS_KEY,
  type Settings,
} from "./settings";
import {
  buildConfig,
  serializeConfig,
  parseConfig,
  configFilename,
  resetBundle,
} from "./config";
import {
  parseViews,
  serializeViews,
  addView,
  removeView,
  updateView,
  moveView,
  activeViewWithLens,
  lensProvenanceNote,
  pureLensViewName,
  findPureLensView,
  renderViewChips,
  filterIsEmpty,
  filtersEqual,
  STORAGE_KEY as VIEWS_KEY,
  type SavedView,
  type ViewFilter,
} from "./views";

const root = document.getElementById("root");
if (!root) throw new Error("missing #root");

root.innerHTML = `
  <div class="app" data-app>
    <div class="offline-banner" data-offline-banner role="status" aria-live="polite" hidden></div>
    <header class="topbar">
      <h1>tsk<span class="dot" data-dot>// loading</span></h1>
      <div class="topbar-right">
        <span class="live-indicator" data-live title="Live updates"></span>
        <button class="theme-toggle" data-theme-toggle type="button" aria-label="Cycle theme" title="Theme"><span class="theme-glyph" data-theme-glyph></span><span class="theme-label" data-theme-label></span></button>
        <div class="export-wrap" data-export-wrap>
          <button class="export-toggle" data-export-toggle type="button" aria-haspopup="menu" aria-expanded="false" aria-label="Export tasks" title="Export tasks">export</button>
          <div class="export-menu" data-export-menu role="menu" hidden></div>
        </div>
        <button class="stats-toggle" data-stats-toggle type="button" aria-pressed="false" aria-label="Toggle stats panel" title="Toggle stats (s)">stats</button>
        <button class="settings-toggle" data-settings-toggle type="button" aria-label="Open settings" title="Settings (,)">&#9881;</button>
        <div class="file" data-file>—</div>
      </div>
    </header>
    <form class="composer" data-composer autocomplete="off">
      <div class="composer-field" data-field>
        <span class="plus" aria-hidden="true">+</span>
        <input
          class="composer-input"
          data-input
          type="text"
          name="quickadd"
          placeholder="Add a task…  try: ship release !high @fri #work depends:#3"
          aria-label="Add a task. Use !priority @due #tag depends:#N for inline metadata."
          spellcheck="false"
          aria-autocomplete="list"
          aria-expanded="false"
          aria-controls="composer-ac"
        >
        <button class="composer-submit" type="submit" data-submit tabindex="-1">Add</button>
        <ul class="composer-ac" id="composer-ac" data-composer-ac role="listbox" aria-label="Suggestions" hidden></ul>
      </div>
      <div class="composer-preview" data-preview></div>
    </form>
    <div class="filterbar" data-filterbar hidden>
      <div class="filter-search">
        <span class="filter-ico" aria-hidden="true">&#9906;</span>
        <input
          class="filter-input"
          data-filter-input
          type="text"
          placeholder="Filter tasks&hellip;  fuzzy match on title + tags"
          aria-label="Filter tasks by fuzzy text match on title and tags"
          spellcheck="false"
        >
        <button class="filter-clear" data-filter-clear type="button" aria-label="Clear all filters" hidden>clear</button>
      </div>
      <div class="filter-facets">
        <div class="filter-prios" data-filter-prios role="group" aria-label="Filter by priority"></div>
        <button class="fpill toggle-done" data-filter-hidedone type="button" aria-pressed="false" title="Hide completed tasks">hide done</button>
        <button class="fpill lens-blocked" data-filter-blocked type="button" aria-pressed="false" title="Showing only blocked tasks — click to clear" hidden>&#9211; blocked <span class="lens-x" aria-hidden="true">&times;</span></button>
        <button class="lens-pin" data-filter-lens-pin type="button" hidden aria-label="Pin this lens as a saved view"></button>
        <span class="lens-from" data-filter-lens-from hidden></span>
        <button class="fpill lens-blocked cohort-chip" data-filter-cohort type="button" aria-pressed="false" title="Showing only the tasks waiting on a chokepoint — click to clear" hidden></button>
      </div>
      <div class="filter-tags" data-filter-tags role="group" aria-label="Filter by tag"></div>
      <div class="filter-views" data-views-row hidden>
        <span class="views-label">Views</span>
        <div class="views-chips" data-views-chips role="group" aria-label="Saved views"></div>
        <button class="views-save" data-views-save type="button" title="Save the current filter as a named view">+ save view</button>
      </div>
    </div>
    <div class="tagpage-banner" data-tagpage hidden>
      <span class="tagpage-label">Tag</span>
      <span class="tagpage-name" data-tagpage-name></span>
      <span class="tagpage-count" data-tagpage-count></span>
      <a class="tagpage-clear" href="#" data-tagpage-clear>&larr; all tasks</a>
    </div>
    <div class="layout" data-layout>
      <div data-content>
        <div class="skeleton" aria-busy="true" aria-label="loading tasks">
          <div class="bar w-80"></div>
          <div class="bar w-60"></div>
          <div class="bar w-40"></div>
        </div>
      </div>
      <aside class="stats-panel" data-stats-panel hidden aria-label="Task statistics"></aside>
    </div>
    <div class="bulkbar" data-bulkbar role="region" aria-label="Bulk actions" hidden></div>
    <footer class="statusline">
      <span class="count" data-count></span>
      <span data-build><kbd class="kbd-hint" data-cmdk-open>⌘K</kbd> &middot; <kbd class="kbd-hint" data-help-open>?</kbd> shortcuts &middot; <a href="/api/tasks" style="color:inherit">api</a></span>
    </footer>
  </div>
`;

const els = {
  content: must<HTMLElement>("[data-content]"),
  file: must<HTMLElement>("[data-file]"),
  dot: must<HTMLElement>("[data-dot]"),
  count: must<HTMLElement>("[data-count]"),
  composer: must<HTMLFormElement>("[data-composer]"),
  field: must<HTMLElement>("[data-field]"),
  input: must<HTMLInputElement>("[data-input]"),
  preview: must<HTMLElement>("[data-preview]"),
  composerAc: must<HTMLElement>("[data-composer-ac]"),
  filterbar: must<HTMLElement>("[data-filterbar]"),
  filterInput: must<HTMLInputElement>("[data-filter-input]"),
  filterClear: must<HTMLButtonElement>("[data-filter-clear]"),
  filterPrios: must<HTMLElement>("[data-filter-prios]"),
  filterTags: must<HTMLElement>("[data-filter-tags]"),
  filterHideDone: must<HTMLButtonElement>("[data-filter-hidedone]"),
  filterBlocked: must<HTMLButtonElement>("[data-filter-blocked]"),
  filterLensPin: must<HTMLButtonElement>("[data-filter-lens-pin]"),
  filterLensFrom: must<HTMLElement>("[data-filter-lens-from]"),
  filterCohort: must<HTMLButtonElement>("[data-filter-cohort]"),
  viewsRow: must<HTMLElement>("[data-views-row]"),
  viewsChips: must<HTMLElement>("[data-views-chips]"),
  viewsSave: must<HTMLButtonElement>("[data-views-save]"),
  statsToggle: must<HTMLButtonElement>("[data-stats-toggle]"),
  statsPanel: must<HTMLElement>("[data-stats-panel]"),
  settingsToggle: must<HTMLButtonElement>("[data-settings-toggle]"),
  themeToggle: must<HTMLButtonElement>("[data-theme-toggle]"),
  themeGlyph: must<HTMLElement>("[data-theme-glyph]"),
  themeLabel: must<HTMLElement>("[data-theme-label]"),
  exportWrap: must<HTMLElement>("[data-export-wrap]"),
  exportToggle: must<HTMLButtonElement>("[data-export-toggle]"),
  exportMenu: must<HTMLElement>("[data-export-menu]"),
  tagpage: must<HTMLElement>("[data-tagpage]"),
  tagpageName: must<HTMLElement>("[data-tagpage-name]"),
  tagpageCount: must<HTMLElement>("[data-tagpage-count]"),
  tagpageClear: must<HTMLAnchorElement>("[data-tagpage-clear]"),
  bulkbar: must<HTMLElement>("[data-bulkbar]"),
  live: must<HTMLElement>("[data-live]"),
  offlineBanner: must<HTMLElement>("[data-offline-banner]"),
};

function must<T extends HTMLElement>(sel: string): T {
  const el = document.querySelector(sel) as T | null;
  if (!el) throw new Error(`missing ${sel}`);
  return el;
}

let currentTasks: Task[] = [];
/** Per-task in-flight toggle guard, prevents double-fire on rapid clicks. */
const inFlight = new Set<number>();
/** Keyboard selection state (F10) + the visible id order it walks. */
let nav: NavState = emptyNav();
let visibleIds: number[] = [];
/** Ids hidden pending an undoable delete (F8); excluded from the rendered list. */
const pendingDeletes = new Set<number>();
/** Active filter (F11): search query, priority + tag facets, hide-done. */
let filter: FilterState = emptyFilter();
/** Per-client settings (F24): density, motion, hide-done default, show-ids. */
let settings: Settings = defaultSettings();
try {
  settings = parseSettings(localStorage.getItem(SETTINGS_KEY));
} catch {
  // ignore (private mode / storage disabled)
}
// Seed the filter's hide-done from the persisted preference so a user who
// prefers a clean board starts that way on every load.
filter.hideDone = settings.hideDone;
/** Stats panel (F13): open state persisted in localStorage, last fetched data. */
let statsOpen = false;
try {
  statsOpen = localStorage.getItem("tsk.stats") === "1";
} catch {
  // ignore (private mode / storage disabled)
}
/** Theme mode (F14): auto/light/dark, persisted in localStorage. */
let themeMode: ThemeMode = "auto";
try {
  themeMode = normalizeMode(localStorage.getItem("tsk.theme"));
} catch {
  // ignore
}
/** Current hash route (F15): all-tasks, a single-tag page, or a saved view (F32). */
let route: Route = parseHash(typeof location !== "undefined" ? location.hash : "");
/**
 * F32: set just before we programmatically change the hash (e.g. recalling a
 * view writes `#view/<id>`), so the resulting hashchange doesn't re-trigger a
 * route apply and fight the state we just set.
 */
let suppressNextHashRoute = false;
/** Bulk selection (F16): a set of ids for multi-toggle / multi-delete. */
let bulk: BulkState = emptyBulk();
/** Saved views (F25): named filter combinations, persisted in localStorage. */
let views: SavedView[] = [];
try {
  views = parseViews(localStorage.getItem(VIEWS_KEY));
} catch {
  // ignore (private mode / storage disabled)
}
/**
 * F32: the id of the view the user last recalled, so we can offer an "update
 * this view to the current filter" affordance once they tweak the filter away
 * from what was saved. Cleared when the filter is cleared or another view is
 * recalled.
 */
let recalledViewId: string | null = null;

/**
 * F66: the single active render-pipeline LENS, or null. A lens narrows the
 * visible list to a derived subset the stats sidebar drives — `blocked`
 * (cross-task: depends on other tasks' done state) or one of the time-relative
 * schedule lenses (`overdue` / `today` / `week` / `nodue`). It lives OUTSIDE
 * FilterState on purpose: these subsets are cross-task or clock-relative, so
 * they must NOT serialize into saved views / settings. Exactly one is active at
 * a time (the subsets are mutually exclusive); clicking a stats tile sets it, a
 * chip in the filter bar clears it. The "Open" tile is the exception — it maps
 * to the real `hideDone` facet (which DOES serialize), so it routes there.
 */
let activeLens: LensKind | null = parseLens(sessionStorage.getItem(LENS_KEY));
// F97: when a lens was persisted (F93), also restore the priority facet that was
// drilled on top of it (the F81 lens+facet combo) so a reload brings back the
// WHOLE drill, not just the lens. Validated against the four known levels by
// parseLensFacet so a stale/garbage stored value can't inject a bad facet. Only
// applied when a lens actually restored — a facet with no lens isn't persisted.
if (activeLens) {
  const restoredFacet = parseLensFacet(sessionStorage.getItem(LENS_FACET_KEY));
  if (restoredFacet.length > 0) {
    filter = { ...filter, priorities: restoredFacet as Priority[] };
  }
}

/**
 * F96: the active cohort focus — an explicit id-set narrowing to the undone
 * tasks waiting on one chokepoint (built from F92's biggest-chokepoint / F87's
 * "N waiting" badge). Like a lens it's a render-pipeline narrowing that does NOT
 * serialize into saved views, and it's mutually exclusive with a lens (both are
 * "special" narrowings layered over the text/facet filter). null when off.
 */
let focusCohort: CohortFocus | null = null;

/**
 * F108: the cohort back-stack — the source ids you focused BEFORE the current
 * cohort, most-recent last. setCohort pushes the outgoing focus's source here so
 * "focus #1's cohort" then "focus #3's cohort" can step BACK to #1 (Escape or
 * the chip's back glyph). Momentary + per-session (never persisted): cohorts are
 * snapshots, so their history is too. Cleared whenever the cohort is dropped.
 */
let cohortHistory: number[] = [];

/**
 * F111: the biggest chokepoint id at the LAST stats paint, so refreshStats can
 * show a quiet "was #M" delta when the worst bottleneck shifts across a refresh
 * (a live-reload after an external edit, or completing the current chokepoint).
 * null before the first paint (no comparison yet). Updated to the fresh id after
 * each paint so the hint shows for exactly the one render where it changed.
 */
let prevBiggestChokepoint: number | null = null;

/**
 * F114: the biggest chokepoint id seen at the LAST refresh, plus the prior id
 * when the worst bottleneck shifted on that refresh. These drive the Cmd-K
 * "Focus the new biggest chokepoint (#N, was #M)" lead command, the keyboard
 * sibling of F111's sidebar "was #M" trend hint.
 *
 * F111's prevBiggestChokepoint only updates while the stats panel is OPEN (it
 * paints the sidebar), so it can't drive a palette command that must work with
 * the panel closed. These two slots are updated in refresh() — which always runs
 * — so the shift is tracked independently. `lastBiggestChokepoint` is the prior
 * refresh's biggest id; `chokepointShiftFrom` holds the id to lead "was #M" with
 * for exactly the window between the shift and the next refresh (or until the
 * user acts on the lead), mirroring the momentary lifetime of F111's hint.
 */
let lastBiggestChokepoint: number | null = null;
let chokepointShiftFrom: number | null = null;

/** Render the current state to the DOM, preserving keyboard selection. */
function render(): void {
  const now = new Date();
  const notDeleted = currentTasks.filter((t) => !pendingDeletes.has(t.id));
  // F15: a tag route pre-narrows the pool to tasks carrying that tag.
  const r = route;
  const routed = r.kind === "tag"
    ? notDeleted.filter((t) => t.tags.includes(r.tag))
    : notDeleted;
  let shown = applyFilter(routed, filter);
  // F66: the active lens runs after the text/facet filter. The done-index is
  // built over ALL live tasks so a blocker hidden by the active filter still
  // counts as blocking, and so the schedule windows see the whole board.
  if (activeLens) {
    shown = applyLens(shown, activeLens, now, doneIndex(notDeleted));
  }
  // F96: a cohort focus (the undone tasks waiting on a chokepoint) is the
  // explicit-id-set sibling of a lens — mutually exclusive with one, applied
  // here as the same kind of post-filter narrowing. Re-derived against the live
  // graph each render (re-using the snapshot ids) so a dependent that completes
  // simply drops out of the focused set on the next paint.
  if (focusCohort) {
    shown = applyCohort(shown, new Set(focusCohort.ids));
  }
  const sections = groupIntoSections(shown, now);
  const prevIds = visibleIds;
  visibleIds = flattenSections(sections).map((t) => t.id);
  nav = reconcile(nav, visibleIds, prevIds);
  // F26: build the done-index over ALL live tasks (not just the filtered view)
  // so a blocker hidden by the current filter still counts as blocking.
  const doneIdx = doneIndex(notDeleted);
  // F30: pass the active search query so matched title chars get highlighted.
  const query = filter.query.trim();
  // F87: thread the whole live (non-deleted) list so each row can show a
  // "N waiting" chokepoint badge for tasks other undone tasks depend on.
  els.content.innerHTML = renderSections(sections, now, {
    done: doneIdx,
    query,
    allTasks: notDeleted as DepStatsTask[],
  });
  els.count.innerHTML = summarize(shown);
  renderFilterBar(routed, shown.length);
  renderTagPage(routed.length);
  bulk = reconcileBulk(bulk, visibleIds);
  applySelection();
  renderBulkSelection();
}

/** Reflect the F15 tag-page banner: name, matching count, and visibility. */
function renderTagPage(routedCount: number): void {
  if (route.kind !== "tag") {
    els.tagpage.hidden = true;
    must<HTMLElement>("[data-app]").classList.remove("on-tagpage");
    return;
  }
  els.tagpage.hidden = false;
  must<HTMLElement>("[data-app]").classList.add("on-tagpage");
  els.tagpageName.textContent = `#${route.tag}`;
  els.tagpageCount.textContent = `${routedCount} task${routedCount === 1 ? "" : "s"}`;
}

/**
 * Repaint the filter bar (F11): show/hide it once there are tasks, render the
 * priority pills + tag chips from the live facet selection, reflect the
 * hide-done toggle, and surface a "clear" affordance + visible/total summary
 * whenever a filter is active.
 */
function renderFilterBar(allTasks: Task[], visibleCount: number): void {
  const hasTasks = allTasks.length > 0;
  els.filterbar.hidden = !hasTasks;
  if (!hasTasks) return;
  els.filterPrios.innerHTML = renderPriorityPills(filter);
  // F86: when a priority facet is active AND a lens is on, append a small
  // "in <lens>" qualifier so it's clear the priority is scoped to the lens
  // (the F81 breakdown-pill drill layers priority ON TOP of the lens), not the
  // whole board. Cleared automatically when either the lens or the facet drops.
  els.filterPrios.innerHTML += renderPriorityScopeNote(
    activeLens ? lensMeta(activeLens).label : null,
    filter,
  );
  els.filterTags.innerHTML = renderTagChips(collectTags(allTasks), filter);
  // F94: sister of F86 for the tag facet — when a tag is selected AND a lens is
  // on, append the same "in <lens>" qualifier so a lens-scoped tag reads as
  // scoped, not whole-board. Cleared automatically when either drops.
  els.filterTags.innerHTML += renderTagScopeNote(
    activeLens ? lensMeta(activeLens).label : null,
    filter,
  );
  const active = isFilterActive(filter) || activeLens !== null || focusCohort !== null;
  els.filterClear.hidden = !active;
  els.filterHideDone.classList.toggle("is-active", filter.hideDone);
  els.filterHideDone.setAttribute("aria-pressed", String(filter.hideDone));
  // F66: reflect the single active-lens chip (hidden unless a lens is on). The
  // chip's body names the lens + a clear affordance; its hue echoes the source
  // stat tile (alert / today / neutral) so it reads as "I clicked that number".
  if (activeLens) {
    els.filterBlocked.hidden = false;
    els.filterBlocked.innerHTML = renderLensChipBody(activeLens);
    const hue = lensMeta(activeLens).hue;
    els.filterBlocked.classList.toggle("is-active", true);
    els.filterBlocked.classList.toggle("lens-hue-alert", hue === "alert");
    els.filterBlocked.classList.toggle("lens-hue-today", hue === "today");
    els.filterBlocked.classList.toggle("lens-hue-neutral", hue === "neutral");
    els.filterBlocked.setAttribute("aria-pressed", "true");
    els.filterBlocked.title = `Showing only ${lensMeta(activeLens).label} tasks — click to clear`;
    // F109: when the active lens was re-applied by recalling a lensed view
    // (F104), surface a "from <view>" provenance readout beside the chip so
    // "why is this lens on?" is answerable. lensProvenanceNote returns the
    // recalled view's name only while that view still equals the live lens (a
    // digit-key lens, or a lens changed after recall, reports nothing).
    const recalled = recalledViewId ? views.find((v) => v.id === recalledViewId) ?? null : null;
    const provenance = lensProvenanceNote(recalled, activeLens);
    if (provenance !== null) {
      els.filterLensFrom.hidden = false;
      els.filterLensFrom.textContent = `from ${provenance}`;
      els.filterLensFrom.title = `The ${lensMeta(activeLens).label} lens came from the saved view “${provenance}”`;
    } else {
      els.filterLensFrom.hidden = true;
      els.filterLensFrom.textContent = "";
    }
    // F110: the one-click "pin this lens" affordance — saves a pure-lens view
    // named after the lens (no prompt) so a frequently-used lens becomes a
    // recallable chip in one click. When the lens is ALREADY pinned the button
    // flips to a filled "pinned" state that recalls the saved view instead of
    // re-saving (and never creates a duplicate). Reflects + acts in the click
    // handler below via findPureLensView.
    const pinned = findPureLensView(views, activeLens);
    els.filterLensPin.hidden = false;
    els.filterLensPin.classList.toggle("is-pinned", pinned !== null);
    els.filterLensPin.textContent = pinned !== null ? "\u2605" : "\u2606"; // ★ / ☆
    els.filterLensPin.title =
      pinned !== null
        ? `“${pinned.name}” is pinned — click to recall it`
        : `Pin the ${lensMeta(activeLens).label} lens as a saved view`;
  } else {
    els.filterBlocked.hidden = true;
    els.filterBlocked.classList.remove("is-active", "lens-hue-alert", "lens-hue-today", "lens-hue-neutral");
    els.filterBlocked.setAttribute("aria-pressed", "false");
    els.filterLensFrom.hidden = true; // F109: no lens, no provenance
    els.filterLensFrom.textContent = "";
    els.filterLensPin.hidden = true; // F110: no lens, no pin affordance
    els.filterLensPin.classList.remove("is-pinned");
  }
  // F96: the cohort-focus chip (hidden unless a chokepoint cohort is focused).
  // It reads "↑ N waiting on #M ×" and clears the focus on click — the
  // explicit-id-set sibling of the lens chip, wearing the same alert hue since
  // a chokepoint cohort is inherently a "what's stuck" view.
  if (focusCohort) {
    els.filterCohort.hidden = false;
    els.filterCohort.innerHTML = renderCohortChipBody(focusCohort, cohortHistory.length);
    els.filterCohort.classList.add("is-active", "lens-hue-alert");
    els.filterCohort.setAttribute("aria-pressed", "true");
    els.filterCohort.title = cohortChipTitle(focusCohort);
  } else {
    els.filterCohort.hidden = true;
    els.filterCohort.classList.remove("is-active", "lens-hue-alert");
    els.filterCohort.setAttribute("aria-pressed", "false");
  }
  els.filterbar.classList.toggle("is-active", active);
  renderViewsRow();
  if (active) {
    els.count.innerHTML = `${summarize(applyFilter(allTasks, filter))} &middot; <span class="filter-note">${filterSummary(visibleCount, allTasks.length)}</span>`;
  }
}

/** Reflect nav.selectedId onto the DOM and scroll it into view. */
function applySelection(): void {
  const rows = els.content.querySelectorAll<HTMLElement>("[data-id]");
  rows.forEach((row) => {
    const on = Number(row.dataset.id) === nav.selectedId;
    row.classList.toggle("is-selected", on);
    if (on) row.setAttribute("aria-current", "true");
    else row.removeAttribute("aria-current");
  });
  if (nav.selectedId !== null) {
    const sel = els.content.querySelector<HTMLElement>(
      `[data-id="${nav.selectedId}"]`,
    );
    sel?.scrollIntoView({ block: "nearest" });
  }
}

// --- F16: bulk selection ---------------------------------------------------

/** Paint the bulk-selected rows + the floating action bar. */
function renderBulkSelection(): void {
  const rows = els.content.querySelectorAll<HTMLElement>("[data-id]");
  rows.forEach((row) => {
    const on = isBulkSelected(bulk, Number(row.dataset.id));
    row.classList.toggle("is-bulk", on);
  });
  must<HTMLElement>("[data-app]").classList.toggle("has-bulk", isBulkActive(bulk));
  const count = bulk.ids.size;
  els.bulkbar.hidden = count === 0;
  els.bulkbar.innerHTML = renderBulkBar(count);
  // F36: inject the bulk-edit cluster (priority / tag / due) into the bar's
  // actions group, beside the core toggle/delete/clear from F16.
  if (count > 0) {
    const actions = els.bulkbar.querySelector<HTMLElement>(".bulkbar-actions");
    if (actions) {
      actions.insertAdjacentHTML("afterbegin", renderBulkEditCluster());
    }
  }
}

/** Toggle a single row into/out of the bulk set (cmd/ctrl-click). */
function bulkToggleOne(id: number): void {
  bulk = toggleOne(bulk, id);
  renderBulkSelection();
}

/** Extend the bulk set as a range to id (shift-click), walking visible order. */
function bulkSelectRange(id: number): void {
  bulk = selectRange(bulk, visibleIds, id);
  renderBulkSelection();
}

/** Clear the entire bulk selection. */
function bulkClear(): void {
  bulk = clearBulk();
  closeBulkEdit();
  renderBulkSelection();
}

/**
 * Toggle done for every bulk-selected task, then clear the selection. Each
 * task is flipped to the OPPOSITE of its current state (so a mixed selection
 * converges by individual state, mirroring per-row toggle semantics). Fires
 * the calls in parallel and refreshes once.
 */
async function bulkToggleDone(): Promise<void> {
  const ids = selectedInOrder(bulk, visibleIds);
  if (ids.length === 0) return;
  // F60: the bulk sibling of F45's single-task guard. If toggling would COMPLETE
  // one or more still-blocked tasks, list them in a single confirm before
  // overriding the dependency gate. Re-opening done tasks never prompts. The
  // done-index is built over ALL live tasks so a blocker outside the selection
  // still counts.
  const blocked = blockedInBulkToggle(ids, currentTasks as DepTask[]);
  if (blocked.length > 0 && typeof confirm === "function") {
    if (!confirm(bulkBlockedConfirm(blocked, ids.length))) return;
  }
  bulkClear();
  setStatus(`toggling ${ids.length}…`, false);
  try {
    await Promise.all(ids.map((id) => api.toggleTask(id)));
    await refresh();
  } catch (err) {
    await refresh();
    setStatus(`bulk toggle failed: ${formatErr(err)}`, true);
    setTimeout(() => setStatus("ready", false), 4_000);
  }
}

/**
 * Delete every bulk-selected task. Unlike the single-row delete (which offers
 * a 5s undo for one task), a bulk delete is a deliberate multi-item action, so
 * it commits immediately after a confirm. Fires the DELETEs in parallel.
 */
async function bulkDelete(): Promise<void> {
  const ids = selectedInOrder(bulk, visibleIds);
  if (ids.length === 0) return;
  const ok =
    typeof confirm === "function"
      ? confirm(`Delete ${ids.length} task${ids.length === 1 ? "" : "s"}? This can't be undone.`)
      : true;
  if (!ok) return;
  bulkClear();
  setStatus(`deleting ${ids.length}…`, false);
  try {
    await Promise.all(ids.map((id) => api.deleteTask(id)));
    await refresh();
  } catch (err) {
    await refresh();
    setStatus(`bulk delete failed: ${formatErr(err)}`, true);
    setTimeout(() => setStatus("ready", false), 4_000);
  }
}

// --- F36: bulk edit (priority / tag / due / pin) ---------------------------

/** The bulk-edit popover currently open, or null. F47 adds "pin". */
type BulkEditKind = "priority" | "tag" | "due" | "pin";
let bulkEditOpen: BulkEditKind | null = null;

/** Remove any open bulk-edit popover and drop its outside-click guard. */
function closeBulkEdit(): void {
  bulkEditOpen = null;
  document.querySelector("[data-bulkedit-pop]")?.remove();
  document.removeEventListener("click", onBulkEditAway, true);
}

/** Outside-click closes the bulk-edit popover (capture so it beats row clicks). */
function onBulkEditAway(e: MouseEvent): void {
  const t = e.target as HTMLElement | null;
  if (t?.closest("[data-bulkedit-pop]")) return;
  if (t?.closest("[data-bulk-edit]")) return; // the opener toggles itself
  closeBulkEdit();
}

/** A monotonic seq so a stale bulk-due parse can't overwrite a newer preview. */
let bulkDueParseSeq = 0;

/**
 * Open the bulk-edit popover for one of the four actions, anchored above the
 * bulk bar. Re-clicking the same opener closes it (toggle). The popover content
 * is a pure render from bulkedit.ts; the inputs/buttons inside carry data hooks
 * a delegated listener dispatches on. F47: "pin" is a click-menu (pin all /
 * unpin all) like "priority"; "due" gains a live NL preview line that resolves
 * via /api/parse-date as you type (reusing the F12 picker's view-model).
 */
function openBulkEdit(kind: BulkEditKind): void {
  if (bulkEditOpen === kind) {
    closeBulkEdit();
    return;
  }
  closeBulkEdit();
  if (bulk.ids.size === 0) return;
  bulkEditOpen = kind;
  const pop = document.createElement("div");
  pop.className = `bulkedit-pop bulkedit-${kind}`;
  pop.setAttribute("data-bulkedit-pop", "");
  pop.setAttribute("role", "dialog");
  pop.setAttribute("aria-label", `Bulk set ${kind}`);
  // F95: the selected tasks (current priority + pin flag) so the priority/pin
  // menus can grey an option that would change nothing and say why ("all already
  // high" / "none are pinned") — the bulk-bar sister of F89's palette hints.
  const selectedTasks = selectedInOrder(bulk, visibleIds)
    .map((id) => currentTasks.find((t) => t.id === id))
    .filter((t): t is Task => t !== undefined);
  const body =
    kind === "priority"
      ? renderBulkPriorityMenu(selectedTasks)
      : kind === "pin"
        ? renderBulkPinMenu(selectedTasks)
        : kind === "tag"
          ? renderBulkTagEditor()
          : renderBulkDueEditor();
  pop.innerHTML = body;
  els.bulkbar.appendChild(pop);

  if (kind === "priority") {
    pop.addEventListener("click", (e) => {
      const btn = (e.target as HTMLElement | null)?.closest<HTMLElement>("[data-bulk-set-prio]");
      if (!btn) return;
      // F95: a greyed (no-op) option just closes the menu — no PATCH fan-out.
      if (btn.getAttribute("aria-disabled") === "true") {
        closeBulkEdit();
        return;
      }
      const prio = btn.dataset.bulkSetPrio as BulkPriority;
      bulkSetPriority(prio);
    });
  } else if (kind === "pin") {
    // F47: pin-all / unpin-all click menu.
    pop.addEventListener("click", (e) => {
      const btn = (e.target as HTMLElement | null)?.closest<HTMLElement>("[data-bulk-set-pin]");
      if (!btn) return;
      // F95: a greyed (no-op) pin/unpin just closes the menu.
      if (btn.getAttribute("aria-disabled") === "true") {
        closeBulkEdit();
        return;
      }
      bulkSetPinned(btn.dataset.bulkSetPin === "1");
    });
  } else {
    const input = pop.querySelector<HTMLInputElement>("input")!;
    input.focus();
    // F100: the live reason slot (sister of F95's priority/pin reason lines) —
    // a quiet note when the typed command would no-op the whole selection: for
    // tags "no change to any selected", for an empty due-clear "all already have
    // no due". selectedTasks is the same selection the menus use above.
    const reasonSlot = pop.querySelector<HTMLElement>("[data-bulk-reason-slot]");
    const paintReason = (raw: string): void => {
      if (!reasonSlot) return;
      const reason =
        kind === "tag"
          ? bulkTagCommandReason(selectedTasks as BulkTaskLike[], raw)
          : raw.trim() === ""
            ? bulkDueClearDisabledReason(selectedTasks as BulkTaskLike[])
            : "";
      reasonSlot.innerHTML = bulkReasonLine(reason);
    };
    paintReason(input.value); // initial (empty due -> "all already have no due")
    // F47: for the due editor, show a live preview of the resolved date below
    // the input (the same /api/parse-date the F12 picker uses), so you confirm
    // what every selected task will get before committing.
    if (kind === "due") {
      const previewLine = pop.querySelector<HTMLElement>("[data-bulk-due-preview]");
      input.addEventListener("input", () => {
        updateBulkDuePreview(input.value, previewLine);
        paintReason(input.value); // F100: refresh the no-op note as you type
      });
    } else {
      // F100: tag editor — refresh the reason note on each keystroke.
      input.addEventListener("input", () => paintReason(input.value));
    }
    input.addEventListener("keydown", (e) => {
      e.stopPropagation();
      if (e.key === "Enter") {
        e.preventDefault();
        if (kind === "tag") bulkApplyTags(input.value);
        else bulkSetDue(input.value);
      } else if (e.key === "Escape") {
        e.preventDefault();
        closeBulkEdit();
      }
    });
  }
  // Defer the outside-click guard so the opening click doesn't immediately close it.
  setTimeout(() => document.addEventListener("click", onBulkEditAway, true), 0);
}

/**
 * F47: fill the bulk-due preview line from a raw NL string. Blank -> "clears
 * the due date"; otherwise hit /api/parse-date and render the resolved date (or
 * an "unrecognized" hint), guarding against out-of-order responses with a seq.
 */
async function updateBulkDuePreview(raw: string, line: HTMLElement | null): Promise<void> {
  if (!line) return;
  const seq = ++bulkDueParseSeq;
  if (raw.trim() === "") {
    line.innerHTML = renderDuePreview(previewVM("", null));
    return;
  }
  line.innerHTML = renderDuePreview(previewVM(raw, null)); // "Parsing…"
  try {
    const res = await api.parseDate(raw);
    if (seq !== bulkDueParseSeq) return; // a newer keystroke superseded us
    line.innerHTML = renderDuePreview(previewVM(raw, res));
  } catch {
    if (seq !== bulkDueParseSeq) return;
    line.innerHTML = renderDuePreview(previewVM(raw, { ok: false, input: raw, error: "offline" }));
  }
}

/** Run a PATCH for every selected id in parallel, then refresh once. */
async function bulkPatch(
  ids: number[],
  patchFor: (t: Task) => Parameters<typeof api.patchTask>[1] | null,
  verb: string,
): Promise<void> {
  if (ids.length === 0) return;
  bulkClear();
  closeBulkEdit();
  setStatus(`${verb} ${ids.length}…`, false);
  try {
    await Promise.all(
      ids.map((id) => {
        const t = currentTasks.find((x) => x.id === id);
        if (!t) return Promise.resolve();
        const patch = patchFor(t);
        if (!patch) return Promise.resolve();
        return api.patchTask(id, patch);
      }),
    );
    await refresh();
  } catch (err) {
    await refresh();
    setStatus(`bulk ${verb} failed: ${formatErr(err)}`, true);
    setTimeout(() => setStatus("ready", false), 4_000);
  }
}

/** F36: set the same priority on every selected task. */
function bulkSetPriority(prio: BulkPriority): void {
  const ids = selectedInOrder(bulk, visibleIds);
  bulkPatch(ids, () => ({ priority: prio }), `set ${prio} on`);
}

/** F36: add/remove tags on every selected task (per-task tag-list rewrite). */
function bulkApplyTags(command: string): void {
  const ops = parseTagOps(command);
  if (isNoopTagOps(ops)) {
    closeBulkEdit();
    return;
  }
  const ids = selectedInOrder(bulk, visibleIds);
  bulkPatch(
    ids,
    (t) => {
      const next = applyTagOps(t.tags, ops);
      // Skip the PATCH when this task's tags don't actually change.
      if (next.length === t.tags.length && next.every((tag, i) => tag === t.tags[i])) {
        return null;
      }
      return { tags: next };
    },
    "tag",
  );
}

/** F36: set the same due date (natural language) on every selected task. */
function bulkSetDue(raw: string): void {
  const due = raw.trim(); // "" clears; server validates non-empty
  const ids = selectedInOrder(bulk, visibleIds);
  bulkPatch(ids, () => ({ due }), due === "" ? "clear due on" : "set due on");
}

/**
 * F47: pin or unpin every selected task. Uses the PATCH `pinned` setter (the
 * same field the F27 row pin writes), skipping any task already in the target
 * state so the round-trip only touches what changes. `pinned` round-trips to
 * .tsk.md as `pin:true` — the CLI/TUI read the same flag.
 */
function bulkSetPinned(pinned: boolean): void {
  const ids = selectedInOrder(bulk, visibleIds);
  bulkPatch(
    ids,
    (t) => (Boolean(t.pinned) === pinned ? null : { pinned }),
    pinned ? "pin" : "unpin",
  );
}

// --- F37: row context menu -------------------------------------------------

/**
 * Dispatch a per-row action by id. This is the SINGLE code path shared by the
 * context menu (F37), so the menu, the command palette, and the keyboard
 * hotkeys all converge on the same behaviour. Selecting the row first keeps the
 * keyboard cursor in sync with whatever the pointer acted on.
 */
function runRowAction(action: RowAction, id: number): void {
  nav = select(nav, visibleIds, id);
  applySelection();
  switch (action) {
    case "toggle":
      toggleTask(id);
      break;
    case "edit":
      enterEditMode(id);
      break;
    case "due":
      openDuePicker(id);
      break;
    case "notes":
      openNotesEditor(id);
      break;
    case "deps":
      openDepEditor(id);
      break;
    case "pin":
      togglePin(id);
      break;
    case "prio-up":
      cyclePriority(id, false);
      break;
    case "prio-down":
      cyclePriority(id, true);
      break;
    case "delete":
      requestDelete(id);
      break;
  }
}

/** Remove any open row context menu and drop its outside-interaction guards. */
function closeContextMenu(): void {
  document.querySelector("[data-ctxmenu]")?.remove();
  document.removeEventListener("click", onContextAway, true);
  document.removeEventListener("keydown", onContextKey, true);
  window.removeEventListener("resize", closeContextMenu);
  window.removeEventListener("scroll", closeContextMenu, true);
}

function onContextAway(e: MouseEvent): void {
  if ((e.target as HTMLElement | null)?.closest("[data-ctxmenu]")) return;
  closeContextMenu();
}

function onContextKey(e: KeyboardEvent): void {
  if (e.key === "Escape") {
    e.preventDefault();
    closeContextMenu();
  }
}

/**
 * Open the row context menu for a task at viewport coords (x, y). Built from the
 * pure renderContextMenu markup; positioned with clampMenuPosition so it never
 * spills off-screen. Outside-click, Escape, resize, and scroll all dismiss it.
 */
function openContextMenu(id: number, x: number, y: number): void {
  closeContextMenu();
  closeBulkEdit();
  const task = currentTasks.find((t) => t.id === id);
  if (!task) return;
  nav = select(nav, visibleIds, id);
  applySelection();

  const menu = document.createElement("div");
  menu.className = "ctxmenu";
  menu.setAttribute("data-ctxmenu", "");
  menu.innerHTML = renderContextMenu({ id: task.id, done: task.done, pinned: task.pinned });
  // Render off-screen first to measure, then clamp into the viewport.
  menu.style.left = "0px";
  menu.style.top = "0px";
  menu.style.visibility = "hidden";
  document.body.appendChild(menu);
  const rect = menu.getBoundingClientRect();
  const { left, top } = clampMenuPosition(
    x,
    y,
    rect.width,
    rect.height,
    window.innerWidth,
    window.innerHeight,
  );
  menu.style.left = `${left}px`;
  menu.style.top = `${top}px`;
  menu.style.visibility = "visible";

  menu.addEventListener("click", (e) => {
    const item = (e.target as HTMLElement | null)?.closest<HTMLElement>("[data-row-action]");
    if (!item) return;
    const action = item.dataset.rowAction as RowAction;
    closeContextMenu();
    runRowAction(action, id);
  });

  // Defer the guards so the opening interaction doesn't immediately close it.
  setTimeout(() => {
    document.addEventListener("click", onContextAway, true);
    document.addEventListener("keydown", onContextKey, true);
    window.addEventListener("resize", closeContextMenu);
    window.addEventListener("scroll", closeContextMenu, true);
  }, 0);
}

// --- F41: touch priority picker --------------------------------------------
// On a phone the chip's alt/shift-click "cycle down" isn't reachable, so a
// long-press on the priority chip opens a 4-way picker (tap a level to set it).
// The long-press itself is detected in the touch handlers below, reusing the
// F28 slop/threshold machine; this just owns the popover lifecycle.

/** Remove any open priority picker and drop its outside-interaction guards. */
function closePriorityPicker(): void {
  document.querySelector("[data-prio-pick]")?.remove();
  document.removeEventListener("click", onPickerAway, true);
  document.removeEventListener("keydown", onPickerKey, true);
  window.removeEventListener("resize", closePriorityPicker);
  window.removeEventListener("scroll", closePriorityPicker, true);
}

function onPickerAway(e: MouseEvent): void {
  if ((e.target as HTMLElement | null)?.closest("[data-prio-pick]")) return;
  closePriorityPicker();
}

function onPickerKey(e: KeyboardEvent): void {
  if (e.key === "Escape") {
    e.preventDefault();
    closePriorityPicker();
  }
}

/**
 * Open the 4-way priority picker for a task at viewport coords (x, y). Built
 * from the pure renderPriorityPicker markup, positioned with clampMenuPosition
 * so it never spills off-screen. Tapping an option sets the priority directly;
 * outside-tap, Escape, resize, and scroll all dismiss it.
 */
function openPriorityPicker(id: number, x: number, y: number): void {
  closePriorityPicker();
  closeContextMenu();
  const task = currentTasks.find((t) => t.id === id);
  if (!task) return;
  nav = select(nav, visibleIds, id);
  applySelection();

  const pick = document.createElement("div");
  pick.className = "prio-pick";
  pick.setAttribute("data-prio-pick", "");
  pick.innerHTML = renderPriorityPicker(task.priority);
  pick.style.left = "0px";
  pick.style.top = "0px";
  pick.style.visibility = "hidden";
  document.body.appendChild(pick);
  const rect = pick.getBoundingClientRect();
  const { left, top } = clampMenuPosition(
    x,
    y,
    rect.width,
    rect.height,
    window.innerWidth,
    window.innerHeight,
  );
  pick.style.left = `${left}px`;
  pick.style.top = `${top}px`;
  pick.style.visibility = "visible";

  pick.addEventListener("click", (e) => {
    const item = (e.target as HTMLElement | null)?.closest<HTMLElement>("[data-set-prio]");
    if (!item) return;
    const prio = item.dataset.setPrio as CyclePriority;
    closePriorityPicker();
    // F55: a tiny haptic confirms the tap on touch devices that support it.
    if (navigator.vibrate) navigator.vibrate(10);
    setPriority(id, prio);
  });

  setTimeout(() => {
    document.addEventListener("click", onPickerAway, true);
    document.addEventListener("keydown", onPickerKey, true);
    window.addEventListener("resize", closePriorityPicker);
    window.addEventListener("scroll", closePriorityPicker, true);
  }, 0);
}

// --- F17: drag-to-reorder --------------------------------------------------

/** The id of the row currently being dragged, or null. */
let draggingId: number | null = null;
/** The section key the dragged row started in (F40: drags stay in-section). */
let draggingSection: string | null = null;
/** The row currently showing a drop indicator, so we can clear it cheaply. */
let dropRow: HTMLElement | null = null;

/** Clear any drop-before/drop-after indicator from the last hovered row. */
function clearDropIndicator(): void {
  if (dropRow) {
    dropRow.classList.remove("drop-before", "drop-after");
    dropRow = null;
  }
}

/** Begin a drag: remember the row id and mark it visually. */
function onDragStart(e: DragEvent): void {
  const row = (e.target as HTMLElement | null)?.closest<HTMLElement>("[data-id]");
  if (!row) return;
  // An in-progress inline edit / due picker / notes editor / dep editor shouldn't be draggable.
  if (editing || duePicking || notesEditing || depEditing) {
    e.preventDefault();
    return;
  }
  const id = Number(row.dataset.id);
  if (!Number.isFinite(id) || id <= 0) return;
  draggingId = id;
  draggingSection = sectionOfRow(row);
  row.classList.add("is-dragging");
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = "move";
    // Some browsers require data to be set for a drag to start.
    e.dataTransfer.setData("text/plain", String(id));
  }
}

/** The section key a row belongs to (from its enclosing <section data-section>). */
function sectionOfRow(row: HTMLElement): string | null {
  return row.closest<HTMLElement>("[data-section]")?.dataset.section ?? null;
}

/** The visible ids currently rendered in a given section, in display order. */
function sectionVisibleIds(key: string): number[] {
  const sec = els.content.querySelector<HTMLElement>(`[data-section="${key}"]`);
  if (!sec) return [];
  return Array.from(sec.querySelectorAll<HTMLElement>("[data-id]")).map((r) => Number(r.dataset.id));
}

/** While dragging over a row, show which edge the drop will land against. */
function onDragOver(e: DragEvent): void {
  if (draggingId === null) return;
  const row = (e.target as HTMLElement | null)?.closest<HTMLElement>("[data-id]");
  if (!row) return;
  const overId = Number(row.dataset.id);
  if (overId === draggingId) {
    clearDropIndicator();
    return;
  }
  // F40: reordering is constrained to within a single section. A cross-section
  // drag is rejected (no drop indicator, default "no-drop" cursor) so you can't
  // drag a Pinned task into Overdue, etc.
  if (draggingSection !== null && sectionOfRow(row) !== draggingSection) {
    clearDropIndicator();
    return;
  }
  e.preventDefault(); // allow the drop
  if (e.dataTransfer) e.dataTransfer.dropEffect = "move";
  const rect = row.getBoundingClientRect();
  const pos: DropPos = dropPosForY(rect.top, rect.height, e.clientY);
  if (dropRow !== row) clearDropIndicator();
  dropRow = row;
  row.classList.toggle("drop-before", pos === "before");
  row.classList.toggle("drop-after", pos === "after");
}

/** Complete a drop: compute the new order, optimistically apply, persist. */
function onDrop(e: DragEvent): void {
  if (draggingId === null) return;
  const row = (e.target as HTMLElement | null)?.closest<HTMLElement>("[data-id]");
  if (!row) return;
  e.preventDefault();
  const targetId = Number(row.dataset.id);
  // F40: only allow a drop within the same section as the dragged row.
  const targetSection = sectionOfRow(row);
  if (draggingSection !== null && targetSection !== draggingSection) return;
  const rect = row.getBoundingClientRect();
  const pos: DropPos = dropPosForY(rect.top, rect.height, e.clientY);
  const moved = draggingId;
  const order = currentTasks.map((t) => t.id);
  // F40: a drag within the Pinned section reorders ONLY the pinned peers, with
  // the global `before` resolved so .tsk.md keeps a coherent file order. Other
  // sections still use the simpler global reorder (they already map 1:1 to file
  // order for their drags).
  const result =
    targetSection === "pinned"
      ? computeSectionReorder(order, sectionVisibleIds("pinned"), moved, targetId, pos)
      : computeReorder(order, moved, targetId, pos);
  if (result.changed) commitReorder(moved, result.before, result.order);
}

/** Reset drag visuals once the gesture ends (drop, cancel, or escape). */
function onDragEnd(): void {
  draggingId = null;
  draggingSection = null;
  clearDropIndicator();
  els.content.querySelectorAll<HTMLElement>(".is-dragging").forEach((r) => r.classList.remove("is-dragging"));
}

/**
 * Persist a reorder. Strategy: reorder currentTasks locally to match the new
 * id order (optimistic), render, then POST the move. On success, replace the
 * model with the server's authoritative ordered list. On error, refetch to
 * recover the true order.
 */
async function commitReorder(moved: number, before: number, newOrder: number[]): Promise<void> {
  const byId = new Map(currentTasks.map((t) => [t.id, t]));
  const reordered = newOrder.map((id) => byId.get(id)).filter((t): t is Task => t !== undefined);
  // Keep any tasks not in newOrder (shouldn't happen, but be safe) at the end.
  for (const t of currentTasks) if (!newOrder.includes(t.id)) reordered.push(t);
  currentTasks = reordered;
  render();
  try {
    const { tasks } = await api.moveTask(moved, before);
    currentTasks = tasks;
    render();
  } catch (err) {
    await refresh();
    setStatus(`reorder failed: ${formatErr(err)}`, true);
    setTimeout(() => setStatus("ready", false), 4_000);
  }
}

/** Re-fetch and re-render the list. Idempotent; safe to call any time. */
async function refresh(): Promise<void> {
  try {
    const { file, tasks } = await api.listTasks();
    currentTasks = tasks;
    els.file.textContent = file;
    // F101: a cohort focus is a SNAPSHOT of who waited on a chokepoint at click
    // time; an external edit (CLI / TUI / hand / another tab, surfaced via the
    // F21 SSE refresh) can complete those waiters or finish the chokepoint
    // itself. Re-derive the focus against the fresh graph so it tracks reality:
    // keep it with a live id set while it still has waiters, drop it (with a
    // quiet toast) when the whole blocked cohort is done. Without this the focus
    // would silently show a stale set — or vanish on the next manual render.
    if (focusCohort) {
      const prevSourceId = focusCohort.sourceId;
      const rec = reconcileCohort(currentTasks as DepStatsTask[], focusCohort);
      focusCohort = rec.focus;
      if (rec.cleared) {
        showInfoToast(`Cohort cleared — nothing waits on #${prevSourceId} anymore`);
      }
    }
    // F114: track the biggest-chokepoint shift independently of the stats panel
    // so the Cmd-K "Focus the new biggest chokepoint (#N, was #M)" lead command
    // works even with the sidebar closed (F111's slot only updates while the
    // panel paints). chokepointTrend returns the prior id ONLY on a real change;
    // we hold it in chokepointShiftFrom until the next refresh or until the user
    // acts on the lead, mirroring the momentary lifetime of F111's sidebar hint.
    const currBiggest = currentChokepointId();
    const shift = chokepointTrend(lastBiggestChokepoint, currBiggest);
    if (shift !== null) chokepointShiftFrom = shift;
    else if (currBiggest === null || currBiggest === lastBiggestChokepoint) {
      // Board went flat, or steadied on the same id — no pending shift to lead.
      chokepointShiftFrom = null;
    }
    lastBiggestChokepoint = currBiggest;
    setStatus("ready", false);
    render();
    refreshStats();
  } catch (err) {
    showError(err);
  }
}

// --- F13: stats sidebar ----------------------------------------------------

/** Apply the open/closed state to the DOM (panel visibility + layout + toggle). */
function applyStatsVisibility(): void {
  els.statsPanel.hidden = !statsOpen;
  els.statsToggle.classList.toggle("is-active", statsOpen);
  els.statsToggle.setAttribute("aria-pressed", String(statsOpen));
  must<HTMLElement>("[data-layout]").classList.toggle("has-stats", statsOpen);
}

/** Fetch /api/stats and paint the panel, but only when it's open. */
async function refreshStats(): Promise<void> {
  if (!statsOpen) return;
  try {
    const stats = await api.stats();
    // F46: the blocked / pinned / chain-depth metrics come from the live task
    // list the client already holds (the server stats DTO doesn't model deps),
    // so we compute them here and pass them alongside the server stats.
    const dep = computeDepStats(currentTasks as DepStatsTask[]);
    // F59: the schedule lens (due-this-week / no-due) is likewise derived from
    // the live list, relative to the client's "today".
    const sched = computeScheduleStats(currentTasks, new Date());
    // F92: the single worst bottleneck — the undone task the most others wait
    // on — surfaced once in the dep row so the biggest chokepoint is visible
    // without scanning rows for F87 badges. Computed over the same live list +
    // whole-list done-index the per-row badges use, so the two always agree.
    const choke = biggestChokepoint(
      currentTasks as DepStatsTask[],
      doneIndex(currentTasks as DepStatsTask[]),
    );
    // F106: the ranked chokepoint list so the sidebar can show the runners-up
    // ("Other bottlenecks") below the single biggest one. Same live list +
    // done-index, so every chokepoint number agrees across the UI. Capped small
    // so the section stays a tidy triage list, not a wall.
    const chokes = topChokepoints(
      currentTasks as DepStatsTask[],
      doneIndex(currentTasks as DepStatsTask[]),
      6,
    );
    // F111: a "was #M" trend hint when the biggest chokepoint CHANGED since the
    // last paint (e.g. an external edit completed the old one and F101 shifted
    // the worst bottleneck). Compare the fresh id against the slot, then update
    // the slot so the hint shows for exactly the one render where it moved.
    const currBiggest = choke ? choke.id : null;
    const trendPrev = chokepointTrend(prevBiggestChokepoint, currBiggest);
    prevBiggestChokepoint = currBiggest;
    let html = renderStatsPanel(stats, dep, sched, choke, chokes, trendPrev);
    // F80: when a lens is active, append a breakdown of the lensed subset so the
    // sidebar reflects what's actually on screen ("12 blocked: 3 urgent, …"),
    // not just whole-board totals. Computed over the same not-deleted pool the
    // render pipeline lenses, with the whole-list done-index so cross-task
    // (blocked) and time windows see the real graph.
    if (activeLens) {
      const notDeleted = currentTasks.filter((t) => !pendingDeletes.has(t.id));
      const bd = computeLensBreakdown(
        notDeleted as LensBreakdownTask[],
        activeLens,
        new Date(),
        doneIndex(notDeleted),
      );
      html += renderLensBreakdown(activeLens, bd, new Set(filter.priorities));
    }
    els.statsPanel.innerHTML = html;
  } catch {
    els.statsPanel.innerHTML = `<div class="stats-empty">Stats unavailable</div>`;
  }
}

/** Toggle the stats panel, persist the choice, and refresh its data when opening. */
function toggleStats(open: boolean): void {
  statsOpen = open;
  try {
    localStorage.setItem("tsk.stats", open ? "1" : "0");
  } catch {
    // ignore
  }
  applyStatsVisibility();
  if (open) refreshStats();
}

// --- F14: theme toggle -----------------------------------------------------

/** Apply the current theme mode to <html data-theme> + the toggle button. */
function applyTheme(): void {
  const attr = themeAttr(themeMode);
  const html = document.documentElement;
  if (attr === null) html.removeAttribute("data-theme");
  else html.setAttribute("data-theme", attr);
  els.themeGlyph.textContent = modeGlyph(themeMode);
  els.themeLabel.textContent = modeLabel(themeMode);
  els.themeToggle.title = modeTitle(themeMode);
  els.themeToggle.setAttribute("aria-label", modeTitle(themeMode));
  // Keep the document theme-color meta in sync for mobile chrome.
  const meta = document.querySelector<HTMLMetaElement>('meta[name="theme-color"]');
  if (meta) {
    const dark = themeMode === "dark" || (themeMode === "auto" && systemPrefersDark());
    meta.content = dark ? "#0b0a09" : "#fbf7ef";
  }
}

/** True when the OS currently prefers a dark color scheme. */
function systemPrefersDark(): boolean {
  return window.matchMedia?.("(prefers-color-scheme: dark)").matches ?? false;
}

/** Advance to the next theme mode, persist it, and repaint. */
function cycleTheme(): void {
  themeMode = nextMode(themeMode);
  try {
    localStorage.setItem("tsk.theme", themeMode);
  } catch {
    // ignore
  }
  applyTheme();
}

// --- F24: settings drawer --------------------------------------------------

let settingsOpen = false;

/** Mirror the current settings onto <html data-*> so CSS reacts (density etc.). */
function applySettings(): void {
  const attrs = settingsAttributes(settings);
  const html = document.documentElement;
  for (const [name, value] of Object.entries(attrs)) {
    if (value === null) html.removeAttribute(name);
    else html.setAttribute(name, value);
  }
}

/** Persist settings to localStorage (best-effort). */
function saveSettings(): void {
  try {
    localStorage.setItem(SETTINGS_KEY, serializeSettings(settings));
  } catch {
    // ignore (private mode / storage disabled)
  }
}

/** Lazily build the drawer shell (backdrop + sliding panel) once. */
function ensureSettingsEl(): HTMLElement {
  let el = document.querySelector<HTMLElement>("[data-settings]");
  if (el) return el;
  el = document.createElement("div");
  el.className = "drawer-overlay";
  el.setAttribute("data-settings", "");
  el.setAttribute("role", "dialog");
  el.setAttribute("aria-modal", "true");
  el.setAttribute("aria-label", "Settings");
  el.innerHTML = `<aside class="drawer" data-settings-panel></aside>`;
  // Backdrop click (outside the panel) closes.
  el.addEventListener("click", (e) => {
    if (e.target === el) toggleSettings(false);
  });
  // Delegated controls inside the panel.
  const panel = el.querySelector<HTMLElement>("[data-settings-panel]")!;
  panel.addEventListener("click", (e) => {
    const t = e.target as HTMLElement | null;
    if (!t) return;
    if (t.closest("[data-settings-close]")) {
      toggleSettings(false);
      return;
    }
    // F34: config management buttons.
    if (t.closest("[data-config-export]")) {
      exportConfig();
      return;
    }
    if (t.closest("[data-config-import]")) {
      importConfig();
      return;
    }
    if (t.closest("[data-config-reset]")) {
      resetConfig();
      return;
    }
    // F35: the injected Install button triggers the captured prompt.
    if (t.closest("[data-config-install]")) {
      triggerInstall();
      return;
    }
    const seg = t.closest<HTMLElement>("[data-set]");
    if (seg) {
      const key = seg.dataset.set as "density" | "motion";
      const value = seg.dataset.value ?? "";
      setSetting(key, value);
      return;
    }
    const sw = t.closest<HTMLElement>("[data-toggle-setting]");
    if (sw) {
      const key = sw.dataset.toggleSetting as "hideDone" | "showIds" | "hideMeta";
      setSetting(key, !settings[key]);
      return;
    }
  });
  document.body.appendChild(el);
  return el;
}

/** Repaint the drawer body from the current settings. */
function paintSettings(): void {
  const el = ensureSettingsEl();
  const panel = el.querySelector<HTMLElement>("[data-settings-panel]")!;
  panel.innerHTML = renderSettings(settings);
  // F35: inject an "Install app" button into the config actions when the
  // browser has offered a deferred install prompt and we're not already
  // running standalone. Kept out of the pure renderSettings so that module
  // stays free of install-prompt state.
  if (installAvailable()) {
    const actions = panel.querySelector<HTMLElement>(".set-config-actions");
    if (actions) {
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "set-cfg-btn set-cfg-install";
      btn.setAttribute("data-config-install", "");
      btn.textContent = "Install app";
      actions.insertBefore(btn, actions.firstChild);
    }
  }
}

/**
 * Apply a single setting change: update state, persist, mirror to <html>, and
 * propagate side effects (hideDone drives the live filter; the rest are pure
 * CSS via data-* attributes). Then repaint the drawer + the list.
 */
function setSetting(key: keyof Settings, value: string | boolean): void {
  switch (key) {
    case "density":
      settings.density = value === "compact" ? "compact" : "comfortable";
      break;
    case "motion":
      settings.motion = value === "reduced" ? "reduced" : "full";
      break;
    case "hideDone":
      settings.hideDone = value === true;
      // Reflect into the live filter immediately so the board updates.
      filter = { ...filter, hideDone: settings.hideDone };
      break;
    case "showIds":
      settings.showIds = value === true;
      break;
    case "hideMeta":
      settings.hideMeta = value === true;
      break;
  }
  saveSettings();
  applySettings();
  paintSettings();
  render();
  // Keep the filter bar's own hide-done pill in sync when it's visible.
  if (!els.filterbar.hidden) {
    els.filterHideDone.classList.toggle("is-active", filter.hideDone);
    els.filterHideDone.setAttribute("aria-pressed", String(filter.hideDone));
  }
}

/** Open or close the settings drawer. */
function toggleSettings(open: boolean): void {
  settingsOpen = open;
  const el = ensureSettingsEl();
  if (open) paintSettings();
  el.classList.toggle("is-open", open);
  els.settingsToggle.classList.toggle("is-active", open);
}

// --- F34: config export / import / reset -----------------------------------

/** Download the whole client config (settings + saved views + theme) as JSON. */
function exportConfig(): void {
  const bundle = buildConfig(settings, views, themeMode);
  const blob = new Blob([serializeConfig(bundle)], { type: "application/json" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = configFilename();
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
  setStatus("exported config", false);
  setTimeout(() => setStatus("ready", false), 2_000);
}

/** Prompt for a JSON file and apply it as the client config. */
function importConfig(): void {
  const input = document.createElement("input");
  input.type = "file";
  input.accept = "application/json,.json";
  input.addEventListener("change", () => {
    const file = input.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
      const result = parseConfig(String(reader.result ?? ""));
      if (!result.ok) {
        setStatus(`import failed: ${result.error}`, true);
        setTimeout(() => setStatus("ready", false), 4_000);
        return;
      }
      applyConfigBundle(result.bundle.settings, result.bundle.views, result.bundle.theme);
      setStatus("imported config", false);
      setTimeout(() => setStatus("ready", false), 2_000);
    };
    reader.readAsText(file);
  });
  input.click();
}

/** Reset all client preferences (settings + views + theme) to defaults. */
function resetConfig(): void {
  const ok =
    typeof confirm === "function"
      ? confirm("Reset all preferences and delete every saved view? This can't be undone.")
      : true;
  if (!ok) return;
  const b = resetBundle();
  applyConfigBundle(b.settings, b.views, b.theme);
  setStatus("reset to defaults", false);
  setTimeout(() => setStatus("ready", false), 2_000);
}

/** Apply an imported/reset config bundle to live state + storage + DOM. */
function applyConfigBundle(nextSettings: Settings, nextViews: SavedView[], nextTheme?: string): void {
  settings = nextSettings;
  views = nextViews;
  recalledViewId = null;
  saveSettings();
  saveViews();
  applySettings();
  // Seed the live filter's hide-done from the (possibly new) preference.
  filter = { ...filter, hideDone: settings.hideDone };
  // Apply the theme if the bundle carried one.
  if (nextTheme === "light" || nextTheme === "dark" || nextTheme === "auto") {
    themeMode = nextTheme;
    try {
      localStorage.setItem("tsk.theme", themeMode);
    } catch {
      // ignore
    }
    applyTheme();
  }
  paintSettings();
  els.filterInput.value = filter.query;
  render();
}

// --- F15: tag pages (hash routing) -----------------------------------------

/** Navigate to a tag's page by setting the URL hash (drives a hashchange). */
function navigateToTag(tag: string): void {
  const t = tag.trim().toLowerCase();
  if (!t) return;
  location.hash = tagHash(t);
}

/** Navigate back to the all-tasks view. */
function navigateToAll(): void {
  // Setting an empty hash leaves a stray "#"; clear it cleanly when we can.
  if (history.pushState) {
    history.pushState("", document.title, location.pathname + location.search);
    onRouteChange();
  } else {
    location.hash = "";
  }
}

/** React to a route change (hashchange or programmatic): re-read + repaint. */
function onRouteChange(): void {
  // F32: a programmatic hash write (recalling a view) sets this so we don't
  // double-apply the route we just established.
  if (suppressNextHashRoute) {
    suppressNextHashRoute = false;
    route = parseHash(location.hash);
    return;
  }
  route = parseHash(location.hash);
  // F32: a #view/<id> hash recalls that saved view's filter. We clear the route
  // back to "all" afterwards because a view is a filter state, not a page.
  if (route.kind === "view") {
    const id = route.id;
    route = { kind: "all" };
    if (views.some((v) => v.id === id)) {
      recallView(id);
      return;
    }
  }
  // Leaving a tag page after filtering shouldn't strand a stale selection.
  render();
}

function setStatus(label: string, error: boolean): void {
  els.dot.textContent = `// ${label}`;
  els.dot.style.color = error
    ? "var(--color-prio-urgent)"
    : "var(--color-text-faint)";
}

function showError(err: unknown): void {
  const message = formatErr(err);
  setStatus("offline", true);
  // F20: a 401 means token auth is on and this session isn't authenticated.
  // Guide the user to the ?token= bootstrap rather than showing a raw error.
  if (err instanceof ApiError && err.status === 401) {
    setStatus("locked", true);
    els.content.innerHTML = `
      <div class="banner" role="alert">
        <span>This tsk server requires a token.</span>
        <code>open http://&lt;host&gt;/?token=YOUR_TOKEN once to start a session,
or send Authorization: Bearer YOUR_TOKEN</code>
      </div>`;
    return;
  }
  els.content.innerHTML = `
    <div class="banner" role="alert">
      <span>Couldn't reach <code>tsk serve</code>:</span>
      <code>${escapeAttr(message)}</code>
    </div>`;
}

function formatErr(err: unknown): string {
  if (err instanceof ApiError) return `${err.status}: ${err.message}`;
  if (err instanceof Error) return err.message;
  return String(err);
}

function escapeAttr(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}

/**
 * Toggle a task by id. Strategy: optimistic flip in the local model, render
 * immediately, fire the server call. On success, replace the local copy with
 * the server's authoritative version (so completed timestamps appear). On
 * error, roll the flip back and surface a transient status.
 */
async function toggleTask(id: number): Promise<void> {
  if (inFlight.has(id)) return;
  const idx = currentTasks.findIndex((t) => t.id === id);
  if (idx < 0) return;
  const before = currentTasks[idx];
  // F45: completing a BLOCKED task asks for confirmation first, mirroring the
  // CLI's `done` dependency gate. The done-index is computed over the live list
  // so done/deleted blockers don't trip the guard. Re-opening never prompts.
  const doneIdx = doneIndex(currentTasks as DepTask[]);
  if (needsBlockedConfirm(before as DepTask, doneIdx)) {
    const ok =
      typeof confirm === "function"
        ? confirm(blockedToggleConfirm(before as DepTask, doneIdx))
        : true; // non-browser/test context: don't block
    if (!ok) return;
  }
  // Optimistic update
  currentTasks[idx] = { ...before, done: !before.done };
  inFlight.add(id);
  render();
  // F42: snapshot the pre-toggle dep state so that, once the server confirms a
  // COMPLETION, we can tell which OTHER tasks just lost their last open blocker
  // and offer to jump straight to one.
  const wasCompleting = !before.done;
  const depBefore = wasCompleting
    ? currentTasks.map((t) => ({ id: t.id, done: t.id === id ? false : t.done, depends_on: t.depends_on }))
    : null;
  try {
    const confirmed = await api.toggleTask(id);
    // Server is authoritative (it knows the completed timestamp etc.).
    currentTasks[idx] = confirmed;
    render();
    refreshStats();
    maybeAnnounceUnblocked(depBefore, confirmed.done);
  } catch (err) {
    // Roll back the optimistic flip and flash a status.
    currentTasks[idx] = before;
    render();
    setStatus(`toggle failed: ${formatErr(err)}`, true);
    setTimeout(() => setStatus("ready", false), 3_000);
  } finally {
    inFlight.delete(id);
  }
}

/**
 * F42: after a completion lands, see whether finishing that task cleared the
 * last open blocker of some OTHER task. If so, surface a toast ("#N is now
 * unblocked — start it?") with a "Start" action that jumps to it. `depBefore`
 * is the pre-toggle dependency snapshot (null when we were re-opening, which
 * can never unblock anything); `becameDone` guards on the toggle having
 * actually completed the task server-side.
 */
function maybeAnnounceUnblocked(depBefore: DepTask[] | null, becameDone: boolean): void {
  if (!depBefore || !becameDone) return;
  const after: DepTask[] = currentTasks.map((t) => ({
    id: t.id,
    done: t.done,
    depends_on: t.depends_on,
  }));
  const freed = newlyUnblocked(depBefore, after);
  if (freed.length === 0) return;
  // F62: a single unblock invites you straight to it ("Start"); when several
  // unblock at once, the action opens a little picker so you choose which to
  // jump to instead of always landing on the first.
  if (freed.length === 1) {
    const first = freed[0];
    showInfoToast(unblockedMessage(freed), 6, {
      label: "Start",
      run: () => jumpToTask(first),
    });
  } else {
    showInfoToast(unblockedMessage(freed), 6, {
      label: "Jump to…",
      run: () => openUnblockedPicker(freed),
    });
  }
}

/**
 * F62: open a small picker popover listing the just-unblocked tasks, each a
 * jump target. Used when more than one task unblocks from a single completion.
 * Anchored near the info toast (bottom of the viewport); outside-click /
 * Escape / a pick all dismiss it. No-ops on an empty / single-id list (the
 * single case jumps directly from the toast).
 */
function closeUnblockedPicker(): void {
  document.querySelector("[data-unblock-pop]")?.remove();
  document.removeEventListener("click", onUnblockAway, true);
  document.removeEventListener("keydown", onUnblockKey, true);
  unblockNavIds = [];
  unblockNavIndex = 0;
}

function onUnblockAway(e: MouseEvent): void {
  const t = e.target as HTMLElement | null;
  if (t?.closest("[data-unblock-pop]")) return;
  closeUnblockedPicker();
}

/**
 * F70: the unblock-picker popover's row list + the highlighted index, so
 * arrow/Home/End/Enter selection works without the mouse — the just-unblocked
 * jump is reachable straight from the keyboard, matching the palette model.
 */
let unblockNavIds: number[] = [];
let unblockNavIndex = 0;

/** Paint the highlighted unblock row + scroll it into view. */
function paintUnblockNav(): void {
  const pop = document.querySelector<HTMLElement>("[data-unblock-pop]");
  if (!pop) return;
  const btns = pop.querySelectorAll<HTMLElement>("[data-unblock-jump]");
  btns.forEach((btn, i) => {
    const on = i === unblockNavIndex;
    btn.classList.toggle("is-active", on);
    if (on) btn.scrollIntoView({ block: "nearest" });
  });
}

function onUnblockKey(e: KeyboardEvent): void {
  const action = keyToPopNavAction(e.key);
  if (action === "none") return;
  e.preventDefault();
  if (action === "close") {
    closeUnblockedPicker();
    return;
  }
  if (action === "activate") {
    const id = unblockNavIds[unblockNavIndex];
    closeUnblockedPicker();
    if (Number.isFinite(id) && id > 0) jumpToTask(id);
    return;
  }
  unblockNavIndex = nextPopNavIndex(unblockNavIndex, unblockNavIds.length, action);
  paintUnblockNav();
}

function openUnblockedPicker(ids: number[]): void {
  closeUnblockedPicker();
  if (ids.length === 0) return;
  const titleById = new Map(currentTasks.map((t) => [t.id, t.title] as const));
  const nodes: ChainNode[] = ids.map((id) => ({ id, title: titleById.get(id) ?? `#${id}` }));
  const pop = document.createElement("div");
  pop.className = "chain-pop unblock-pop";
  pop.setAttribute("data-unblock-pop", "");
  pop.innerHTML = `<div class="chain-pop-head">Newly unblocked — jump to<span class="chain-pop-keys">&#8593;&#8595; &#8629;</span></div>${renderUnblockedPicker(nodes, filter.query.trim())}`;
  pop.style.position = "fixed";
  pop.style.visibility = "hidden";
  document.body.appendChild(pop);
  const rect = pop.getBoundingClientRect();
  // Anchor above the bottom-left info toast.
  const { left, top } = clampMenuPosition(
    16,
    window.innerHeight - rect.height - 72,
    rect.width,
    rect.height,
    window.innerWidth,
    window.innerHeight,
  );
  pop.style.left = `${left}px`;
  pop.style.top = `${top}px`;
  pop.style.visibility = "visible";

  // F70: seed keyboard nav with the unblocked order, highlighting the first row.
  unblockNavIds = nodes.map((n) => n.id);
  unblockNavIndex = 0;
  paintUnblockNav();

  pop.addEventListener("click", (e) => {
    const btn = (e.target as HTMLElement | null)?.closest<HTMLElement>("[data-unblock-jump]");
    if (!btn) return;
    const id = Number(btn.dataset.unblockJump);
    closeUnblockedPicker();
    if (Number.isFinite(id) && id > 0) jumpToTask(id);
  });

  setTimeout(() => {
    document.addEventListener("click", onUnblockAway, true);
    document.addEventListener("keydown", onUnblockKey, true);
  }, 0);
}

/**
 * F27: toggle the pin flag on a task. Optimistic flip (so it jumps to/from the
 * Pinned section instantly), server confirm, rollback on error. The pin
 * round-trips to .tsk.md as `pin:true` — the same flag the CLI/TUI read.
 */
async function togglePin(id: number): Promise<void> {
  const key = -id; // distinct in-flight namespace from toggle (avoids clobber)
  if (inFlight.has(key)) return;
  const idx = currentTasks.findIndex((t) => t.id === id);
  if (idx < 0) return;
  const before = currentTasks[idx];
  currentTasks[idx] = { ...before, pinned: !before.pinned };
  inFlight.add(key);
  render();
  // Keep the just-pinned task selected so keyboard flow follows it to its new
  // section position.
  nav = select(nav, visibleIds, id);
  applySelection();
  try {
    const confirmed = await api.pinTask(id);
    currentTasks[idx] = confirmed;
    render();
  } catch (err) {
    currentTasks[idx] = before;
    render();
    setStatus(`pin failed: ${formatErr(err)}`, true);
    setTimeout(() => setStatus("ready", false), 3_000);
  } finally {
    inFlight.delete(key);
  }
}

/**
 * F44: pin a task AND float it to the very top of the file order in one
 * gesture (the `shift+P` shortcut). If the task isn't pinned yet, pin it first
 * (so it lands in the Pinned section), then persist a move to the front of the
 * file so it sits at the top of that section's hand-curated order. Idempotent:
 * an already-pinned, already-first task is a no-op.
 */
async function pinToTop(id: number): Promise<void> {
  const idx = currentTasks.findIndex((t) => t.id === id);
  if (idx < 0) return;
  // Ensure the task is pinned (togglePin only flips, so guard on current state).
  if (!currentTasks[idx].pinned) {
    await togglePin(id);
  }
  // Then move it to the very top of the file order.
  const order = currentTasks.map((t) => t.id);
  const result = computePinToTop(order, id);
  if (result.changed) {
    await commitReorder(id, result.before, result.order);
  }
  nav = select(nav, visibleIds, id);
  applySelection();
}

/**
 * F29: cycle a task's priority with an optimistic PATCH. Up by default
 * (low->medium->high->urgent->low); `down` reverses. No full edit dialog
 * needed — just click the chip (raise) or alt/shift-click (lower).
 */
async function cyclePriority(id: number, down = false): Promise<void> {
  const idx = currentTasks.findIndex((t) => t.id === id);
  if (idx < 0) return;
  const before = currentTasks[idx];
  const next = (down ? prevPriority(before.priority) : nextPriority(before.priority)) as CyclePriority;
  await setPriority(id, next);
}

/**
 * F41: set a task's priority to a specific value with an optimistic PATCH.
 * Shared by the F29 chip cycle (via cyclePriority) and the F41 touch priority
 * picker (tap a level to set it directly). A no-op when the value is unchanged.
 */
async function setPriority(id: number, next: CyclePriority): Promise<void> {
  const key = id + 1_000_000; // separate in-flight namespace
  if (inFlight.has(key)) return;
  const idx = currentTasks.findIndex((t) => t.id === id);
  if (idx < 0) return;
  const before = currentTasks[idx];
  if (before.priority === next) return; // nothing to change
  currentTasks[idx] = { ...before, priority: next };
  inFlight.add(key);
  render();
  try {
    const confirmed = await api.patchTask(id, { priority: next });
    currentTasks[idx] = confirmed;
    render();
    refreshStats();
  } catch (err) {
    currentTasks[idx] = before;
    render();
    setStatus(`priority change failed: ${formatErr(err)}`, true);
    setTimeout(() => setStatus("ready", false), 3_000);
  } finally {
    inFlight.delete(key);
  }
}

// --- F8: delete with undo --------------------------------------------------

interface PendingDelete {
  task: Task;
  timer: number;
}
/** Active undoable delete, if any. Only one at a time keeps the UX simple. */
let pending: PendingDelete | null = null;
const UNDO_SECONDS = 5;

function toastEl(): HTMLElement {
  let el = document.querySelector<HTMLElement>("[data-toast]");
  if (el) return el;
  el = document.createElement("div");
  el.className = "toast";
  el.setAttribute("data-toast", "");
  el.setAttribute("role", "status");
  el.setAttribute("aria-live", "polite");
  el.addEventListener("click", (e) => {
    const target = e.target as HTMLElement | null;
    if (target?.dataset.toastAction !== undefined) undoDelete();
  });
  document.body.appendChild(el);
  return el;
}

function hideToast(): void {
  const el = document.querySelector<HTMLElement>("[data-toast]");
  el?.classList.remove("is-open");
}

/**
 * Request an undoable delete. The task is hidden immediately and a toast with
 * an Undo button appears; only when the timer expires do we fire the actual
 * DELETE. This preserves the task's id and full fidelity if you undo (a
 * delete-then-recreate approach would lose both). If another delete is already
 * pending, it is committed first so we never drop one silently.
 */
function requestDelete(id: number): void {
  const task = currentTasks.find((t) => t.id === id);
  if (!task) return;
  // Commit any prior pending delete before starting a new one.
  if (pending) commitDelete();

  pendingDeletes.add(id);
  render();

  const el = toastEl();
  el.innerHTML = renderToast({
    message: deletedMessage(task.title),
    actionLabel: "Undo",
    seconds: UNDO_SECONDS,
  });
  el.classList.add("is-open");

  const timer = window.setTimeout(commitDelete, UNDO_SECONDS * 1_000);
  pending = { task, timer };
}

/** Fire the real DELETE for the pending task. Called on timer expiry. */
async function commitDelete(): Promise<void> {
  if (!pending) return;
  const { task, timer } = pending;
  window.clearTimeout(timer);
  pending = null;
  hideToast();
  try {
    await api.deleteTask(task.id);
    currentTasks = currentTasks.filter((t) => t.id !== task.id);
    pendingDeletes.delete(task.id);
    render();
    refreshStats();
  } catch (err) {
    // Server refused — restore the row so nothing is silently lost.
    pendingDeletes.delete(task.id);
    render();
    setStatus(`delete failed: ${formatErr(err)}`, true);
    setTimeout(() => setStatus("ready", false), 4_000);
  }
}

/** Cancel the pending delete and restore the row. */
function undoDelete(): void {
  if (!pending) return;
  window.clearTimeout(pending.timer);
  pendingDeletes.delete(pending.task.id);
  const restoredId = pending.task.id;
  pending = null;
  hideToast();
  render();
  // Re-select the restored task so keyboard flow continues naturally.
  nav = select(nav, visibleIds, restoredId);
  applySelection();
}

// --- F33: transient info toast (e.g. "file changed on disk — refreshed") -----

/** Handle for the auto-dismiss timer of the info toast. */
let infoToastTimer: number | null = null;

/**
 * F42: a one-shot action handler stashed for the info-toast's action button
 * (e.g. "Start" on the just-unblocked notice). Cleared after it fires or the
 * toast auto-dismisses, so a stale handler never runs on a later toast.
 */
let infoToastAction: (() => void) | null = null;

/** Lazily build the info-toast element (separate from the undo toast). */
function infoToastEl(): HTMLElement {
  let el = document.querySelector<HTMLElement>("[data-info-toast]");
  if (el) return el;
  el = document.createElement("div");
  el.className = "toast toast-info";
  el.setAttribute("data-info-toast", "");
  el.setAttribute("role", "status");
  el.setAttribute("aria-live", "polite");
  // F42: the optional action button dispatches the stashed one-shot handler.
  el.addEventListener("click", (e) => {
    const target = e.target as HTMLElement | null;
    if (target?.dataset.toastAction === undefined) return;
    const run = infoToastAction;
    infoToastAction = null;
    el!.classList.remove("is-open");
    if (infoToastTimer !== null) {
      window.clearTimeout(infoToastTimer);
      infoToastTimer = null;
    }
    if (run) run();
  });
  document.body.appendChild(el);
  return el;
}

/**
 * Show a brief, message-only toast that auto-dismisses. Used by the F33
 * live-reload notice. Distinct from the undo toast so a live refresh never
 * stomps a pending-delete's Undo affordance.
 *
 * F42: an optional action (label + handler) turns it into an actionable
 * notice — e.g. "#N is now unblocked — start it?" with a "Start" button that
 * jumps to the task. The action button reuses the toast's `data-toast-action`
 * hook; a one-shot handler is stashed so the click can dispatch it.
 */
function showInfoToast(
  message: string,
  seconds = 3,
  action?: { label: string; run: () => void },
): void {
  const el = infoToastEl();
  el.innerHTML = renderToast({ message, seconds, actionLabel: action?.label });
  infoToastAction = action?.run ?? null;
  el.classList.add("is-open");
  if (infoToastTimer !== null) window.clearTimeout(infoToastTimer);
  infoToastTimer = window.setTimeout(() => {
    el.classList.remove("is-open");
    infoToastTimer = null;
    infoToastAction = null;
  }, seconds * 1_000);
}

// --- F7: inline title edit -------------------------------------------------

/** True while an inline edit input is mounted, so nav keys stand down. */
let editing = false;

/**
 * Enter inline-edit mode for a task's title. Swaps the title span for an
 * input seeded with the current title; Enter or blur commits, Escape reverts.
 * Commit goes through PATCH with an optimistic update + rollback, matching the
 * toggle/add patterns.
 */
function enterEditMode(id: number): void {
  if (editing) return;
  const row = els.content.querySelector<HTMLElement>(`[data-id="${id}"]`);
  if (!row) return;
  const titleEl = row.querySelector<HTMLElement>(".title");
  if (!titleEl) return;
  const task = currentTasks.find((t) => t.id === id);
  if (!task) return;

  editing = true;
  nav = select(nav, visibleIds, id);
  applySelection();

  const input = document.createElement("input");
  input.className = "title-edit";
  input.type = "text";
  input.value = task.title;
  input.spellcheck = false;
  input.setAttribute("aria-label", "Edit task title");
  titleEl.replaceWith(input);
  input.focus();
  input.select();

  let settled = false;
  const finish = (cancelled: boolean): void => {
    if (settled) return;
    settled = true;
    editing = false;
    const outcome = resolveEdit(task.title, input.value, cancelled);
    if (outcome.kind === "commit") {
      commitEdit(id, outcome.title);
    } else {
      // noop / cancel: just re-render to restore the original row.
      render();
    }
    flushPendingLiveRefresh();
  };

  input.addEventListener("keydown", (e) => {
    e.stopPropagation(); // keep list nav keys from firing while editing
    if (e.key === "Enter") {
      e.preventDefault();
      finish(false);
    } else if (e.key === "Escape") {
      e.preventDefault();
      finish(true);
    }
  });
  input.addEventListener("blur", () => finish(false));
}

/** Persist an edited title via PATCH, optimistic with rollback. */
async function commitEdit(id: number, title: string): Promise<void> {
  const idx = currentTasks.findIndex((t) => t.id === id);
  if (idx < 0) return;
  const before = currentTasks[idx];
  currentTasks[idx] = { ...before, title };
  render();
  try {
    const confirmed = await api.patchTask(id, { title });
    currentTasks[idx] = confirmed;
    render();
  } catch (err) {
    currentTasks[idx] = before;
    render();
    setStatus(`edit failed: ${formatErr(err)}`, true);
    setTimeout(() => setStatus("ready", false), 4_000);
  }
}

// --- F12: natural-language due-date picker ---------------------------------

/** True while the due-picker popover is mounted, so list nav keys stand down. */
let duePicking = false;
/** Monotonic token so a slow parse response for stale input is ignored. */
let dueParseSeq = 0;

/**
 * Open the due-date picker for a task. Renders a small popover anchored under
 * the row with a natural-language input (today, fri, in 3d, eom, 2026-07-04),
 * quick presets, and a live "resolves to Sat, Jul 4" preview validated by the
 * server's /api/parse-date. Enter / preset-click commits via PATCH; Escape or
 * a click-away cancels. Clearing the field removes the due date.
 */
function openDuePicker(id: number): void {
  if (duePicking || editing) return;
  const row = els.content.querySelector<HTMLElement>(`[data-id="${id}"]`);
  if (!row) return;
  const task = currentTasks.find((t) => t.id === id);
  if (!task) return;

  duePicking = true;
  nav = select(nav, visibleIds, id);
  applySelection();

  const pop = document.createElement("div");
  pop.className = "due-pop";
  pop.setAttribute("data-due-pop", "");
  pop.setAttribute("role", "dialog");
  pop.setAttribute("aria-label", "Set due date");
  pop.innerHTML = `
    <div class="due-pop-row">
      <input class="due-input" data-due-input type="text" spellcheck="false"
             placeholder="today, fri, in 3d, eom, 2026-07-04…"
             aria-label="Due date (natural language)">
    </div>
    <div class="due-presets" data-due-presets>${renderPresets()}</div>
    <div class="due-preview-line" data-due-preview-line>${renderDuePreview(previewVM(task.due ?? "", task.due ? { ok: true, input: task.due, date: task.due, pretty: prettyLocal(task.due) } : null))}</div>`;
  row.appendChild(pop);

  const input = pop.querySelector<HTMLInputElement>("[data-due-input]")!;
  const previewLine = pop.querySelector<HTMLElement>("[data-due-preview-line]")!;
  input.value = task.due ?? "";
  input.focus();
  input.select();

  let settled = false;
  const close = (): void => {
    if (settled) return;
    settled = true;
    duePicking = false;
    pop.remove();
    document.removeEventListener("click", onAway, true);
    render();
    flushPendingLiveRefresh();
  };
  const commit = async (raw: string): Promise<void> => {
    const patch = resolveDueCommit(raw, task.due);
    close();
    if (patch) await commitDue(id, patch.due);
  };

  const updatePreview = async (raw: string): Promise<void> => {
    const seq = ++dueParseSeq;
    if (raw.trim() === "") {
      previewLine.innerHTML = renderDuePreview(previewVM("", null));
      return;
    }
    previewLine.innerHTML = renderDuePreview(previewVM(raw, null)); // "Parsing…"
    try {
      const res = await api.parseDate(raw);
      if (seq !== dueParseSeq) return; // a newer keystroke superseded us
      const vm: DuePreviewVM = previewVM(raw, res);
      previewLine.innerHTML = renderDuePreview(vm);
    } catch {
      if (seq !== dueParseSeq) return;
      previewLine.innerHTML = renderDuePreview(previewVM(raw, { ok: false, input: raw, error: "offline" }));
    }
  };

  input.addEventListener("input", () => updatePreview(input.value));
  input.addEventListener("keydown", (e) => {
    e.stopPropagation();
    if (e.key === "Enter") {
      e.preventDefault();
      commit(input.value);
    } else if (e.key === "Escape") {
      e.preventDefault();
      close();
    }
  });
  pop.addEventListener("click", (e) => {
    const preset = (e.target as HTMLElement | null)?.closest<HTMLElement>("[data-due-preset]");
    if (preset) {
      e.stopPropagation();
      commit(preset.dataset.duePreset ?? "");
    }
  });
  // Click outside the popover closes it (capture so it beats row handlers).
  const onAway = (e: MouseEvent): void => {
    if (!pop.contains(e.target as Node)) close();
  };
  setTimeout(() => document.addEventListener("click", onAway, true), 0);
}

/** Persist a due-date change via PATCH, optimistic with rollback. */
async function commitDue(id: number, due: string): Promise<void> {
  const idx = currentTasks.findIndex((t) => t.id === id);
  if (idx < 0) return;
  const before = currentTasks[idx];
  try {
    const confirmed = await api.patchTask(id, { due });
    currentTasks[idx] = confirmed;
    render();
    refreshStats();
  } catch (err) {
    currentTasks[idx] = before;
    render();
    setStatus(`due failed: ${formatErr(err)}`, true);
    setTimeout(() => setStatus("ready", false), 4_000);
  }
}

/** Best-effort local pretty-print of a YYYY-MM-DD for the picker's seed preview. */
function prettyLocal(due: string): string {
  const [y, m, d] = due.split("-").map((n) => parseInt(n, 10));
  if (!y || !m || !d) return due;
  return new Date(y, m - 1, d).toLocaleDateString(undefined, {
    weekday: "short",
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

// --- F23: notes editor -----------------------------------------------------

/** True while the notes panel is mounted, so list nav keys stand down. */
let notesEditing = false;

/**
 * Open the multi-line notes editor for a task. Mounts an expanding panel under
 * the row with a textarea seeded from the task's notes. Cmd/Ctrl-Enter or the
 * Save button commits via PATCH (optimistic + rollback, matching the title/due
 * patterns); Escape or a click-away cancels. An empty textarea clears the notes
 * (the store drops the continuation lines). Round-trips the `.tsk.md` 6-space
 * continuation block through the existing `notes` PATCH field.
 */
function openNotesEditor(id: number): void {
  if (notesEditing || editing || duePicking) return;
  const row = els.content.querySelector<HTMLElement>(`[data-id="${id}"]`);
  if (!row) return;
  const task = currentTasks.find((t) => t.id === id);
  if (!task) return;

  notesEditing = true;
  nav = select(nav, visibleIds, id);
  applySelection();

  const original = task.notes ?? "";
  const panel = document.createElement("div");
  panel.className = "notes-pop";
  panel.setAttribute("data-notes-pop", "");
  panel.setAttribute("role", "dialog");
  panel.setAttribute("aria-label", "Edit notes");
  panel.innerHTML = `
    <textarea class="notes-area" data-notes-area spellcheck="true"
              placeholder="Notes… markdown-ish, multi-line. Saved as indented lines under the task in .tsk.md."
              aria-label="Task notes"></textarea>
    <div class="notes-foot">
      <span class="notes-hint"><kbd>&#8984;&#9166;</kbd> save &middot; <kbd>esc</kbd> cancel</span>
      <span class="notes-foot-actions">
        <button class="notes-cancel" data-notes-cancel type="button">Cancel</button>
        <button class="notes-save" data-notes-save type="button">Save</button>
      </span>
    </div>`;
  row.appendChild(panel);

  const area = panel.querySelector<HTMLTextAreaElement>("[data-notes-area]")!;
  area.value = original;
  area.focus();
  // Put the caret at the end rather than selecting everything.
  area.setSelectionRange(area.value.length, area.value.length);
  autoGrow(area);

  let settled = false;
  const close = (): void => {
    if (settled) return;
    settled = true;
    notesEditing = false;
    panel.remove();
    document.removeEventListener("click", onAway, true);
    render();
    flushPendingLiveRefresh();
  };
  const save = (): void => {
    const outcome = resolveNotes(original, area.value);
    close();
    if (outcome.kind === "commit") commitNotes(id, outcome.notes);
  };

  area.addEventListener("input", () => autoGrow(area));
  area.addEventListener("keydown", (e) => {
    e.stopPropagation(); // keep list nav keys from firing while editing
    if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
      e.preventDefault();
      save();
    } else if (e.key === "Escape") {
      e.preventDefault();
      close();
    }
  });
  panel.addEventListener("click", (e) => {
    const t = e.target as HTMLElement | null;
    if (t?.closest("[data-notes-save]")) {
      e.stopPropagation();
      save();
    } else if (t?.closest("[data-notes-cancel]")) {
      e.stopPropagation();
      close();
    }
  });
  const onAway = (e: MouseEvent): void => {
    if (!panel.contains(e.target as Node)) save(); // click-away saves, like a doc editor
  };
  setTimeout(() => document.addEventListener("click", onAway, true), 0);
}

/** Grow a textarea to fit its content (cheap auto-resize, capped by CSS max-height). */
function autoGrow(area: HTMLTextAreaElement): void {
  area.style.height = "auto";
  area.style.height = `${area.scrollHeight}px`;
}

/** Persist a notes change via PATCH, optimistic with rollback. */
async function commitNotes(id: number, notes: string): Promise<void> {
  const idx = currentTasks.findIndex((t) => t.id === id);
  if (idx < 0) return;
  const before = currentTasks[idx];
  currentTasks[idx] = { ...before, notes };
  render();
  try {
    const confirmed = await api.patchTask(id, { notes });
    currentTasks[idx] = confirmed;
    render();
  } catch (err) {
    currentTasks[idx] = before;
    render();
    setStatus(`notes failed: ${formatErr(err)}`, true);
    setTimeout(() => setStatus("ready", false), 4_000);
  }
}

// --- F39: dependency editor (add/remove blockers) --------------------------

/** True while the dependency editor popover is mounted. */
let depEditing = false;

/** Project currentTasks down to the minimal graph shape the editor needs. */
function depGraph(): DepGraphTask[] {
  return currentTasks.map((t) => ({
    id: t.id,
    title: t.title,
    done: t.done,
    depends_on: t.depends_on,
  }));
}

/**
 * F39: open the "blocked by" editor for a task. Shows the current blockers as
 * removable chips and an add-input with a candidate dropdown (filtered to
 * acyclic, non-self, not-yet-added tasks). Adds/removes mutate a local working
 * set; each change PATCHes depends_on (optimistic + rollback like the other
 * editors). Self-refs and cycles are refused client-side before any request.
 */
function openDepEditor(id: number): void {
  if (depEditing || editing || duePicking || notesEditing) return;
  const row = els.content.querySelector<HTMLElement>(`[data-id="${id}"]`);
  if (!row) return;
  if (!currentTasks.find((t) => t.id === id)) return;

  depEditing = true;
  closeContextMenu();
  nav = select(nav, visibleIds, id);
  applySelection();

  const pop = document.createElement("div");
  pop.className = "depedit-pop";
  pop.setAttribute("data-depedit-pop", "");
  pop.setAttribute("role", "dialog");
  pop.setAttribute("aria-label", "Edit blockers");
  pop.innerHTML = renderDepEditor(depGraph(), id, walkableBlockers());
  row.appendChild(pop);

  const input = pop.querySelector<HTMLInputElement>("[data-dep-input]")!;
  const acList = pop.querySelector<HTMLElement>("[data-dep-ac]")!;
  let candIndex = 0;
  let candidates: DepGraphTask[] = [];
  input.focus();

  let settled = false;
  const close = (): void => {
    if (settled) return;
    settled = true;
    depEditing = false;
    pop.remove();
    document.removeEventListener("click", onAway, true);
    render();
    flushPendingLiveRefresh();
  };

  /** Repaint just the chips + empty-state after a working-set change. */
  const repaintChips = (): void => {
    const fresh = renderDepEditor(depGraph(), id, walkableBlockers());
    // Replace everything above the input row by re-rendering and swapping the
    // chips region. Simplest: re-render the whole pop body, re-grab refs.
    pop.innerHTML = fresh;
    wire();
  };

  const paintCandidates = (): void => {
    const ac = pop.querySelector<HTMLElement>("[data-dep-ac]")!;
    ac.innerHTML = renderDepCandidates(candidates, candIndex, walkableCandidates());
    ac.hidden = candidates.length === 0;
  };

  /**
   * F74: which of the current candidates have their own open-blocker chain
   * worth previewing, so renderDepCandidates can show a "walk chain" button only
   * on those. Computed over the live graph each paint.
   */
  const walkableCandidates = (): Set<number> => {
    const graph = currentTasks as DepStatsTask[];
    const set = new Set<number>();
    for (const c of candidates) {
      if (hasWalkableChain(graph, c.id)) set.add(c.id);
    }
    return set;
  };

  /**
   * F79: which of the task's CURRENT blockers chain further into their own open
   * blockers, so renderDepEditor can show a "walk chain" button on those chips —
   * the sister of F74's affordance on the candidate list. Computed over the live
   * graph each repaint so it stays correct as blockers are added/removed. A
   * hoisted function (not a const) so the initial render above can call it.
   */
  function walkableBlockers(): Set<number> {
    const graph = currentTasks as DepStatsTask[];
    const set = new Set<number>();
    for (const dep of currentDeps(depGraph(), id)) {
      if (hasWalkableChain(graph, dep)) set.add(dep);
    }
    return set;
  }

  const refreshCandidates = (): void => {
    candidates = depCandidates(depGraph(), id, input.value);
    candIndex = Math.min(candIndex, Math.max(0, candidates.length - 1));
    paintCandidates();
  };

  const addDep = async (dep: number): Promise<void> => {
    const check = validateAddDep(depGraph(), id, dep);
    if (!check.ok) {
      setStatus(check.message, true);
      setTimeout(() => setStatus("ready", false), 3_000);
      return;
    }
    const next = withDepAdded(currentDeps(depGraph(), id), dep);
    input.value = "";
    await commitDeps(id, next);
    repaintChips();
    refreshCandidates();
  };

  const removeDep = async (dep: number): Promise<void> => {
    const next = withDepRemoved(currentDeps(depGraph(), id), dep);
    await commitDeps(id, next);
    repaintChips();
    refreshCandidates();
  };

  /** (Re)bind the listeners after a body re-render. */
  function wire(): void {
    const freshInput = pop.querySelector<HTMLInputElement>("[data-dep-input]")!;
    freshInput.focus();
    freshInput.addEventListener("input", () => {
      candIndex = 0;
      candidates = depCandidates(depGraph(), id, freshInput.value);
      paintCandidates();
    });
    freshInput.addEventListener("keydown", (e) => {
      e.stopPropagation();
      if (e.key === "ArrowDown" && candidates.length) {
        e.preventDefault();
        candIndex = moveIndex(candIndex, candidates.length, 1);
        paintCandidates();
      } else if (e.key === "ArrowUp" && candidates.length) {
        e.preventDefault();
        candIndex = moveIndex(candIndex, candidates.length, -1);
        paintCandidates();
      } else if (e.key === "Enter") {
        e.preventDefault();
        // Prefer the highlighted candidate; else parse a bare #id / id.
        const chosen = candidates[candIndex];
        if (chosen) {
          addDep(chosen.id);
        } else {
          const n = parseInt(freshInput.value.replace(/^#/, "").trim(), 10);
          if (Number.isFinite(n) && n > 0) addDep(n);
        }
      } else if (e.key === "Escape") {
        e.preventDefault();
        close();
      }
    });
    pop.querySelectorAll<HTMLElement>("[data-dep-remove]").forEach((btn) => {
      btn.addEventListener("click", (e) => {
        e.stopPropagation();
        removeDep(Number(btn.dataset.depRemove));
      });
    });
    // F79: the "walk chain" button on a CURRENT blocker chip opens the chain
    // drill for that blocker's deepest open-blocker path — so you can audit what
    // an already-added blocker chains into. stopPropagation so it doesn't bubble
    // to the away-click that closes the editor.
    pop.querySelectorAll<HTMLElement>("[data-dep-chip-walk]").forEach((btn) => {
      btn.addEventListener("click", (e) => {
        e.stopPropagation();
        e.preventDefault();
        const wid = Number(btn.dataset.depChipWalk);
        if (Number.isFinite(wid) && wid > 0) openChainDrill(wid);
      });
    });
    const ac = pop.querySelector<HTMLElement>("[data-dep-ac]")!;
    ac.addEventListener("mousedown", (e) => {
      // F74: the "walk chain" button previews a candidate's blocker chain
      // without adding it; handle it first so it doesn't fall through to add.
      const walk = (e.target as HTMLElement | null)?.closest<HTMLElement>("[data-dep-walk]");
      if (walk) {
        e.preventDefault();
        const wid = Number(walk.dataset.depWalk);
        if (Number.isFinite(wid) && wid > 0) openChainDrill(wid);
        return;
      }
      const item = (e.target as HTMLElement | null)?.closest<HTMLElement>("[data-dep-cand]");
      if (!item) return;
      e.preventDefault();
      addDep(Number(item.dataset.depCand));
    });
  }

  // Initial wire + candidates.
  void acList;
  wire();
  candIndex = 0;
  refreshCandidates();

  const onAway = (e: MouseEvent): void => {
    if (!pop.contains(e.target as Node)) close();
  };
  setTimeout(() => document.addEventListener("click", onAway, true), 0);
}

/** Persist a new depends_on set via PATCH, optimistic with rollback (F39). */
async function commitDeps(id: number, deps: number[]): Promise<void> {
  const idx = currentTasks.findIndex((t) => t.id === id);
  if (idx < 0) return;
  const before = currentTasks[idx];
  currentTasks[idx] = { ...before, depends_on: deps.length ? deps : undefined };
  // Don't full-render (would tear down the open popover); just refresh stats.
  refreshStats();
  try {
    const confirmed = await api.patchTask(id, { depends_on: deps });
    currentTasks[idx] = confirmed;
  } catch (err) {
    currentTasks[idx] = before;
    setStatus(`blockers failed: ${formatErr(err)}`, true);
    setTimeout(() => setStatus("ready", false), 4_000);
  }
}

// --- F6: quick-add composer ------------------------------------------------
/** Re-render the live token preview and toggle the submit-enabled state. */
function updateComposerPreview(): void {
  const parsed = parseQuickAdd(els.input.value);
  els.preview.innerHTML = renderComposerPreview(parsed);
  els.field.classList.toggle("can-submit", isSubmittable(parsed));
}

// --- F38: composer autocomplete (#tag + @due) ------------------------------

/** Current suggestion list + highlighted index for the composer dropdown. */
let acSuggestions: Suggestion[] = [];
let acIndex = 0;
let acOpen = false;

/** True while the autocomplete dropdown is showing suggestions. */
function isAutocompleteOpen(): boolean {
  return acOpen;
}

/** Hide + reset the composer autocomplete dropdown. */
function closeAutocomplete(): void {
  if (!acOpen && els.composerAc.hidden) return;
  acOpen = false;
  acSuggestions = [];
  acIndex = 0;
  els.composerAc.hidden = true;
  els.composerAc.innerHTML = "";
  els.input.setAttribute("aria-expanded", "false");
}

/** Recompute suggestions for the token under the caret and paint the dropdown. */
function refreshAutocomplete(): void {
  const caret = els.input.selectionStart ?? els.input.value.length;
  const token = activeToken(els.input.value, caret);
  const tags = collectTags(currentTasks).map((t) => ({ tag: t.tag, count: t.count }));
  acSuggestions = suggestFor(token, tags);
  if (acSuggestions.length === 0) {
    closeAutocomplete();
    return;
  }
  acOpen = true;
  acIndex = clampIndex(acIndex, acSuggestions.length);
  paintAutocomplete();
  els.input.setAttribute("aria-expanded", "true");
}

/** Repaint the dropdown list + active row. */
function paintAutocomplete(): void {
  els.composerAc.hidden = false;
  els.composerAc.innerHTML = renderAutocomplete(acSuggestions, acIndex);
  const active = els.composerAc.querySelector<HTMLElement>(".is-active");
  active?.scrollIntoView({ block: "nearest" });
}

/** Insert the highlighted (or given) suggestion into the input at its token. */
function acceptAutocomplete(index = acIndex): void {
  const sugg = acSuggestions[index];
  if (!sugg) return;
  const caret = els.input.selectionStart ?? els.input.value.length;
  const token = activeToken(els.input.value, caret);
  if (!token) {
    closeAutocomplete();
    return;
  }
  const { text, caret: nextCaret } = applySuggestion(els.input.value, token, sugg.value);
  els.input.value = text;
  els.input.setSelectionRange(nextCaret, nextCaret);
  closeAutocomplete();
  updateComposerPreview();
  // Re-run in case the caret now sits in a new token (rare, e.g. chained).
  refreshAutocomplete();
}

/**
 * Submit the composer: parse the inline syntax, POST the task, then refresh.
 * The input clears immediately (optimistic) so you can keep typing the next
 * task without waiting on the round-trip. On error we restore the text and
 * flash a status so nothing is lost.
 *
 * F38: a multi-line value (a paste of several lines) is split into N tasks and
 * added sequentially, so you can dump a checklist and get N rows.
 */
async function submitComposer(): Promise<void> {
  const raw = els.input.value;
  closeAutocomplete();

  // F38: multi-line paste -> one task per non-blank line (list markers stripped).
  if (isMultiLine(raw)) {
    await submitMultiLine(raw);
    return;
  }

  const parsed = parseQuickAdd(raw);
  if (!isSubmittable(parsed)) return;

  els.input.value = "";
  updateComposerPreview();
  setStatus("adding…", false);
  try {
    await api.createTask({
      title: parsed.title,
      priority: parsed.priority,
      due: parsed.due,
      tags: parsed.tags.length ? parsed.tags : undefined,
      depends_on: parsed.dependsOn.length ? parsed.dependsOn : undefined,
    });
    await refresh();
  } catch (err) {
    // Restore what they typed so a typo'd due date isn't lost.
    els.input.value = raw;
    updateComposerPreview();
    setStatus(`add failed: ${formatErr(err)}`, true);
    setTimeout(() => setStatus("ready", false), 4_000);
    els.input.focus();
  }
}

/**
 * F38: add one task per line of a multi-line paste. Each line goes through the
 * same inline token grammar, so "- buy milk #shop" still parses its tag. Runs
 * sequentially (not parallel) so ids stay in paste order and a later
 * depends:#N can reference an earlier line by its known id. Reports a count.
 */
async function submitMultiLine(raw: string): Promise<void> {
  const lines = splitPasteLines(raw).filter((l) => isSubmittable(parseQuickAdd(l)));
  if (lines.length === 0) return;
  els.input.value = "";
  updateComposerPreview();
  setStatus(`adding ${lines.length} tasks…`, false);
  let added = 0;
  try {
    for (const line of lines) {
      const p = parseQuickAdd(line);
      await api.createTask({
        title: p.title,
        priority: p.priority,
        due: p.due,
        tags: p.tags.length ? p.tags : undefined,
        depends_on: p.dependsOn.length ? p.dependsOn : undefined,
      });
      added++;
    }
    await refresh();
    setStatus(`added ${added} tasks`, false);
    setTimeout(() => setStatus("ready", false), 2_500);
  } catch (err) {
    await refresh();
    setStatus(`added ${added}/${lines.length}, then failed: ${formatErr(err)}`, true);
    setTimeout(() => setStatus("ready", false), 5_000);
  }
}

els.input.addEventListener("input", () => {
  updateComposerPreview();
  refreshAutocomplete(); // F38: live #tag / @due suggestions
});
// F38: clicking a suggestion inserts it.
els.composerAc.addEventListener("mousedown", (e) => {
  // mousedown (not click) so the input doesn't blur-and-close first.
  const item = (e.target as HTMLElement | null)?.closest<HTMLElement>("[data-ac-value]");
  if (!item) return;
  e.preventDefault();
  const i = Array.from(els.composerAc.children).indexOf(item);
  if (i >= 0) acceptAutocomplete(i);
});
// Close the dropdown when the input loses focus (slight delay for the click).
els.input.addEventListener("blur", () => setTimeout(closeAutocomplete, 120));
els.composer.addEventListener("submit", (e) => {
  e.preventDefault();
  submitComposer();
});

// --- F11: filter bar wiring ------------------------------------------------

/** Update filter state and repaint. Selection is reconciled inside render(). */
function setFilter(next: Partial<FilterState>): void {
  filter = { ...filter, ...next };
  // F97: when a lens is active, a facet change is part of the lens+facet drill
  // (F81), so persist it alongside the lens — a reload restores the whole combo.
  if (activeLens) persistLens();
  render();
}

/**
 * F66: set (or clear, with null) the single active render-pipeline lens. Driven
 * by clicking a stats tile (Blocked / Overdue / Due today / Due this week / No
 * due → sets it) and the filter-bar lens chip / clear-all (clears it). Exactly
 * one lens is active at a time. Re-renders through the normal pipeline.
 *
 * F93: persists the choice per-tab in sessionStorage so a reload restores the
 * lens (it's time-relative, so sessionStorage — not localStorage — is the right
 * scope: it shouldn't leak across sessions). persistLens centralizes the write
 * so the clear paths (clear-all, jump-to-task) stay in sync too.
 */
function setLens(kind: LensKind | null): void {
  if (activeLens === kind) return;
  activeLens = kind;
  // F96: a lens and a cohort focus are mutually exclusive narrowings — setting
  // a lens drops any active cohort so the two never stack confusingly.
  if (kind !== null && focusCohort !== null) {
    focusCohort = null;
    cohortHistory = []; // F108: dropping the cohort drops its history
  }
  persistLens();
  render();
}

/**
 * F96: focus the board on the cohort of undone tasks waiting on `sourceId` (a
 * chokepoint). Built from the live graph via cohort.buildCohort; a no-op (with a
 * brief status note) when nothing waits on it. Mutually exclusive with a lens
 * (clears it) since both are "special" post-filter narrowings. Re-renders through
 * the normal pipeline. NOT persisted — the id set is a momentary snapshot.
 */
function setCohort(sourceId: number): void {
  const cohort = buildCohort(currentTasks as DepStatsTask[], sourceId);
  if (cohort === null) {
    setStatus(`nothing waits on #${sourceId}`, false);
    setTimeout(() => setStatus("ready", false), 2_000);
    return;
  }
  // F108: record the outgoing focus's source on the back-stack so a later "back"
  // (Escape / the chip's ‹ glyph) returns to it. Re-focusing the SAME cohort
  // (no change) doesn't push, and switching to the cohort we'd step back to
  // anyway is de-duped by pushCohortHistory's top-of-stack guard.
  if (focusCohort !== null && focusCohort.sourceId !== sourceId) {
    cohortHistory = pushCohortHistory(cohortHistory, focusCohort.sourceId);
  }
  focusCohort = cohort;
  if (activeLens !== null) {
    activeLens = null; // mutually exclusive with a lens
    persistLens();
  }
  render();
  setStatus(`focused ${cohort.ids.length} waiting on #${sourceId}`, false);
  setTimeout(() => setStatus("ready", false), 2_000);
}

/** F96: clear any active cohort focus and repaint. */
function clearCohort(): void {
  if (focusCohort === null) return;
  focusCohort = null;
  cohortHistory = []; // F108: dropping the focus drops its history
  render();
}

/**
 * F108: step the cohort back-stack one level — return to the most recent
 * ancestor cohort that still has live waiters (popCohortHistory skips ancestors
 * whose chokepoint completed or whose waiters all finished). When the stack
 * empties with no live ancestor, this clears the focus entirely (same as
 * clearCohort). A no-op when there's no history to step back through, so the
 * caller (Escape) can fall through to its other Escape behaviours. Returns true
 * when it handled a back-step (focus changed), false when there was nothing to
 * do — letting the keydown handler decide whether to consume the event.
 */
function cohortBack(): boolean {
  if (focusCohort === null || cohortHistory.length === 0) return false;
  const back = popCohortHistory(currentTasks as DepStatsTask[], cohortHistory);
  cohortHistory = back.stack;
  focusCohort = back.focus;
  if (back.focus === null) {
    setStatus("cohort history cleared", false);
  } else {
    setStatus(`back to ${back.focus.ids.length} waiting on #${back.focus.sourceId}`, false);
  }
  setTimeout(() => setStatus("ready", false), 2_000);
  render();
  return true;
}

/**
 * F103: the id of the current biggest chokepoint (the undone task the most
 * others wait on), or null on a flat board. Computed over the same live list +
 * whole-list done-index the F92 sidebar line uses, so the Cmd-K "Focus biggest
 * chokepoint" command targets exactly what the sidebar shows. Kept as a small
 * helper so buildCommands stays declarative.
 */
function currentChokepointId(): number | null {
  const choke = biggestChokepoint(
    currentTasks as DepStatsTask[],
    doneIndex(currentTasks as DepStatsTask[]),
  );
  return choke ? choke.id : null;
}

/**
 * F107: the ranked top-N chokepoints (biggest first), over the same live list +
 * whole-list done-index the F92 sidebar line + F106 "Other bottlenecks" list
 * use, so the Cmd-K "Focus chokepoint #N" commands target exactly what the
 * sidebar shows. Capped to match refreshStats' sidebar cap (6) so the palette
 * never offers a focus command for a chokepoint the sidebar doesn't list.
 */
function currentChokepoints(): Chokepoint[] {
  return topChokepoints(
    currentTasks as DepStatsTask[],
    doneIndex(currentTasks as DepStatsTask[]),
    6,
  );
}

/** F93: mirror the active lens (or its absence) into sessionStorage.
 *
 * F97: also persist the priority FACET drilled on top of the lens (the F81
 * breakdown-pill drill: lens AND urgent), so a reload restores the WHOLE
 * lens+facet combo, not just the lens. Written only while a lens is active;
 * cleared whenever the lens clears (so a plain facet with no lens keeps its
 * existing non-persisted behaviour). Called from setLens + every clear path.
 */
function persistLens(): void {
  try {
    if (activeLens) {
      sessionStorage.setItem(LENS_KEY, activeLens);
      // F97: persist the priority facet riding the lens (or drop the key when
      // the drill is empty) so the restored board matches what you left.
      if (filter.priorities.length > 0) {
        sessionStorage.setItem(LENS_FACET_KEY, serializeLensFacet(filter.priorities));
      } else {
        sessionStorage.removeItem(LENS_FACET_KEY);
      }
    } else {
      sessionStorage.removeItem(LENS_KEY);
      sessionStorage.removeItem(LENS_FACET_KEY); // F97: no lens -> no drilled facet
    }
  } catch {
    // ignore (private mode / storage disabled) — lens still works in-session
  }
}

// --- F25: saved views ------------------------------------------------------

/** Extract the saveable filter slice (query + facets + hide-done) from state. */
function currentViewFilter(): ViewFilter {
  return {
    query: filter.query,
    priorities: filter.priorities,
    tags: filter.tags,
    hideDone: filter.hideDone,
  };
}

/** Persist the views list to localStorage (best-effort). */
function saveViews(): void {
  try {
    localStorage.setItem(VIEWS_KEY, serializeViews(views));
  } catch {
    // ignore (private mode / storage disabled)
  }
}

/** Repaint the saved-views chip row + the enabled state of "save view". */
function renderViewsRow(): void {
  const f = currentViewFilter();
  const lens: string | null = activeLens; // F104: the live lens half of the drill
  const hasViews = views.length > 0;
  // The row shows whenever there are saved views OR the current filter/lens is
  // worth saving — otherwise it stays out of the way. F104: a lens alone (no
  // facet) is savable too (a pure-lens view).
  const savable = !filterIsEmpty(f) || lens !== null;
  els.viewsRow.hidden = !hasViews && !savable;
  // F32/F104: if the last-recalled view still exists but its saved filter OR
  // lens no longer matches the live state, that chip becomes "updatable" so you
  // can save your tweaks back onto it. The match is lens-aware so toggling the
  // lens off (or to another) marks the recalled view as diverged.
  const recalled = recalledViewId ? views.find((v) => v.id === recalledViewId) : undefined;
  const recalledMatches =
    recalled &&
    filtersEqual(recalled.filter, f) &&
    (recalled.lens ?? null) === lens;
  const updatableId = recalled && !recalledMatches && savable ? recalled.id : null;
  els.viewsChips.innerHTML = renderViewChips(views, f, {
    draggable: true,
    updatableId,
    liveLens: lens, // F104: lens-aware active-chip highlight
    // F112: give a pure-lens bookmark (F110's one-click pin) the lens's own
    // glyph so it reads as a "lens bookmark" vs a filter bookmark. parseLens
    // validates the stored kind (a stale/garbage lens -> no glyph) before
    // lensMeta resolves it, keeping the chip robust to a hand-poked store.
    lensGlyph: (kind) => {
      const k = parseLens(kind);
      return k ? lensMeta(k).glyph : "";
    },
  });
  // Disable "save view" when there's nothing to save, or the exact filter+lens
  // combo is already saved (activeViewWithLens non-null means an identical
  // view, lens included, exists).
  const dup = activeViewWithLens(views, f, lens) !== null;
  els.viewsSave.disabled = !savable || dup;
  els.viewsSave.textContent = dup ? "saved" : "+ save view";
}

/** Prompt for a name and save the current filter as a view.
 *
 * F104: also capture the active render-pipeline lens (if any) so a lens+facet
 * drill (F81/F97) — e.g. "Urgent (overdue)" — can be saved as one named view
 * and recalled whole. A lens makes the view savable even when the plain filter
 * is otherwise empty (a pure-lens view like "blocked" is meaningful). The lens
 * kind is stored as a string on the view; recallView validates + re-applies it.
 */
function saveCurrentView(): void {
  const f = currentViewFilter();
  const lens = activeLens ?? undefined; // F104: capture the lens half of the drill
  if (filterIsEmpty(f) && !lens) {
    setStatus("nothing to save — set a filter or lens first", true);
    setTimeout(() => setStatus("ready", false), 3_000);
    return;
  }
  // F104: suggest a name that names the lens drill, e.g. "urgent (overdue)".
  const suggested = lens ? defaultViewName(f, lens) : "";
  const name = typeof prompt === "function" ? prompt("Name this view:", suggested) : null;
  if (name === null) return; // cancelled
  const trimmed = name.trim();
  if (trimmed === "") return;
  views = addView(views, trimmed, f, lens);
  saveViews();
  render();
  setStatus(`saved view "${trimmed}"`, false);
  setTimeout(() => setStatus("ready", false), 2_000);
}

/**
 * F110: one-click "pin this lens" — save the active render-pipeline lens as a
 * pure-lens view (empty filter + the lens), named after the lens, with NO
 * prompt. The common case (a frequently-used lens you want as a recallable chip)
 * shouldn't need the F104 name dialog. Idempotent: if this lens is already
 * pinned (findPureLensView), recall that view instead of saving a duplicate, so
 * the star doubles as a quick "jump to my pinned lens". A no-op when no lens is
 * active (the button is hidden then anyway).
 */
function pinCurrentLens(): void {
  if (activeLens === null) return;
  const existing = findPureLensView(views, activeLens);
  if (existing) {
    recallView(existing.id); // already pinned — recall it (no duplicate)
    return;
  }
  const name = pureLensViewName(lensMeta(activeLens).label);
  // Pin = a pure-lens view: empty filter, the lens captured. addView allows an
  // empty filter precisely because a lens is supplied (F104).
  views = addView(views, name, emptyFilter(), activeLens);
  saveViews();
  render();
  setStatus(`pinned lens "${name}"`, false);
  setTimeout(() => setStatus("ready", false), 2_000);
}

/**
 * F104: a sensible default name for a lens+facet view — the facet priorities
 * (if any) with the lens label in parentheses, e.g. "urgent (overdue)" or just
 * "(blocked)" for a pure-lens view. Keeps the saved-view name self-describing so
 * the chip row reads as a set of named drills.
 */
function defaultViewName(f: ViewFilter, lens: LensKind): string {
  const label = lensMeta(lens).label;
  const facet = f.priorities.length ? f.priorities.join("/") : "";
  return facet ? `${facet} (${label})` : `(${label})`;
}

/** Recall a saved view by id: apply its filter and repaint. */
function recallView(id: string): void {
  const v = views.find((x) => x.id === id);
  if (!v) return;
  // Make sure the filter bar is reachable + the search box reflects the query.
  filter = {
    ...emptyFilter(),
    query: v.filter.query,
    priorities: [...v.filter.priorities],
    tags: [...v.filter.tags],
    hideDone: v.filter.hideDone,
  };
  els.filterInput.value = v.filter.query;
  // F104: re-apply the captured lens half of the drill. A view that saved a
  // lens (F81/F97 lens+facet combo) restores BOTH the facet (above) and the
  // lens here; parseLens validates the stored kind against LENS_ORDER so a
  // stale/garbage value degrades to "no lens" rather than wedging the board. A
  // filter-only view (no v.lens) clears any active lens so recall is clean.
  activeLens = parseLens(v.lens ?? null);
  persistLens();
  // F32: remember which view is active (drives the "update" affordance) and
  // reflect it into the URL hash so the view is shareable/bookmarkable.
  recalledViewId = id;
  const want = viewHash(id);
  if (location.hash !== want) {
    suppressNextHashRoute = true;
    location.hash = want;
  }
  render();
  setStatus(`view: ${v.name}`, false);
  setTimeout(() => setStatus("ready", false), 1_500);
}

/** F32: overwrite a saved view's filter with the live filter, then repaint.
 *
 * F104: also re-capture the live lens so updating a recalled lens+facet view
 * keeps both halves in sync (toggling the lens off updates it to filter-only).
 */
function updateViewToCurrent(id: string): void {
  const f = currentViewFilter();
  const lens = activeLens ?? undefined;
  if (filterIsEmpty(f) && !lens) return;
  const v = views.find((x) => x.id === id);
  views = updateView(views, id, f, lens);
  saveViews();
  recalledViewId = id; // it now matches again
  render();
  setStatus(`updated view${v ? ` "${v.name}"` : ""}`, false);
  setTimeout(() => setStatus("ready", false), 2_000);
}

/** F32: persist a drag-reorder of the saved-view chips. */
function reorderViews(movedId: string, beforeId: string | null): void {
  const next = moveView(views, movedId, beforeId);
  if (next === views) return; // no-op drop
  views = next;
  saveViews();
  render();
}

/** Forget a saved view by id. */
function deleteView(id: string): void {
  views = removeView(views, id);
  if (recalledViewId === id) recalledViewId = null;
  saveViews();
  render();
}

// Free-text search box: debounced not needed at this scale, filter on input.
els.filterInput.addEventListener("input", () => {
  setFilter({ query: els.filterInput.value });
});
els.filterInput.addEventListener("keydown", (e) => {
  if (e.key === "Escape") {
    e.stopPropagation();
    if (els.filterInput.value !== "") {
      els.filterInput.value = "";
      setFilter({ query: "" });
    } else {
      els.filterInput.blur();
    }
  }
});

// Priority pills: toggle membership on click (delegated).
els.filterPrios.addEventListener("click", (e) => {
  const target = e.target as HTMLElement | null;
  // F99: the "in <lens>" scope note clears the LENS (keeping the priority facet)
  // so you can lift a lens-scoped drill to the whole board in one click.
  if (target?.closest("[data-lens-scope-clear]")) {
    setLens(null);
    return;
  }
  const btn = target?.closest<HTMLElement>("[data-prio]");
  if (!btn) return;
  const prio = btn.dataset.prio as Priority;
  setFilter({ priorities: toggleMember(filter.priorities, prio) });
});

// Tag chips: toggle membership on click (delegated).
els.filterTags.addEventListener("click", (e) => {
  const target = e.target as HTMLElement | null;
  // F99: the tag-row "in <lens>" scope note clears the LENS (keeping the tag
  // facet) — sister of the priority-row note above.
  if (target?.closest("[data-lens-scope-clear]")) {
    setLens(null);
    return;
  }
  const btn = target?.closest<HTMLElement>("[data-tag]");
  if (!btn) return;
  const tag = btn.dataset.tag ?? "";
  setFilter({ tags: toggleMember(filter.tags, tag) });
});

// Hide-done toggle.
els.filterHideDone.addEventListener("click", () => {
  setFilter({ hideDone: !filter.hideDone });
});

// F66: the active-lens chip clears the lens when clicked.
els.filterBlocked.addEventListener("click", () => {
  setLens(null);
});

// F110: the "pin this lens" star — one click saves a pure-lens view named after
// the lens (no prompt), or recalls the existing pinned view if the lens is
// already pinned. Never creates a duplicate.
els.filterLensPin.addEventListener("click", () => {
  pinCurrentLens();
});

// F96: the cohort-focus chip clears the focus when clicked.
// F108: …unless the click landed on the leading ‹ back glyph, which steps back
// to the previous cohort in the drill instead of clearing outright.
els.filterCohort.addEventListener("click", (e) => {
  const target = e.target as HTMLElement | null;
  if (target?.closest("[data-cohort-back]")) {
    cohortBack();
    return;
  }
  clearCohort();
});

// Clear-all affordance resets every facet.
els.filterClear.addEventListener("click", () => {
  els.filterInput.value = "";
  filter = emptyFilter();
  activeLens = null; // F66: clear the active lens too
  focusCohort = null; // F96: clear any cohort focus too
  cohortHistory = []; // F108: and its back-stack
  persistLens(); // F93: forget the persisted lens on a full clear
  recalledViewId = null; // F32: dropping the filter forgets the active view
  render();
  els.filterInput.focus();
});

// --- F25: saved-views wiring -----------------------------------------------

els.viewsSave.addEventListener("click", saveCurrentView);
els.viewsChips.addEventListener("click", (e) => {
  const target = e.target as HTMLElement | null;
  const del = target?.closest<HTMLElement>("[data-view-del]");
  if (del) {
    e.stopPropagation();
    deleteView(del.dataset.viewDel ?? "");
    return;
  }
  // F32: the circular-arrow button overwrites the saved view with the live filter.
  const upd = target?.closest<HTMLElement>("[data-view-update]");
  if (upd) {
    e.stopPropagation();
    updateViewToCurrent(upd.dataset.viewUpdate ?? "");
    return;
  }
  const recall = target?.closest<HTMLElement>("[data-view-recall]");
  if (recall) {
    recallView(recall.dataset.viewRecall ?? "");
  }
});

// F32: drag-to-reorder the saved-view chips. Delegated on the chip row so it
// survives re-renders. The drop computes the `before` chip and persists.
let draggingViewId: string | null = null;
els.viewsChips.addEventListener("dragstart", (e) => {
  const chip = (e.target as HTMLElement | null)?.closest<HTMLElement>("[data-view-id]");
  if (!chip) return;
  draggingViewId = chip.dataset.viewId ?? null;
  chip.classList.add("is-dragging");
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = "move";
    e.dataTransfer.setData("text/plain", draggingViewId ?? "");
  }
});
els.viewsChips.addEventListener("dragover", (e) => {
  if (draggingViewId === null) return;
  e.preventDefault();
  if (e.dataTransfer) e.dataTransfer.dropEffect = "move";
});
els.viewsChips.addEventListener("drop", (e) => {
  if (draggingViewId === null) return;
  e.preventDefault();
  const chip = (e.target as HTMLElement | null)?.closest<HTMLElement>("[data-view-id]");
  // Drop onto a chip -> land before it (left half) or after (right half).
  // Drop onto empty row space -> move to the end (beforeId = null).
  let beforeId: string | null = null;
  if (chip && chip.dataset.viewId !== draggingViewId) {
    const rect = chip.getBoundingClientRect();
    const after = e.clientX > rect.left + rect.width / 2;
    if (after) {
      const sibling = chip.nextElementSibling as HTMLElement | null;
      beforeId = sibling?.dataset.viewId ?? null;
    } else {
      beforeId = chip.dataset.viewId ?? null;
    }
  }
  reorderViews(draggingViewId, beforeId);
  draggingViewId = null;
});
els.viewsChips.addEventListener("dragend", () => {
  draggingViewId = null;
  els.viewsChips.querySelectorAll(".is-dragging").forEach((c) => c.classList.remove("is-dragging"));
});

// --- F13: stats sidebar wiring ---------------------------------------------

els.statsToggle.addEventListener("click", () => toggleStats(!statsOpen));
// F24: the gear opens the settings drawer.
els.settingsToggle.addEventListener("click", () => toggleSettings(!settingsOpen));
// Clicking a top-tag row drives the F11 tag filter (and opens the filter view).
els.statsPanel.addEventListener("click", (e) => {
  const target = e.target as HTMLElement | null;
  // F96: the chokepoint "focus" button narrows the board to exactly the undone
  // tasks waiting on #N (a cohort focus), so you can act on the blocked cohort.
  const cohortBtn = target?.closest<HTMLElement>("[data-cohort-focus]");
  if (cohortBtn) {
    const sourceId = Number(cohortBtn.dataset.cohortFocus);
    if (Number.isFinite(sourceId) && sourceId > 0) setCohort(sourceId);
    return;
  }
  // F92/F85: the chokepoint line itself opens the dependent chain-drill popover
  // (direction "dependent") so you can WALK what waits on #N. (The row-badge
  // variant is handled in the content listener; the sidebar needs its own wire.)
  const waitingWalk = target?.closest<HTMLElement>("[data-waiting-walk]");
  if (waitingWalk) {
    const sourceId = Number(waitingWalk.dataset.waitingWalk);
    if (Number.isFinite(sourceId) && sourceId > 0) openChainDrill(sourceId, "dependent");
    return;
  }
  // F81: clicking a priority pill in the F80 lens breakdown layers that priority
  // onto the active lens (lens AND urgent), turning the sidebar readout into a
  // drill-down. Toggling the same pill again clears the facet. Routes through
  // setFilter so the chip + repaint happen exactly as a filter-bar pill would.
  const bdPill = target?.closest<HTMLElement>("[data-lens-bd-prio]");
  if (bdPill) {
    const prio = lensBreakdownPriority(bdPill.dataset.lensBdPrio ?? "");
    if (prio) setFilter({ priorities: toggleMember(filter.priorities, prio) });
    return;
  }
  // F56: clicking the "chain depth" tile opens the longest blocker chain as a
  // jump-list so you can walk #downstream -> ... -> #root blocker.
  if (target?.closest("[data-chain-drill]")) {
    openChainDrill();
    return;
  }
  // F66/F69: clicking a metric tile drives a render-pipeline lens. The "open"
  // tile is special — it maps to the real hide-done FACET (which serializes into
  // views), so it routes through setFilter; every other tile sets the matching
  // cross-task / time lens. Clicking the same lens's tile again is a no-op (the
  // chip clears it); the page already shows that subset.
  const lensTile = target?.closest<HTMLElement>("[data-lens-drill]");
  if (lensTile) {
    const lens = lensTile.dataset.lensDrill ?? "";
    if (lens === "open") {
      setLens(null); // the hide-done facet and a lens are mutually exclusive views
      setFilter({ hideDone: true });
    } else if (lens === "blocked" || lens === "overdue" || lens === "today" || lens === "week" || lens === "nodue") {
      setLens(lens as LensKind);
    }
    return;
  }
  const row = target?.closest<HTMLElement>("[data-stat-tag]");
  if (!row) return;
  const tag = row.dataset.statTag ?? "";
  if (!tag) return;
  setFilter({ tags: filter.tags.includes(tag) ? filter.tags : [...filter.tags, tag] });
});

// --- F16: bulk action bar wiring -------------------------------------------

els.bulkbar.addEventListener("click", (e) => {
  const btn = (e.target as HTMLElement | null)?.closest<HTMLElement>("button");
  if (!btn) return;
  // F36: the priority/tag/due openers toggle their popover.
  if (btn.dataset.bulkEdit !== undefined) {
    openBulkEdit(btn.dataset.bulkEdit as "priority" | "tag" | "due" | "pin");
    return;
  }
  if (btn.dataset.bulkToggle !== undefined) bulkToggleDone();
  else if (btn.dataset.bulkDelete !== undefined) bulkDelete();
  else if (btn.dataset.bulkClear !== undefined) bulkClear();
});

// --- F19: export menu wiring -----------------------------------------------

let exportOpen = false;
/**
 * F84: which scope segment the export menu's "all / shown" toggle is on. Only
 * meaningful while scoping is active; defaults to "shown" (the F75 auto-scope)
 * each time the menu opens so the common case (export what you see) stays one
 * click, while "All" is now reachable too — mirroring the palette's split.
 *
 * F88: the choice is persisted per-session in sessionStorage so a user who
 * always wants "All" isn't re-toggling on every open. Seeded from storage at
 * boot (default "shown" when unset), and re-read on each open so the toggle
 * sticks within the session but resets cleanly on a new tab/session.
 */
let exportScopeShown = parseExportScope(sessionStorage.getItem(EXPORT_SCOPE_KEY));

/** Show/hide the export dropdown, painting its items lazily on first open. */
function toggleExportMenu(open: boolean): void {
  exportOpen = open;
  els.exportMenu.hidden = !open;
  els.exportToggle.setAttribute("aria-expanded", String(open));
  els.exportToggle.classList.toggle("is-active", open);
  if (open) {
    // F75/F84: rebuild each open so the header reflects the live lens/filter
    // scope. F88: re-read the persisted scope choice so it sticks across opens
    // within the session (no longer force-reset to "shown" each time).
    exportScopeShown = parseExportScope(sessionStorage.getItem(EXPORT_SCOPE_KEY));
    paintExportMenu();
  }
}

/**
 * F84: (re)render the export menu body for the current scope + toggle choice.
 * Split out from toggleExportMenu so flipping the "all / shown" segment can
 * repaint in place without closing the menu.
 */
function paintExportMenu(): void {
  const scopedCount = isExportScoped() ? visibleIds.length : null;
  els.exportMenu.innerHTML = renderExportMenu(scopedCount, exportScopeShown);
}

/**
 * Trigger a download of the task list in the chosen format. Uses a temporary
 * anchor with the download attribute so the browser saves the file (the server
 * also sets Content-Disposition as a belt-and-suspenders). Closes the menu.
 *
 * F75: when a lens / filter / tag-route is narrowing the board, export only the
 * subset you SEE (the visible ids, in store order) rather than the whole store,
 * so "what you see is what you get". With nothing active it's the full export.
 *
 * F78: `forceScope` lets the palette's explicit commands override the auto
 * behaviour — `true` always exports the visible subset, `false` always exports
 * the whole store, `undefined` keeps the F75 auto-scope (used by the export
 * menu + the legacy whole-store commands).
 */
function downloadExport(format: ExportFormat, forceScope?: boolean): void {
  const scoped = forceScope ?? isExportScoped();
  const href = scoped ? scopedExportUrl(format, visibleIds) : exportUrl(format);
  const a = document.createElement("a");
  a.href = href;
  a.download = exportFilename(format);
  document.body.appendChild(a);
  a.click();
  a.remove();
  toggleExportMenu(false);
  setStatus(scoped ? `exported ${format} (${visibleIds.length} shown)` : `exported ${format}`, false);
  setTimeout(() => setStatus("ready", false), 2_000);
}

/**
 * F75: is the export currently scoped to a visible subset? True when a stats
 * lens, a search/facet filter, or a tag route is narrowing the board — the same
 * conditions that make `visibleIds` a strict subset of the store.
 */
function isExportScoped(): boolean {
  return activeLens !== null || focusCohort !== null || isFilterActive(filter) || route.kind === "tag";
}

els.exportToggle.addEventListener("click", (e) => {
  e.stopPropagation();
  toggleExportMenu(!exportOpen);
});
els.exportMenu.addEventListener("click", (e) => {
  const target = e.target as HTMLElement | null;
  // F84: the "all / shown" segmented toggle flips the menu's scope in place.
  const seg = target?.closest<HTMLElement>("[data-export-scope-toggle]");
  if (seg) {
    exportScopeShown = seg.dataset.exportScopeToggle === "shown";
    // F88: persist the choice for the session so it survives reopening.
    sessionStorage.setItem(EXPORT_SCOPE_KEY, serializeExportScope(exportScopeShown));
    paintExportMenu();
    return;
  }
  const item = target?.closest<HTMLElement>("[data-export-format]");
  if (!item) return;
  // F84: honour the item's advertised scope — "shown" forces the visible subset,
  // "all" forces the whole store — so the toggle reaches both targets just like
  // the palette's two-command split.
  const force = item.dataset.exportScope === "shown" ? true : item.dataset.exportScope === "all" ? false : undefined;
  downloadExport(item.dataset.exportFormat as ExportFormat, force);
});
// Click anywhere outside closes the menu.
document.addEventListener("click", (e) => {
  if (exportOpen && !els.exportWrap.contains(e.target as Node | null)) {
    toggleExportMenu(false);
  }
});

// --- F14: theme toggle wiring ----------------------------------------------

els.themeToggle.addEventListener("click", cycleTheme);

// When in auto mode, repaint the theme-color meta if the OS preference flips.
window.matchMedia?.("(prefers-color-scheme: dark)").addEventListener?.("change", () => {
  if (themeMode === "auto") applyTheme();
});

// --- F15: tag-page wiring ---------------------------------------------------

window.addEventListener("hashchange", onRouteChange);

// "all tasks" link in the tag-page banner clears the route.
els.tagpageClear.addEventListener("click", (e) => {
  e.preventDefault();
  navigateToAll();
});

// Escape clears + blurs the composer.
els.input.addEventListener("keydown", (e) => {
  // F38: when the autocomplete dropdown is open, it owns the arrow/enter/tab keys.
  if (isAutocompleteOpen()) {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      acIndex = moveIndex(acIndex, acSuggestions.length, 1);
      paintAutocomplete();
      return;
    }
    if (e.key === "ArrowUp") {
      e.preventDefault();
      acIndex = moveIndex(acIndex, acSuggestions.length, -1);
      paintAutocomplete();
      return;
    }
    if (e.key === "Enter" || e.key === "Tab") {
      e.preventDefault();
      acceptAutocomplete();
      return;
    }
    if (e.key === "Escape") {
      e.preventDefault();
      closeAutocomplete();
      return;
    }
  }
  if (e.key === "Escape") {
    els.input.value = "";
    updateComposerPreview();
    closeAutocomplete();
    els.input.blur();
  }
});

/**
 * Event delegation: a single listener on the content container catches every
 * checkbox flip in the list. Cheap and immune to row re-renders.
 */
els.content.addEventListener("change", (e) => {
  const target = e.target as HTMLElement | null;
  if (!target || !(target instanceof HTMLInputElement)) return;
  if (target.dataset.toggle === undefined) return;
  const row = target.closest("[data-id]") as HTMLElement | null;
  if (!row) return;
  const id = Number(row.dataset.id);
  if (!Number.isFinite(id) || id <= 0) return;
  toggleTask(id);
});

// Clicking a row selects it (so mouse + keyboard stay in sync). The delete
// button (data-del) requests an undoable delete instead of selecting.
els.content.addEventListener("click", (e) => {
  const target = e.target as HTMLElement | null;
  if (!target || target instanceof HTMLInputElement) return;
  const row = target.closest("[data-id]") as HTMLElement | null;
  if (!row) return;
  const id = Number(row.dataset.id);
  if (!Number.isFinite(id) || id <= 0) return;
  // F16: modifier-clicks drive bulk selection instead of normal nav select.
  // Shift = range from anchor; cmd/ctrl = toggle one. Suppress text selection.
  if (e.shiftKey) {
    e.preventDefault();
    bulkSelectRange(id);
    return;
  }
  if (e.metaKey || e.ctrlKey) {
    e.preventDefault();
    bulkToggleOne(id);
    return;
  }
  if (target.closest("[data-del]")) {
    requestDelete(id);
    return;
  }
  // F37: the "⋯" overflow button opens the row context menu, anchored to it.
  const menuBtn = target.closest<HTMLElement>("[data-row-menu]");
  if (menuBtn) {
    e.preventDefault();
    const rect = menuBtn.getBoundingClientRect();
    openContextMenu(id, rect.left, rect.bottom + 4);
    return;
  }
  // F27: the pin star toggles the sticky flag (floats to/from Pinned section).
  if (target.closest("[data-pin]")) {
    togglePin(id);
    return;
  }
  // F29: clicking the priority chip cycles it up; shift/alt-click cycles down.
  if (target.closest("[data-prio-cycle]")) {
    cyclePriority(id, e.shiftKey || e.altKey);
    return;
  }
  // F61: the small chain button on the "blocked by" badge opens the chain-drill
  // popover for THIS task's deepest blocker path (checked before data-dep-jump
  // since it's the more specific affordance inside the badge group).
  const chainFrom = target.closest<HTMLElement>("[data-chain-from]");
  if (chainFrom) {
    const fromId = Number(chainFrom.dataset.chainFrom);
    if (Number.isFinite(fromId) && fromId > 0) openChainDrill(fromId);
    return;
  }
  // F87: the "N waiting" chokepoint badge opens the dependent chain-drill
  // popover (F85, direction "dependent") so you can see WHAT waits on this task.
  const waitingWalk = target.closest<HTMLElement>("[data-waiting-walk]");
  if (waitingWalk) {
    const fromId = Number(waitingWalk.dataset.waitingWalk);
    if (Number.isFinite(fromId) && fromId > 0) openChainDrill(fromId, "dependent");
    return;
  }
  // F26: the "blocked by #N" badge jumps to (selects + scrolls to) the blocker.
  const depJump = target.closest<HTMLElement>("[data-dep-jump]");
  if (depJump) {
    const blockerId = Number(depJump.dataset.depJump);
    if (Number.isFinite(blockerId) && blockerId > 0) jumpToTask(blockerId);
    return;
  }
  // F15: a row tag pill navigates to that tag's page.
  const tagnav = target.closest<HTMLElement>("[data-tagnav]");
  if (tagnav) {
    navigateToTag(tagnav.dataset.tagnav ?? "");
    return;
  }
  if (target.closest("[data-due]")) {
    openDuePicker(id);
    return;
  }
  if (target.closest("[data-notes]")) {
    openNotesEditor(id);
    return;
  }
  nav = select(nav, visibleIds, id);
  applySelection();
});

// Double-click a title to edit it in place (F7).
els.content.addEventListener("dblclick", (e) => {
  const target = e.target as HTMLElement | null;
  if (!target || !target.closest(".title")) return;
  const row = target.closest("[data-id]") as HTMLElement | null;
  if (!row) return;
  const id = Number(row.dataset.id);
  if (!Number.isFinite(id) || id <= 0) return;
  e.preventDefault();
  enterEditMode(id);
});

// F17: drag-to-reorder. Delegated on the content container so it survives
// row re-renders. The whole row is draggable; the handle is just an affordance.
els.content.addEventListener("dragstart", onDragStart);
els.content.addEventListener("dragover", onDragOver);
els.content.addEventListener("drop", onDrop);
els.content.addEventListener("dragend", onDragEnd);
els.content.addEventListener("dragleave", (e) => {
  // Only clear when actually leaving the content area, not crossing rows.
  if (!els.content.contains(e.relatedTarget as Node | null)) clearDropIndicator();
});

// F37: right-click a row opens the context menu at the pointer. We suppress the
// native menu on rows only (so the browser menu still works elsewhere). Clicks
// on the inline editors / inputs keep their default context menu.
els.content.addEventListener("contextmenu", (e) => {
  const target = e.target as HTMLElement | null;
  if (!target) return;
  if (target.closest("input, textarea, [data-due-pop], [data-notes-pop], [data-depedit-pop]")) return;
  const row = target.closest<HTMLElement>("[data-id]");
  if (!row) return;
  const id = Number(row.dataset.id);
  if (!Number.isFinite(id) || id <= 0) return;
  // F55: a right-click landing ON the priority chip opens the 4-way priority
  // picker at the pointer (desktop parity with the F41 touch long-press), so
  // the chip has the same menu the row's F37 context menu exposes for priority.
  if (target.closest("[data-prio-cycle]")) {
    e.preventDefault();
    openPriorityPicker(id, e.clientX, e.clientY);
    return;
  }
  e.preventDefault();
  openContextMenu(id, e.clientX, e.clientY);
});

// --- F28: long-press to bulk-select on touch devices ------------------------
// Touch has no hover/right-click, so a still long-press (~500ms) on a row is
// the mobile gesture for "select this for bulk actions". A press that moves
// (a scroll) or lifts early (a tap) is NOT a long-press.
// F41: a long-press that STARTS on the priority chip instead opens the 4-way
// priority picker (the chip's desktop alt-click isn't reachable on a phone).
let press: PressState | null = null;

function cancelPress(): void {
  if (press) {
    window.clearTimeout(press.timer);
    press = null;
  }
}

els.content.addEventListener(
  "touchstart",
  (e) => {
    if (e.touches.length !== 1) {
      cancelPress();
      return;
    }
    const target = e.target as HTMLElement | null;
    if (!target) return;
    const t = e.touches[0];
    // F41: a long-press starting on the priority chip opens the picker. Checked
    // BEFORE the generic interactive-control bail (the chip is a <button>).
    const chip = target.closest<HTMLElement>("[data-prio-cycle]");
    if (chip) {
      const row = chip.closest("[data-id]") as HTMLElement | null;
      if (!row) return;
      const id = Number(row.dataset.id);
      if (!Number.isFinite(id) || id <= 0) return;
      const rect = chip.getBoundingClientRect();
      const ax = rect.left;
      const ay = rect.bottom + 4;
      const timer = window.setTimeout(() => {
        openPriorityPicker(id, ax, ay);
        if (navigator.vibrate) navigator.vibrate(15);
        press = null;
      }, LONG_PRESS_MS);
      press = { id, start: { x: t.clientX, y: t.clientY }, moved: false, timer };
      return;
    }
    // Don't hijack a press that starts on an interactive control or the drag
    // handle — those have their own behaviour.
    if (
      target.closest(
        "input, button, a, textarea, [data-drag-handle], [data-due], [data-notes]",
      )
    ) {
      return;
    }
    const row = target.closest("[data-id]") as HTMLElement | null;
    if (!row) return;
    const id = Number(row.dataset.id);
    if (!Number.isFinite(id) || id <= 0) return;
    const timer = window.setTimeout(() => {
      // Fire the long-press: enter bulk mode by toggling this row in.
      bulkToggleOne(id);
      if (navigator.vibrate) navigator.vibrate(15);
      press = null;
    }, LONG_PRESS_MS);
    press = { id, start: { x: t.clientX, y: t.clientY }, moved: false, timer };
  },
  { passive: true },
);

els.content.addEventListener(
  "touchmove",
  (e) => {
    if (!press || e.touches.length !== 1) return;
    const t = e.touches[0];
    press.moved = trackMove(press, { x: t.clientX, y: t.clientY });
    if (press.moved) cancelPress(); // it's a scroll, not a press
  },
  { passive: true },
);

els.content.addEventListener("touchend", cancelPress, { passive: true });
els.content.addEventListener("touchcancel", cancelPress, { passive: true });

// Pick up external edits (CLI / TUI / hand-edit) when the tab regains focus.
document.addEventListener("visibilitychange", () => {
  if (document.visibilityState === "visible") {
    refresh();
  }
});

// --- F10: keyboard navigation ----------------------------------------------

function isTypingTarget(el: EventTarget | null): boolean {
  const t = el as HTMLElement | null;
  return !!t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.isContentEditable);
}

/** Move the keyboard selection and repaint just the selection state. */
function navMove(dir: NavMove): void {
  nav = move(nav, visibleIds, dir);
  applySelection();
}

/**
 * F56: open the longest open-blocker chain as a jump-list popover anchored on
 * the stats panel. Each step jumps to (selects + scrolls to) that task, so you
 * can walk from the most-downstream blocked task to its deepest root blocker.
 * No-ops when the graph is flat. Outside-click / Escape / a jump all dismiss it.
 */
function closeChainDrill(): void {
  document.querySelector("[data-chain-pop]")?.remove();
  document.removeEventListener("click", onChainAway, true);
  document.removeEventListener("keydown", onChainKey, true);
  chainNavIds = [];
  chainNavIndex = 0;
}

function onChainAway(e: MouseEvent): void {
  const t = e.target as HTMLElement | null;
  // Don't close on a click inside the popover, the stats tile, or a row's F61
  // chain-from badge (those toggle / re-open it themselves).
  if (
    t?.closest("[data-chain-pop]") ||
    t?.closest("[data-chain-drill]") ||
    t?.closest("[data-chain-from]")
  )
    return;
  closeChainDrill();
}

/**
 * F70: the chain-drill popover's row list (in render order) + the highlighted
 * index, so arrow/Home/End/Enter selection works without the mouse — matching
 * the palette's keyboard model.
 */
let chainNavIds: number[] = [];
let chainNavIndex = 0;

/** Paint the highlighted chain row + scroll it into view. */
function paintChainNav(): void {
  const pop = document.querySelector<HTMLElement>("[data-chain-pop]");
  if (!pop) return;
  const btns = pop.querySelectorAll<HTMLElement>("[data-chain-jump]");
  btns.forEach((btn, i) => {
    const on = i === chainNavIndex;
    btn.classList.toggle("is-active", on);
    if (on) btn.scrollIntoView({ block: "nearest" });
  });
}

function onChainKey(e: KeyboardEvent): void {
  const action = keyToPopNavAction(e.key);
  if (action === "none") return;
  e.preventDefault();
  if (action === "close") {
    closeChainDrill();
    return;
  }
  if (action === "activate") {
    const id = chainNavIds[chainNavIndex];
    closeChainDrill();
    if (Number.isFinite(id) && id > 0) jumpToTask(id);
    return;
  }
  chainNavIndex = nextPopNavIndex(chainNavIndex, chainNavIds.length, action);
  paintChainNav();
}

/**
 * F56/F61/F85: open the chain-drill popover. Direction:
 *   - "blocker" (default): walk DOWNSTREAM — what blocks this (or the global
 *     longest blocker chain when no fromId), #this -> ... -> root blocker.
 *   - "dependent": walk UPSTREAM — what WAITS on this, #this -> ... -> the most
 *     downstream waiter (F85). Only meaningful with a fromId.
 * When a fromId is given, the popover header carries a flip button so you can
 * audit impact in BOTH directions from one popover without reopening it.
 */
function openChainDrill(fromId?: number, dir: "blocker" | "dependent" = "blocker"): void {
  closeChainDrill();
  // The path depends on direction. Blocker: F56 global longest (no id) or F61
  // deepest-from-id. Dependent: F85 deepest dependent chain from the id.
  const path =
    dir === "dependent" && fromId !== undefined
      ? deepestDependentChainFrom(currentTasks as DepStatsTask[], fromId)
      : fromId !== undefined
        ? deepestChainFrom(currentTasks as DepStatsTask[], fromId)
        : longestChainPath(currentTasks as DepStatsTask[]);
  if (path.length === 0) return;
  const titleById = new Map(currentTasks.map((t) => [t.id, t.title] as const));
  const nodes: ChainNode[] = path.map((id) => ({ id, title: titleById.get(id) ?? `#${id}` }));
  const pop = document.createElement("div");
  pop.className = "chain-pop";
  pop.setAttribute("data-chain-pop", "");
  // F85: the header names the direction; with a fromId it also offers a flip
  // button to the OTHER direction (downstream blockers <-> upstream dependents),
  // shown only when that other direction actually has something to walk.
  const head =
    fromId === undefined
      ? "Longest blocker chain"
      : dir === "dependent"
        ? `Waiting on #${fromId}`
        : `Blocker chain from #${fromId}`;
  let flip = "";
  if (fromId !== undefined) {
    const other: "blocker" | "dependent" = dir === "dependent" ? "blocker" : "dependent";
    const otherHasPath =
      other === "dependent"
        ? hasWalkableDependents(currentTasks as DepStatsTask[], fromId)
        : hasWalkableChain(currentTasks as DepStatsTask[], fromId);
    if (otherHasPath) {
      const flipLabel = other === "dependent" ? "what waits on this?" : "what blocks this?";
      flip = `<button type="button" class="chain-pop-flip" data-chain-flip="${other}" title="${flipLabel}" aria-label="${flipLabel}">&#8646;</button>`;
    }
  }
  // F102: in the "what waits on this?" (dependent) direction, offer a "focus
  // these" button that narrows the board to the cohort of undone tasks waiting
  // on #fromId — so you can ACT on any chokepoint you're walking (not just the
  // single biggest one F96 surfaces in the sidebar), reusing the F96 cohort
  // wiring via the shared data-cohort-focus hook. Only meaningful when fromId
  // actually has open waiters to focus.
  const focusBtn =
    dir === "dependent" &&
    fromId !== undefined &&
    buildCohort(currentTasks as DepStatsTask[], fromId) !== null
      ? renderCohortFocusButton(fromId)
      : "";
  pop.innerHTML = `<div class="chain-pop-head">${head}${flip}${focusBtn}<span class="chain-pop-keys">&#8593;&#8595; &#8629;</span></div>${renderChainDrill(nodes, filter.query.trim())}`;
  // Anchor under the chain tile if present (F56), else the badge that opened it
  // (F61/F85/F87), else the stats panel corner. For the dependent direction the
  // opener may be the F87 "N waiting" badge (data-waiting-walk) rather than the
  // F61 blocker-chain badge (data-chain-from), so try both.
  const tile = els.statsPanel.querySelector<HTMLElement>("[data-chain-drill]");
  const badge =
    fromId !== undefined
      ? els.content.querySelector<HTMLElement>(`[data-chain-from="${fromId}"]`) ??
        els.content.querySelector<HTMLElement>(`[data-waiting-walk="${fromId}"]`)
      : null;
  const anchor = (badge ?? tile ?? els.statsPanel).getBoundingClientRect();
  pop.style.position = "fixed";
  pop.style.visibility = "hidden";
  document.body.appendChild(pop);
  const rect = pop.getBoundingClientRect();
  const { left, top } = clampMenuPosition(
    anchor.left,
    anchor.bottom + 4,
    rect.width,
    rect.height,
    window.innerWidth,
    window.innerHeight,
  );
  pop.style.left = `${left}px`;
  pop.style.top = `${top}px`;
  pop.style.visibility = "visible";

  // F70: seed keyboard nav with the chain order, highlighting the first row.
  chainNavIds = nodes.map((n) => n.id);
  chainNavIndex = 0;
  paintChainNav();

  pop.addEventListener("click", (e) => {
    const target = e.target as HTMLElement | null;
    // F85: the flip button re-opens the popover walking the other direction.
    const flipBtn = target?.closest<HTMLElement>("[data-chain-flip]");
    if (flipBtn && fromId !== undefined) {
      const next = flipBtn.dataset.chainFlip === "dependent" ? "dependent" : "blocker";
      openChainDrill(fromId, next);
      return;
    }
    // F102: the "focus these" button drops the board into the cohort of undone
    // tasks waiting on #fromId (the same setCohort path the sidebar focus button
    // drives), so you can act on the cohort you've been walking. Close the
    // popover first, then focus.
    const focusBtnEl = target?.closest<HTMLElement>("[data-cohort-focus]");
    if (focusBtnEl) {
      const focusId = Number(focusBtnEl.dataset.cohortFocus);
      closeChainDrill();
      if (Number.isFinite(focusId) && focusId > 0) setCohort(focusId);
      return;
    }
    const btn = target?.closest<HTMLElement>("[data-chain-jump]");
    if (!btn) return;
    const id = Number(btn.dataset.chainJump);
    closeChainDrill();
    if (Number.isFinite(id) && id > 0) jumpToTask(id);
  });

  setTimeout(() => {
    document.addEventListener("click", onChainAway, true);
    document.addEventListener("keydown", onChainKey, true);
  }, 0);
}

/**
 * F26: jump to a task by id — select it and scroll it into view. Used by the
 * "blocked by #N" badge so you can hop straight to the blocker. If the target
 * isn't currently visible (filtered out, on another tag page), clear the filter
 * / route first so the jump always lands.
 */
function jumpToTask(id: number): void {
  if (!visibleIds.includes(id)) {
    // The blocker is hidden by the active filter, an active lens, a cohort
    // focus, or a tag route — reset whichever is in play so the jump can land.
    if (route.kind === "tag") navigateToAll();
    let needsRender = false;
    if (activeLens) {
      activeLens = null; // F66: clear the active lens too
      persistLens(); // F93: a jump that drops the lens forgets it from storage
      needsRender = true;
    }
    if (focusCohort) {
      focusCohort = null; // F96: a jump that drops the cohort clears the focus
      cohortHistory = []; // F108: and its back-stack
      needsRender = true;
    }
    if (isFilterActive(filter)) {
      filter = emptyFilter();
      filter.hideDone = settings.hideDone;
      els.filterInput.value = "";
      needsRender = true;
    }
    if (needsRender) render();
  }
  nav = select(nav, visibleIds, id);
  applySelection();
}

document.addEventListener("keydown", (e) => {
  // F18: Cmd/Ctrl-K opens the palette from anywhere (even while typing).
  if ((e.metaKey || e.ctrlKey) && (e.key === "k" || e.key === "K")) {
    e.preventDefault();
    if (paletteOpen) closePalette();
    else openPalette();
    return;
  }
  // While the palette is open it owns the keyboard; its own input handles keys.
  if (paletteOpen) return;
  // The help overlay swallows Escape/?; handle that first.
  if (helpOpen) {
    if (e.key === "Escape" || e.key === "?") {
      e.preventDefault();
      toggleHelp(false);
    }
    return;
  }
  // F24: the settings drawer is modal — Escape closes it and it owns the kb.
  if (settingsOpen) {
    if (e.key === "Escape") {
      e.preventDefault();
      toggleSettings(false);
    }
    return;
  }
  if (e.metaKey || e.ctrlKey || e.altKey) return;
  if (isTypingTarget(e.target)) return;
  if (editing || duePicking || notesEditing) return; // inline edit / due picker / notes handle their own keys

  // F71: number keys 1-5 toggle a stats lens (blocked / overdue / today / week /
  // no-due) without opening the sidebar. Pressing the active lens's digit again
  // clears it. Lives before the switch so it doesn't collide with letter keys.
  const digitLens = lensForDigit(e.key);
  if (digitLens !== null) {
    e.preventDefault();
    setLens(activeLens === digitLens ? null : digitLens);
    return;
  }

  switch (e.key) {
    case "j":
    case "ArrowDown":
      e.preventDefault();
      navMove("next");
      break;
    case "k":
    case "ArrowUp":
      e.preventDefault();
      navMove("prev");
      break;
    case "g":
    case "Home":
      e.preventDefault();
      navMove("first");
      break;
    case "G":
    case "End":
      e.preventDefault();
      navMove("last");
      break;
    case " ":
    case "Enter":
      if (nav.selectedId !== null) {
        e.preventDefault();
        toggleTask(nav.selectedId);
      }
      break;
    case "x":
    case "Delete":
    case "Backspace":
      if (nav.selectedId !== null) {
        e.preventDefault();
        requestDelete(nav.selectedId);
      }
      break;
    case "e":
      if (nav.selectedId !== null) {
        e.preventDefault();
        enterEditMode(nav.selectedId);
      }
      break;
    case "d":
      if (nav.selectedId !== null) {
        e.preventDefault();
        openDuePicker(nav.selectedId);
      }
      break;
    case "i":
      if (nav.selectedId !== null) {
        e.preventDefault();
        openNotesEditor(nav.selectedId);
      }
      break;
    case "b":
      // F39: edit blockers (dependency editor) on the selected row.
      if (nav.selectedId !== null) {
        e.preventDefault();
        openDepEditor(nav.selectedId);
      }
      break;
    case "p":
      if (nav.selectedId !== null) {
        e.preventDefault();
        togglePin(nav.selectedId);
      }
      break;
    case "P":
      // F44: shift+P pins the selected task AND floats it to the top.
      if (nav.selectedId !== null) {
        e.preventDefault();
        pinToTop(nav.selectedId);
      }
      break;
    case "]":
      // F44: raise the selected task's priority (sister of the F29 chip click).
      if (nav.selectedId !== null) {
        e.preventDefault();
        cyclePriority(nav.selectedId, false);
      }
      break;
    case "[":
      // F44: lower the selected task's priority.
      if (nav.selectedId !== null) {
        e.preventDefault();
        cyclePriority(nav.selectedId, true);
      }
      break;
    case "}":
      // F58: shift+] jumps the selected task's priority to the ceiling (urgent)
      // in one keystroke (shift turns "]" into "}").
      if (nav.selectedId !== null) {
        e.preventDefault();
        setPriority(nav.selectedId, ceilPriority());
      }
      break;
    case "{":
      // F58: shift+[ jumps the selected task's priority to the floor (low).
      if (nav.selectedId !== null) {
        e.preventDefault();
        setPriority(nav.selectedId, floorPriority());
      }
      break;
    case "u":
      if (pending) {
        e.preventDefault();
        undoDelete();
      }
      break;
    case "r":
      e.preventDefault();
      refresh();
      break;
    case "n":
      e.preventDefault();
      els.input.focus();
      break;
    case "/":
      if (!els.filterbar.hidden) {
        e.preventDefault();
        els.filterInput.focus();
        els.filterInput.select();
      }
      break;
    case "s":
      e.preventDefault();
      toggleStats(!statsOpen);
      break;
    case "t":
      e.preventDefault();
      cycleTheme();
      break;
    case ",":
      e.preventDefault();
      toggleSettings(!settingsOpen);
      break;
    case "Escape":
      // F108: when a cohort focus has back-history, Escape steps BACK one level
      // (to the most recent still-live ancestor cohort) rather than clearing
      // outright — so a multi-step chokepoint drill is reversible. cohortBack()
      // returns false (and does nothing) when there's no history to step, so we
      // fall through to the other Escape behaviours below.
      if (cohortBack()) {
        e.preventDefault();
        break;
      }
      // F16: a bulk selection is the first thing Escape clears.
      if (isBulkActive(bulk)) {
        e.preventDefault();
        bulkClear();
        break;
      }
      // On a tag page (and not otherwise busy), Escape returns to all tasks.
      if (route.kind === "tag") {
        e.preventDefault();
        navigateToAll();
      }
      break;
    case "?":
      e.preventDefault();
      toggleHelp(true);
      break;
  }
});

// --- F10: help overlay ------------------------------------------------------

let helpOpen = false;

const HELP_ROWS: ReadonlyArray<[string, string]> = [
  ["cmd/ctrl K", "Open the command palette"],
  ["j / \u2193", "Move selection down"],
  ["k / \u2191", "Move selection up"],
  ["g / G", "Jump to first / last"],
  ["space / enter", "Toggle the selected task done"],
  ["e", "Edit the selected task's title"],
  ["d", "Set / change the due date"],
  ["i", "Edit the selected task's notes"],
  ["b", "Edit the selected task's blockers"],
  ["p", "Pin / unpin the selected task"],
  ["shift P", "Pin the selected task and float it to the top"],
  ["[ / ]", "Lower / raise the selected task's priority"],
  ["shift [ / ]", "Jump priority to the floor (low) / ceiling (urgent)"],
  ["1 \u2026 5", "Toggle a stats lens (blocked / overdue / today / week / no-due)"],
  ["x / del", "Delete the selected task (undoable)"],
  ["cmd/shift-click", "Bulk-select rows (then toggle / delete many)"],
  ["drag ⠿", "Reorder a task (persists to .tsk.md)"],
  ["u", "Undo the last delete"],
  ["n", "Focus the add-task field"],
  ["/", "Focus the filter box"],
  ["s", "Toggle the stats panel"],
  ["t", "Cycle theme (auto / light / dark)"],
  [",", "Open settings"],
  ["r", "Refresh from disk"],
  ["esc", "Clear the add field / close this help"],
  ["?", "Toggle this help"],
];

function ensureHelpEl(): HTMLElement {
  let el = document.querySelector<HTMLElement>("[data-help]");
  if (el) return el;
  el = document.createElement("div");
  el.className = "help-overlay";
  el.setAttribute("data-help", "");
  el.setAttribute("role", "dialog");
  el.setAttribute("aria-modal", "true");
  el.setAttribute("aria-label", "Keyboard shortcuts");
  el.innerHTML = `
    <div class="help-card">
      <div class="help-title">Keyboard shortcuts</div>
      <dl class="help-list">
        ${HELP_ROWS.map(([keys, desc]) => {
          // F82: tag the lens-shortcut row so toggleHelp can bold it when a
          // stats lens is active, making the live shortcut obvious in the `?`
          // overlay too (it's where the chip's digit comes from).
          const lensRow = /stats lens/i.test(desc) ? " data-help-lens-row" : "";
          return `<div class="help-row"${lensRow}><dt><kbd>${escapeAttr(keys)}</kbd></dt><dd>${escapeAttr(desc)}</dd></div>`;
        }).join("")}
      </dl>
      <div class="help-foot" data-help-active></div>
      <div class="help-foot" data-help-cohort></div>
      <div class="help-foot" data-help-legend></div>
      <div class="help-foot">Press <kbd>?</kbd> or <kbd>esc</kbd> to close</div>
    </div>`;
  // Click the backdrop (not the card) to dismiss.
  el.addEventListener("click", (e) => {
    const target = e.target as HTMLElement | null;
    // F91: the F90 digit-map legend entries are now actionable — clicking one
    // toggles that lens (same path as its number key / stat tile) and closes the
    // overlay, so the `?` panel doubles as a lens switcher, not just a readout.
    const legend = target?.closest<HTMLElement>("[data-lens-legend]");
    if (legend) {
      const kind = legend.dataset.lensLegend ?? "";
      if (kind === "blocked" || kind === "overdue" || kind === "today" || kind === "week" || kind === "nodue") {
        setLens(activeLens === kind ? null : (kind as LensKind));
        toggleHelp(false);
      }
      return;
    }
    if (e.target === el) toggleHelp(false);
  });
  document.body.appendChild(el);
  return el;
}

function toggleHelp(open: boolean): void {
  helpOpen = open;
  const el = ensureHelpEl();
  // F71/F82: reflect the active stats lens (if any) so the overlay doubles as a
  // "what am I currently looking at?" readout — with the lens label bolded and
  // its number-key shortcut shown as a <kbd>. Empty -> the line collapses.
  const activeEl = el.querySelector<HTMLElement>("[data-help-active]");
  if (activeEl) {
    // F116: when the active lens came from a recalled lensed view (F104), append
    // a "from <view>" note to the help line so the keyboard-only `?` summary
    // answers "why is this lens on?" — the same provenance F109 shows beside the
    // filter-bar chip. lensProvenanceNote returns the view name only while the
    // recalled view still equals the live lens (a digit-key / stat-tile lens, or
    // a lens changed after recall, reports nothing -> the line stays as before).
    const recalled = recalledViewId ? views.find((v) => v.id === recalledViewId) ?? null : null;
    const provenance = lensProvenanceNote(recalled, activeLens);
    const html = renderActiveLensHelp(activeLens, provenance);
    activeEl.hidden = html === "";
    activeEl.innerHTML = html;
  }
  // F117: reflect the active COHORT focus (if any) as the cohort sibling of the
  // active-lens line, so the keyboard-only `?` summary answers "what am I
  // focused on, and how deep did I drill?" without looking at the filter bar.
  // Reuses renderCohortHelp (which reuses cohortSummary) so the overlay line,
  // the Cmd-K command label, and the chip can't drift; the back-stack depth
  // echoes the F113 chip badge. Empty -> the line collapses.
  const cohortEl = el.querySelector<HTMLElement>("[data-help-cohort]");
  if (cohortEl) {
    const html = renderCohortHelp(focusCohort, cohortHistory.length);
    cohortEl.hidden = html === "";
    cohortEl.innerHTML = html;
  }
  // F90: a live mini-legend of the WHOLE lens digit map ("1 blocked · 2
  // overdue · …"), with the active lens marked, so the overlay teaches every
  // number-key shortcut at a glance — not just the one currently active.
  const legendEl = el.querySelector<HTMLElement>("[data-help-legend]");
  if (legendEl) {
    legendEl.innerHTML = renderLensDigitMap(activeLens);
  }
  // F82: bold the lens-shortcut row while a lens is active so the digit map is
  // visibly "live" in the overlay, matching the chip's digit badge.
  const lensRow = el.querySelector<HTMLElement>("[data-help-lens-row]");
  lensRow?.classList.toggle("is-active", activeLens !== null);
  el.classList.toggle("is-open", open);
}

// Clickable "?" hint in the footer opens the help overlay.
must<HTMLElement>("[data-help-open]").addEventListener("click", () => toggleHelp(true));
// Clickable "⌘K" hint in the footer opens the command palette.
must<HTMLElement>("[data-cmdk-open]").addEventListener("click", () => openPalette());

// --- F18: Cmd-K command palette --------------------------------------------

let paletteOpen = false;
let paletteIndex = 0;
let paletteResults: Command[] = [];
/** F57: the live palette query, so the result list can highlight the match. */
let paletteQuery = "";
/** F73: seq guard for the highlighted-command due preview's async parse. */
let paletteDueParseSeq = 0;

/**
 * Build the command registry from the live app state. Rebuilt each open so
 * enabled/disabled flags (e.g. undo) and the current selection reflect reality.
 */
function maybeCommand(cmd: Command | null): Command[] {
  // F114: spread-helper so an optional command (one that only exists in some
  // states, e.g. the chokepoint-shift lead) drops cleanly out of the registry
  // when its builder returns null — keeps buildCommands declarative.
  return cmd ? [cmd] : [];
}

function buildCommands(): Command[] {
  const sel = nav.selectedId;
  const hasSel = sel !== null;
  const selTask = sel !== null ? currentTasks.find((t) => t.id === sel) : undefined;
  const selPinned = selTask?.pinned ?? false;
  // F63: the selected task's current priority, so the matching "Set priority"
  // command can be disabled (it's already in effect).
  const selPriority = selTask?.priority;
  const viewCommands: Command[] = views.map((v) => ({
    id: `view:${v.id}`,
    title: `View: ${v.name}`,
    group: "Views",
    keywords: ["saved", "filter", "recall", ...v.filter.tags, ...v.filter.priorities],
  }));
  return [
    { id: "add", title: "Add task", group: "Task", keywords: ["new", "create", "compose"], hint: "n" },
    {
      id: "toggle",
      title: "Toggle selected done",
      group: "Task",
      keywords: ["complete", "check", "done"],
      hint: "space",
      disabled: !hasSel,
    },
    {
      id: "edit",
      title: "Edit selected title",
      group: "Task",
      keywords: ["rename"],
      hint: "e",
      disabled: !hasSel,
    },
    {
      id: "due",
      title: "Set due date on selected",
      group: "Task",
      keywords: ["date", "deadline", "schedule"],
      hint: "d",
      disabled: !hasSel,
    },
    {
      id: "notes",
      title: "Edit notes on selected",
      group: "Task",
      keywords: ["note", "comment", "description", "detail"],
      hint: "i",
      disabled: !hasSel,
    },
    {
      id: "deps",
      title: "Edit blockers on selected",
      group: "Task",
      keywords: ["depend", "depends", "blocker", "blocked", "prereq", "prerequisite"],
      hint: "b",
      disabled: !hasSel,
    },
    {
      id: "pin",
      title: selPinned ? "Unpin selected task" : "Pin selected task",
      group: "Task",
      keywords: ["pin", "sticky", "favorite", "star", "top"],
      hint: "p",
      disabled: !hasSel,
    },
    {
      id: "prio-up",
      title: "Raise priority of selected",
      group: "Task",
      keywords: ["priority", "urgent", "important", "bump", "cycle"],
      disabled: !hasSel,
    },
    {
      id: "prio-down",
      title: "Lower priority of selected",
      group: "Task",
      keywords: ["priority", "low", "demote", "cycle"],
      disabled: !hasSel,
    },
    // F63: a "Set priority ▸" group — pick an exact level keyboard-only from the
    // palette, without leaving it for the chip / picker. Each acts on the
    // selection via setPriority; the one already in effect is disabled so the
    // palette doubles as a "current priority" readout.
    ...buildPriorityCommands(hasSel, selPriority),
    // F67: a "Set due ▸" group — pick a due date keyboard-only from the palette
    // (today / tomorrow / this weekend / next week / end of month / clear). Each
    // hands its natural-language token to the same commitDue PATCH the picker
    // uses, so the dates resolve identically via the server's dateparse.
    ...buildDueCommands(hasSel),
    {
      id: "delete",
      title: "Delete selected task",
      group: "Task",
      keywords: ["remove", "trash"],
      hint: "x",
      disabled: !hasSel,
    },
    { id: "undo", title: "Undo last delete", group: "Task", keywords: ["restore"], hint: "u", disabled: !pending },
    { id: "filter", title: "Focus filter / search", group: "View", keywords: ["search", "find"], hint: "/", disabled: Boolean(els.filterbar.hidden) },
    { id: "stats", title: statsOpen ? "Hide stats panel" : "Show stats panel", group: "View", keywords: ["metrics", "summary"], hint: "s" },
    // F98: a context-aware "Clear lens (<label>)" command — the keyboard-only
    // way to exit a render-pipeline lens (digit keys / stat tiles SET one, but
    // there was no palette CLEAR). Shown always so it's discoverable; disabled
    // (with a "no lens active" reason via F89) when no lens is on.
    clearLensCommand(activeLens ? lensMeta(activeLens).label : null),
    // F115: the keyboard-only sister of F110's mouse pin star — "Pin lens
    // (<label>)" (or "Recall pinned lens (<label>)" when already pinned) routes
    // through the same pinCurrentLens path. Disabled (with "no lens active" via
    // F89) when no lens is on. `pinned` reflects findPureLensView so the title
    // matches the star's pin-vs-recall state.
    pinLensCommand(
      activeLens ? lensMeta(activeLens).label : null,
      activeLens !== null && findPureLensView(views, activeLens) !== null,
    ),
    // F103: the cohort-focus sisters of clearLensCommand — the keyboard-only
    // "Clear cohort focus (<summary>)" readout + a "Focus biggest chokepoint"
    // into the biggest chokepoint's cohort (naming it, "#N"). F96's sidebar
    // focus button + filter-bar clear chip are mouse-only; these reach the same
    // setCohort path from Cmd-K. Each disabled (with a reason via F89) when its
    // precondition is absent (no cohort focused / a flat board).
    // F118: pass the back-stack depth so the clear label warns when clearing
    // drops a multi-step drill (the command-label sibling of the F113 chip badge).
    clearCohortCommand(focusCohort ? cohortSummary(focusCohort) : null, cohortHistory.length),
    // F114: when the biggest chokepoint JUST shifted (tracked in refresh(),
    // independent of the stats panel), lead the focus group with "Focus the new
    // biggest chokepoint (#N, was #M)" so the keyboard path opens straight onto
    // the change F111's sidebar hint flags. Filtered out (null) on a steady or
    // flat board, so it appears only when there's a shift worth leading with.
    ...maybeCommand(focusShiftedChokepointCommand(currentChokepointId(), chokepointShiftFrom)),
    focusChokepointCommand(currentChokepointId()),
    // F107: a "Focus chokepoint #N (K waiting)" command per RUNNER-UP
    // chokepoint, so every bottleneck the F106 sidebar lists is reachable
    // keyboard-only (not just the biggest, which focusChokepointCommand covers).
    // The ids are "cohort-focus-<id>"; runCommand decodes + routes to setCohort.
    ...buildChokepointFocusCommands(currentChokepoints()),
    { id: "theme", title: "Cycle theme (auto/light/dark)", group: "View", keywords: ["dark", "light", "color"], hint: "t" },
    { id: "settings", title: "Open settings", group: "View", keywords: ["preferences", "density", "compact", "options", "config"], hint: "," },
    { id: "refresh", title: "Refresh from disk", group: "View", keywords: ["reload", "sync"], hint: "r" },
    // F19/F78: the export group. buildExportCommands emits the three whole-store
    // "Export tasks as <FMT>" commands always, and — when a lens/filter/tag is
    // narrowing the board — three extra "Export N shown as <FMT>" commands so
    // the visible subset is reachable keyboard-only, distinct from the full set.
    ...buildExportCommands(isExportScoped() ? visibleIds.length : null),
    { id: "help", title: "Show keyboard shortcuts", group: "View", keywords: ["keys", "?"], hint: "?" },
    { id: "alltasks", title: "Go to all tasks", group: "View", keywords: ["home", "clear tag"], disabled: route.kind !== "tag" },
    {
      id: "save-view",
      title: "Save current filter as a view",
      group: "Views",
      keywords: ["bookmark", "store", "remember", "lens"],
      // F104: savable when a filter OR a lens is in effect; disabled when the
      // exact filter+lens combo is already saved (lens-aware dup check).
      disabled:
        (filterIsEmpty(currentViewFilter()) && activeLens === null) ||
        activeViewWithLens(views, currentViewFilter(), activeLens) !== null,
    },
    ...viewCommands,
  ];
}

/** Dispatch a command by id, then close the palette. */
function runCommand(id: string): void {
  closePalette();
  const sel = nav.selectedId;
  switch (id) {
    case "add":
      els.input.focus();
      break;
    case "toggle":
      if (sel !== null) toggleTask(sel);
      break;
    case "edit":
      if (sel !== null) enterEditMode(sel);
      break;
    case "due":
      if (sel !== null) openDuePicker(sel);
      break;
    case "notes":
      if (sel !== null) openNotesEditor(sel);
      break;
    case "deps":
      if (sel !== null) openDepEditor(sel);
      break;
    case "pin":
      if (sel !== null) togglePin(sel);
      break;
    case "prio-up":
      if (sel !== null) cyclePriority(sel, false);
      break;
    case "prio-down":
      if (sel !== null) cyclePriority(sel, true);
      break;
    // F63: exact priority set from the palette.
    case "prio-set-urgent":
      if (sel !== null) setPriority(sel, "urgent");
      break;
    case "prio-set-high":
      if (sel !== null) setPriority(sel, "high");
      break;
    case "prio-set-medium":
      if (sel !== null) setPriority(sel, "medium");
      break;
    case "prio-set-low":
      if (sel !== null) setPriority(sel, "low");
      break;
    // F67: exact due-date set from the palette. The id encodes the natural-
    // language token ("due-set-today" -> "today", "due-set-1w" -> "1w",
    // "due-set-clear" -> ""), which commitDue hands to the server's PATCH due
    // field — the same dateparse path the picker uses.
    case "due-set-clear":
      if (sel !== null) commitDue(sel, "");
      break;
    case "delete":
      if (sel !== null) requestDelete(sel);
      break;
    case "undo":
      undoDelete();
      break;
    case "filter":
      if (!els.filterbar.hidden) {
        els.filterInput.focus();
        els.filterInput.select();
      }
      break;
    case "stats":
      toggleStats(!statsOpen);
      break;
    case "lens-clear":
      // F98: drop the active render-pipeline lens (the keyboard-only exit).
      setLens(null);
      break;
    case "lens-pin":
      // F115: pin the active lens as a recallable view (or recall it if already
      // pinned), keyboard-only — the sister of F110's mouse pin star. No-op when
      // no lens is active (the command is disabled then anyway).
      pinCurrentLens();
      break;
    case "cohort-clear":
      // F103: drop the active cohort focus (keyboard-only sister of the
      // filter-bar clear chip).
      clearCohort();
      break;
    case "cohort-focus-biggest": {
      // F103: focus the biggest chokepoint's cohort keyboard-only (sister of
      // the sidebar focus button). setCohort no-ops with a status note if the
      // board went flat between palette-build and dispatch.
      const id = currentChokepointId();
      if (id !== null) setCohort(id);
      break;
    }
    case "cohort-focus-new": {
      // F114: focus the NEW biggest chokepoint after a shift (the lead command).
      // Same setCohort path as cohort-focus-biggest, but it also consumes the
      // pending shift so the "was #M" lead doesn't linger after you've acted on
      // it (the next refresh would clear it anyway; this is the immediate path).
      const id = currentChokepointId();
      chokepointShiftFrom = null;
      if (id !== null) setCohort(id);
      break;
    }
    case "theme":
      cycleTheme();
      break;
    case "settings":
      toggleSettings(true);
      break;
    case "refresh":
      refresh();
      break;
    case "help":
      toggleHelp(true);
      break;
    case "alltasks":
      if (route.kind === "tag") navigateToAll();
      break;
    case "save-view":
      saveCurrentView();
      break;
  }
  // F25: dynamic per-view recall commands (id shaped "view:<id>").
  if (id.startsWith("view:")) {
    recallView(id.slice("view:".length));
  }
  // F107: dynamic per-chokepoint focus commands (id shaped "cohort-focus-<id>",
  // distinct from the static "cohort-focus-biggest"). Decode the target id and
  // route through the same setCohort path the sidebar focus buttons + the
  // biggest-chokepoint command use — so every bottleneck is keyboard-reachable.
  if (
    id.startsWith("cohort-focus-") &&
    id !== "cohort-focus-biggest" &&
    id !== "cohort-focus-new" // F114: the shift-lead is handled in the switch above
  ) {
    const focusId = Number(id.slice("cohort-focus-".length));
    if (Number.isFinite(focusId) && focusId > 0) setCohort(focusId);
  }
  // F67: dynamic "Set due: <preset>" commands (id shaped "due-set-<token>").
  // The clear case is handled in the switch above; the rest carry their NL
  // token in the id suffix and route through the same commitDue PATCH.
  if (id.startsWith("due-set-") && id !== "due-set-clear") {
    const token = id.slice("due-set-".length);
    if (sel !== null && token !== "") commitDue(sel, token);
  }
  // F19/F78: dynamic export commands. exportCommandTarget decodes both the
  // whole-store ids ("export-csv") and the scoped ids ("export-scoped-csv"),
  // forcing the scope so the explicit palette commands always do exactly what
  // they say regardless of the live lens/filter auto-scope.
  const exportTarget = exportCommandTarget(id);
  if (exportTarget) {
    downloadExport(exportTarget.format, exportTarget.scoped);
  }
}

function ensurePaletteEl(): HTMLElement {
  let el = document.querySelector<HTMLElement>("[data-cmdk]");
  if (el) return el;
  el = document.createElement("div");
  el.className = "cmdk-overlay";
  el.setAttribute("data-cmdk", "");
  el.setAttribute("role", "dialog");
  el.setAttribute("aria-modal", "true");
  el.setAttribute("aria-label", "Command palette");
  el.innerHTML = `
    <div class="cmdk-card">
      <input class="cmdk-input" data-cmdk-input type="text" spellcheck="false"
             placeholder="Type a command…" aria-label="Command palette" role="combobox"
             aria-expanded="true" aria-controls="cmdk-list" aria-autocomplete="list">
      <ul class="cmdk-list" id="cmdk-list" data-cmdk-list role="listbox"></ul>
      <div class="cmdk-due-preview" data-cmdk-due-preview hidden></div>
      <div class="cmdk-foot"><kbd>↑↓</kbd> navigate <kbd>↵</kbd> run <kbd>esc</kbd> close</div>
    </div>`;
  // Backdrop click closes; clicks inside the card don't.
  el.addEventListener("click", (e) => {
    if (e.target === el) closePalette();
  });
  const input = el.querySelector<HTMLInputElement>("[data-cmdk-input]")!;
  const list = el.querySelector<HTMLElement>("[data-cmdk-list]")!;
  input.addEventListener("input", () => updatePalette(input.value));
  input.addEventListener("keydown", (e) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      paletteIndex = moveIndex(paletteIndex, paletteResults.length, 1);
      paintPalette();
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      paletteIndex = moveIndex(paletteIndex, paletteResults.length, -1);
      paintPalette();
    } else if (e.key === "Enter") {
      e.preventDefault();
      const cmd = paletteResults[paletteIndex];
      if (cmd && !cmd.disabled) runCommand(cmd.id);
    } else if (e.key === "Escape") {
      e.preventDefault();
      closePalette();
    }
  });
  // Click a row to run it.
  list.addEventListener("click", (e) => {
    const row = (e.target as HTMLElement | null)?.closest<HTMLElement>("[data-cmd-id]");
    if (!row) return;
    const id = row.dataset.cmdId ?? "";
    const cmd = paletteResults.find((c) => c.id === id);
    if (cmd && !cmd.disabled) runCommand(id);
  });
  // Track hover to move the highlight (mouse + keyboard stay in sync).
  list.addEventListener("mousemove", (e) => {
    const row = (e.target as HTMLElement | null)?.closest<HTMLElement>("[data-cmd-id]");
    if (!row) return;
    const id = row.dataset.cmdId ?? "";
    const i = paletteResults.findIndex((c) => c.id === id);
    if (i >= 0 && i !== paletteIndex) {
      paletteIndex = i;
      paintPalette();
    }
  });
  document.body.appendChild(el);
  return el;
}

const allCommands = (): Command[] => buildCommands();

/** Recompute results for the query and repaint, resetting the highlight. */
function updatePalette(query: string): void {
  paletteQuery = query;
  paletteResults = filterCommands(allCommands(), query);
  paletteIndex = clampIndex(paletteIndex, paletteResults.length);
  paintPalette();
}

/** Repaint the list + active highlight, scrolling it into view. */
function paintPalette(): void {
  const el = ensurePaletteEl();
  const list = el.querySelector<HTMLElement>("[data-cmdk-list]")!;
  list.innerHTML = renderPaletteList(paletteResults, paletteIndex, paletteQuery);
  const active = list.querySelector<HTMLElement>(".is-active");
  active?.scrollIntoView({ block: "nearest" });
  // F73: when the highlighted command sets a due date, live-preview the resolved
  // date below the list (mirrors the F47 bulk-due preview) so you can confirm
  // "this weekend = Jun 28" before Enter.
  paintPaletteDuePreview();
}

/**
 * F73/F77: drive the palette's preview line from the highlighted command.
 *   - "Set due: <preset>"  -> resolve its NL token via /api/parse-date (async,
 *     seq-guarded) and render the date; "clear" shows "Clears the due date".
 *   - "Set priority: <X>"  -> render the selected task's "current -> new"
 *     transition synchronously (no parse needed; F77).
 *   - anything else        -> hide the line.
 * Both kinds share the one `[data-cmdk-due-preview]` slot + the `.due-preview`
 * state classes so they read identically.
 */
function paintPaletteDuePreview(): void {
  const el = document.querySelector<HTMLElement>("[data-cmdk]");
  const line = el?.querySelector<HTMLElement>("[data-cmdk-due-preview]");
  if (!line) return;
  const cmd = paletteResults[paletteIndex];
  // F83/F89: a highlighted DISABLED command explains itself in the slot instead
  // of just greying — "select a task first" for the per-task verbs + the Set
  // due/priority group, "nothing to undo" / "no tasks to filter" / "already on
  // all tasks" for the other gated commands. Only shown when the command is
  // actually disabled; the "already <level>" case is left to priorityPreviewVM's
  // empty state below. commandDisabledReason subsumes the F83 set-command path.
  const sel = nav.selectedId;
  const reason =
    cmd && cmd.disabled
      ? commandDisabledReason(cmd.id, {
          hasSel: sel !== null,
          canUndo: pending !== null,
          hasTasks: !els.filterbar.hidden,
          onTag: route.kind === "tag",
          hasLens: activeLens !== null,
          hasCohort: focusCohort !== null,
          hasChokepoint: currentChokepointId() !== null,
        })
      : null;
  if (reason !== null) {
    paletteDueParseSeq++; // invalidate any in-flight due parse — the hint wins the slot
    line.hidden = false;
    line.innerHTML = renderDisabledReason(reason);
    return;
  }
  // F77: a highlighted "Set priority" command previews the level change
  // synchronously from the selected task's current priority.
  const prio = cmd ? priorityForCommandId(cmd.id) : null;
  if (prio !== null) {
    paletteDueParseSeq++; // invalidate any in-flight due parse — priority wins the slot
    const selTask = sel !== null ? currentTasks.find((t) => t.id === sel) : undefined;
    line.hidden = false;
    line.innerHTML = renderPriorityPreview(priorityPreviewVM(selTask?.priority, prio));
    return;
  }
  const token = cmd ? dueTokenForCommandId(cmd.id) : null;
  if (token === null) {
    line.hidden = true;
    line.innerHTML = "";
    return;
  }
  line.hidden = false;
  const seq = ++paletteDueParseSeq;
  if (token.trim() === "") {
    // The clear command — no parse needed.
    line.innerHTML = renderDuePreview(previewVM("", null));
    return;
  }
  line.innerHTML = renderDuePreview(previewVM(token, null)); // "Parsing…"
  void api
    .parseDate(token)
    .then((res) => {
      if (seq !== paletteDueParseSeq) return; // a newer highlight superseded us
      line.innerHTML = renderDuePreview(previewVM(token, res));
    })
    .catch(() => {
      if (seq !== paletteDueParseSeq) return;
      line.innerHTML = renderDuePreview(previewVM(token, { ok: false, input: token, error: "offline" }));
    });
}

function openPalette(): void {
  if (paletteOpen) return;
  paletteOpen = true;
  paletteIndex = 0;
  const el = ensurePaletteEl();
  el.classList.add("is-open");
  const input = el.querySelector<HTMLInputElement>("[data-cmdk-input]")!;
  input.value = "";
  updatePalette("");
  input.focus();
}

function closePalette(): void {
  if (!paletteOpen) return;
  paletteOpen = false;
  paletteDueParseSeq++; // F73: invalidate any in-flight due preview parse
  const el = document.querySelector<HTMLElement>("[data-cmdk]");
  el?.classList.remove("is-open");
}


// --- F21: live-reload via Server-Sent Events --------------------------------

/** Last file fingerprint we acted on; null until the first `ready`/`change`. */
let liveFingerprint: FileFingerprint | null = null;
/** The active EventSource, if any. */
let liveSource: EventSource | null = null;
/** Backoff handle for reconnect attempts after the stream drops. */
let liveReconnectTimer: number | null = null;
/**
 * F33: per-tab "pause live" flag. When paused, the SSE stream stays connected
 * (so resuming is instant) but a change frame only updates the baseline — it
 * never auto-refreshes. Persisted per-tab in sessionStorage so a reload in the
 * same tab keeps your choice, but a fresh tab starts live.
 */
let livePaused = false;
try {
  livePaused = sessionStorage.getItem("tsk.live.paused") === "1";
} catch {
  // ignore (private mode / storage disabled)
}

/** Paint the live indicator pill for the given connection status. */
function setLiveStatus(status: LiveStatus): void {
  // F33: a paused tab shows the paused state regardless of the socket state
  // (unless we're actively offline, which the user should still see).
  const shown: LiveStatus = livePaused && status !== "offline" ? "paused" : status;
  els.live.innerHTML = renderLiveIndicator(shown);
  els.live.title = liveTitle(shown);
  els.live.dataset.status = shown;
  // F35: the stream status feeds the offline/server banner.
  updateOfflineBanner();
}

/**
 * F35: paint the offline/server banner from the SSE stream status + the
 * browser's online flag. Distinguishes "tsk serve is restarting" (network up,
 * stream down) from "you're offline" (device offline) so the copy is honest.
 * A paused stream is intentionally muted — that's a user choice, not a fault.
 */
function updateOfflineBanner(): void {
  const streamOffline = liveConnStatus === "offline" && !livePaused;
  const online = typeof navigator === "undefined" ? true : navigator.onLine;
  const c = classifyConnectivity(streamOffline, online);
  if (!shouldShowOfflineBanner(c)) {
    els.offlineBanner.hidden = true;
    els.offlineBanner.innerHTML = "";
    must<HTMLElement>("[data-app]").classList.remove("is-offline");
    return;
  }
  els.offlineBanner.hidden = false;
  els.offlineBanner.innerHTML = renderOfflineBanner(c);
  els.offlineBanner.dataset.connectivity = c;
  must<HTMLElement>("[data-app]").classList.add("is-offline");
}

/** The last real connection status (so un-pausing restores the right pill). */
let liveConnStatus: LiveStatus = "connecting";

/**
 * Handle a fingerprint frame (ready or change). The first frame only seeds the
 * baseline; subsequent frames whose fingerprint moved trigger a silent refresh
 * so external CLI/TUI/hand edits flow into the open tab without a manual reload.
 */
function onLiveFrame(data: string): void {
  const fp = parseFingerprint(data);
  if (!fp) return;
  if (shouldRefresh(liveFingerprint, fp)) {
    liveFingerprint = fp;
    // F33: when paused, swallow the auto-refresh — just keep the baseline so
    // resuming doesn't immediately fire on an already-seen change.
    if (livePaused) return;
    // Don't clobber an in-progress edit/due-pick: those own the keyboard and a
    // re-render would tear down their input. Defer the refresh until they close.
    if (editing || duePicking) {
      liveRefreshPending = true;
      return;
    }
    refresh();
    // F33: surface a subtle toast so an external edit landing is visible, not
    // a silent jump.
    showInfoToast(liveChangeMessage(false));
  } else {
    liveFingerprint = fp;
  }
}

/** F33: toggle the per-tab pause-live flag, persist it, and repaint the pill. */
function toggleLivePaused(): void {
  livePaused = !livePaused;
  try {
    sessionStorage.setItem("tsk.live.paused", livePaused ? "1" : "0");
  } catch {
    // ignore
  }
  setLiveStatus(liveConnStatus);
  if (!livePaused) {
    // Resuming: pull once so we're not stale on whatever we skipped.
    refresh();
  }
}

/** Set when a live change arrives mid-edit; flushed when the edit settles. */
let liveRefreshPending = false;

/** Flush a deferred live refresh once an inline edit / due picker has closed. */
function flushPendingLiveRefresh(): void {
  if (liveRefreshPending && !editing && !duePicking) {
    liveRefreshPending = false;
    if (livePaused) return; // F33: don't auto-pull while paused
    refresh();
    showInfoToast(liveChangeMessage(true));
  }
}

/**
 * Open (or reopen) the SSE connection. EventSource auto-reconnects on transient
 * drops, but on a hard error we also schedule our own reconnect so a server
 * restart is recovered from. Safe to call repeatedly; tears down any prior
 * source first. No-ops when EventSource is unavailable (very old browsers) —
 * the visibilitychange refresh remains as a fallback.
 */
function connectLive(): void {
  if (typeof EventSource === "undefined") {
    liveConnStatus = "offline";
    setLiveStatus("offline");
    return;
  }
  if (liveReconnectTimer !== null) {
    clearTimeout(liveReconnectTimer);
    liveReconnectTimer = null;
  }
  if (liveSource) {
    liveSource.close();
    liveSource = null;
  }
  liveConnStatus = "connecting";
  setLiveStatus("connecting");
  const es = new EventSource("/api/events");
  liveSource = es;
  es.addEventListener("ready", (e) => {
    liveConnStatus = "live";
    setLiveStatus("live");
    onLiveFrame((e as MessageEvent).data);
  });
  es.addEventListener("change", (e) => {
    liveConnStatus = "live";
    setLiveStatus("live");
    onLiveFrame((e as MessageEvent).data);
  });
  es.addEventListener("open", () => {
    liveConnStatus = "live";
    setLiveStatus("live");
  });
  es.addEventListener("error", () => {
    liveConnStatus = "offline";
    setLiveStatus("offline");
    // EventSource will retry on its own for network blips; on a closed stream
    // (readyState CLOSED) we re-create it after a short backoff.
    if (es.readyState === EventSource.CLOSED && liveReconnectTimer === null) {
      liveReconnectTimer = window.setTimeout(connectLive, 3_000);
    }
  });
}

// F33: click the live pill to pause / resume auto-refresh for this tab.
els.live.style.cursor = "pointer";
els.live.setAttribute("role", "button");
els.live.setAttribute("tabindex", "0");
els.live.addEventListener("click", toggleLivePaused);
els.live.addEventListener("keydown", (e) => {
  if (e.key === "Enter" || e.key === " ") {
    e.preventDefault();
    toggleLivePaused();
  }
});

// --- F35: install prompt capture + offline/online detection -----------------

/** The deferred beforeinstallprompt event, captured so the settings button can
 * trigger the native install flow on demand. Null until the browser offers it. */
let deferredInstall: InstallPromptEvent | null = null;

window.addEventListener("beforeinstallprompt", (e) => {
  // Prevent the mini-infobar so we control where the prompt lives (settings).
  e.preventDefault();
  deferredInstall = e as InstallPromptEvent;
  // Repaint the drawer if it's open so the Install button appears immediately.
  if (settingsOpen) paintSettings();
});

window.addEventListener("appinstalled", () => {
  deferredInstall = null;
  if (settingsOpen) paintSettings();
  setStatus("installed", false);
  setTimeout(() => setStatus("ready", false), 2_000);
});

// F35: the device going on/offline updates the banner immediately, independent
// of the SSE stream (which may not have noticed yet).
window.addEventListener("online", updateOfflineBanner);
window.addEventListener("offline", updateOfflineBanner);

/** Whether the Install affordance should be shown (deferred prompt + not installed). */
function installAvailable(): boolean {
  return canInstall(deferredInstall, isStandalone());
}

/** Fire the captured install prompt; clears it after the user chooses. */
async function triggerInstall(): Promise<void> {
  if (!deferredInstall) return;
  const evt = deferredInstall;
  deferredInstall = null;
  if (settingsOpen) paintSettings();
  try {
    await evt.prompt();
    await evt.userChoice;
  } catch {
    // user dismissed or the browser refused — non-fatal.
  }
}

// Escape hatches for upcoming slices.
declare global {
  interface Window {
    tsk: {
      refresh: () => Promise<void>;
      tasks: () => Task[];
      toggle: (id: number) => Promise<void>;
      add: (line: string) => Promise<void>;
      selected: () => number | null;
      help: (open: boolean) => void;
      del: (id: number) => void;
      undo: () => void;
      edit: (id: number) => void;
      due: (id: number) => void;
      notes: (id: number) => void;
      tag: (tag: string) => void;
      palette: (open: boolean) => void;
    };
  }
}
window.tsk = {
  refresh,
  tasks: () => currentTasks,
  toggle: toggleTask,
  add: async (line: string) => {
    els.input.value = line;
    await submitComposer();
  },
  selected: () => nav.selectedId,
  help: toggleHelp,
  del: requestDelete,
  undo: undoDelete,
  edit: enterEditMode,
  due: openDuePicker,
  notes: openNotesEditor,
  tag: navigateToTag,
  palette: (open: boolean) => (open ? openPalette() : closePalette()),
};

// Restore the persisted stats-panel state before the first paint.
applyStatsVisibility();
// Restore the persisted theme before the first paint to avoid a flash.
applyTheme();
// F24: mirror persisted settings (density/motion/show-ids) before first paint.
applySettings();
// F21: open the live-reload stream so external edits flow into the open tab.
connectLive();
// F35: reflect initial connectivity in the banner (e.g. loaded while offline).
updateOfflineBanner();
// F22: register the offline-shell service worker (no-op where unsupported).
registerServiceWorker();
// F32: if the page loaded on a #view/<id> hash, recall that saved view once
// the initial route is read. (recallView itself runs after refresh paints.)
if (route.kind === "view") {
  const bootViewId = route.id;
  route = { kind: "all" };
  if (views.some((v) => v.id === bootViewId)) {
    // Defer until after the first refresh so the filter applies to a painted list.
    setTimeout(() => recallView(bootViewId), 0);
  }
}
refresh();
