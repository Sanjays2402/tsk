package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Sanjays2402/tsk/internal/dateparse"
	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	tuiLocOnce sync.Once
	tuiLocVal  *time.Location
)

// pacificLoc returns the cached location tsk uses for natural-language
// date parsing. The name is kept for continuity; actual resolution follows
// the same priority order as commands.ResolveTZ ($TSK_TZ, $TZ, time.Local,
// America/Los_Angeles).
func pacificLoc() *time.Location {
	tuiLocOnce.Do(func() {
		for _, candidate := range []string{os.Getenv("TSK_TZ"), os.Getenv("TZ")} {
			if candidate == "" {
				continue
			}
			if l, err := time.LoadLocation(candidate); err == nil {
				tuiLocVal = l
				return
			}
		}
		if time.Local != time.UTC {
			tuiLocVal = time.Local
			return
		}
		if l, err := time.LoadLocation("America/Los_Angeles"); err == nil {
			tuiLocVal = l
			return
		}
		tuiLocVal = time.Local
	})
	return tuiLocVal
}

// App is the bubbletea Model for tsk's interactive UI.
type App struct {
	store      *store.Store
	pal        Palette
	keys       Keymap
	now        time.Time
	width      int
	height     int
	selection  int
	collapsed  map[sectionKind]bool
	form       formMode
	inputCur   inputBox
	editing    int
	confirm    int
	status     string
	showHelp   bool
	filter     string
	sortMode   string
	pinnedOnly bool
}

// inputBox is a tiny stand-in that abstracts textinput to avoid importing the
// whole bubbles package in this model struct (keeps test helpers simple).
type inputBox struct {
	label string
	value string
	focus bool
}

func (b inputBox) View() string {
	caret := "█"
	if !b.focus {
		caret = ""
	}
	return fmt.Sprintf("%s: %s%s", b.label, b.value, caret)
}

// New constructs a new TUI app wrapped around a loaded store.
func New(s *store.Store) *App {
	return &App{
		store:     s,
		pal:       NewPalette(),
		keys:      DefaultKeymap(),
		now:       time.Now(),
		collapsed: map[sectionKind]bool{sectionDone: true},
		sortMode:  "priority",
	}
}

// Init satisfies tea.Model.
func (a *App) Init() tea.Cmd { return nil }

// Update handles keypresses and window events.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = m.Width, m.Height
		return a, nil
	case tea.KeyMsg:
		return a.handleKey(m)
	}
	return a, nil
}

func (a *App) handleKey(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.form != formNone {
		return a.handleFormKey(m)
	}
	if a.confirm != 0 {
		return a.handleConfirmKey(m)
	}
	if handled, model, cmd := a.handleGlobalKey(m); handled {
		return model, cmd
	}
	a.handleNavKey(m)
	return a, nil
}

func (a *App) handleGlobalKey(m tea.KeyMsg) (bool, tea.Model, tea.Cmd) {
	switch {
	case matches(m, a.keys.Quit):
		return true, a, tea.Quit
	case matches(m, a.keys.Help):
		a.showHelp = !a.showHelp
		return true, a, nil
	}
	return false, a, nil
}

func (a *App) handleNavKey(m tea.KeyMsg) {
	switch {
	case matches(m, a.keys.Down):
		a.moveSelection(1)
	case matches(m, a.keys.Up):
		a.moveSelection(-1)
	case matches(m, a.keys.Top):
		a.jumpTop()
	case matches(m, a.keys.Bottom):
		a.jumpBottom()
	case matches(m, a.keys.Toggle):
		a.toggleCurrent()
	case matches(m, a.keys.Add):
		a.form = formAdd
		a.inputCur = inputBox{label: "new task", focus: true}
	case matches(m, a.keys.Edit):
		a.startEditTitle()
	case matches(m, a.keys.Delete):
		if id := a.currentID(); id != 0 {
			a.confirm = id
		}
	case matches(m, a.keys.PriorityCycle):
		a.cyclePriority()
	case matches(m, a.keys.TagEdit):
		a.startEditTags()
	case matches(m, a.keys.DueEdit):
		a.startEditDue()
	case matches(m, a.keys.Search):
		a.form = formSearch
		a.inputCur = inputBox{label: "search", value: a.filter, focus: true}
	case matches(m, a.keys.SortMenu):
		a.form = formSort
		a.inputCur = inputBox{label: "sort (priority|due|created|id)", value: a.sortMode, focus: true}
	case matches(m, a.keys.Section):
		a.toggleSection()
	case matches(m, a.keys.Reload):
		a.reloadFromDisk()
	case matches(m, a.keys.ReloadClear):
		a.reloadFromDiskClearingFilter()
	case matches(m, a.keys.Clone):
		a.cloneCurrent()
	case matches(m, a.keys.JumpNext):
		a.jumpToNextUnblocked()
	case matches(m, a.keys.FocusPinned):
		a.toggleFocusPinned()
	case matches(m, a.keys.ArchiveCurrent):
		a.archiveCurrent()
	}
}

