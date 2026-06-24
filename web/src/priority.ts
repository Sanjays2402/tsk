/**
 * Priority cycling (F29) — a pure, testable layer for the "click the priority
 * chip to bump it" interaction, without touching the DOM or the network.
 *
 * The chip cycles low -> medium -> high -> urgent -> low, mirroring the CLI's
 * `tsk pri --up` wrap behaviour. A shift/alt-click (handled in main.ts) cycles
 * DOWN instead. Keeping the ladder logic here means the wrap-around and the
 * glyph mapping are unit-tested with zero browser.
 */

export type Priority = "low" | "medium" | "high" | "urgent";

/** The ladder in ascending urgency. Index 0 is the least urgent. */
export const PRIORITY_LADDER: ReadonlyArray<Priority> = ["low", "medium", "high", "urgent"];

/** Where a priority sits on the ladder (0..3). Unknown values clamp to medium. */
export function priorityRank(p: string): number {
  const i = PRIORITY_LADDER.indexOf(p as Priority);
  return i < 0 ? PRIORITY_LADDER.indexOf("medium") : i;
}

/**
 * The next priority going UP (more urgent), wrapping urgent -> low so a
 * repeated click never dead-ends. Mirrors `tsk pri --up`'s wrap.
 */
export function nextPriority(p: string): Priority {
  const i = priorityRank(p);
  return PRIORITY_LADDER[(i + 1) % PRIORITY_LADDER.length];
}

/**
 * The next priority going DOWN (less urgent), wrapping low -> urgent. Mirrors
 * `tsk pri --down`'s wrap. Used by shift/alt-click on the chip.
 */
export function prevPriority(p: string): Priority {
  const i = priorityRank(p);
  return PRIORITY_LADDER[(i - 1 + PRIORITY_LADDER.length) % PRIORITY_LADDER.length];
}

/** Single-letter glyph for a priority, matching the list chip + TUI. */
export function priorityGlyph(p: string): string {
  return { low: "L", medium: "M", high: "H", urgent: "U" }[p as Priority] ?? "M";
}
