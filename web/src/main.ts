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
import { parseQuickAdd, isSubmittable } from "./quickadd";
import { renderComposerPreview } from "./composer";
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
import { renderStatsPanel } from "./stats";
import {
  normalizeMode,
  nextMode,
  themeAttr,
  modeLabel,
  modeGlyph,
  modeTitle,
  type ThemeMode,
} from "./theme";
import { parseHash, tagHash, type Route } from "./router";
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
import { computeReorder, dropPosForY, type DropPos } from "./reorder";
import {
  filterCommands,
  moveIndex,
  clampIndex,
  renderPaletteList,
  type Command,
} from "./palette";
import {
  exportUrl,
  exportFilename,
  renderExportMenu,
  type ExportFormat,
} from "./export";
import {
  parseFingerprint,
  shouldRefresh,
  liveTitle,
  renderLiveIndicator,
  type FileFingerprint,
  type LiveStatus,
} from "./live";
import { registerServiceWorker } from "./pwa";

const root = document.getElementById("root");
if (!root) throw new Error("missing #root");

root.innerHTML = `
  <div class="app" data-app>
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
          placeholder="Add a task…  try: ship release !high @fri #work"
          aria-label="Add a task. Use !priority @due #tag for inline metadata."
          spellcheck="false"
        >
        <button class="composer-submit" type="submit" data-submit tabindex="-1">Add</button>
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
      </div>
      <div class="filter-tags" data-filter-tags role="group" aria-label="Filter by tag"></div>
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
  filterbar: must<HTMLElement>("[data-filterbar]"),
  filterInput: must<HTMLInputElement>("[data-filter-input]"),
  filterClear: must<HTMLButtonElement>("[data-filter-clear]"),
  filterPrios: must<HTMLElement>("[data-filter-prios]"),
  filterTags: must<HTMLElement>("[data-filter-tags]"),
  filterHideDone: must<HTMLButtonElement>("[data-filter-hidedone]"),
  statsToggle: must<HTMLButtonElement>("[data-stats-toggle]"),
  statsPanel: must<HTMLElement>("[data-stats-panel]"),
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
/** Current hash route (F15): all-tasks or a single-tag page. */
let route: Route = parseHash(typeof location !== "undefined" ? location.hash : "");
/** Bulk selection (F16): a set of ids for multi-toggle / multi-delete. */
let bulk: BulkState = emptyBulk();

/** Render the current state to the DOM, preserving keyboard selection. */
function render(): void {
  const now = new Date();
  const notDeleted = currentTasks.filter((t) => !pendingDeletes.has(t.id));
  // F15: a tag route pre-narrows the pool to tasks carrying that tag.
  const r = route;
  const routed = r.kind === "tag"
    ? notDeleted.filter((t) => t.tags.includes(r.tag))
    : notDeleted;
  const shown = applyFilter(routed, filter);
  const sections = groupIntoSections(shown, now);
  const prevIds = visibleIds;
  visibleIds = flattenSections(sections).map((t) => t.id);
  nav = reconcile(nav, visibleIds, prevIds);
  els.content.innerHTML = renderSections(sections, now);
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
  els.filterTags.innerHTML = renderTagChips(collectTags(allTasks), filter);
  const active = isFilterActive(filter);
  els.filterClear.hidden = !active;
  els.filterHideDone.classList.toggle("is-active", filter.hideDone);
  els.filterHideDone.setAttribute("aria-pressed", String(filter.hideDone));
  els.filterbar.classList.toggle("is-active", active);
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

// --- F17: drag-to-reorder --------------------------------------------------

/** The id of the row currently being dragged, or null. */
let draggingId: number | null = null;
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
  // An in-progress inline edit / due picker shouldn't be draggable.
  if (editing || duePicking) {
    e.preventDefault();
    return;
  }
  const id = Number(row.dataset.id);
  if (!Number.isFinite(id) || id <= 0) return;
  draggingId = id;
  row.classList.add("is-dragging");
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = "move";
    // Some browsers require data to be set for a drag to start.
    e.dataTransfer.setData("text/plain", String(id));
  }
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
  const rect = row.getBoundingClientRect();
  const pos: DropPos = dropPosForY(rect.top, rect.height, e.clientY);
  const moved = draggingId;
  const order = currentTasks.map((t) => t.id);
  const result = computeReorder(order, moved, targetId, pos);
  if (result.changed) commitReorder(moved, result.before, result.order);
}

/** Reset drag visuals once the gesture ends (drop, cancel, or escape). */
function onDragEnd(): void {
  draggingId = null;
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
    els.statsPanel.innerHTML = renderStatsPanel(stats);
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
  route = parseHash(location.hash);
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
  // Optimistic update
  currentTasks[idx] = { ...before, done: !before.done };
  inFlight.add(id);
  render();
  try {
    const confirmed = await api.toggleTask(id);
    // Server is authoritative (it knows the completed timestamp etc.).
    currentTasks[idx] = confirmed;
    render();
    refreshStats();
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

// --- F6: quick-add composer ------------------------------------------------
/** Re-render the live token preview and toggle the submit-enabled state. */
function updateComposerPreview(): void {
  const parsed = parseQuickAdd(els.input.value);
  els.preview.innerHTML = renderComposerPreview(parsed);
  els.field.classList.toggle("can-submit", isSubmittable(parsed));
}

/**
 * Submit the composer: parse the inline syntax, POST the task, then refresh.
 * The input clears immediately (optimistic) so you can keep typing the next
 * task without waiting on the round-trip. On error we restore the text and
 * flash a status so nothing is lost.
 */
async function submitComposer(): Promise<void> {
  const raw = els.input.value;
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

els.input.addEventListener("input", updateComposerPreview);
els.composer.addEventListener("submit", (e) => {
  e.preventDefault();
  submitComposer();
});

// --- F11: filter bar wiring ------------------------------------------------

/** Update filter state and repaint. Selection is reconciled inside render(). */
function setFilter(next: Partial<FilterState>): void {
  filter = { ...filter, ...next };
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
  const btn = (e.target as HTMLElement | null)?.closest<HTMLElement>("[data-prio]");
  if (!btn) return;
  const prio = btn.dataset.prio as Priority;
  setFilter({ priorities: toggleMember(filter.priorities, prio) });
});

// Tag chips: toggle membership on click (delegated).
els.filterTags.addEventListener("click", (e) => {
  const btn = (e.target as HTMLElement | null)?.closest<HTMLElement>("[data-tag]");
  if (!btn) return;
  const tag = btn.dataset.tag ?? "";
  setFilter({ tags: toggleMember(filter.tags, tag) });
});

// Hide-done toggle.
els.filterHideDone.addEventListener("click", () => {
  setFilter({ hideDone: !filter.hideDone });
});

// Clear-all affordance resets every facet.
els.filterClear.addEventListener("click", () => {
  els.filterInput.value = "";
  filter = emptyFilter();
  render();
  els.filterInput.focus();
});

// --- F13: stats sidebar wiring ---------------------------------------------

els.statsToggle.addEventListener("click", () => toggleStats(!statsOpen));
// Clicking a top-tag row drives the F11 tag filter (and opens the filter view).
els.statsPanel.addEventListener("click", (e) => {
  const row = (e.target as HTMLElement | null)?.closest<HTMLElement>("[data-stat-tag]");
  if (!row) return;
  const tag = row.dataset.statTag ?? "";
  if (!tag) return;
  setFilter({ tags: filter.tags.includes(tag) ? filter.tags : [...filter.tags, tag] });
});

// --- F16: bulk action bar wiring -------------------------------------------

els.bulkbar.addEventListener("click", (e) => {
  const btn = (e.target as HTMLElement | null)?.closest<HTMLElement>("button");
  if (!btn) return;
  if (btn.dataset.bulkToggle !== undefined) bulkToggleDone();
  else if (btn.dataset.bulkDelete !== undefined) bulkDelete();
  else if (btn.dataset.bulkClear !== undefined) bulkClear();
});

// --- F19: export menu wiring -----------------------------------------------

let exportOpen = false;

/** Show/hide the export dropdown, painting its items lazily on first open. */
function toggleExportMenu(open: boolean): void {
  exportOpen = open;
  els.exportMenu.hidden = !open;
  els.exportToggle.setAttribute("aria-expanded", String(open));
  els.exportToggle.classList.toggle("is-active", open);
  if (open && els.exportMenu.childElementCount === 0) {
    els.exportMenu.innerHTML = renderExportMenu();
  }
}

/**
 * Trigger a download of the task list in the chosen format. Uses a temporary
 * anchor with the download attribute so the browser saves the file (the server
 * also sets Content-Disposition as a belt-and-suspenders). Closes the menu.
 */
function downloadExport(format: ExportFormat): void {
  const a = document.createElement("a");
  a.href = exportUrl(format);
  a.download = exportFilename(format);
  document.body.appendChild(a);
  a.click();
  a.remove();
  toggleExportMenu(false);
  setStatus(`exported ${format}`, false);
  setTimeout(() => setStatus("ready", false), 2_000);
}

els.exportToggle.addEventListener("click", (e) => {
  e.stopPropagation();
  toggleExportMenu(!exportOpen);
});
els.exportMenu.addEventListener("click", (e) => {
  const item = (e.target as HTMLElement | null)?.closest<HTMLElement>("[data-export-format]");
  if (!item) return;
  downloadExport(item.dataset.exportFormat as ExportFormat);
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
  if (e.key === "Escape") {
    els.input.value = "";
    updateComposerPreview();
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
  if (e.metaKey || e.ctrlKey || e.altKey) return;
  if (isTypingTarget(e.target)) return;
  if (editing || duePicking) return; // inline edit / due picker handle their own keys

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
    case "Escape":
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
  ["x / del", "Delete the selected task (undoable)"],
  ["cmd/shift-click", "Bulk-select rows (then toggle / delete many)"],
  ["drag ⠿", "Reorder a task (persists to .tsk.md)"],
  ["u", "Undo the last delete"],
  ["n", "Focus the add-task field"],
  ["/", "Focus the filter box"],
  ["s", "Toggle the stats panel"],
  ["t", "Cycle theme (auto / light / dark)"],
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
        ${HELP_ROWS.map(
          ([keys, desc]) =>
            `<div class="help-row"><dt><kbd>${escapeAttr(keys)}</kbd></dt><dd>${escapeAttr(desc)}</dd></div>`,
        ).join("")}
      </dl>
      <div class="help-foot">Press <kbd>?</kbd> or <kbd>esc</kbd> to close</div>
    </div>`;
  // Click the backdrop (not the card) to dismiss.
  el.addEventListener("click", (e) => {
    if (e.target === el) toggleHelp(false);
  });
  document.body.appendChild(el);
  return el;
}

