/**
 * Row context menu (F37) — a pure, testable layer that builds the list of
 * per-row actions and renders the menu markup. Right-click (desktop) or
 * long-press (touch, via the F28 machine) opens it anchored at the pointer;
 * main.ts owns the positioning, the outside-click guard, and the dispatch,
 * while this module owns WHAT actions exist for a given task and how each row
 * reads.
 *
 * The action ids deliberately mirror the command-palette + keyboard verbs so a
 * single dispatcher (runRowAction in main.ts) serves the menu, the palette, and
 * the hotkeys — one code path, three surfaces.
 */

export type RowAction =
  | "toggle"
  | "edit"
  | "due"
  | "notes"
  | "pin"
  | "prio-up"
  | "prio-down"
  | "delete";

/** The minimal task shape the menu needs to label its items. */
export interface MenuTask {
  id: number;
  done: boolean;
  pinned?: boolean;
}

export interface MenuItem {
  action: RowAction;
  label: string;
  /** Optional keyboard hint shown on the right (mirrors the row hotkeys). */
  hint?: string;
  /** True to render a divider ABOVE this item (visual grouping). */
  divider?: boolean;
  /** True to style as destructive (delete). */
  danger?: boolean;
}

/**
 * Build the ordered action list for a task. Labels reflect the task's current
 * state (Mark done vs Mark not done, Pin vs Unpin) so the menu reads naturally.
 * Grouped: state/edit actions, then priority, then the destructive delete.
 */
export function menuItemsFor(task: MenuTask): MenuItem[] {
  return [
    { action: "toggle", label: task.done ? "Mark not done" : "Mark done", hint: "space" },
    { action: "edit", label: "Edit title", hint: "e" },
    { action: "due", label: "Set due date", hint: "d" },
    { action: "notes", label: "Edit notes", hint: "i" },
    { action: "pin", label: task.pinned ? "Unpin" : "Pin to top", hint: "p", divider: true },
    { action: "prio-up", label: "Raise priority", hint: "]" },
    { action: "prio-down", label: "Lower priority", hint: "[" },
    { action: "delete", label: "Delete", hint: "x", divider: true, danger: true },
  ];
}

/** Escape strings before injecting into innerHTML. Local copy keeps this pure. */
function escapeHTML(s: string): string {
  return s.replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}

/**
 * Render the context menu's inner list for a task. Each item carries a
 * data-row-action hook a delegated listener dispatches on; dividers become a
 * top border via the is-divider class. The active/danger styling is CSS.
 */
export function renderContextMenu(task: MenuTask): string {
  const items = menuItemsFor(task)
    .map((it) => {
      const cls = ["ctxmenu-item", it.divider ? "is-divider" : "", it.danger ? "is-danger" : ""]
        .filter(Boolean)
        .join(" ");
      const hint = it.hint ? `<kbd class="ctxmenu-hint">${escapeHTML(it.hint)}</kbd>` : "";
      return `<li class="${cls}" role="menuitem" data-row-action="${it.action}" tabindex="-1">
        <span class="ctxmenu-label">${escapeHTML(it.label)}</span>${hint}
      </li>`;
    })
    .join("");
  return `<ul class="ctxmenu-list" role="menu" aria-label="Task #${task.id} actions">${items}</ul>`;
}

/**
 * Clamp a menu's top-left so it stays fully inside the viewport. Given the
 * desired anchor (x,y), the menu's size, and the viewport size, nudge it left/up
 * when it would overflow the right/bottom edges, never past 0. Pure so the
 * positioning math is unit-tested with zero DOM.
 */
export function clampMenuPosition(
  x: number,
  y: number,
  menuW: number,
  menuH: number,
  viewW: number,
  viewH: number,
  margin = 8,
): { left: number; top: number } {
  let left = x;
  let top = y;
  if (left + menuW + margin > viewW) left = viewW - menuW - margin;
  if (top + menuH + margin > viewH) top = viewH - menuH - margin;
  if (left < margin) left = margin;
  if (top < margin) top = margin;
  return { left, top };
}
