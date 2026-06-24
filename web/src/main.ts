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

const root = document.getElementById("root");
if (!root) throw new Error("missing #root");

root.innerHTML = `
  <div class="app" data-app>
    <header class="topbar">
      <h1>tsk<span class="dot" data-dot>// loading</span></h1>
      <div class="file" data-file>—</div>
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
    <div data-content>
      <div class="skeleton" aria-busy="true" aria-label="loading tasks">
        <div class="bar w-80"></div>
        <div class="bar w-60"></div>
        <div class="bar w-40"></div>
      </div>
    </div>
    <footer class="statusline">
      <span class="count" data-count></span>
      <span data-build><kbd class="kbd-hint" data-help-open>?</kbd> shortcuts &middot; <a href="/api/tasks" style="color:inherit">api</a></span>
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

/** Render the current state to the DOM, preserving keyboard selection. */
function render(): void {
  const now = new Date();
  const sections = groupIntoSections(currentTasks, now);
  const prevIds = visibleIds;
  visibleIds = flattenSections(sections).map((t) => t.id);
  nav = reconcile(nav, visibleIds, prevIds);
  els.content.innerHTML = renderSections(sections, now);
  els.count.innerHTML = summarize(currentTasks);
  applySelection();
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

/** Re-fetch and re-render the list. Idempotent; safe to call any time. */
async function refresh(): Promise<void> {
  try {
    const { file, tasks } = await api.listTasks();
    currentTasks = tasks;
    els.file.textContent = file;
    setStatus("ready", false);
    render();
  } catch (err) {
    showError(err);
  }
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

// Clicking a row selects it (so mouse + keyboard stay in sync). Ignore clicks
// on the checkbox itself, which has its own change handler.
els.content.addEventListener("click", (e) => {
  const target = e.target as HTMLElement | null;
  if (!target || target instanceof HTMLInputElement) return;
  const row = target.closest("[data-id]") as HTMLElement | null;
  if (!row) return;
  const id = Number(row.dataset.id);
  if (!Number.isFinite(id) || id <= 0) return;
  nav = select(nav, visibleIds, id);
  applySelection();
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
    case "r":
      e.preventDefault();
      refresh();
      break;
    case "n":
      e.preventDefault();
      els.input.focus();
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
  ["j / \u2193", "Move selection down"],
  ["k / \u2191", "Move selection up"],
  ["g / G", "Jump to first / last"],
  ["space / enter", "Toggle the selected task done"],
  ["n", "Focus the add-task field"],
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
};

refresh();
