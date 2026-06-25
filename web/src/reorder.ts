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
 * F44: compute the reorder that moves `moved` to the very TOP of the file
 * order (used by the "pin to top" shortcut). Returns the new order plus the
 * `before` id to persist — which is whatever currently sits first that ISN'T
 * the moved task, so `store.Move(moved, before)` drops it in front of the
 * whole list. A no-op when `moved` is already first or unknown.
 */
export function computePinToTop(order: number[], moved: number): ReorderResult {
  const from = order.indexOf(moved);
  if (from < 0) {
    return { order: order.slice(), before: moved, changed: false };
  }
  // The first id that isn't the moved task — moved will land in front of it.
  const firstOther = order.find((id) => id !== moved);
  if (firstOther === undefined || from === 0) {
    // Already at the top (or the only task): nothing to do.
    return { order: order.slice(), before: moved, changed: false };
  }
  const without = order.filter((id) => id !== moved);
  const next = [moved, ...without];
  return { order: next, before: firstOther, changed: !arraysEqual(order, next) };
}

/**
 * Given the bounding rect of a row and the pointer's clientY, decide whether
 * the drop should land before (top half) or after (bottom half) the row.
 */
export function dropPosForY(top: number, height: number, clientY: number): DropPos {
  return clientY < top + height / 2 ? "before" : "after";
}

/**
 * F40: compute a reorder that's CONSTRAINED to a single section (e.g. Pinned).
 * Dragging within a section should only shuffle that section's members; the
 * `before` id we persist must therefore be resolved against the GLOBAL file
 * order so `store.Move` drops the task in the right absolute slot, while the
 * *visible* result only moves the dragged row among its section peers.
 *
 * Inputs:
 *   - globalOrder: every task id in file order (what /move rewrites)
 *   - sectionIds:  the ids currently shown in this section, in display order
 *   - moved/target/pos: the drag gesture, where target is a section peer
 *
 * Strategy: compute the new SECTION order first (simple, local), then translate
 * "moved now sits in front of X within the section" into a global `before`:
 *   - if moved lands before some section peer P, persist before:P
 *   - if moved lands at the section's end, persist before: the id of whatever
 *     globally follows the section's last member (or 0 = file end)
 * The moved id keeps all NON-section tasks exactly where they were globally.
 */
export function computeSectionReorder(
  globalOrder: number[],
  sectionIds: number[],
  moved: number,
  target: number,
  pos: DropPos,
): ReorderResult {
  const from = sectionIds.indexOf(moved);
  const ti = sectionIds.indexOf(target);
  if (from < 0 || ti < 0 || moved === target) {
    return { order: globalOrder.slice(), before: moved, changed: false };
  }
  // New SECTION order after the move.
  const insertBeforeIdx = pos === "before" ? ti : ti + 1;
  const withoutMoved = sectionIds.filter((id) => id !== moved);
  // Adjust the insert index for the removal of `moved` when it preceded target.
  const adj = from < insertBeforeIdx ? insertBeforeIdx - 1 : insertBeforeIdx;
  const nextSection = [
    ...withoutMoved.slice(0, adj),
    moved,
    ...withoutMoved.slice(adj),
  ];
  // The section peer the moved id now sits in front of (or null = section end).
  const posInSection = nextSection.indexOf(moved);
  const nextPeer = posInSection + 1 < nextSection.length ? nextSection[posInSection + 1] : null;

  // Translate to a global `before`.
  let beforeId: number;
  if (nextPeer !== null) {
    beforeId = nextPeer;
  } else {
    // Moved to the END of the section: persist before whatever globally follows
    // the section's last (non-moved) member; 0 if that's the file end.
    const lastPeer = withoutMoved[withoutMoved.length - 1];
    const gi = globalOrder.indexOf(lastPeer);
    beforeId = gi >= 0 && gi + 1 < globalOrder.length ? globalOrder[gi + 1] : 0;
    // Guard: if the global-next is the moved id itself, look one further.
    if (beforeId === moved) {
      beforeId = gi + 2 < globalOrder.length ? globalOrder[gi + 2] : 0;
    }
  }
  if (beforeId === moved) {
    return { order: globalOrder.slice(), before: moved, changed: false };
  }
  // Build the resulting GLOBAL order: remove moved, insert before beforeId.
  const without = globalOrder.filter((id) => id !== moved);
  let insertAt: number;
  if (beforeId === 0) {
    insertAt = without.length;
  } else {
    insertAt = without.indexOf(beforeId);
    if (insertAt < 0) insertAt = without.length;
  }
  const next = [...without.slice(0, insertAt), moved, ...without.slice(insertAt)];
  const changed = !arraysEqual(globalOrder, next);
  return { order: next, before: beforeId, changed };
}
