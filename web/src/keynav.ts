/**
 * Keyboard navigation state — a pure, testable selection model over the
 * currently-visible task ids (in their flattened top-to-bottom section order).
 *
 * main.ts owns the DOM; this module owns "which id is selected and how does it
 * move". Keeping it pure means the j/k/g/G/home/end logic is unit-tested with
 * zero DOM, and the wiring in main.ts stays a thin translation layer.
 */

export interface NavState {
  /** Currently selected task id, or null when nothing is selected. */
  selectedId: number | null;
}

export type NavMove = "next" | "prev" | "first" | "last";

/** A fresh, empty navigation state. */
export function emptyNav(): NavState {
  return { selectedId: null };
}

/**
 * Reconcile the selection against a new list of visible ids (e.g. after a
 * refresh, add, delete, or toggle re-sections the list). Rules:
 *   - empty list      -> nothing selected
 *   - selection valid -> keep it
 *   - selection gone  -> snap to the same index position if possible (so
 *                        deleting row 3 lands you on the new row 3), else the
 *                        last row. prevIds lets us recover the old index.
 */
export function reconcile(
  state: NavState,
  ids: number[],
  prevIds: number[] = [],
): NavState {
  if (ids.length === 0) return { selectedId: null };
  if (state.selectedId !== null && ids.includes(state.selectedId)) {
    return state;
  }
  // Try to hold position by the old index.
  const oldIdx = state.selectedId === null ? -1 : prevIds.indexOf(state.selectedId);
  if (oldIdx >= 0) {
    const clamped = Math.min(oldIdx, ids.length - 1);
    return { selectedId: ids[clamped] };
  }
  return { selectedId: ids[0] };
}

/** Compute the next selection for a movement command. Never goes out of range. */
export function move(state: NavState, ids: number[], dir: NavMove): NavState {
  if (ids.length === 0) return { selectedId: null };
  if (dir === "first") return { selectedId: ids[0] };
  if (dir === "last") return { selectedId: ids[ids.length - 1] };

  const cur = state.selectedId === null ? -1 : ids.indexOf(state.selectedId);
  if (cur < 0) {
    // Nothing selected yet: next -> first, prev -> last.
    return { selectedId: dir === "next" ? ids[0] : ids[ids.length - 1] };
  }
  const delta = dir === "next" ? 1 : -1;
  const clamped = Math.max(0, Math.min(ids.length - 1, cur + delta));
  return { selectedId: ids[clamped] };
}

/** Explicitly select an id if it's visible; otherwise leave state unchanged. */
export function select(state: NavState, ids: number[], id: number): NavState {
  return ids.includes(id) ? { selectedId: id } : state;
}
