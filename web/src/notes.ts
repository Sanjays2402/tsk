/**
 * Notes editor model (F23) — pure helpers for expanding a task's multi-line
 * notes into an editable panel and deciding what to persist. The `.tsk.md`
 * store keeps notes as 6-space continuation lines under a task; the server's
 * PATCH `notes` field round-trips them, so the web editor is just a textarea
 * over that string. Keeping the decision logic here means it's unit-tested
 * without a DOM; main.ts owns the panel mount + PATCH call.
 */

export type NotesOutcome =
  | { kind: "commit"; notes: string } // changed -> PATCH notes
  | { kind: "noop" }; // unchanged (after normalize) -> just close

/**
 * Normalize a notes draft for storage: trim trailing whitespace on each line
 * (the store does this on save anyway, so we match it to avoid a spurious
 * "changed" diff) and strip leading/trailing blank lines. Interior blank lines
 * are preserved — they're meaningful paragraph breaks.
 */
export function normalizeNotes(draft: string): string {
  const lines = draft.replace(/\r\n?/g, "\n").split("\n").map((l) => l.replace(/[ \t]+$/, ""));
  // Drop leading blank lines.
  while (lines.length > 0 && lines[0] === "") lines.shift();
  // Drop trailing blank lines.
  while (lines.length > 0 && lines[lines.length - 1] === "") lines.pop();
  return lines.join("\n");
}

/**
 * Decide the outcome of finishing a notes edit. Escape-cancel is handled by the
 * caller (it never calls this); here we only ever commit-or-noop. The draft is
 * normalized and compared against the original (also normalized) so trailing
 * whitespace and edge blank lines never trigger a pointless write.
 */
export function resolveNotes(original: string, draft: string): NotesOutcome {
  const next = normalizeNotes(draft);
  if (next === normalizeNotes(original)) return { kind: "noop" };
  return { kind: "commit", notes: next };
}

/** A short one-line preview of notes for the collapsed row affordance. */
export function notesPreview(notes: string, max = 48): string {
  const firstLine = normalizeNotes(notes).split("\n").find((l) => l.trim() !== "") ?? "";
  if (firstLine.length <= max) return firstLine;
  return firstLine.slice(0, max - 1) + "\u2026";
}

/** True when a task has any non-blank notes content. */
export function hasNotes(notes: string | undefined): boolean {
  return !!notes && normalizeNotes(notes) !== "";
}

/** Count the non-blank lines in a notes blob (for the row badge tooltip). */
export function notesLineCount(notes: string | undefined): number {
  if (!notes) return 0;
  return normalizeNotes(notes).split("\n").filter((l) => l.trim() !== "").length;
}

/** Escape strings before injecting into innerHTML. Local copy keeps this dependency-free. */
function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}

/**
 * Render the notes button shown in a row's meta cluster. Carries `data-notes`
 * so a delegated listener opens the editor, and reflects whether the task has
 * notes (filled glyph + count) or not (a subtle "+note" affordance).
 */
export function renderNotesButton(notes: string | undefined): string {
  if (hasNotes(notes)) {
    const n = notesLineCount(notes);
    const title = escapeHTML(notesPreview(notes ?? ""));
    return `<button class="notes-btn has-notes" data-notes type="button" aria-label="Edit notes" title="${title}">&#9776;<span class="notes-n">${n}</span></button>`;
  }
  return `<button class="notes-btn" data-notes type="button" aria-label="Add notes" title="Add notes (i)">+note</button>`;
}
