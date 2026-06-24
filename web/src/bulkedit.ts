/**
 * Bulk edit (F36) — a pure, testable layer that extends the bulk-selection
 * model (F16) beyond toggle/delete into "change a field on every selected
 * task at once": set priority, add/remove a tag, or set a due date.
 *
 * main.ts owns the floating-bar buttons, the little popover, and the parallel
 * PATCH fan-out; this module owns the data: how a "+dev -home" tag command
 * parses into add/remove sets, how that op rewrites one task's tag list, and
 * the popover markup for each action. Keeping it pure means the set algebra is
 * unit-tested with zero DOM.
 *
 * Tag-op grammar (whitespace-delimited, order-independent):
 *   +tag  or  #tag  or  tag   -> ADD that tag
 *   -tag                      -> REMOVE that tag
 * Tags are lower-cased + de-duplicated to match the store. A bare "+", "-", or
 * "#" with nothing after it is ignored.
 */

export type Priority = "low" | "medium" | "high" | "urgent";

/** The 4-way priority ladder shown in the bulk priority menu. */
export const BULK_PRIORITIES: ReadonlyArray<Priority> = ["low", "medium", "high", "urgent"];

/** A parsed tag command: which tags to add and which to remove. */
export interface TagOps {
  add: string[];
  remove: string[];
}

/**
 * Parse a "+dev -home #urgent" style command into add/remove sets. A token's
 * leading sigil decides the action: `-` removes, `+`/`#`/bare adds. Tags are
 * lower-cased; each set is de-duplicated in first-seen order. A tag that
 * appears in BOTH add and remove resolves to remove (the explicit minus wins),
 * so "+x -x" clears x rather than churning it.
 */
export function parseTagOps(input: string): TagOps {
  const tokens = input.trim().split(/\s+/).filter(Boolean);
  const add: string[] = [];
  const remove: string[] = [];
  for (const tok of tokens) {
    const sigil = tok[0];
    let name: string;
    let bucket: string[];
    if (sigil === "-") {
      name = tok.slice(1);
      bucket = remove;
    } else if (sigil === "+" || sigil === "#") {
      name = tok.slice(1);
      bucket = add;
    } else {
      name = tok;
      bucket = add;
    }
    name = name.toLowerCase();
    if (name === "") continue;
    if (!bucket.includes(name)) bucket.push(name);
  }
  // An explicit remove overrides a same-token add.
  const removeSet = new Set(remove);
  const cleanedAdd = add.filter((t) => !removeSet.has(t));
  return { add: cleanedAdd, remove };
}

/** True when the op would do nothing (no adds, no removes). */
export function isNoopTagOps(ops: TagOps): boolean {
  return ops.add.length === 0 && ops.remove.length === 0;
}

/**
 * Apply a tag op to one task's existing tag list, returning the new list.
 * Removes first, then unions the adds (preserving the original order, then
 * appending genuinely-new tags). Lower-cases existing tags for comparison so a
 * differently-cased duplicate doesn't sneak through. Never mutates the input.
 */
export function applyTagOps(existing: string[], ops: TagOps): string[] {
  const removeSet = new Set(ops.remove);
  const out: string[] = [];
  const seen = new Set<string>();
  for (const raw of existing) {
    const t = raw.toLowerCase();
    if (removeSet.has(t) || seen.has(t)) continue;
    seen.add(t);
    out.push(t);
  }
  for (const t of ops.add) {
    if (seen.has(t)) continue;
    seen.add(t);
    out.push(t);
  }
  return out;
}

/** Escape strings before injecting into innerHTML. Local copy keeps this pure. */
function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}

/** Single-letter glyph for a priority, matching the row chip + TUI. */
export function priorityGlyph(p: Priority): string {
  return { low: "L", medium: "M", high: "H", urgent: "U" }[p];
}

/**
 * The extra action buttons injected into the bulk bar (F36): openers for the
 * priority menu, the tag editor, and the due editor. Each carries a
 * data-bulk-edit hook a delegated listener in main.ts dispatches on. Rendered
 * separately from F16's core bar so this slice stays additive + revertible.
 */
export function renderBulkEditCluster(): string {
  return `
    <button type="button" class="bulkbar-btn" data-bulk-edit="priority" title="Set priority for all selected">priority</button>
    <button type="button" class="bulkbar-btn" data-bulk-edit="tag" title="Add or remove a tag on all selected">tag</button>
    <button type="button" class="bulkbar-btn" data-bulk-edit="due" title="Set due date for all selected">due</button>`;
}

/** The 4-way priority menu shown when "priority" is chosen. */
export function renderBulkPriorityMenu(): string {
  const buttons = BULK_PRIORITIES.map(
    (p) =>
      `<button type="button" class="bulkedit-prio ${escapeHTML(p)}" data-bulk-set-prio="${escapeHTML(p)}" title="Set ${escapeHTML(p)}">${priorityGlyph(p)}<span class="bulkedit-prio-word">${escapeHTML(p)}</span></button>`,
  ).join("");
  return `<div class="bulkedit-pop-row">${buttons}</div>`;
}

/** The tag-editor popover content (an input + a hint). */
export function renderBulkTagEditor(): string {
  return `
    <div class="bulkedit-pop-row">
      <input class="bulkedit-input" data-bulk-tag-input type="text" spellcheck="false"
             placeholder="+add -remove (e.g. +urgent -someday)" aria-label="Bulk tag command">
    </div>
    <div class="bulkedit-hint">Enter to apply &middot; <code>+tag</code> adds, <code>-tag</code> removes</div>`;
}

/** The due-editor popover content (a natural-language input + a hint). */
export function renderBulkDueEditor(): string {
  return `
    <div class="bulkedit-pop-row">
      <input class="bulkedit-input" data-bulk-due-input type="text" spellcheck="false"
             placeholder="today, fri, in 3d, eom, 2026-07-04…" aria-label="Bulk due date">
    </div>
    <div class="bulkedit-hint">Enter to apply to all selected &middot; empty + Enter clears</div>`;
}
