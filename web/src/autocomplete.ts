/**
 * Composer autocomplete (F38) — a pure, testable layer that powers the
 * dropdown under the quick-add input: type `#dev` and get matching existing
 * tags; type `@` and get the common due presets.
 *
 * main.ts owns the dropdown DOM, the keyboard navigation, and reading the
 * caret position; this module owns the data: which token (if any) the caret is
 * sitting in, what suggestions that token yields, and how replacing it rewrites
 * the input string. Keeping it pure means the token-scan and splice math are
 * unit-tested with zero DOM.
 */

export type SuggestKind = "tag" | "due";

export interface ActiveToken {
  kind: SuggestKind;
  /** The text after the sigil typed so far (lower-cased for tags). */
  query: string;
  /** Index of the sigil char in the input (so we can splice a replacement). */
  start: number;
  /** Index just past the token (caret position). */
  end: number;
}

export interface Suggestion {
  /** What to insert after the sigil (e.g. "dev" or "tomorrow"). */
  value: string;
  /** Display label (may include a count for tags). */
  label: string;
  /** The sigil this suggestion belongs to. */
  kind: SuggestKind;
}

/** The due presets offered when the caret is in an `@` token (mirrors the picker). */
export const DUE_PRESETS: ReadonlyArray<string> = [
  "today",
  "tomorrow",
  "mon",
  "tue",
  "wed",
  "thu",
  "fri",
  "sat",
  "sun",
  "in 3d",
  "in 1w",
  "eow",
  "eom",
];

/**
 * Find the token the caret is currently inside, if it's an autocompletable one
 * (`#tag` or `@due`). Scans backward from the caret to the token start; returns
 * null when the caret isn't in such a token (e.g. it's in plain title text, or
 * right after whitespace). The sigil must either start the string or follow
 * whitespace — a mid-word `#`/`@` (like `c#`) is not a token.
 */
export function activeToken(text: string, caret: number): ActiveToken | null {
  // Walk back to the start of the current whitespace-delimited word.
  let i = caret - 1;
  while (i >= 0 && !/\s/.test(text[i])) i--;
  const wordStart = i + 1;
  if (wordStart >= caret) return null; // caret right after a space / at start
  const sigil = text[wordStart];
  if (sigil !== "#" && sigil !== "@") return null;
  const raw = text.slice(wordStart + 1, caret);
  // A token's payload shouldn't contain another sigil/space (defensive).
  if (/[\s#@]/.test(raw)) return null;
  return {
    kind: sigil === "#" ? "tag" : "due",
    query: sigil === "#" ? raw.toLowerCase() : raw.toLowerCase(),
    start: wordStart,
    end: caret,
  };
}

/**
 * Suggestions for a `#tag` token: existing tags whose name contains the query,
 * ranked prefix-first then by descending usage count, capped at `limit`. An
 * empty query returns the most-used tags. Already-typed exact matches are kept
 * (so you still see the count) but a tag identical to the full query sinks.
 */
export function tagSuggestions(
  query: string,
  tags: ReadonlyArray<{ tag: string; count: number }>,
  limit = 8,
): Suggestion[] {
  const q = query.toLowerCase();
  const scored = tags
    .filter((t) => t.tag.includes(q))
    .map((t) => {
      const idx = t.tag.indexOf(q);
      const prefix = idx === 0 ? 0 : 1;
      return { t, prefix };
    })
    .sort((a, b) => {
      if (a.prefix !== b.prefix) return a.prefix - b.prefix;
      if (a.t.count !== b.t.count) return b.t.count - a.t.count;
      return a.t.tag < b.t.tag ? -1 : 1;
    });
  return scored.slice(0, limit).map(({ t }) => ({
    value: t.tag,
    label: `#${t.tag}`,
    kind: "tag",
  }));
}

/** Suggestions for an `@due` token: presets that contain the typed query. */
export function dueSuggestions(query: string, limit = 8): Suggestion[] {
  const q = query.toLowerCase();
  return DUE_PRESETS.filter((p) => p.includes(q))
    .sort((a, b) => {
      const ap = a.startsWith(q) ? 0 : 1;
      const bp = b.startsWith(q) ? 0 : 1;
      if (ap !== bp) return ap - bp;
      return a.length - b.length;
    })
    .slice(0, limit)
    .map((p) => ({ value: p, label: `@${p}`, kind: "due" }));
}

/** Compute suggestions for whatever token the caret is in (or [] if none). */
export function suggestFor(
  token: ActiveToken | null,
  tags: ReadonlyArray<{ tag: string; count: number }>,
): Suggestion[] {
  if (!token) return [];
  return token.kind === "tag"
    ? tagSuggestions(token.query, tags)
    : dueSuggestions(token.query);
}

/**
 * Apply a suggestion by replacing the active token's text with the chosen value,
 * leaving a trailing space so you can keep typing the next token. Returns the
 * new input string and the caret position to place after the inserted value.
 */
export function applySuggestion(
  text: string,
  token: ActiveToken,
  value: string,
): { text: string; caret: number } {
  const sigil = token.kind === "tag" ? "#" : "@";
  const insert = `${sigil}${value} `;
  const next = text.slice(0, token.start) + insert + text.slice(token.end);
  return { text: next, caret: token.start + insert.length };
}

/** Escape strings before injecting into innerHTML. Local copy keeps this pure. */
function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}

/**
 * Render the autocomplete dropdown list. The active row carries is-active +
 * aria-selected; each row carries data-ac-value + data-ac-kind for dispatch.
 * Returns "" when there are no suggestions so the dropdown collapses.
 */
export function renderAutocomplete(suggestions: Suggestion[], activeIndex: number): string {
  if (suggestions.length === 0) return "";
  return suggestions
    .map((s, i) => {
      const active = i === activeIndex ? " is-active" : "";
      return `<li class="ac-item${active}" role="option" aria-selected="${i === activeIndex}" data-ac-value="${escapeHTML(s.value)}" data-ac-kind="${s.kind}">
        <span class="ac-label">${escapeHTML(s.label)}</span>
      </li>`;
    })
    .join("");
}
