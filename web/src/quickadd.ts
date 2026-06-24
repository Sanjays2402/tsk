/**
 * Quick-add inline syntax parser — pure, framework-free, unit-tested.
 *
 * Lets you type a task and its metadata in one line, the way you would in the
 * CLI's muscle memory:
 *
 *     buy milk !high @tomorrow #shopping #home
 *
 * Tokens (whitespace-delimited) are pulled out of the title:
 *   - `!<prio>`  sets priority. Accepts the same forms as the Go
 *                model.ParsePriority: l/low, m/med/medium, h/high,
 *                u/urgent/critical. Unknown `!words` are left in the title.
 *   - `@<due>`   sets a single-token due date passed verbatim to the server,
 *                which validates it via the dateparse package (today, tomorrow,
 *                fri, 3d, 2w, eow, eom, jul, 2026-07-04, ...). Multi-word due
 *                strings ("next week") aren't expressible inline — use the row
 *                editor or CLI for those.
 *   - `#<tag>`   adds a tag (lower-cased + de-duplicated to match the store).
 *   - `depends:#N` / `dep:N` / `needs:N` adds a dependency on task N (F38).
 *                Accepts a comma list (`depends:1,2`) and an optional `#`.
 *
 * Everything not matched as a token is the title. A bare `!`, `@`, or `#`
 * (nothing after it) is treated as literal title text, and an `@`/`#`/`!` in
 * the middle of a word (e.g. `bob@acme.com`, `C#`, `done!`) is never a token
 * because tokens must START with the sigil.
 */

export type Priority = "low" | "medium" | "high" | "urgent";

export interface ParsedQuickAdd {
  /** The title with all recognized tokens stripped and whitespace collapsed. */
  title: string;
  /** Priority, when a recognized `!prio` token was present. */
  priority?: Priority;
  /** Raw due string (server validates), when an `@due` token was present. */
  due?: string;
  /** Lower-cased, de-duplicated tags from `#tag` tokens, in first-seen order. */
  tags: string[];
  /**
   * Task ids this new task should depend on, from `depends:#N` / `dep:N`
   * tokens (F38). De-duplicated, first-seen order. The server validates them
   * (rejects unknown ids) at create time.
   */
  dependsOn: number[];
}

/** Maps every accepted priority word (short + long) to its canonical form. */
const PRIORITY_WORDS: Readonly<Record<string, Priority>> = {
  l: "low",
  low: "low",
  m: "medium",
  med: "medium",
  medium: "medium",
  h: "high",
  high: "high",
  u: "urgent",
  urgent: "urgent",
  critical: "urgent",
};

/**
 * Parse a single-line quick-add string into structured fields. Never throws;
 * malformed tokens degrade gracefully into literal title text so the user
 * always gets *something* added rather than an error.
 */
export function parseQuickAdd(input: string): ParsedQuickAdd {
  const tokens = input.trim().split(/\s+/).filter(Boolean);
  const titleParts: string[] = [];
  const tags: string[] = [];
  const dependsOn: number[] = [];
  let priority: Priority | undefined;
  let due: string | undefined;

  for (const tok of tokens) {
    const sigil = tok[0];
    const rest = tok.slice(1);

    // F38: a `depends:#N` / `dep:N` token adds a blocker. Accepts an optional
    // `#` before the number; multiple ids via repeats or a comma list
    // (`depends:1,2`). Non-numeric payloads fall through to the title.
    const depMatch = /^(?:depends|dep|needs):(.+)$/i.exec(tok);
    if (depMatch) {
      const ids = depMatch[1]
        .split(",")
        .map((s) => parseInt(s.replace(/^#/, "").trim(), 10))
        .filter((n) => Number.isFinite(n) && n > 0);
      if (ids.length > 0) {
        for (const n of ids) if (!dependsOn.includes(n)) dependsOn.push(n);
        continue;
      }
      // No valid ids parsed — keep the token in the title.
    }

    if (sigil === "#" && rest.length > 0) {
      const tag = rest.toLowerCase();
      if (!tags.includes(tag)) tags.push(tag);
      continue;
    }
    if (sigil === "!" && rest.length > 0) {
      const p = PRIORITY_WORDS[rest.toLowerCase()];
      if (p) {
        priority = p; // last one wins
        continue;
      }
      // Unknown !word — fall through and keep it in the title.
    }
    if (sigil === "@" && rest.length > 0) {
      due = rest; // last one wins; server validates
      continue;
    }
    titleParts.push(tok);
  }

  return { title: titleParts.join(" "), priority, due, tags, dependsOn };
}

/** True when a parse yields a non-empty title (i.e. it's submittable). */
export function isSubmittable(parsed: ParsedQuickAdd): boolean {
  return parsed.title.trim().length > 0;
}

/**
 * Split a multi-line paste into individual quick-add lines (F38). Trims each
 * line, drops blanks, and strips common list markers a user might paste from a
 * markdown or plain-text checklist:
 *   - `- buy milk`        (dash bullet)
 *   - `* buy milk`        (star bullet)
 *   - `1. buy milk`       (ordered list)
 *   - `- [ ] buy milk`    (markdown task — open)
 *   - `- [x] buy milk`    (markdown task — done marker stripped, not honored)
 * Each cleaned line is still subject to the inline token grammar when added.
 * A single line (no newline) yields a one-element array, so callers can always
 * treat the result uniformly.
 */
export function splitPasteLines(input: string): string[] {
  return input
    .split(/\r?\n/)
    .map((line) => stripListMarker(line.trim()))
    .filter((line) => line.length > 0);
}

/** Strip a leading bullet / ordinal / task-checkbox marker from one line. */
export function stripListMarker(line: string): string {
  // Order matters: peel an optional bullet/ordinal, then an optional checkbox.
  let out = line.replace(/^(?:[-*+]|\d+[.)])\s+/, "");
  out = out.replace(/^\[[ xX]\]\s+/, "");
  return out.trim();
}

/** True when the input contains more than one non-blank line (a multi-paste). */
export function isMultiLine(input: string): boolean {
  return splitPasteLines(input).length > 1;
}
