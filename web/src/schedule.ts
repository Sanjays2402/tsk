/**
 * Schedule stats (F59) — a pure, testable layer that answers two at-a-glance
 * questions the existing metrics don't: "how much is due in the coming week?"
 * and "how much has no due date at all?". Both are computed from the live task
 * list the client already holds (the server stats DTO models Open / Due today /
 * Overdue, but not these two lenses), so main.ts computes them alongside the
 * F46 dependency stats and threads them into the sidebar.
 *
 * Definitions (documented so the UI copy can match):
 *   - dueThisWeek: UNDONE tasks whose due date is in [today, today+6] — the
 *     coming week INCLUDING today. Overdue tasks are deliberately excluded
 *     (they have their own alert tile); this is "what's on deck", not "what's
 *     late". A done task never counts (you already finished it).
 *   - noDue: UNDONE tasks with no due date — the backlog you haven't scheduled.
 *
 * Kept dependency-free so the day-window math is unit-tested with zero browser.
 */

/** The minimal shape a task needs for schedule reasoning. */
export interface ScheduleTask {
  done: boolean;
  due?: string; // YYYY-MM-DD, or missing/"" for no due date
}

/** The two derived counts the sidebar renders. */
export interface ScheduleStats {
  /** Undone tasks due within the next 7 days (today..today+6). */
  dueThisWeek: number;
  /** Undone tasks with no due date set. */
  noDue: number;
}

/** Parse a YYYY-MM-DD string to a local-midnight day-number, or null if absent/bad. */
function dayNum(due: string | undefined): number | null {
  if (!due) return null;
  const [y, m, d] = due.split("-").map((n) => parseInt(n, 10));
  if (!y || !m || !d) return null;
  return Math.floor(new Date(y, m - 1, d).getTime() / 86_400_000);
}

/** The local-midnight day-number for a reference `now`. */
function todayNum(now: Date): number {
  return Math.floor(
    new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime() / 86_400_000,
  );
}

/**
 * Compute the "due this week" + "no due" counts across the task list, relative
 * to `now`. Pure → unit-tested. The week window is the 7 days starting today
 * (today..today+6 inclusive); a malformed due string counts as "no due" since
 * it can't be scheduled (matching how the section grouping treats it).
 */
export function computeScheduleStats(tasks: ScheduleTask[], now: Date): ScheduleStats {
  const today = todayNum(now);
  const weekEnd = today + 6;
  let dueThisWeek = 0;
  let noDue = 0;
  for (const t of tasks) {
    if (t.done) continue; // schedule lenses are about outstanding work
    const day = dayNum(t.due);
    if (day === null) {
      noDue++;
      continue;
    }
    if (day >= today && day <= weekEnd) dueThisWeek++;
  }
  return { dueThisWeek, noDue };
}
