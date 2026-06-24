/**
 * Theme model (F14) — pure logic for the auto/light/dark cycle. The DOM side
 * (reading localStorage, setting the data-theme attribute, listening for
 * system changes) lives in main.ts; this module owns the state machine so the
 * cycle order, labels, glyphs, and the resolved-mode computation are
 * unit-tested without a browser.
 *
 * Three user-facing modes:
 *   - "auto"  follow the OS (prefers-color-scheme)
 *   - "light" force the cream/ochre light palette
 *   - "dark"  force the charcoal/amber dark palette
 *
 * The resolved mode (what actually paints) is "light" | "dark"; "auto" maps to
 * the system preference. tokens.css keys its overrides off a data-theme="light"
 * | "dark" attribute on <html>; for "auto" we set NO attribute and let the
 * @media (prefers-color-scheme) rules win.
 */

export type ThemeMode = "auto" | "light" | "dark";
export type ResolvedTheme = "light" | "dark";

/** The cycle order when the toggle is clicked. */
export const THEME_CYCLE: ReadonlyArray<ThemeMode> = ["auto", "light", "dark"];

const VALID = new Set<ThemeMode>(["auto", "light", "dark"]);

/** Coerce an untrusted stored string into a ThemeMode, defaulting to "auto". */
export function normalizeMode(raw: string | null | undefined): ThemeMode {
  return raw && VALID.has(raw as ThemeMode) ? (raw as ThemeMode) : "auto";
}

/** The next mode in the cycle (auto -> light -> dark -> auto). */
export function nextMode(mode: ThemeMode): ThemeMode {
  const i = THEME_CYCLE.indexOf(mode);
  return THEME_CYCLE[(i + 1) % THEME_CYCLE.length];
}

/**
 * Resolve a mode to the palette that actually paints, given whether the system
 * prefers dark. "auto" defers to systemPrefersDark; the explicit modes win.
 */
export function resolveTheme(mode: ThemeMode, systemPrefersDark: boolean): ResolvedTheme {
  if (mode === "light") return "light";
  if (mode === "dark") return "dark";
  return systemPrefersDark ? "dark" : "light";
}

/**
 * The value to put on <html data-theme="...">. "auto" yields null so the CSS
 * media query controls the palette; explicit modes yield their name.
 */
export function themeAttr(mode: ThemeMode): ResolvedTheme | null {
  return mode === "auto" ? null : mode;
}

/** Short label for the toggle button, e.g. "auto", "light", "dark". */
export function modeLabel(mode: ThemeMode): string {
  return mode;
}

/** A small glyph hint for the toggle: sun/moon/circle. */
export function modeGlyph(mode: ThemeMode): string {
  switch (mode) {
    case "light":
      return "\u2600"; // sun
    case "dark":
      return "\u263d"; // moon
    default:
      return "\u25d1"; // half-filled circle = auto
  }
}

/** Title attribute copy for the toggle (announces the NEXT action). */
export function modeTitle(mode: ThemeMode): string {
  const next = nextMode(mode);
  return `Theme: ${mode} — click for ${next}`;
}