// reloadFromDisk re-reads the active .tsk.md from disk and swaps
// the in-memory store, discarding any unsaved in-memory mutations
// EXCEPT those already persisted (which is the steady-state case —
// the TUI saves after every mutation, so the in-memory store is
// always in sync with disk before this runs).
//
// Use case: an external mutation happens during a TUI session
// (the user runs `tsk add` in another terminal, or an editor
// rewrites the file, or `tsk lint --autofix-all` cleans things up
// from a pre-commit hook). Without reload, the TUI keeps stale
// state and the user has to quit + re-launch to see the change.
// With `r`, the TUI picks up the external edit live.
//
// Selection is preserved by ID: if the task that was selected
// before the reload still exists in the new store, the cursor
// snaps to it; otherwise the cursor falls back to position 0
// (the first visible task in the new ordering). This is the
// least-surprising behavior — the user's mental cursor anchor
// (the TASK, not the row index) is what they'd want to keep.
//
// Errors are surfaced into the status footer rather than crashing
// the app — a transient read failure (file rotated, sync glitch)
// should be visible but not fatal. The user can hit `r` again
// once the issue clears.
//
// IMPORTANT: this clears a.filter and a.editing because both are
// tied to the OLD store's view. Re-applying the filter against the
// new store is the user's call (they may want a fresh look at the
// reloaded data).
func (a *App) reloadFromDisk() {
	if a.store == nil || a.store.Path == "" {
		a.status = "reload: no store path"
		return
	}
	prevID := a.currentID()
	fresh, err := store.Load(a.store.Path)
	if err != nil {
		a.status = "reload failed: " + err.Error()
		return
	}
	a.store = fresh
	// Reset transient editing state that pointed at the old store.
	a.editing = 0
	a.form = formNone
	a.confirm = 0
	// Preserve selection-by-id when possible; otherwise snap to top.
	if prevID > 0 {
		vt := a.visibleTasks()
		newIdx := -1
		for i, t := range vt {
			if t.ID == prevID {
				newIdx = i
				break
			}
		}
		if newIdx >= 0 {
			a.selection = newIdx
		} else {
			a.selection = 0
		}
	} else {
		a.selection = 0
	}
	a.status = "reloaded"
}

// reloadFromDiskClearingFilter is the uppercase-R sister of
// reloadFromDisk: same disk re-read mechanics, but ALSO clears
// any active search filter so the user lands on the full
// untrimmed list after the reload.
//
// Use case: the user has been narrowing the visible set with `/`
// to focus on one slice, picks up an external edit, and wants
// to see the WHOLE new state without first hitting `/` then Esc.
// Lowercase `r` preserves the filter (steady-state "refresh,
// keep my view"), uppercase `R` resets it (escape hatch
// "refresh and show me everything"). Same convention as the
// `g` / `G` (top / bottom) keymap.
//
// Selection-by-id behavior matches reloadFromDisk: if the
// previously-selected task still exists, the cursor snaps to it
// (now in the FULL list, not the filtered subset, so the
// position number may shift); otherwise falls back to 0.
//
// The clear is unconditional: if no filter was active, this is
// equivalent to lowercase `r` (a small surprise minimizer — the
// user never sees the filter "appear" cleared when nothing was
// there). The status footer says "reloaded (filter cleared)"
// so the user knows uppercase did the extra work.
func (a *App) reloadFromDiskClearingFilter() {
	a.filter = ""
	a.reloadFromDisk()
	// Override the status message reloadFromDisk just set so the
	// uppercase action is distinguishable in the footer (helps the
	// user notice when they meant to hit lowercase but capslock was
	// on, or vice versa).
	if a.status == "reloaded" {
		a.status = "reloaded (filter cleared)"
	}
}

