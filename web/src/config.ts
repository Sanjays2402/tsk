/**
 * Client config bundle (F34) — export/import the whole per-client preference
 * set (settings + saved views) as one portable JSON blob, plus a "reset to
 * defaults" helper. None of this touches .tsk.md; it's purely the localStorage
 * preferences this browser holds.
 *
 * Pure + dependency-free (unit-tested under `node --test`). main.ts owns the
 * file download / upload plumbing and the localStorage writes; this module owns
 * the bundle shape, (de)serialization, and validation so a hand-edited or
 * foreign blob can't corrupt the app state.
 */

import {
  normalizeSettings,
  defaultSettings,
  type Settings,
} from "./settings.ts";
import { normalizeViews, type SavedView } from "./views.ts";

/** A versioned bundle so future shape changes can migrate cleanly. */
export interface ConfigBundle {
  /** Bundle format version. */
  version: number;
  /** A marker so an arbitrary JSON file isn't mistaken for a tsk config. */
  kind: "tsk-web-config";
  settings: Settings;
  views: SavedView[];
  /** Theme mode, if captured. Optional so older bundles still import. */
  theme?: string;
}

export const CONFIG_VERSION = 1;
export const CONFIG_KIND = "tsk-web-config";

/** Valid theme modes the bundle may carry. */
function normalizeTheme(raw: unknown): string | undefined {
  return raw === "light" || raw === "dark" || raw === "auto" ? raw : undefined;
}

/**
 * Build a config bundle from the live preference pieces. The settings + views
 * are normalized so the exported blob is always clean.
 */
export function buildConfig(settings: Settings, views: SavedView[], theme?: string): ConfigBundle {
  return {
    version: CONFIG_VERSION,
    kind: CONFIG_KIND,
    settings: normalizeSettings(settings),
    views: normalizeViews(views),
    theme: normalizeTheme(theme),
  };
}

/** Serialize a config bundle to a pretty JSON string for download. */
export function serializeConfig(bundle: ConfigBundle): string {
  return JSON.stringify(bundle, null, 2);
}

/**
 * The result of importing a config blob: either a clean bundle, or an error
 * describing why it was rejected. Never throws — the caller surfaces the message.
 */
export type ImportResult =
  | { ok: true; bundle: ConfigBundle }
  | { ok: false; error: string };

/**
 * Parse + validate an imported config blob. Tolerant of a missing version
 * (assumes v1) but rejects a blob that isn't an object or whose `kind` marker
 * is wrong, so importing a random JSON file fails loudly instead of wiping
 * preferences. Settings + views are normalized, dropping any junk entries.
 */
export function parseConfig(text: string): ImportResult {
  let raw: unknown;
  try {
    raw = JSON.parse(text);
  } catch {
    return { ok: false, error: "Not valid JSON" };
  }
  if (typeof raw !== "object" || raw === null || Array.isArray(raw)) {
    return { ok: false, error: "Config must be a JSON object" };
  }
  const o = raw as Record<string, unknown>;
  if (o.kind !== CONFIG_KIND) {
    return { ok: false, error: "Not a tsk web config file" };
  }
  const bundle: ConfigBundle = {
    version: typeof o.version === "number" ? o.version : CONFIG_VERSION,
    kind: CONFIG_KIND,
    settings: normalizeSettings(o.settings),
    views: normalizeViews(o.views),
    theme: normalizeTheme(o.theme),
  };
  return { ok: true, bundle };
}

/** A timestamped filename for the exported config, e.g. tsk-config-2026-06-24.json. */
export function configFilename(now = new Date()): string {
  const y = now.getFullYear();
  const m = String(now.getMonth() + 1).padStart(2, "0");
  const d = String(now.getDate()).padStart(2, "0");
  return `tsk-config-${y}-${m}-${d}.json`;
}

/** The default-reset bundle: factory settings, no saved views, auto theme. */
export function resetBundle(): ConfigBundle {
  return {
    version: CONFIG_VERSION,
    kind: CONFIG_KIND,
    settings: defaultSettings(),
    views: [],
    theme: "auto",
  };
}
