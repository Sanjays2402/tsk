/**
 * Touch priority picker (F41) — a pure, testable layer that builds a 4-way
 * priority picker for a task. On a phone there's no alt/shift-click to reach
 * the "cycle down" affordance the desktop chip offers (F29), so a LONG PRESS on
 * the priority chip (reusing the F28 long-press machine) opens this picker:
 * tap any of Low / Medium / High / Urgent to set it directly.
 *
 * main.ts owns the touch events, the anchor/positioning, and the dispatch;
 * this module owns WHAT options exist, which one is active, and the markup —
 * so the option list and the active-state logic are unit-tested with zero DOM.
 * The ladder + glyphs mirror priority.ts so the picker and the chip never drift.
 */

import { PRIORITY_LADDER, priorityGlyph, type Priority } from "./priority.ts";

export interface PriorityOption {
  priority: Priority;
  /** Single-letter glyph (L/M/H/U), matching the chip + TUI. */
  glyph: string;
  /** Human label for the option row. */
  label: string;
  /** True when this is the task's current priority (rendered as selected). */
  active: boolean;
}

/** Title-case a priority name for the option label. */
function labelFor(p: Priority): string {
  return p.charAt(0).toUpperCase() + p.slice(1);
}

/**
 * Build the ordered option list for a task at `current` priority. Order is the
 * urgency ladder REVERSED (Urgent first) so the most-urgent option is at the
 * top of the picker — the quickest tap target for the common "bump it up" move.
 * Unknown current values mark nothing active (the picker still works as a
 * setter).
 */
export function priorityOptions(current: string): PriorityOption[] {
  return [...PRIORITY_LADDER]
    .slice()
    .reverse()
    .map((p) => ({
      priority: p,
      glyph: priorityGlyph(p),
      label: labelFor(p),
      active: p === current,
    }));
}

/** Escape strings before injecting into innerHTML. Local copy keeps this pure. */
function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", "'": "&#39;", '"': "&quot;" })[c]!,
  );
}

/**
 * Render the picker's inner list for a task. Each option carries
 * `data-set-prio="<priority>"` so a delegated listener dispatches on it; the
 * active option carries `is-active` + aria-checked. The glyph chips reuse the
 * `.priority` color classes so each level reads the same as it does on the row.
 */
export function renderPriorityPicker(current: string): string {
  const items = priorityOptions(current)
    .map((opt) => {
      const cls = ["prio-pick-item", opt.active ? "is-active" : ""].filter(Boolean).join(" ");
      return `<li class="${cls}" role="menuitemradio" aria-checked="${opt.active}" data-set-prio="${opt.priority}" tabindex="-1">
        <span class="prio-pick-glyph priority ${opt.priority}">${escapeHTML(opt.glyph)}</span>
        <span class="prio-pick-label">${escapeHTML(opt.label)}</span>
      </li>`;
    })
    .join("");
  return `<ul class="prio-pick-list" role="menu" aria-label="Set priority">${items}</ul>`;
}