function toggleHelp(open: boolean): void {
  helpOpen = open;
  const el = ensureHelpEl();
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

/**
 * Build the command registry from the live app state. Rebuilt each open so
 * enabled/disabled flags (e.g. undo) and the current selection reflect reality.
 */
function buildCommands(): Command[] {
  const sel = nav.selectedId;
  const hasSel = sel !== null;
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
    { id: "theme", title: "Cycle theme (auto/light/dark)", group: "View", keywords: ["dark", "light", "color"], hint: "t" },
    { id: "refresh", title: "Refresh from disk", group: "View", keywords: ["reload", "sync"], hint: "r" },
    { id: "export-json", title: "Export tasks as JSON", group: "Export", keywords: ["download", "save"] },
    { id: "export-csv", title: "Export tasks as CSV", group: "Export", keywords: ["download", "spreadsheet"] },
    { id: "export-markdown", title: "Export tasks as Markdown", group: "Export", keywords: ["download", "md"] },
    { id: "help", title: "Show keyboard shortcuts", group: "View", keywords: ["keys", "?"], hint: "?" },
    { id: "alltasks", title: "Go to all tasks", group: "View", keywords: ["home", "clear tag"], disabled: route.kind !== "tag" },
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
    case "theme":
      cycleTheme();
      break;
    case "refresh":
      refresh();
      break;
    case "export-json":
      downloadExport("json");
      break;
    case "export-csv":
      downloadExport("csv");
      break;
    case "export-markdown":
      downloadExport("markdown");
      break;
    case "help":
      toggleHelp(true);
      break;
    case "alltasks":
      if (route.kind === "tag") navigateToAll();
      break;
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
  paletteResults = filterCommands(allCommands(), query);
  paletteIndex = clampIndex(paletteIndex, paletteResults.length);
  paintPalette();
}

/** Repaint the list + active highlight, scrolling it into view. */
function paintPalette(): void {
  const el = ensurePaletteEl();
  const list = el.querySelector<HTMLElement>("[data-cmdk-list]")!;
  list.innerHTML = renderPaletteList(paletteResults, paletteIndex);
  const active = list.querySelector<HTMLElement>(".is-active");
  active?.scrollIntoView({ block: "nearest" });
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

/** Paint the live indicator pill for the given connection status. */
function setLiveStatus(status: LiveStatus): void {
  els.live.innerHTML = renderLiveIndicator(status);
  els.live.title = liveTitle(status);
  els.live.dataset.status = status;
}

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
    // Don't clobber an in-progress edit/due-pick: those own the keyboard and a
    // re-render would tear down their input. Defer the refresh until they close.
    if (editing || duePicking) {
      liveRefreshPending = true;
      return;
    }
    refresh();
  } else {
    liveFingerprint = fp;
  }
}