// cloneCurrent duplicates the currently-selected task in place,
// the TUI sister of the `tsk clone <id>` CLI verb. Mirrors that
// verb's contract: the clone gets a fresh id and a fresh Created
// timestamp, starts open (regardless of the source's done state),
// inherits priority + due + tags + notes by value, and the title
// gets a " (copy)" suffix so the duplicate is easy to spot in the
// list.
//
// Why a TUI shortcut for a CLI verb? Because the muscle-memory
// workflow for "use this completed task as a template for the
// next instance" (the most common clone use case) is one keystroke
// in the TUI, vs. dropping back to the shell, typing the id, and
// re-entering. Same reason `tsk reopen` exists alongside `tsk undo`
// — discoverability matters when you're inside a focused-work
// session.
//
// Selection-by-id behavior: after the clone, the cursor is moved
// to the NEW task (the one just created) so the user can
// immediately edit/tag/repri it without hunting for the new row.
// Falls back to the original selection if the new id isn't in
// the visible set (defensive — the visible filter might exclude
// the clone, e.g. if a filter is active and the clone doesn't
// match).
//
// Errors (Save failure, missing source) surface into the status
// footer rather than crashing the app — same contract as the
// reloadFromDisk error path. The clone shares the existing
// cloneTask body (no duplication of the title-suffix logic).
//
// Why not bind to lowercase 'c'? Because 'c' is reserved for
// future "change" cluster verbs (a common vim convention) and
// uppercase 'C' is the conventional "create a copy" shortcut in
// modal editors. The TUI 'r' / 'R' pair (reload vs. reload+clear)
// follows the same lowercase/uppercase convention.
func (a *App) cloneCurrent() {
	id := a.currentID()
	if id == 0 {
		a.status = "clone: no task selected"
		return
	}
	src := a.store.ByID(id)
	if src == nil {
		a.status = "clone: source not found"
		return
	}
	// Shared cloneTask helper lives in internal/commands but
	// the model+timestamp logic it embodies is small enough to
	// inline here without dragging the CLI package into the TUI
	// (which would risk import cycles). Mirrors cloneTask's
	// contract exactly so the two surfaces agree on "what a
	// clone is".
	clone := model.Task{
		Title:    src.Title + " (copy)",
		Priority: src.Priority,
		Notes:    src.Notes,
		Created:  time.Now(),
	}
	if len(src.Tags) > 0 {
		clone.Tags = append([]string(nil), src.Tags...)
	}
	if src.Due != nil {
		d := *src.Due
		clone.Due = &d
	}
	// Clones start open regardless of source — matches the
	// "use this as a template for the next instance" intent.
	clone.Done = false
	clone.Completed = nil
	newID := a.store.Add(clone)
	if err := a.store.Save(); err != nil {
		a.status = "clone: save failed: " + err.Error()
		return
	}
	// Move the selection to the new clone so the user can
	// immediately edit it. Fall back to the original position
	// if the clone isn't in the visible set (filter, collapsed
	// section, etc).
	vt := a.visibleTasks()
	for i, t := range vt {
		if t.ID == newID {
			a.selection = i
			break
		}
	}
	a.status = fmt.Sprintf("cloned #%d → #%d", id, newID)
}

// jumpToNextUnblocked moves the cursor to the highest-priority
// undone, non-waiting, non-blocked task in the visible list — the
// TUI sister of `tsk next --respect-deps`. One keystroke ('N')
// instead of dropping back to the shell to ask "what should I work
// on next?" in the middle of a TUI session.
//
// Selection contract (mirrors `tsk next --respect-deps`):
//   - pinned tasks beat unpinned (sticky bookmarks float)
//   - within the same pin state, higher Priority wins
//     (urgent > high > medium > low)
//   - dated tasks beat undated; earliest due wins among dated
//   - lower id breaks remaining ties (stable, deterministic)
//
// Done / waiting / blocked tasks are EXCLUDED from the candidate
// pool — they're not actionable right now, and surfacing them
// would defeat the "what can I actually work on?" intent. If every
// candidate is blocked, the cursor falls back to the highest-
// priority BLOCKED task (annotated in the status footer so the
// user knows what's gating progress) rather than going silent — a
// "(blocked)" annotation is more honest than "no task found" when
// the answer is "everything's stuck on X".
//
// Visibility-aware: only tasks the user can currently SEE (via
// visibleTasks(), which respects collapse state + filter) are
// candidates. So if Done is collapsed (the default), done tasks
// are ignored anyway; if the user filtered to "#work", only work
// tasks are picked from. This matches the rest of the TUI's
// jump-style helpers (jumpTop / jumpBottom).
//
// Errors / empty-pool surface into the status footer; selection
// never moves past the visible bounds.
//
// Why a separate verb from jumpBottom? Because "next" isn't
// positional — it's a SCORE function over the candidate pool.
// jumpBottom always lands on the last row regardless of priority;
// jumpToNextUnblocked lands on the BEST row given the current
// dep / priority / due state. The two share no semantics beyond
// "move the cursor".
func (a *App) jumpToNextUnblocked() {
	vt := a.visibleTasks()
	if len(vt) == 0 {
		a.status = "no tasks visible"
		return
	}
	now := time.Now()
	var best *model.Task
	var bestIdx int = -1
	var bestBlocked *model.Task
	var bestBlockedIdx int = -1
	var bestBlockedReasons []int
	for i := range vt {
		t := &vt[i]
		if t.Done {
			continue
		}
		if t.IsWaiting(now) {
			continue
		}
		blockers := tuiUnmetBlockers(a.store, t)
		if len(blockers) > 0 {
			if isBetterNextTUI(t, bestBlocked) {
				bestBlocked = t
				bestBlockedIdx = i
				bestBlockedReasons = blockers
			}
			continue
		}
		if isBetterNextTUI(t, best) {
			best = t
			bestIdx = i
		}
	}
	if best != nil {
		a.selection = bestIdx
		a.status = fmt.Sprintf("next: #%d %s", best.ID, best.Title)
		return
	}
	if bestBlocked != nil {
		// All visible candidates blocked; surface the best one
		// with its blockers so the user knows what's gating.
		a.selection = bestBlockedIdx
		blockerLabels := make([]string, len(bestBlockedReasons))
		for i, id := range bestBlockedReasons {
			blockerLabels[i] = fmt.Sprintf("#%d", id)
		}
		a.status = fmt.Sprintf("next: #%d %s (blocked by %s)",
			bestBlocked.ID, bestBlocked.Title, strings.Join(blockerLabels, ", "))
		return
	}
	a.status = "all caught up"
}

