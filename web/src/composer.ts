/**
 * Composer preview renderer — turns a parsed quick-add into a row of pills
 * showing how the inline tokens will be interpreted before you hit enter.
 *
 * Pure + dependency-free (so it unit-tests cleanly under `node --test`) and
 * main.ts can drop the HTML straight into the preview container.
 */

import type { ParsedQuickAdd } from "./quickadd";

/** Escape strings before injecting into innerHTML. Local copy keeps this module dependency-free. */
function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}

/** One-letter priority glyph, matching the list's priority chips. */
function prioShort(p: string): string {
  return { urgent: "U", high: "H", low: "L", medium: "M" }[p] ?? "M";
}

/**
 * Render the live preview pills for a parsed quick-add line. Returns "" when
 * there's nothing worth previewing (no title and no metadata), which lets the
 * `:empty` CSS collapse the row entirely.
 */
export function renderComposerPreview(parsed: ParsedQuickAdd): string {
  const pills: string[] = [];

  if (parsed.priority) {
    pills.push(
      `<span class="pill prio ${parsed.priority}" title="${parsed.priority} priority">${prioShort(parsed.priority)}</span>`,
    );
  }
  if (parsed.due) {
    pills.push(`<span class="pill due">@${escapeHTML(parsed.due)}</span>`);
  }
  for (const tag of parsed.tags) {
    pills.push(`<span class="pill tag">#${escapeHTML(tag)}</span>`);
  }

  if (pills.length === 0) return "";

  const titlePart = parsed.title.trim()
    ? `<span class="ghost">${escapeHTML(parsed.title.trim())}</span>`
    : `<span class="ghost">(needs a title)</span>`;

  return titlePart + pills.join("");
}
