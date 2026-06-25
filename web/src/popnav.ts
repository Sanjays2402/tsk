/**
 * Popover keyboard navigation (F70) — a pure, testable model for arrow-key +
 * Enter selection inside the small jump-list popovers (the F56 chain drill and
 * the F62 newly-unblocked picker), so they're reachable without the mouse and
 * match the command palette's keyboard model.
 *
 * main.ts owns the popover DOM, the active-row paint, and the actual jump; this
 * module owns the data: which navigation ACTION a key maps to, and how the
 * highlighted index moves under that action. Keeping it pure means the wrap /
 * clamp math + the key mapping are unit-tested with zero DOM.
 *
 * The model mirrors the palette: Up/Down (and j/k) move with wrap, Home/End
 * (and g/G) jump to the ends, Enter activates the highlighted row, Escape
 * closes. Anything else is a no-op so the handler can fall through.
 */

export type PopNavAction = "next" | "prev" | "first" | "last" | "activate" | "close" | "none";

/**
 * Map a KeyboardEvent.key to a navigation action. Vim-style j/k and g/G are
 * accepted alongside the arrows / Home / End so the popovers feel native to
 * the keyboard-first list (which already uses j/k/g/G). Case-sensitive on the
 * single letters: lowercase g = first, uppercase G = last (matching the list's
 * own g/G binding). Unmapped keys return "none".
 */
export function keyToPopNavAction(key: string): PopNavAction {
  switch (key) {
    case "ArrowDown":
    case "j":
      return "next";
    case "ArrowUp":
    case "k":
      return "prev";
    case "Home":
    case "g":
      return "first";
    case "End":
    case "G":
      return "last";
    case "Enter":
      return "activate";
    case "Escape":
      return "close";
    default:
      return "none";
  }
}

/**
 * Compute the new highlighted index for a nav action over a list of `len`
 * items, given the `current` index. next/prev wrap past the ends (so holding a
 * direction cycles, like the palette); first/last jump to 0 / len-1. activate /
 * close / none leave the index unchanged (the caller acts on those separately).
 * An empty list pins the index at 0. The result is always a valid index in
 * [0, len) for a non-empty list.
 */
export function nextPopNavIndex(current: number, len: number, action: PopNavAction): number {
  if (len <= 0) return 0;
  const clamped = Math.max(0, Math.min(current, len - 1));
  switch (action) {
    case "next":
      return (clamped + 1) % len;
    case "prev":
      return (clamped - 1 + len) % len;
    case "first":
      return 0;
    case "last":
      return len - 1;
    default:
      return clamped;
  }
}