// tuiUnmetBlockers is the TUI's local copy of the open-prereq
// check. We don't import internal/commands (cycle), so this is
// a small duplicate of commands.unmetBlockers's body — only the
// "batchIDs" parameter is dropped (it's meaningful only for
// `tsk done` batch ops, not for a TUI cursor move).
//
// A dangling dep (id with no task in the store) is treated as
// satisfied: the user can't be expected to clean up an
// out-of-band reference just to navigate. `tsk lint` is where
// dangling deps get surfaced for cleanup.
func tuiUnmetBlockers(s *store.Store, t *model.Task) []int {
	if !t.HasDependencies() {
		return nil
	}
	out := make([]int, 0, len(t.DependsOn))
	for _, dep := range t.DependsOn {
		bt := s.ByID(dep)
		if bt == nil {
			continue
		}
		if !bt.Done {
			out = append(out, dep)
		}
	}
	return out
}

// isBetterNextTUI is the TUI's local copy of the next-pick
// tie-break order. Duplicated rather than imported (cycle
// avoidance) but keeps the order in lockstep with the CLI's
// `tsk next` selector:
//
//	pin > priority desc > dated-before-undated > earliest-due > lowest-id
//
// Returns true when t should beat current. nil current always
// loses (the first valid candidate is best by default).
func isBetterNextTUI(t, current *model.Task) bool {
	if current == nil {
		return true
	}
	if t.Pinned != current.Pinned {
		return t.Pinned
	}
	if t.Priority != current.Priority {
		return t.Priority > current.Priority
	}
	switch {
	case t.Due != nil && current.Due == nil:
		return true
	case t.Due == nil && current.Due != nil:
		return false
	case t.Due != nil && current.Due != nil:
		if !t.Due.Equal(*current.Due) {
			return t.Due.Before(*current.Due)
		}
	}
	return t.ID < current.ID
}

// toggleFocusPinned flips the pinned-only filter on/off. When on,
// visibleTasks() further narrows to only PINNED tasks — the TUI
// sister of `tsk top --pinned-only`. When off, the unfiltered
// (or text-filtered) full view returns.
//
// Use case: the user has a small handful of bookmark tasks pinned
// (via `tsk pin`) and wants a focus mode that hides everything
// else without permanently filtering. One keystroke in, one
// keystroke out — toggling rather than entering a long-lived
// search filter.
//
// Selection preservation: when toggling ON, if the previously-
// selected task is pinned (i.e. it survives the new filter), the
// cursor stays on it. Otherwise it snaps to the first visible
// pinned task. When toggling OFF, the cursor tries to preserve
// the same task in the now-broader list; falls back to position
// 0 if not found.
//
// Composes with the existing text filter: pinned-only AND the
// text filter both apply (intersection). So the user can hit `F`
// to focus pinned, then `/` to narrow further within the pinned
// set — e.g. "show me only my pinned 'release' tasks".
//
// Status footer reflects the state change so the user knows
// what changed: "pinned only" when on, "all tasks" when off.
// Uppercase 'F' (not 'f') to leave lowercase free for future
// "f" commands (find, filter — none currently bound) and to
// match the lowercase-positional / uppercase-explicit convention
// the TUI has been settling into (g/G, r/R, n/N as cancel vs
// next-jump).
//
// Empty pinned set: still works — visibleTasks just becomes
// empty, the status footer says "pinned only (no pinned
// tasks)". Helpful diagnostic when the user thinks they've
// pinned things but hasn't.
func (a *App) toggleFocusPinned() {
	prevID := a.currentID()
	a.pinnedOnly = !a.pinnedOnly
	vt := a.visibleTasks()
	// Try to preserve the selection by id; fall back to 0.
	if prevID > 0 {
		for i, t := range vt {
			if t.ID == prevID {
				a.selection = i
				if a.pinnedOnly {
					a.status = "pinned only"
				} else {
					a.status = "all tasks"
				}
				return
			}
		}
	}
	a.selection = 0
	switch {
	case a.pinnedOnly && len(vt) == 0:
		a.status = "pinned only (no pinned tasks)"
	case a.pinnedOnly:
		a.status = "pinned only"
	default:
		a.status = "all tasks"
	}
}

