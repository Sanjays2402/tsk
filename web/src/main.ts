/**
 * tsk web entry — vanilla DOM, framework-free.
 *
 * Slice F3 ships only the shell + a loading skeleton. The end-to-end vertical
 * (load + render the actual list) lands in slice F4, and interactive toggle
 * in F5 — both in this same tick.
 */

import { api } from "./api";

const root = document.getElementById("root");
if (!root) throw new Error("missing #root");

root.innerHTML = `
  <div class="app" data-app>
    <header class="topbar">
      <h1>tsk<span class="dot">// loading</span></h1>
      <div class="file" data-file>—</div>
    </header>
    <div data-content>
      <div class="skeleton" aria-busy="true" aria-label="loading tasks">
        <div class="bar w-80"></div>
        <div class="bar w-60"></div>
        <div class="bar w-40"></div>
      </div>
    </div>
    <footer class="statusline">
      <span class="count" data-count></span>
      <span data-build>tsk web</span>
    </footer>
  </div>
`;

/**
 * boot probes /api/health to confirm the server is reachable and updates the
 * shell. Slice F4 replaces the loading skeleton with the live task list.
 */
async function boot(): Promise<void> {
  const fileEl = document.querySelector("[data-file]") as HTMLElement;
  const dotEl = document.querySelector(".topbar h1 .dot") as HTMLElement;
  try {
    const health = await api.health();
    fileEl.textContent = health.file;
    dotEl.textContent = "// ready";
    dotEl.style.color = "var(--color-text-faint)";
  } catch (err) {
    dotEl.textContent = "// offline";
    dotEl.style.color = "var(--color-prio-urgent)";
    showBanner(err);
  }
}

function showBanner(err: unknown): void {
  const content = document.querySelector("[data-content]") as HTMLElement;
  const message = err instanceof Error ? err.message : String(err);
  content.innerHTML = `
    <div class="banner" role="alert">
      <span>Couldn't reach <code>tsk serve</code>:</span>
      <code>${escapeHTML(message)}</code>
    </div>`;
}

function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}

boot();
