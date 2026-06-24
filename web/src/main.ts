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
import { renderTasks, summarize } from "./render";
import { parseQuickAdd, isSubmittable } from "./quickadd";
import { renderComposerPreview } from "./composer";

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
      <span data-build>tsk web &middot; <a href="/api/tasks" style="color:inherit">api</a></span>
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

/** Render the current state to the DOM. */
function render(): void {
  const now = new Date();
  els.content.innerHTML = renderTasks(currentTasks, now);
  els.count.innerHTML = summarize(currentTasks);
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

// Pick up external edits (CLI / TUI / hand-edit) when the tab regains focus.
document.addEventListener("visibilitychange", () => {
  if (document.visibilityState === "visible") {
    refresh();
  }
});

// `r` refreshes, `n` focuses the composer — both ignored while typing.
function isTypingTarget(el: EventTarget | null): boolean {
  const t = el as HTMLElement | null;
  return !!t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.isContentEditable);
}

document.addEventListener("keydown", (e) => {
  if (e.metaKey || e.ctrlKey || e.altKey) return;
  if (isTypingTarget(e.target)) return;
  if (e.key === "r") {
    e.preventDefault();
    refresh();
  } else if (e.key === "n") {
    e.preventDefault();
    els.input.focus();
  }
});

// Escape hatches for upcoming slices.
declare global {
  interface Window {
    tsk: {
      refresh: () => Promise<void>;
      tasks: () => Task[];
      toggle: (id: number) => Promise<void>;
      add: (line: string) => Promise<void>;
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
};

refresh();