// archiveCurrent moves the currently-selected DONE task into the
// sibling .tsk.archive.md file — the TUI sister of `tsk archive`
// (flat strategy, default sibling file). One keystroke ('X') for
// the common end-of-day "clear my completed work out of the active
// list" workflow, vs. dropping back to the shell to run
// `tsk archive --all`.
//
// Why DONE-only? The CLI `tsk archive` already filters to Done tasks
// by predicate; archiving an open task would be a category error
// (the whole point of the archive file is completed work). Refusing
// the action surfaces a "task is not done — mark it done first"
// status hint rather than silently no-opping, so the user
// understands what's blocked.
//
// Why flat-strategy default (no bucket-by, no merge-into)? The TUI
// shortcut is for the single-task quick action; the full CLI is
// still where bucketed / merge-into / --strategy live. A TUI verb
// that surfaced every flag would clutter the keymap and be no
// faster than just dropping to the shell. The flat append keeps the
// single-keystroke promise.
//
// Atomic-ish: the archive store is loaded, the task is appended
// with a fresh archive id (continuing the archive's max+1, same
// contract as the CLI), the archive is saved, THEN the task is
// removed from the active store and the active is saved. If the
// archive save fails, the active is untouched (the task stays
// where it was — no half-archived state). If the active save
// fails after a successful archive save, the task DOES exist in
// both files momentarily (the user can re-run X or `tsk archive
// --all` to recover). This matches the CLI's two-save approach
// since wrapping both in a single atomic write isn't feasible
// across two distinct files.
//
// Selection contract: after the archive, the cursor stays on the
// same visible position (so pressing X repeatedly walks down the
// done section without surprise jumps). If the cursor was on the
// last item in the visible list, it shifts up by one. Falls back
// to position 0 if the active store ends up empty.
//
// Errors surface into the status footer; the active store is
// reloaded from the freshly-saved file so the in-memory view
// matches disk after the operation completes.
//
// Why uppercase 'X' (not 'x')? Lowercase 'x' is the conventional
// vim "delete one character" — reserved for a future single-
// character edit primitive. Uppercase 'X' is the conventional
// "destructive bulk-action" key in modal editors (vim's "delete
// backwards", less's "kill"), which matches the "this task is
// leaving the visible list" semantic.
//
// dependents-aware safety: if the task being archived is named as
// a prereq by any other OPEN task in the active store, the archive
// is REFUSED with a "blocked by N dependents" status — archiving
// would create a dangling dep reference in the active store, which
// then surfaces as a `tsk lint` finding. Better to flag it here
// and let the user `tsk depend --remove-all` or `tsk depend
// <other> --remove <id>` first. Done dependents are tolerated
// (they're already satisfied, the dangling ref is no-op).
func (a *App) archiveCurrent() {
	id := a.currentID()
	if id == 0 {
		a.status = "archive: no task selected"
		return
	}
	src := a.store.ByID(id)
	if src == nil {
		a.status = "archive: source not found"
		return
	}
	if !src.Done {
		a.status = "archive: task is not done — mark it done first"
		return
	}
	// dependents-aware safety: refuse if any OPEN task names this
	// id in its DependsOn. Otherwise the archive leaves a dangling
	// ref in the active store. Done dependents are fine — they're
	// already satisfied.
	blocking := make([]int, 0)
	for _, t := range a.store.Tasks {
		if t.ID == id || t.Done {
			continue
		}
		for _, dep := range t.DependsOn {
			if dep == id {
				blocking = append(blocking, t.ID)
				break
			}
		}
	}
	if len(blocking) > 0 {
		labels := make([]string, len(blocking))
		for i, bid := range blocking {
			labels[i] = fmt.Sprintf("#%d", bid)
		}
		a.status = fmt.Sprintf("archive: #%d is a prereq for %s — clear deps first", id, strings.Join(labels, ","))
		return
	}
	archivePath := tuiArchivePath(a.store.Path)
	arch, err := store.Load(archivePath)
	if err != nil {
		a.status = "archive: load failed: " + err.Error()
		return
	}
	// Stamp the archive header if the file didn't exist before
	// (Load on a missing file returns a fresh store with empty
	// header).
	if _, statErr := os.Stat(archivePath); os.IsNotExist(statErr) {
		arch.Header = "# tsk archive\n"
	}
	// Continue the archive's id space (max+1). The active id is
	// dropped on copy — the archive has its own monotonic sequence
	// so re-imports / cross-file references stay clean.
	nextArchiveID := 1
	for _, t := range arch.Tasks {
		if t.ID >= nextArchiveID {
			nextArchiveID = t.ID + 1
		}
	}
	copy := *src
	copy.ID = nextArchiveID
	arch.Tasks = append(arch.Tasks, copy)
	if err := arch.Save(); err != nil {
		a.status = "archive: save archive failed: " + err.Error()
		return
	}
	// Now remove from active. If this Save fails, the user has a
	// duplicate (in both files) — surface it clearly so they know
	// to re-run.
	a.store.Remove(id)
	if err := a.store.Save(); err != nil {
		a.status = "archive: archived OK but active save failed (duplicate exists): " + err.Error()
		return
	}
	// Move cursor down by one visible position (so repeated X walks
	// through the done section), or up by one if we were on the
	// last visible row.
	vt := a.visibleTasks()
	if a.selection >= len(vt) && len(vt) > 0 {
		a.selection = len(vt) - 1
	}
	if len(vt) == 0 {
		a.selection = 0
	}
	a.status = fmt.Sprintf("archived #%d -> #%d in %s", id, copy.ID, archivePath)
}

