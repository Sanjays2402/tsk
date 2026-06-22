package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
)

// newPauseCmd implements `tsk pause <id>`: a discoverable verb that
// pairs visually with `tsk start`. Semantically identical to
// `tsk stop`: clears the started: timestamp.
//
// Why a dedicated command rather than a cobra Alias on `stop`? Same
// reason `blocked` (alias of `depend --list`) and `reachable` (alias
// of `graph --reachable`) got top-level commands instead: a cobra
// Alias surfaces as `tsk stop pause` in help/man output, burying it.
// A top-level verb appears in `tsk --help` and gets its own shell
// completion entry.
//
// The pair start/pause/done reads more naturally than start/stop/done
// for users coming from time-tracker apps (Toggl, Harvest, RescueTime),
// where "stop" usually means "end the timer entirely" and "pause"
// means "I'm coming back to this". In tsk both verbs do the same
// thing (clear started:), but `pause` is the right name when the
// task is still on the active list and you intend to resume.
//
// Runtime delegates to runStartStop(false, nil) — the same body
// `tsk stop` uses — so semantics literally cannot drift between
// the two surfaces (positional id case).
//
// --all is the end-of-day-clear shortcut: pause every task currently
// in-progress at once. Without it the user has to type
// `tsk wip --json | jq '.[].id' | xargs tsk pause` which is hostile
// at the end of a long day. With --all the call is a single verb
// that resolves the wip set internally, in one Save, with the same
// "no in-progress tasks" message wip already prints when the set
// is empty (so the empty case isn't confusing).
//
// --all is mutually exclusive with positional ids: the whole point
// is "don't bother typing them out", and combining the two would
// hide a typo (e.g. `tsk pause --all 3` could plausibly mean
// "pause everything except 3" or "also pause 3 if not started";
// rejecting the combination forces the user to be explicit).
//
// --tag and --priority OPTIONALLY narrow --all to a curated subset
// of the in-progress set ("pause everything tagged work" rather
// than literally everything). Sister of `tsk start --all`'s
// REQUIRED filter, but optional here because the wip set is
// usually small and curated — "pause everything" is a sensible
// default for the end-of-day reset, where start --all has no such
// natural scope (every open task would flood the wip view).
// Filters compose as AND (intersection), matching start --all's
// semantics so the two verbs read symmetrically.
func newPauseCmd() *cobra.Command {
	var (
		all       bool
		pauseTag  string
		pausePrio string
		dryRun    bool
		asJSON    bool
	)
	cmd := &cobra.Command{
		Use:     "pause [<id>...]",
		Aliases: []string{"hold"},
		Short:   "Pause tasks (alias for `stop`; pairs visually with `start`)",
		Long: `Pause one or more tasks. Equivalent to ` + "`tsk stop`" + ` —
clears the started: timestamp — but named to pair visually with
` + "`tsk start`" + ` for users who think in start/pause/resume cycles
(time-tracker muscle memory).

Pair with:
  tsk start <id>       resume work (re-sets started:<now>)
  tsk done  <id>       finish (clears started, sets completed)
  tsk wip              list everything currently in-progress

Pass --all to pause every task currently in-progress at once
(end-of-day clear). Mutually exclusive with positional ids.

Pass --tag and/or --priority with --all to narrow the bulk-pause
to a curated subset (e.g. "pause everything tagged work"). Sister
of ` + "`tsk start --all`" + `'s filter — same AND semantics. Optional
here (the wip set is usually small and "pause everything" is a
sensible end-of-day default), unlike start --all where a filter is
required.

Pass --dry-run with --all to preview which tasks WOULD be paused
without actually clearing any started: timestamps. Writes nothing
to disk; the .bak chain stays untouched. Useful for previewing a
tag/priority filter before committing to the bulk-pause (sister of
` + "`tsk start --all --dry-run`" + `'s preview for the inverse verb).

Pass --json with --dry-run to emit a machine-readable preview
(stable schema) for scripted pipelines — same shape as start
--all --dry-run --json so both bulk verbs feed identical pipes.

Idempotent: pausing a non-started task is a no-op with a "no change"
message.

Examples:
  tsk pause 3
  tsk pause 3 5 7
  tsk hold 3                  # alias
  tsk pause --all             # pause everything currently in-progress
  tsk pause --all --tag work  # only pause in-progress tasks tagged work
  tsk pause --all --priority urgent  # only pause in-progress urgent tasks
  tsk pause --all --dry-run   # preview without clearing started:
  tsk pause --all --tag work --dry-run  # preview a curated subset
  tsk pause --all --dry-run --json | jq '.would_pause[].id'  # scripted
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				if len(args) > 0 {
					return usageErrorf("--all takes no positional ids (pause every in-progress task)")
				}
				return runPauseAll(cmd, pauseTag, pausePrio, dryRun, asJSON)
			}
			if pauseTag != "" || pausePrio != "" {
				return usageErrorf("--tag/--priority only apply to --all (single-id pause is already explicit)")
			}
			if dryRun {
				return usageErrorf("--dry-run only applies to --all (single-id pause is already explicit)")
			}
			if asJSON {
				return usageErrorf("--json only applies to --all --dry-run (the preview path)")
			}
			if len(args) == 0 {
				return usageErrorf("missing <id> (or pass --all)")
			}
			return runStartStop(false, nil)(cmd, args)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "pause every task currently in-progress (end-of-day clear)")
	cmd.Flags().StringVar(&pauseTag, "tag", "", "for --all: only pause in-progress tasks carrying this tag (case-insensitive)")
	cmd.Flags().StringVar(&pausePrio, "priority", "", "for --all: only pause in-progress tasks at this priority (low/medium/high/urgent)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "for --all: print which tasks would be paused without writing")
	cmd.Flags().BoolVar(&asJSON, "json", false, "for --all --dry-run: emit JSON preview (stable schema for scripted pipelines)")
	return cmd
}

// runPauseAll resolves the in-progress set inside the store and
// dispatches to runStartStop with those ids. Keeps the runStartStop
// body the single source of truth for the actual mutation (so any
// future enforcement, e.g. \"done tasks reject pause\", applies here
// too without a parallel guard).
//
// Empty in-progress set is reported with the same \"no in-progress
// tasks\" line `tsk wip` uses, so the empty case is consistent across
// the two verbs (the user sees the same answer whether they checked
// first or just ran pause --all).
//
// --tag / --priority narrow the wip set BEFORE the runStartStop
// dispatch. When neither is set, behavior is the original "every
// in-progress task" — backward compatible. When set, the filter
// shapes mirror start --all's: tag/priority compose as AND, the
// empty-result wording mirrors start --all's empty message ("no
// in-progress tasks match (<filter>)").
//
// --dry-run short-circuits BEFORE the runStartStop dispatch: it
// prints the would-be-paused ids and exits without writing. Critical
// invariant: the .bak chain is untouched (no Save called). Dry-run
// on an empty set reports the empty case the same way the non-dry
// path does — same wording so the two paths answer "what would this
// do?" identically. Sister of start --all --dry-run; same shape and
// guarantees for the inverse verb.
//
// --json (only meaningful with --dry-run) emits a stable schema with
// the would-pause list, total counts, and the filter summary. Sister
// of start --all --dry-run --json so the two bulk-verb previews feed
// identical jq pipelines.
func runPauseAll(cmd *cobra.Command, tag, prioRaw string, dryRun, asJSON bool) error {
	tag = strings.TrimSpace(tag)
	prio, prioActive, err := parsePendingPriority(prioRaw)
	if err != nil {
		return err
	}
	if asJSON && !dryRun {
		return usageErrorf("--json only applies to --all --dry-run (the preview path)")
	}
	s, err := resolveStore(cmd, true)
	if err != nil {
		return err
	}
	ids := filterPauseAllIDs(s.Tasks, tag, prio, prioActive)
	if len(ids) == 0 {
		// Two distinct empty cases:
		//   1. No tasks are in-progress at all — same "no in-progress
		//      tasks" wording wip uses (backward compat with the
		//      pre-filter version).
		//   2. Tasks ARE in-progress but none match the filter — say
		//      so explicitly with the filter summary so a typo is
		//      visible (sister of start --all's empty wording).
		anyWip := false
		for _, t := range s.Tasks {
			if t.IsInProgress() {
				anyWip = true
				break
			}
		}
		if dryRun && asJSON {
			return emitPauseAllDryRunJSON(cmd.OutOrStdout(), s, nil, tag, prioRaw, prioActive)
		}
		if !anyWip || (tag == "" && !prioActive) {
			pln(cmd.OutOrStdout(), "no in-progress tasks")
			return nil
		}
		filters := buildStartAllFilterSummary(tag, prioRaw, prioActive)
		pf(cmd.OutOrStdout(), "no in-progress tasks match (%s)\n", filters)
		return nil
	}
	if dryRun {
		if asJSON {
			return emitPauseAllDryRunJSON(cmd.OutOrStdout(), s, ids, tag, prioRaw, prioActive)
		}
		filters := buildStartAllFilterSummary(tag, prioRaw, prioActive)
		if filters == "" {
			pf(cmd.OutOrStdout(), "[dry-run] would pause %d task(s):\n", len(ids))
		} else {
			pf(cmd.OutOrStdout(), "[dry-run] would pause %d task(s) (%s):\n",
				len(ids), filters)
		}
		for _, id := range ids {
			t := s.ByID(id)
			title := ""
			if t != nil {
				title = "  " + t.Title
			}
			pf(cmd.OutOrStdout(), "  #%d%s\n", id, title)
		}
		return nil
	}
	args := make([]string, len(ids))
	for i, id := range ids {
		args[i] = fmt.Sprintf("%d", id)
	}
	return runStartStop(false, nil)(cmd, args)
}

// pauseAllDryRunRow is the per-task entry in the JSON preview. Stable
// schema: id + title + filter-matched status flag. Mirrors the shape
// of startAllDryRunRow so the two bulk-verb previews can share jq
// pipelines.
type pauseAllDryRunRow struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

// pauseAllDryRunDoc is the top-level JSON envelope. Stable shape:
// would_pause[] (per-task rows) + counts + filter summary. Empty
// result emits would_pause: [] (not null) so consumers iterating
// the array don't crash.
type pauseAllDryRunDoc struct {
	WouldPause []pauseAllDryRunRow `json:"would_pause"`
	TotalCount int                 `json:"total_count"`
	Filter     string              `json:"filter,omitempty"`
	Tag        string              `json:"tag,omitempty"`
	Priority   string              `json:"priority,omitempty"`
}

// emitPauseAllDryRunJSON renders the stable preview shape for the
// pause --all --dry-run --json path. Empty would_pause is rendered
// as an empty array, not null, so jq pipelines that iterate don't
// crash on a no-match case. Filter fields are omitted when not set
// so the JSON stays minimal for the bare-pause-all preview.
func emitPauseAllDryRunJSON(w io.Writer, s *store.Store, ids []int, tag, prioRaw string, prioActive bool) error {
	rows := make([]pauseAllDryRunRow, 0, len(ids))
	for _, id := range ids {
		t := s.ByID(id)
		title := ""
		if t != nil {
			title = t.Title
		}
		rows = append(rows, pauseAllDryRunRow{ID: id, Title: title})
	}
	doc := pauseAllDryRunDoc{
		WouldPause: rows,
		TotalCount: len(rows),
		Filter:     buildStartAllFilterSummary(tag, prioRaw, prioActive),
		Tag:        tag,
	}
	if prioActive {
		doc.Priority = strings.ToLower(strings.TrimSpace(prioRaw))
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

// filterPauseAllIDs returns the sorted-ascending ids of every IN-
// PROGRESS task matching the filter. When both tag and prioActive
// are empty/false, this is equivalent to inProgressIDs (every wip
// task) — backward compatible with the pre-filter behavior.
func filterPauseAllIDs(tasks []model.Task, tag string, prio model.Priority, prioActive bool) []int {
	ids := make([]int, 0)
	for _, t := range tasks {
		if !t.IsInProgress() {
			continue
		}
		if tag != "" && !t.HasTag(tag) {
			continue
		}
		if prioActive && t.Priority != prio {
			continue
		}
		ids = append(ids, t.ID)
	}
	sort.Ints(ids)
	return ids
}

// inProgressIDs returns the sorted-ascending ids of every task with
// IsInProgress() true. Sorted for reproducible output ordering in
// runStartStop's summary line and to make tests deterministic.
func inProgressIDs(tasks []model.Task) []int {
	ids := make([]int, 0)
	for _, t := range tasks {
		if t.IsInProgress() {
			ids = append(ids, t.ID)
		}
	}
	sort.Ints(ids)
	return ids
}
