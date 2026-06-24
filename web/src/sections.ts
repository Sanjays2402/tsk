/**
 * Section grouping — splits a flat task list into the same buckets the TUI
 * and `tsk daily` use, so the web list reads the same way.
 *
 * Pure + dependency-free (unit-tested under `node --test`). The renderer in
 * render.ts consumes the ordered buckets; main.ts never sees the raw sort.
 *
 * Buckets, in display order:
 *   - Pinned     undone + pinned (floats to the very top, F27)
 *   - Overdue    undone, due before today
 *   - Today      undone, due today
 *   - Upcoming   undone, due after today
 *   - No Due     undone, no due date
 *   - Done       completed (regardless of due)
 *
 * A pinned undone task is pulled OUT of its due-based bucket into Pinned so it
 * stays visible regardless of priority/due — matching the CLI's `tsk pin`,
 * which floats a task to the top of `top`/`next`. Pinned + done tasks stay in
 * Done (a finished pin isn't something you need surfaced).
 *
 * Within each undone DUE bucket, tasks sort by priority (urgent -> low) then
 * file order. The Pinned bucket is the exception (F40): it preserves FILE
 * ORDER exactly (hand-curated), so you can drag-to-reorder your pinned "work
 * on these" list into any sequence you like and it sticks — priority does not
 * override it. Done sorts most-recently-completed first when timestamps exist,
 * else by id descending so the freshest completions sit on top.
 */

export type SectionKey = "pinned" | "overdue" | "today" | "upcoming" | "nodue" | "done";

export interface TaskLike {
  id: number;
  done: boolean;
  pinned?: boolean; // F27: sticky-flag
  priority: string;
  due?: string; // YYYY-MM-DD or undefined/""
  completed?: string; // RFC3339 or undefined
}

export interface Section<T extends TaskLike> {
  key: SectionKey;
  label: string;
  tasks: T[];
}

const SECTION_ORDER: ReadonlyArray<{ key: SectionKey; label: string }> = [
  { key: "pinned", label: "Pinned" },
  { key: "overdue", label: "Overdue" },
  { key: "today", label: "Today" },
  { key: "upcoming", label: "Upcoming" },
  { key: "nodue", label: "No Due" },
  { key: "done", label: "Done" },
];

const PRIORITY_RANK: Readonly<Record<string, number>> = {
  urgent: 0,
  high: 1,
  medium: 2,
  low: 3,
};

/** Parse a YYYY-MM-DD string to a comparable day-number, or null if absent/bad. */
function dayNum(due: string | undefined): number | null {
  if (!due) return null;
  const [y, m, d] = due.split("-").map((n) => parseInt(n, 10));
  if (!y || !m || !d) return null;
  // Days since epoch in local time; only used for ordering/comparison.
  return Math.floor(new Date(y, m - 1, d).getTime() / 86_400_000);
}

/** Classify a single task into its section, relative to `now`. */
export function sectionFor(t: TaskLike, now: Date): SectionKey {
  if (t.done) return "done";
  // F27: a pinned, still-undone task floats to the Pinned section regardless
  // of its due date, so it stays in view.
  if (t.pinned) return "pinned";
  const due = dayNum(t.due);
  if (due === null) return "nodue";
  const today = Math.floor(
    new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime() / 86_400_000,
  );
  if (due < today) return "overdue";
  if (due === today) return "today";
  return "upcoming";
}

function comparePriority(a: TaskLike, b: TaskLike): number {
  const pa = PRIORITY_RANK[a.priority] ?? 9;
  const pb = PRIORITY_RANK[b.priority] ?? 9;
  if (pa !== pb) return pa - pb;
  // Equal priority: preserve the INPUT (file) order. groupIntoSections is fed
  // tasks in .tsk.md order, and Array.prototype.sort is stable, so returning 0
  // here keeps same-priority peers in file order — which is what makes
  // drag-to-reorder (F17) visibly "stick" within a section.
  return 0;
}

/**
 * Order the Pinned section (F40). Pinned items are the user's hand-curated
 * "work on these" list, so we preserve FILE ORDER exactly — `groupIntoSections`
 * is fed tasks in `.tsk.md` order and Array.prototype.sort is stable, so
 * returning 0 keeps every pin in the order the file (and thus drag-to-reorder
 * via the /move endpoint) dictates. Priority deliberately does NOT reorder
 * pins, so a manual sequence sticks until you drag it again.
 */
function comparePinned(_a: TaskLike, _b: TaskLike): number {
  return 0;
}

function compareDone(a: TaskLike, b: TaskLike): number {
  // Most recently completed first; tasks with timestamps outrank those without.
  if (a.completed && b.completed) {
    if (a.completed !== b.completed) return a.completed < b.completed ? 1 : -1;
    return b.id - a.id;
  }
  if (a.completed) return -1;
  if (b.completed) return 1;
  return b.id - a.id;
}

/**
 * Group tasks into ordered, non-empty sections. Empty sections are omitted so
 * the UI never renders a bare header with nothing under it.
 */
export function groupIntoSections<T extends TaskLike>(tasks: T[], now: Date): Section<T>[] {
  const buckets: Record<SectionKey, T[]> = {
    pinned: [],
    overdue: [],
    today: [],
    upcoming: [],
    nodue: [],
    done: [],
  };
  for (const t of tasks) {
    buckets[sectionFor(t, now)].push(t);
  }
  buckets.pinned.sort(comparePinned);
  buckets.overdue.sort(comparePriority);
  buckets.today.sort(comparePriority);
  buckets.upcoming.sort(comparePriority);
  buckets.nodue.sort(comparePriority);
  buckets.done.sort(compareDone);

  const out: Section<T>[] = [];
  for (const { key, label } of SECTION_ORDER) {
    if (buckets[key].length > 0) {
      out.push({ key, label, tasks: buckets[key] });
    }
  }
  return out;
}

/**
 * Flatten grouped sections back into the visible top-to-bottom task order.
 * This is the order keyboard navigation (F10) walks.
 */
export function flattenSections<T extends TaskLike>(sections: Section<T>[]): T[] {
  return sections.flatMap((s) => s.tasks);
}
