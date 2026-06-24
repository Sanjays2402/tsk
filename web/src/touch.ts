/**
 * Touch / long-press support (F28) — a pure, testable state machine for
 * detecting a long-press without timers in the logic itself.
 *
 * Touch devices have no hover and no right-click, so the desktop affordances
 * (cmd/shift-click to bulk-select, hover-reveal action buttons) don't reach.
 * The standard mobile gesture for "select this" is a LONG PRESS: hold a row
 * still for ~500ms. main.ts owns the actual touch events and the setTimeout;
 * this module owns the decision logic so it can be unit-tested:
 *
 *   - a press that MOVES more than a small slop radius is a scroll, not a
 *     long-press, and must cancel (so the page still scrolls normally);
 *   - a press that ENDS before the threshold is a tap, not a long-press;
 *   - only a still press held past the threshold fires the long-press.
 */

/** A captured pointer position. */
export interface Point {
  x: number;
  y: number;
}

/** How far (px) a touch may drift before it counts as a scroll, not a press. */
export const MOVE_SLOP = 10;

/** How long (ms) a still press must be held to count as a long-press. */
export const LONG_PRESS_MS = 500;

/**
 * Has the touch moved far enough from its start to be a scroll/drag rather than
 * a long-press? Uses squared distance to avoid a sqrt.
 */
export function exceededSlop(start: Point, current: Point, slop = MOVE_SLOP): boolean {
  const dx = current.x - start.x;
  const dy = current.y - start.y;
  return dx * dx + dy * dy > slop * slop;
}

/**
 * Given the elapsed hold time and whether the finger stayed within slop, should
 * a long-press fire? True only when the press was still AND held past the
 * threshold.
 */
export function shouldLongPress(
  elapsedMs: number,
  movedBeyondSlop: boolean,
  thresholdMs = LONG_PRESS_MS,
): boolean {
  if (movedBeyondSlop) return false;
  return elapsedMs >= thresholdMs;
}

/** The live state of an in-progress press, owned by main.ts. */
export interface PressState {
  id: number; // the task id under the finger
  start: Point;
  moved: boolean;
  timer: number; // setTimeout handle
}

/**
 * Decide what a touch move does to the press: returns the next `moved` flag.
 * Once a press has moved beyond slop it stays moved (a brief pause back inside
 * the radius shouldn't resurrect a cancelled long-press).
 */
export function trackMove(state: PressState, current: Point): boolean {
  if (state.moved) return true;
  return exceededSlop(state.start, current);
}
