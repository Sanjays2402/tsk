/**
 * Reorder intent (F17) — pure helpers that translate a drag-and-drop gesture
 * into the `before` id the move API expects, and compute the optimistic local
 * reordering so the list can repaint instantly before the server confirms.
 *
 * main.ts owns the HTML5 drag events and the DOM; this module owns the math:
 * given the current id order, a dragged id, and a drop target (plus whether
 * the pointer landed on the top or bottom half of the target row), what is the
 * resulting order and what `before` id persists it? Keeping it pure means the
 * index juggling is unit-tested with zero DOM.
 *
 * The store renders .tsk.md in slice order and `Move(moved, before)` drops the
 * moved task immediately in front of `before` (0 = end), so these helpers
 * mirror that contract exactly.
 *
 * IMPORTANT: reordering only makes sense within a single flat order. The UI
 * restricts drags to within one section (you can't drag an Overdue task into
 * Done), and the order we persist is the *global* file order — so callers pass
 * the full id list, and we keep every non-dragged id where it was.
 */

export type DropPos = "before" | "after";

export interface ReorderResult {
  /** The new full id order after the move. */
  order: number[];
  /** The id to send as `before` to the move API (0 = move to end). */
  before: number;
  /** False when the drop is a no-op (same position); callers can skip the call. */
  changed: boolean;
}

/**
 * Compute the reorder for dragging `moved` onto `target`, dropping on the
 * `pos` half of the target row. Returns the new order, the `before` id to
 * persist, and whether anything actually changed.
 *
 * Edge cases:
 *   - dropping a task onto itself is a no-op
 *   - an unknown moved/target id is a no-op (returns the input order)
 *   - dropping "after" the last row maps to before:0 (move to end)
 */
export function computeReorder(
  order: number[],
  moved: number,
  target: number,
  pos: DropPos,
): ReorderResult {
  const from = order.indexOf(moved);
  const ti = order.indexOf(target);
  if (from < 0 || ti < 0 || moved === target) {
    return { order: order.slice(), before: moved, changed: false };
  }
  // The id the moved task should land in front of, in the ORIGINAL order.
  // "before" target -> land in front of target.
  // "after"  target -> land in front of whatever currently follows target
  //                    (or the end, if target is last).
  let beforeId: number;
  if (pos === "before") {
    beforeId = target;
  } else {
    const afterIdx = ti + 1;
    beforeId = afterIdx < order.length ? order[afterIdx] : 0;
  }
  // If we'd be dropping the task right back where it already sits, no-op.
  if (beforeId === moved) {
    return { order: order.slice(), before: moved, changed: false };
  }
  // Build the new order: remove `moved`, then insert in front of beforeId
  // (or at the end when beforeId is 0 / not found post-removal).
  const without = order.filter((id) => id !== moved);
  let insertAt: number;
  if (beforeId === 0) {
    insertAt = without.length;
  } else {
    insertAt = without.indexOf(beforeId);
    if (insertAt < 0) insertAt = without.length;
  }
  const next = [...without.slice(0, insertAt), moved, ...without.slice(insertAt)];
  // Did the order actually change?
  const changed = !arraysEqual(order, next);
  return { order: next, before: beforeId, changed };
}

/** Shallow equality for number arrays. */
export function arraysEqual(a: number[], b: number[]): boolean {
  if (a.length !== b.length) return false;
  for (let i = 0; i < a.length; i++) if (a[i] !== b[i]) return false;
  return true;
}

/**
 * Given the bounding rect of a row and the pointer's clientY, decide whether
 * the drop should land before (top half) or after (bottom half) the row.
 */
export function dropPosForY(top: number, height: number, clientY: number): DropPos {
  return clientY < top + height / 2 ? "before" : "after";
}