// tuiArchivePath resolves the sibling archive path next to the
// active .tsk.md. Mirrors the CLI's resolveArchivePath default
// (mergeInto==""): same directory, fixed ".tsk.archive.md" name.
// Inlined rather than imported because internal/tui can't import
// internal/commands (would cycle); the path-shape is small and
// stable.
func tuiArchivePath(activePath string) string {
	return filepath.Join(filepath.Dir(activePath), ".tsk.archive.md")
}

func (a *App) startEditTitle() {
	id := a.currentID()
	if id == 0 {
		return
	}
	a.editing = id
	a.form = formEditTitle
	t := a.store.ByID(id)
	a.inputCur = inputBox{label: "edit title", value: t.Title, focus: true}
}

func (a *App) startEditTags() {
	id := a.currentID()
	if id == 0 {
		return
	}
	a.editing = id
	a.form = formTags
	t := a.store.ByID(id)
	a.inputCur = inputBox{label: "tags (comma-sep)", value: strings.Join(t.Tags, ","), focus: true}
}

func (a *App) startEditDue() {
	id := a.currentID()
	if id == 0 {
		return
	}
	a.editing = id
	a.form = formDue
	t := a.store.ByID(id)
	cur := ""
	if t != nil && t.Due != nil {
		cur = t.Due.Format(model.DateLayout)
	}
	a.inputCur = inputBox{label: "due (YYYY-MM-DD, tomorrow, fri, in 3d, eow; empty to clear)", value: cur, focus: true}
}

func (a *App) handleFormKey(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.Type {
	case tea.KeyEsc:
		a.form = formNone
		a.editing = 0
		return a, nil
	case tea.KeyEnter:
		return a.commitForm()
	case tea.KeyBackspace:
		if len(a.inputCur.value) > 0 {
			a.inputCur.value = a.inputCur.value[:len(a.inputCur.value)-1]
		}
	case tea.KeyRunes, tea.KeySpace:
		a.inputCur.value += string(m.Runes)
	}
	return a, nil
}

func (a *App) commitForm() (tea.Model, tea.Cmd) {
	val := strings.TrimSpace(a.inputCur.value)
	switch a.form {
	case formAdd:
		if val != "" {
			a.store.Add(model.Task{Title: val, Priority: model.PriorityMedium, Created: time.Now()})
			a.saveWithStatus()
			a.status = "added"
		}
	case formEditTitle:
		if t := a.store.ByID(a.editing); t != nil && val != "" {
			t.Title = val
			a.saveWithStatus()
			a.status = "edited"
		}
	case formTags:
		if t := a.store.ByID(a.editing); t != nil {
			t.Tags = splitTags(val)
			t.NormalizeTags()
			a.saveWithStatus()
			a.status = "tags updated"
		}
	case formDue:
		a.commitDue(val)
	case formSearch:
		a.filter = val
	case formSort:
		if val == "priority" || val == "due" || val == "created" || val == "id" {
			a.sortMode = val
		}
	}
	a.form = formNone
	a.editing = 0
	return a, nil
}