/** Set when a live change arrives mid-edit; flushed when the edit settles. */
let liveRefreshPending = false;

/** Flush a deferred live refresh once an inline edit / due picker has closed. */
function flushPendingLiveRefresh(): void {
  if (liveRefreshPending && !editing && !duePicking) {
    liveRefreshPending = false;
    refresh();
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
  setLiveStatus("connecting");
  const es = new EventSource("/api/events");
  liveSource = es;
  es.addEventListener("ready", (e) => {
    setLiveStatus("live");
    onLiveFrame((e as MessageEvent).data);
  });
  es.addEventListener("change", (e) => {
    setLiveStatus("live");
    onLiveFrame((e as MessageEvent).data);
  });
  es.addEventListener("open", () => setLiveStatus("live"));
  es.addEventListener("error", () => {
    setLiveStatus("offline");
    // EventSource will retry on its own for network blips; on a closed stream
    // (readyState CLOSED) we re-create it after a short backoff.
    if (es.readyState === EventSource.CLOSED && liveReconnectTimer === null) {
      liveReconnectTimer = window.setTimeout(connectLive, 3_000);
    }
  });
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
  tag: navigateToTag,
  palette: (open: boolean) => (open ? openPalette() : closePalette()),
};

// Restore the persisted stats-panel state before the first paint.
applyStatsVisibility();
// Restore the persisted theme before the first paint to avoid a flash.
applyTheme();
// F21: open the live-reload stream so external edits flow into the open tab.
connectLive();
// F22: register the offline-shell service worker (no-op where unsupported).
registerServiceWorker();
refresh();
