/**
 * Search highlighting (F30, extended F43) — a pure, testable layer that marks
 * WHY a row matched the active filter, by wrapping the matched characters of a
 * piece of text (title, a tag pill, or a notes snippet) in <mark> tags.
 *
 * The filter (filter.ts) matches with a fuzzy SUBSEQUENCE test: a query is
 * split on whitespace, and every token must appear, in order, as a subsequence
 * of the lower-cased haystack (title + tags). This module mirrors that exact
 * matching so the highlight lines up with the rule that actually let the row
 * through — no "matched but nothing highlighted" surprises. A token only marks
 * a given field (title/tag/notes) when it fully subsequence-matches THAT field,
 * so a query that landed on a tag doesn't smear partial marks across the title.
 *
 * Safety: the input text is UNTRUSTED, so we escape every character as we emit
 * it. The only markup we introduce is the <mark> wrapper around matched
 * characters. The output is therefore safe to drop into innerHTML.
 */

/** Escape a single character for safe innerHTML insertion. */
function escapeChar(c: string): string {
  switch (c) {
    case "&":
      return "&amp;";
    case "<":
      return "&lt;";
    case ">":
      return "&gt;";
    case '"':
      return "&quot;";
    case "'":
      return "&#39;";
    default:
      return c;
  }
}

/**
 * Compute the set of text indices that the query's tokens match, walking each
 * token as a subsequence over the lower-cased text. Greedy left-to-right, the
 * same direction filter.ts's isSubsequence walks, so the marks land on the
 * earliest matching characters. A token that does NOT fully match the text
 * (it may have matched on another field instead) contributes no marks — we
 * only highlight what genuinely lands in THIS text.
 */
export function matchedIndices(query: string, text: string): Set<number> {
  const marks = new Set<number>();
  const tokens = query.toLowerCase().split(/\s+/).filter(Boolean);
  if (tokens.length === 0) return marks;
  const lower = text.toLowerCase();
  for (const tok of tokens) {
    const hits: number[] = [];
    let ti = 0;
    for (let li = 0; li < lower.length && ti < tok.length; li++) {
      if (lower[li] === tok[ti]) {
        hits.push(li);
        ti++;
      }
    }
    // Only commit this token's hits if the whole token matched the TEXT.
    if (ti === tok.length) {
      for (const h of hits) marks.add(h);
    }
  }
  return marks;
}

/**
 * Render arbitrary text with the query's matched characters wrapped in <mark>.
 * The text is fully HTML-escaped; only <mark> spans are introduced. Consecutive
 * matched characters are coalesced into a single <mark> to keep the markup
 * tight. An empty/blank query returns the plain escaped text. This is the
 * generic engine behind highlightTitle (titles), the tag-pill highlight, and
 * the notes-snippet highlight (F43).
 */
export function highlightText(text: string, query: string): string {
  const marks = matchedIndices(query, text);
  if (marks.size === 0) {
    // Fast path: just escape.
    let out = "";
    for (const c of text) out += escapeChar(c);
    return out;
  }
  let out = "";
  let inMark = false;
  // Iterate by code unit so indices line up with matchedIndices (which walks
  // the string the same way filter.ts does).
  for (let i = 0; i < text.length; i++) {
    const hit = marks.has(i);
    if (hit && !inMark) {
      out += "<mark>";
      inMark = true;
    } else if (!hit && inMark) {
      out += "</mark>";
      inMark = false;
    }
    out += escapeChar(text[i]);
  }
  if (inMark) out += "</mark>";
  return out;
}

/**
 * Highlight a task title (F30). A thin alias over highlightText kept for the
 * existing call sites and tests that read in title terms.
 */
export function highlightTitle(title: string, query: string): string {
  return highlightText(title, query);
}
