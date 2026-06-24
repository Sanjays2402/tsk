/**
 * Toast renderer — a single transient notification with an optional action
 * button (used by delete-with-undo, F8). Pure + dependency-free so the markup
 * is unit-tested; main.ts owns mounting, timers, and the click wiring.
 */

/** Escape strings before injecting into innerHTML. Local copy keeps this dependency-free. */
function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}

export interface ToastSpec {
  /** Main message, e.g. `Deleted "buy milk"`. */
  message: string;
  /** Action button label, e.g. `Undo`. Omit for a message-only toast. */
  actionLabel?: string;
  /** Seconds until auto-dismiss; drives the progress bar animation. */
  seconds?: number;
}

/**
 * Render the inner HTML for a toast. The action button (when present) carries
 * `data-toast-action` so a single delegated listener can catch it. A progress
 * bar animates the countdown when `seconds` is given.
 */
export function renderToast(spec: ToastSpec): string {
  const action = spec.actionLabel
    ? `<button class="toast-action" data-toast-action type="button">${escapeHTML(spec.actionLabel)}</button>`
    : "";
  const bar =
    spec.seconds && spec.seconds > 0
      ? `<div class="toast-bar" style="animation-duration:${spec.seconds}s"></div>`
      : "";
  return `
    <div class="toast-body">
      <span class="toast-msg">${escapeHTML(spec.message)}</span>
      ${action}
    </div>
    ${bar}`;
}

/** Build the message for a deleted task, quoting and truncating long titles. */
export function deletedMessage(title: string): string {
  const max = 40;
  const shown = title.length > max ? title.slice(0, max - 1) + "\u2026" : title;
  return `Deleted "${shown}"`;
}
