/**
 * Settings model (F24) — per-client preferences persisted in localStorage and
 * applied as attributes on <html data-*>, so CSS can react without JS in the
 * hot path. Pure + dependency-free (unit-tested under `node --test`); main.ts
 * owns the drawer DOM and the load/save plumbing.
 *
 * Preferences (all client-side; none touch .tsk.md):
 *   - density:    "comfortable" | "compact"   row padding
 *   - motion:     "full" | "reduced"          opt out of animations regardless
 *                                             of the OS prefers-reduced-motion
 *   - hideDone:   boolean                      start with completed tasks hidden
 *   - showIds:    boolean                      show the #N id chip on each row
 *   - hideMeta:   boolean                      hide the row meta cluster (tags,
 *                                              due, notes, priority, delete)
 *                                              until the row is hovered/selected
 */

export type Density = "comfortable" | "compact";
export type Motion = "full" | "reduced";

export interface Settings {
  density: Density;
  motion: Motion;
  hideDone: boolean;
  showIds: boolean;
  hideMeta: boolean;
}

export const STORAGE_KEY = "tsk.settings";

/** Factory for the default settings (comfortable, full motion, ids shown). */
export function defaultSettings(): Settings {
  return { density: "comfortable", motion: "full", hideDone: false, showIds: true, hideMeta: false };
}

/** Coerce an unknown (e.g. parsed JSON) into a valid Settings, filling gaps. */
export function normalizeSettings(raw: unknown): Settings {
  const d = defaultSettings();
  if (typeof raw !== "object" || raw === null) return d;
  const o = raw as Record<string, unknown>;
  return {
    density: o.density === "compact" ? "compact" : "comfortable",
    motion: o.motion === "reduced" ? "reduced" : "full",
    hideDone: o.hideDone === true,
    showIds: o.showIds !== false, // default true; only an explicit false hides
    hideMeta: o.hideMeta === true,
  };
}

/** Parse a stored JSON string into Settings, falling back to defaults on junk. */
export function parseSettings(stored: string | null): Settings {
  if (stored === null) return defaultSettings();
  try {
    return normalizeSettings(JSON.parse(stored));
  } catch {
    return defaultSettings();
  }
}

/** Serialize settings for storage. */
export function serializeSettings(s: Settings): string {
  return JSON.stringify(s);
}

/**
 * The data-* attributes to mirror onto <html> for a given settings object.
 * Returned as a map so both main.ts and the test can assert the same mapping.
 * `null` means "remove the attribute" (the default state needs no marker).
 */
export function settingsAttributes(s: Settings): Record<string, string | null> {
  return {
    "data-density": s.density === "compact" ? "compact" : null,
    "data-motion": s.motion === "reduced" ? "reduced" : null,
    "data-show-ids": s.showIds ? null : "off",
    "data-hide-meta": s.hideMeta ? "on" : null,
  };
}

/** Escape strings before injecting into innerHTML. Local copy keeps this dependency-free. */
function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}

interface ToggleRow {
  key: keyof Settings;
  label: string;
  desc: string;
  /** For boolean prefs: the on-state. For enum prefs: a two-option segmented set. */
  kind: "bool" | "density" | "motion";
}

const ROWS: ReadonlyArray<ToggleRow> = [
  { key: "density", label: "Density", desc: "Row spacing", kind: "density" },
  { key: "motion", label: "Motion", desc: "Animations & transitions", kind: "motion" },
  { key: "hideDone", label: "Hide completed by default", desc: "Start with done tasks hidden", kind: "bool" },
  { key: "showIds", label: "Show task IDs", desc: "The #N chip on each row", kind: "bool" },
  { key: "hideMeta", label: "Reveal meta on hover", desc: "Hide tags/due/notes until a row is hovered", kind: "bool" },
];

/** Render a segmented two-option control. */
function segmented(key: string, options: Array<[string, string]>, current: string): string {
  const buttons = options
    .map(
      ([value, text]) =>
        `<button type="button" class="seg${value === current ? " is-active" : ""}" data-set="${key}" data-value="${value}" aria-pressed="${value === current}">${escapeHTML(text)}</button>`,
    )
    .join("");
  return `<div class="seg-group" role="group">${buttons}</div>`;
}

/** Render a boolean toggle switch. */
function boolToggle(key: string, on: boolean): string {
  return `<button type="button" class="switch${on ? " is-on" : ""}" data-toggle-setting="${key}" role="switch" aria-checked="${on}"><span class="switch-knob"></span></button>`;
}

/**
 * Render the settings drawer body (header + a row per preference). Pure → the
 * markup is unit-tested. main.ts mounts this inside the drawer shell and wires
 * the delegated click handlers via the data-* hooks.
 */
export function renderSettings(s: Settings): string {
  const rows = ROWS.map((row) => {
    let control = "";
    if (row.kind === "density") {
      control = segmented("density", [["comfortable", "Comfortable"], ["compact", "Compact"]], s.density);
    } else if (row.kind === "motion") {
      control = segmented("motion", [["full", "Full"], ["reduced", "Reduced"]], s.motion);
    } else {
      control = boolToggle(row.key, s[row.key] as boolean);
    }
    return `
      <div class="set-row">
        <div class="set-text">
          <div class="set-label">${escapeHTML(row.label)}</div>
          <div class="set-desc">${escapeHTML(row.desc)}</div>
        </div>
        <div class="set-control">${control}</div>
      </div>`;
  }).join("");
  return `
    <div class="drawer-head">
      <h2>Settings</h2>
      <button class="drawer-close" data-settings-close type="button" aria-label="Close settings">&times;</button>
    </div>
    <div class="set-rows">${rows}</div>
    <div class="set-config">
      <div class="set-config-label">Configuration</div>
      <div class="set-config-actions">
        <button type="button" class="set-cfg-btn" data-config-export>Export config</button>
        <button type="button" class="set-cfg-btn" data-config-import>Import config</button>
        <button type="button" class="set-cfg-btn set-cfg-danger" data-config-reset>Reset to defaults</button>
      </div>
      <div class="set-config-note">Export bundles your settings + saved views as a JSON file. Reset clears both.</div>
    </div>
    <div class="drawer-foot">Preferences are stored in this browser only. They never touch your .tsk.md.</div>`;
}