// commitDue applies the parsed due-date form value to the current task, or
// clears the due date if the input is empty.
func (a *App) commitDue(val string) {
	t := a.store.ByID(a.editing)
	if t == nil {
		return
	}
	if val == "" {
		t.Due = nil
		a.saveWithStatus()
		a.status = "due cleared"
		return
	}
	loc := pacificLoc()
	due, err := dateparse.Parse(val, time.Now().In(loc), loc)
	if err != nil {
		a.status = err.Error()
		return
	}
	t.Due = &due
	a.saveWithStatus()
	a.status = "due updated"
}

// saveWithStatus persists the store and surfaces any error to the footer
// instead of swallowing it. On success it deliberately leaves a.status
// unchanged so callers that set a success message (e.g. "deleted") win.
func (a *App) saveWithStatus() {
	if err := a.store.Save(); err != nil {
		a.status = "save failed: " + err.Error()
	}
}

func (a *App) handleConfirmKey(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	if matches(m, a.keys.Confirm) {
		a.store.Remove(a.confirm)
		if err := a.store.Save(); err != nil {
			a.status = "save failed: " + err.Error()
		} else {
			a.status = "deleted"
		}
	}
	a.confirm = 0
	return a, nil
}

func splitTags(s string) []string {
	out := []string{}
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func matches(m tea.KeyMsg, binding interface{ Keys() []string }) bool {
	s := m.String()
	for _, k := range binding.Keys() {
		if k == s {
			return true
		}
	}
	return false
}

// visibleTasks returns the flat ordered list of tasks rendered to the user.
func (a *App) visibleTasks() []model.Task {
	groups := groupedTasks(a.store.Tasks, a.now)
	var out []model.Task
	order := []sectionKind{sectionOverdue, sectionToday, sectionUpcoming, sectionNoDue, sectionDone}
	for _, k := range order {
		if a.collapsed[k] {
			continue
		}
		g := groups[k]
		if a.sortMode != "" {
			model.SortBy(g, a.sortMode)
		}
		if a.filter != "" {
			filtered := g[:0]
			q := strings.ToLower(a.filter)
			for _, t := range g {
				if strings.Contains(strings.ToLower(t.Title), q) {
					filtered = append(filtered, t)
				}
			}
			g = filtered
		}
		if a.pinnedOnly {
			filtered := g[:0]
			for _, t := range g {
				if t.Pinned {
					filtered = append(filtered, t)
				}
			}
			g = filtered
		}
		out = append(out, g...)
	}
	return out
}

func (a *App) currentID() int {
	vt := a.visibleTasks()
	if a.selection < 0 || a.selection >= len(vt) {
		return 0
	}
	return vt[a.selection].ID
}

func (a *App) moveSelection(d int) {
	n := len(a.visibleTasks())
	if n == 0 {
		a.selection = 0
		return
	}
	a.selection = (a.selection + d + n) % n
}

// jumpTop snaps the selection to the first visible task. Vim-style 'g'
// (also bound to Home). Safe on an empty list: selection stays at 0
// so subsequent navigation behaves identically to a fresh app.
func (a *App) jumpTop() {
	a.selection = 0
}

// jumpBottom snaps the selection to the last visible task. Vim-style
// 'G' (also bound to End). Operates on visibleTasks() so the result
// respects the current filter/collapse state — \"bottom\" means the
// last task the user can actually see right now, not the last task
// in the underlying store.
func (a *App) jumpBottom() {
	n := len(a.visibleTasks())
	if n == 0 {
		a.selection = 0
		return
	}
	a.selection = n - 1
}

func (a *App) toggleCurrent() {
	id := a.currentID()
	if id == 0 {
		return
	}
	t := a.store.ByID(id)
	a.store.SetDone(id, !t.Done)
	a.saveWithStatus()
}

func (a *App) cyclePriority() {
	id := a.currentID()
	if id == 0 {
		return
	}
	t := a.store.ByID(id)
	t.Priority = model.Priority((int(t.Priority) + 1) % 4)
	a.saveWithStatus()
}

func (a *App) toggleSection() {
	vt := a.visibleTasks()
	if a.selection >= len(vt) {
		return
	}
	// Find which section the current selection belongs to and toggle it.
	id := a.currentID()
	t := a.store.ByID(id)
	if t == nil {
		return
	}
	switch {
	case t.Done:
		a.collapsed[sectionDone] = !a.collapsed[sectionDone]
	case t.IsOverdue(a.now):
		a.collapsed[sectionOverdue] = !a.collapsed[sectionOverdue]
	case t.IsDueToday(a.now):
		a.collapsed[sectionToday] = !a.collapsed[sectionToday]
	case t.IsUpcoming(a.now):
		a.collapsed[sectionUpcoming] = !a.collapsed[sectionUpcoming]
	default:
		a.collapsed[sectionNoDue] = !a.collapsed[sectionNoDue]
	}
}

// View renders the UI as a string.
func (a *App) View() string {
	var b strings.Builder
	b.WriteString(a.pal.Primary.Render("tsk"))
	b.WriteString("  ")
	b.WriteString(a.pal.Muted.Render(a.store.Path))
	b.WriteByte('\n')
	b.WriteByte('\n')

	groups := groupedTasks(a.store.Tasks, a.now)
	order := []sectionKind{sectionOverdue, sectionToday, sectionUpcoming, sectionNoDue, sectionDone}
	cursor := 0
	done, total := 0, 0
	for _, t := range a.store.Tasks {
		if t.Done {
			done++
		}
		total++
	}

	for _, k := range order {
		g := groups[k]
		if a.sortMode != "" {
			model.SortBy(g, a.sortMode)
		}
		if a.filter != "" {
			filtered := g[:0]
			q := strings.ToLower(a.filter)
			for _, t := range g {
				if strings.Contains(strings.ToLower(t.Title), q) {
					filtered = append(filtered, t)
				}
			}
			g = filtered
		}
		if a.pinnedOnly {
			filtered := g[:0]
			for _, t := range g {
				if t.Pinned {
					filtered = append(filtered, t)
				}
			}
			g = filtered
		}
		marker := "▾"
		if a.collapsed[k] {
			marker = "▸"
		}
		b.WriteString(a.pal.Section.Render(fmt.Sprintf("%s %s (%d)", marker, k.label(), len(g))))
		b.WriteByte('\n')
		if a.collapsed[k] {
			continue
		}
		for _, t := range g {
			line := a.renderTaskLine(t, cursor == a.selection)
			b.WriteString(line)
			b.WriteByte('\n')
			cursor++
		}
	}
	b.WriteByte('\n')
	b.WriteString(a.pal.Accent.Render(progressBar(done, total, 24)))
	b.WriteByte('\n')

	if a.form != formNone {
		b.WriteByte('\n')
		b.WriteString(a.inputCur.View())
	}
	if a.confirm != 0 {
		b.WriteByte('\n')
		b.WriteString(a.pal.Urgent.Render(fmt.Sprintf("delete #%d? (y/n)", a.confirm)))
	}
	if a.status != "" {
		b.WriteByte('\n')
		b.WriteString(a.pal.Muted.Render(a.status))
	}
	if a.showHelp {
		b.WriteByte('\n')
		b.WriteString(a.helpView())
	} else {
		b.WriteByte('\n')
		b.WriteString(a.pal.Help.Render("j/k move · g/G top/bottom · N next · F pin-focus · X archive · ␣ toggle · a add · e edit · d delete · D due · p prio · t tags · / search · s sort · tab collapse · ? help · q quit"))
	}
	return b.String()
}

func (a *App) renderTaskLine(t model.Task, selected bool) string {
	prefix := "  "
	if selected {
		prefix = a.pal.Primary.Render("▸ ")
	}
	check := checkbox(t.Done)
	prio := priorityLabel(t.Priority, a.pal)
	title := t.Title
	style := lipgloss.NewStyle()
	if t.Done {
		style = a.pal.Done
	}
	meta := ""
	if t.Due != nil {
		meta = a.pal.Muted.Render("  " + t.Due.Format(model.DateLayout))
	}
	if len(t.Tags) > 0 {
		sorted := append([]string(nil), t.Tags...)
		sort.Strings(sorted)
		meta += a.pal.Accent.Render("  #" + strings.Join(sorted, " #"))
	}
	return fmt.Sprintf("%s%s %s %s%s", prefix, check, prio, style.Render(title), meta)
}

func (a *App) helpView() string {
	rows := [][2]string{
		{"j/k", "move selection"},
		{"g/G", "jump top / bottom"},
		{"N", "jump to next-unblocked"},
		{"F", "focus pinned only (toggle)"},
		{"⏎/␣", "toggle done"},
		{"a", "add task"},
		{"e", "edit title"},
		{"t", "edit tags"},
		{"D", "set due date"},
		{"p", "cycle priority"},
		{"d", "delete (confirm)"},
		{"/", "fuzzy filter"},
		{"s", "sort: priority|due|created|id"},
		{"tab", "collapse current section"},
		{"r/R", "reload (R also clears filter)"},
		{"C", "clone current task"},
		{"X", "archive current (done only)"},
		{"?", "toggle help"},
		{"q", "quit"},
	}
	var b strings.Builder
	b.WriteString(a.pal.Section.Render("Help"))
	b.WriteByte('\n')
	for _, r := range rows {
		fmt.Fprintf(&b, "  %-5s  %s\n", r[0], r[1])
	}
	return b.String()
}
