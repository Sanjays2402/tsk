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
 * F95: the minimal shape the bulk-bar no-op detectors need from a selected task
 * — its current priority and pin flag. The bulk menus compute whether an action
 * would change anything over the whole selection so a greyed option can explain
 * WHY (mirroring F89's palette disabled-reason hints, now extended to the
 * floating bulk bar).
 */
export interface BulkTaskLike {
  priority?: string;
  pinned?: boolean;
}

/**
 * F95: why (if at all) setting the whole selection to `level` would be a no-op.
 * Returns "all already <level>" when EVERY selected task is already at that
 * priority (so the PATCH fan-out would touch nothing), else "". An empty
 * selection returns "" — the bar is hidden then anyway. Pure → unit-tested.
 */
export function bulkPriorityDisabledReason(selected: BulkTaskLike[], level: Priority): string {
  if (selected.length === 0) return "";
  return selected.every((t) => t.priority === level) ? `all already ${level}` : "";
}

/**
 * F95: why (if at all) a pin/unpin-all would be a no-op. `pin` true = "Pin all"
 * (no-op when every selected task is ALREADY pinned -> "all already pinned");
 * `pin` false = "Unpin all" (no-op when NONE are pinned -> "none are pinned").
 * Returns "" when at least one task would flip. Pure → unit-tested.
 */
export function bulkPinDisabledReason(selected: BulkTaskLike[], pin: boolean): string {
  if (selected.length === 0) return "";
  if (pin) return selected.every((t) => t.pinned) ? "all already pinned" : "";
  return selected.every((t) => !t.pinned) ? "none are pinned" : "";
}

/** F95: a quiet reason line for the bulk-edit popover; "" collapses it. */
function bulkReasonLine(reason: string): string {
  if (reason === "") return "";
  return `<div class="bulkedit-reason" role="note">${escapeHTML(reason)}</div>`;
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
    <button type="button" class="bulkbar-btn" data-bulk-edit="due" title="Set due date for all selected">due</button>
    <button type="button" class="bulkbar-btn" data-bulk-edit="pin" title="Pin or unpin all selected">pin</button>`;
}

/**
 * The 4-way priority menu shown when "priority" is chosen.
 *
 * F95: when `selected` is supplied, an option that every selected task is
 * ALREADY at is greyed (`is-disabled` + aria-disabled) and a quiet reason line
 * ("all already high") explains why — the bulk-bar sister of F89's palette
 * disabled-reason hints. The disabled button keeps its `data-bulk-set-prio` hook
 * so the delegated handler in main.ts can short-circuit a no-op without a PATCH
 * fan-out. With no `selected` (the default) nothing is disabled, so existing
 * callers/tests render exactly as before.
 */
export function renderBulkPriorityMenu(selected: BulkTaskLike[] = []): string {
  const reasons = BULK_PRIORITIES.map((p) => bulkPriorityDisabledReason(selected, p));
  const buttons = BULK_PRIORITIES.map((p, i) => {
    const dis = reasons[i] !== "" ? " is-disabled" : "";
    const aria = reasons[i] !== "" ? ' aria-disabled="true"' : "";
    return `<button type="button" class="bulkedit-prio ${escapeHTML(p)}${dis}" data-bulk-set-prio="${escapeHTML(p)}"${aria} title="Set ${escapeHTML(p)}">${priorityGlyph(p)}<span class="bulkedit-prio-word">${escapeHTML(p)}</span></button>`;
  }).join("");
  // Show the reason for the FIRST disabled option (they share the "all already
  // <level>" shape; only one level can match when the whole selection is uniform).
  const reason = reasons.find((r) => r !== "") ?? "";
  return `<div class="bulkedit-pop-row">${buttons}</div>${bulkReasonLine(reason)}`;
}

/**
 * F47: the pin/unpin menu shown when "pin" is chosen — two actions over the
 * selection (pin all to the top, or unpin all). Buttons carry data-bulk-set-pin
 * ("1" pins, "0" unpins) a delegated listener dispatches on. Pure → unit-tested.
 *
 * F95: when `selected` is supplied, an action that would change nothing is
 * greyed with a reason ("all already pinned" for Pin all when everything's
 * pinned; "none are pinned" for Unpin all when nothing is). Default empty
 * selection disables nothing, so existing callers/tests are unaffected.
 */
export function renderBulkPinMenu(selected: BulkTaskLike[] = []): string {
  const pinReason = bulkPinDisabledReason(selected, true);
  const unpinReason = bulkPinDisabledReason(selected, false);
  const pinDis = pinReason !== "" ? " is-disabled" : "";
  const unpinDis = unpinReason !== "" ? " is-disabled" : "";
  const pinAria = pinReason !== "" ? ' aria-disabled="true"' : "";
  const unpinAria = unpinReason !== "" ? ' aria-disabled="true"' : "";
  const reason = pinReason || unpinReason;
  return `<div class="bulkedit-pop-row">
      <button type="button" class="bulkedit-pin${pinDis}" data-bulk-set-pin="1"${pinAria} title="Pin all selected to the top">&#9733; Pin all</button>
      <button type="button" class="bulkedit-pin${unpinDis}" data-bulk-set-pin="0"${unpinAria} title="Unpin all selected">&#9734; Unpin all</button>
    </div>${bulkReasonLine(reason)}`;
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

/**
 * The due-editor popover content (a natural-language input + a hint). F47 adds
 * a live-preview line below the input that main.ts fills from /api/parse-date
 * as you type (reusing the F12 picker's previewVM + renderDuePreview), so you
 * see the resolved date before applying it to the whole selection.
 */
export function renderBulkDueEditor(): string {
  return `
    <div class="bulkedit-pop-row">
      <input class="bulkedit-input" data-bulk-due-input type="text" spellcheck="false"
             placeholder="today, fri, in 3d, eom, 2026-07-04…" aria-label="Bulk due date">
    </div>
    <div class="bulkedit-due-preview" data-bulk-due-preview></div>
    <div class="bulkedit-hint">Enter to apply to all selected &middot; empty + Enter clears</div>`;
}
