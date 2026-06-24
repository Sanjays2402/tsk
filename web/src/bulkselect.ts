/**
 * Bulk-selection model (F16) — a pure, testable layer for selecting many
 * tasks at once and acting on them in one go (multi-toggle, multi-delete).
 *
 * main.ts owns the pointer/keyboard events and the DOM; this module owns the
 * data: which ids are in the bulk set, how a shift-click range resolves
 * against the visible order, and how the action-bar summary reads. Keeping it
 * pure means the range math and set algebra are unit-tested with zero DOM.
 *
 * Interaction model (documented so the UI copy can match):
 *   - cmd/ctrl + click a row  -> toggle that single row in/out of the set
 *   - shift + click a row     -> select the inclusive range from the anchor
 *                                (last cmd/shift-clicked row) to the clicked row,
 *                                walking the visible top-to-bottom order
 *   - the anchor seeds future shift-clicks; a plain click clears the set
 *   - Escape / Clear empties the set; acting (toggle/delete) clears it too
 */

export interface BulkState {
  /** The set of bulk-selected task ids. */
  ids: Set<number>;
  /** Anchor row for range selection (last single-selected id), or null. */
  anchor: number | null;
}

/** A fresh, empty bulk state. */
export function emptyBulk(): BulkState {
  return { ids: new Set(), anchor: null };
}

/** True when at least one row is bulk-selected. */
export function isBulkActive(state: BulkState): boolean {
  return state.ids.size > 0;
}

/** Is this id part of the current bulk selection? */
export function isSelected(state: BulkState, id: number): boolean {
  return state.ids.has(id);
}

/**
 * Toggle a single id in/out of the set (cmd/ctrl-click). The toggled row
 * becomes the new anchor so a following shift-click ranges from here. Returns
 * a new state; never mutates the input.
 */
export function toggleOne(state: BulkState, id: number): BulkState {
  const ids = new Set(state.ids);
  if (ids.has(id)) ids.delete(id);
  else ids.add(id);
  return { ids, anchor: id };
}

/**
 * Select the inclusive range between the anchor and target id, walking the
 * visible order (shift-click). The range is UNIONed onto the existing set so
 * you can build a multi-range selection. If there is no anchor yet, or either
 * endpoint isn't visible, this falls back to selecting just the target. The
 * anchor is preserved so successive shift-clicks re-range from the same start.
 */
export function selectRange(
  state: BulkState,
  visibleIds: number[],
  target: number,
): BulkState {
  const anchor = state.anchor;
  const ai = anchor === null ? -1 : visibleIds.indexOf(anchor);
  const ti = visibleIds.indexOf(target);
  if (ti < 0) return state;
  if (ai < 0) {
    // No usable anchor: select just the target and seed the anchor.
    const ids = new Set(state.ids);
    ids.add(target);
    return { ids, anchor: target };
  }
  const lo = Math.min(ai, ti);
  const hi = Math.max(ai, ti);
  const ids = new Set(state.ids);
  for (let i = lo; i <= hi; i++) ids.add(visibleIds[i]);
  // Keep the original anchor so re-ranging shrinks/grows from the same start.
  return { ids, anchor };
}

/** Empty the selection entirely (Escape / Clear / after an action). */
export function clearBulk(): BulkState {
  return emptyBulk();
}

/**
 * Drop any selected ids that are no longer visible (e.g. after a filter change,
 * delete, or refresh). Returns the same reference when nothing changed so
 * callers can cheaply skip a re-render.
 */
export function reconcileBulk(state: BulkState, visibleIds: number[]): BulkState {
  if (state.ids.size === 0) return state;
  const visible = new Set(visibleIds);
  let changed = false;
  const ids = new Set<number>();
  for (const id of state.ids) {
    if (visible.has(id)) ids.add(id);
    else changed = true;
  }
  const anchor = state.anchor !== null && visible.has(state.anchor) ? state.anchor : null;
  if (!changed && anchor === state.anchor) return state;
  return { ids, anchor };
}

/** The selected ids as a stable array in visible order (for batch actions). */
export function selectedInOrder(state: BulkState, visibleIds: number[]): number[] {
  return visibleIds.filter((id) => state.ids.has(id));
}

/** Summary line for the bulk action bar, e.g. "3 selected". */
export function bulkSummary(count: number): string {
  return `${count} selected`;
}

/** Escape strings before injecting into innerHTML. Local copy keeps this pure. */
function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}

/**
 * Render the floating bulk action bar. Pure -> unit-tested. Returns "" when
 * nothing is selected so the bar collapses. Buttons carry data-bulk-* hooks a
 * delegated listener in main.ts dispatches on.
 */
export function renderBulkBar(count: number): string {
  if (count <= 0) return "";
  return `
    <div class="bulkbar-inner">
      <span class="bulkbar-count">${escapeHTML(bulkSummary(count))}</span>
      <div class="bulkbar-actions">
        <button type="button" class="bulkbar-btn" data-bulk-toggle title="Toggle done for all selected">toggle done</button>
        <button type="button" class="bulkbar-btn is-danger" data-bulk-delete title="Delete all selected">delete</button>
        <button type="button" class="bulkbar-btn is-quiet" data-bulk-clear title="Clear selection (esc)">clear</button>
      </div>
    </div>`;
}
