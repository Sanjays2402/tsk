/**
 * Inline-edit helpers (F7) — pure decision logic for committing or cancelling
 * an in-place title edit. The DOM wiring lives in main.ts; this module decides
 * *what* should happen so the rules are unit-tested without a browser.
 */

export type EditOutcome =
  | { kind: "commit"; title: string }
  | { kind: "noop" } // unchanged or empty-after-trim -> just close, no request
  | { kind: "cancel" };

/**
 * Decide the outcome of finishing an inline edit.
 *
 *   - Escape always cancels (revert, no request).
 *   - Otherwise we're committing: trim the draft.
 *       - empty after trim  -> noop (don't allow clearing a title; the server
 *         rejects empty titles anyway, so we avoid a guaranteed 400)
 *       - same as original  -> noop (nothing to save)
 *       - changed           -> commit with the trimmed title
 */
export function resolveEdit(
  original: string,
  draft: string,
  cancelled: boolean,
): EditOutcome {
  if (cancelled) return { kind: "cancel" };
  const trimmed = draft.trim();
  if (trimmed === "") return { kind: "noop" };
  if (trimmed === original.trim()) return { kind: "noop" };
  return { kind: "commit", title: trimmed };
}
