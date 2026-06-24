/**
 * Search highlighting (F30) — a pure, testable layer that marks WHY a row
 * matched the active filter, by wrapping the matched characters of the title
 * in <mark> tags.
 *
 * The filter (filter.ts) matches with a fuzzy SUBSEQUENCE test: a query is
 * split on whitespace, and every token must appear, in order, as a subsequence
 * of the lower-cased haystack (title + tags). This module mirrors that exact
 * matching so the highlight lines up with the rule that actually let the row
 * through — no "matched but nothing highlighted" surprises.
 *
 * Safety: the input title is UNTRUSTED, so we escape every character as we
 * emit it. The only markup we introduce is the <mark> wrapper around matched
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
 * Compute the set of title indices that the query's tokens match, walking each
 * token as a subsequence over the lower-cased title. Greedy left-to-right, the
 * same direction filter.ts's isSubsequence walks, so the marks land on the
 * earliest matching characters. A token that does NOT fully match the title
 * (it may have matched on a tag instead) contributes no marks — we only
 * highlight what genuinely lands in the title.
 */
export function matchedIndices(query: string, title: string): Set<number> {
  const marks = new Set<number>();
  const tokens = query.toLowerCase().split(/\s+/).filter(Boolean);
  if (tokens.length === 0) return marks;
  const lower = title.toLowerCase();
  for (const tok of tokens) {
    const hits: number[] = [];
    let ti = 0;
    for (let li = 0; li < lower.length && ti < tok.length; li++) {
      if (lower[li] === tok[ti]) {
        hits.push(li);
        ti++;
      }
    }
    // Only commit this token's hits if the whole token matched the TITLE.
    if (ti === tok.length) {
      for (const h of hits) marks.add(h);
    }
  }
  return marks;
}

/**
 * Render a title with the query's matched characters wrapped in <mark>. The
 * title is fully HTML-escaped; only <mark> spans are introduced. Consecutive
 * matched characters are coalesced into a single <mark> to keep the markup
 * tight. An empty/blank query returns the plain escaped title.
 */
export function highlightTitle(title: string, query: string): string {
  const marks = matchedIndices(query, title);
  if (marks.size === 0) {
    // Fast path: just escape.
    let out = "";
    for (const c of title) out += escapeChar(c);
    return out;
  }
  let out = "";
  let inMark = false;
  // Iterate by code unit so indices line up with matchedIndices (which walks
  // the string the same way filter.ts does).
  for (let i = 0; i < title.length; i++) {
    const hit = marks.has(i);
    if (hit && !inMark) {
      out += "<mark>";
      inMark = true;
    } else if (!hit && inMark) {
      out += "</mark>";
      inMark = false;
    }
    out += escapeChar(title[i]);
  }
  if (inMark) out += "</mark>";
  return out;
}
