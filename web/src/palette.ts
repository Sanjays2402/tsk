/**
 * Command palette model (F18) — a pure, testable core for a Cmd-K palette that
 * fuzzy-finds every action in the app and runs it keyboard-only.
 *
 * main.ts owns the overlay DOM, the global Cmd-K shortcut, and the actual
 * action callbacks; this module owns the data: what a Command is, how the
 * query filters + ranks the list, and how the highlighted index moves. Keeping
 * it pure means the fuzzy ranking and selection math are unit-tested with zero
 * DOM.
 *
 * Ranking (best first), all case-insensitive:
 *   1. exact title match
 *   2. title starts with the query
 *   3. query is a substring of the title
 *   4. query is a subsequence of "title + keywords" (fuzzy)
 * Ties break by the command's declared order, so the list is stable.
 */

import { highlightText } from "./highlight.ts";

export interface Command {
  /** Stable id used as the action key. */
  id: string;
  /** Human label shown in the list. */
  title: string;
  /** Optional section grouping label (e.g. "Task", "View"). */
  group?: string;
  /** Extra words that should match the query but aren't shown. */
  keywords?: string[];
  /** Optional shortcut hint string shown on the right (e.g. "n"). */
  hint?: string;
  /** True to grey it out + skip running (e.g. "undo" with nothing to undo). */
  disabled?: boolean;
}

export interface ScoredCommand {
  cmd: Command;
  score: number;
}

/** Case-insensitive subsequence test. */
export function isSubseq(needle: string, hay: string): boolean {
  if (needle === "") return true;
  let ni = 0;
  for (let hi = 0; hi < hay.length && ni < needle.length; hi++) {
    if (hay[hi] === needle[ni]) ni++;
  }
  return ni === needle.length;
}

/**
 * Score a command against a query. Returns a number where higher is better, or
 * -1 when it doesn't match at all. An empty query matches everything with a
 * neutral score so the full list shows in declared order.
 */
export function scoreCommand(cmd: Command, query: string): number {
  const q = query.trim().toLowerCase();
  if (q === "") return 0;
  const title = cmd.title.toLowerCase();
  if (title === q) return 100;
  if (title.startsWith(q)) return 80;
  const idx = title.indexOf(q);
  if (idx >= 0) return 60 - Math.min(idx, 20); // earlier substring ranks higher
  // Fuzzy over title + keywords.
  const hay = (cmd.title + " " + (cmd.keywords ?? []).join(" ")).toLowerCase();
  if (isSubseq(q, hay)) return 30;
  return -1;
}

/**
 * Filter + rank commands for a query, preserving declared order on ties.
 * Disabled commands still appear (greyed) but sink slightly so live actions
 * surface first at equal score.
 */
export function filterCommands(commands: Command[], query: string): Command[] {
  const scored: Array<{ cmd: Command; score: number; order: number }> = [];
  commands.forEach((cmd, order) => {
    const score = scoreCommand(cmd, query);
    if (score < 0) return;
    scored.push({ cmd, score: cmd.disabled ? score - 0.5 : score, order });
  });
  scored.sort((a, b) => (b.score !== a.score ? b.score - a.score : a.order - b.order));
  return scored.map((s) => s.cmd);
}

/** Clamp the highlighted index into [0, len), wrapping past the ends. */
export function moveIndex(current: number, len: number, delta: number): number {
  if (len <= 0) return 0;
  return (((current + delta) % len) + len) % len;
}

/** Clamp an index into range without wrapping (used after the list shrinks). */
export function clampIndex(current: number, len: number): number {
  if (len <= 0) return 0;
  return Math.max(0, Math.min(current, len - 1));
}

/** Escape strings before injecting into innerHTML. Local copy keeps this pure. */
function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}

/**
 * Render the results list. Pure -> unit-tested. The highlighted row carries
 * `is-active` + aria-selected; each row carries data-cmd-id for dispatch.
 * Renders an empty-state row when nothing matches.
 *
 * F57: when a `query` is passed, the matched subsequence in each command title
 * is wrapped in <mark> (reusing the generic highlightText engine that powers
 * the row title / tag / notes highlight), so it's obvious why a fuzzy match
 * surfaced. The title is HTML-escaped by highlightText; without a query it
 * falls back to plain escaping.
 */
export function renderPaletteList(
  commands: Command[],
  activeIndex: number,
  query = "",
): string {
  if (commands.length === 0) {
    return `<li class="cmdk-empty" role="option" aria-disabled="true">No matching commands</li>`;
  }
  return commands
    .map((cmd, i) => {
      const active = i === activeIndex ? " is-active" : "";
      const disabled = cmd.disabled ? " is-disabled" : "";
      const group = cmd.group
        ? `<span class="cmdk-group">${escapeHTML(cmd.group)}</span>`
        : "";
      const hint = cmd.hint
        ? `<kbd class="cmdk-hint">${escapeHTML(cmd.hint)}</kbd>`
        : "";
      const title = highlightText(cmd.title, query);
      return `<li class="cmdk-item${active}${disabled}" role="option" aria-selected="${i === activeIndex}" data-cmd-id="${escapeHTML(cmd.id)}">
        <span class="cmdk-title">${title}</span>
        ${group}${hint}
      </li>`;
    })
    .join("");
}
