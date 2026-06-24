package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
)

// newStartCmd implements `tsk start <id>`: mark a task as in-progress
// by stamping `started:<now>` into its meta. Pairs with `tsk stop`
// (clear) and `tsk done` (which auto-clears it on completion).
//
// In-progress is a NEW state — it sits between "open" and "done":
//
//	open      → start → in-progress → done    (normal flow)
//	open      → done                          (didn't track work)
//	in-progress → stop → open                 (paused, will resume later)
//	in-progress → done                        (finished; started: dropped
//	                                           because Completed wins)
//
// The persisted form is `started:<RFC3339>` in the existing meta
// comment — strictly additive, the way `pin:true` and `wait:<date>`
// were. Old files round-trip unchanged. Tasks with a `started:` meta
// from a hand-edit also work (just like pin/wait).
//
// Multi-id: `tsk start 3 5 7` flips all three on in a single Save.
// Idempotent: starting an already-started task is a no-op with a
// "no change" message, NOT an error (matches pin/freeze conventions).
//
// One subtle interaction worth flagging:
//
//   - Restarting an already-started task: `tsk start --reset` lets the
//     user explicitly bump the started: timestamp to NOW. Without
//     --reset the existing start time is preserved (so "I forgot I
//     had started this an hour ago" doesn't accidentally zero the
//     elapsed time).
//
// `tsk done` clears Started on completion (Completed is the more
// useful timestamp at that point — the task moved past in-progress).
//
// --all is the bulk-start sibling of `tsk pause --all`. It REQUIRES
// a scope flag (--tag or --priority) because "start every open
// task in the store" is almost never what a user wants — that
// would flood `tsk wip` with dozens of irrelevant rows. Forcing
// a filter is the only way to keep the verb honest:
//
//	tsk start --all --tag standup     # start every open standup task
//	tsk start --all --priority urgent # start every open urgent task
//	tsk start --all --tag x --priority high  # the AND of both
//
// Done tasks are excluded (start/done is meaningless once a task
// is done — same guard runStartStop enforces per-id). Already-
// started tasks are silently skipped (idempotent, matching the
// per-id contract). Empty result set ("no open tasks match")
// is a clean no-op with a clear message, NOT an error — a typo
// in --tag should surface as "no matches", not as a non-zero
// exit; the user knows quickly to fix the filter.
//
// Why a required scope rather than allowing `tsk start --all`
// to mean "literally everything"? The pause sister has the
// natural scope of "every wip task", which is usually a small
// set the user just curated themselves. start --all has no
// such natural scope — the "every open task" interpretation
// would start dozens of items the user has no current context
// for, which is the opposite of what start: is for. Requiring
// a filter forces the verb to mean what it should: "start a
// curated subset I'm about to focus on".
func newStartCmd() *cobra.Command {
	var (
		reset             bool
		all               bool
		startTag          string
		startStrictAndTag string
		startPrio         string
		dryRun            bool
		asJSON            bool
	)
	cmd := &cobra.Command{
		Use:   "start [<id>...]",
		Short: "Mark tasks as in-progress (stamp started:<now>)",
		Long: `Mark one or more tasks as in-progress. Stamps started:<now> into
the task's meta block, persisted alongside other metadata.

The "in-progress" state sits between open and done. Pair with:
  tsk stop <id>       pause work (clear started:)
  tsk done <id>       finish (Completed wins; started: is dropped)
  tsk show <id>       see started: in the detail view
  tsk in-progress     list everything currently being worked on

By default, starting an already-started task is a no-op so you can
re-run safely. Pass --reset to bump started: to right now.

Pass --all with --tag and/or --priority to start every open task
matching the filter. Sister of ` + "`tsk pause --all`" + `: bulk-start
a curated subset rather than typing ids one at a time. Requires
at least one filter (no "start literally every open task" form).

Pass --strict-and-tag <CSV> to narrow --all by INTERSECTION of
multiple tags (the all-of variant). Sister of --tag's union-style
single-tag filter: --tag work narrows by ONE tag,
--strict-and-tag work,p0 narrows by tasks carrying BOTH 'work'
AND 'p0'. Mutually exclusive with --tag (each is a different
selector axis). Composes with --priority as AND. Mirrors
` + "`tsk pause --all --strict-and-tag`" + ` and
` + "`tsk depend --pending --strict-and-tag`" + ` so the three tag-
axis intersection filters read symmetrically across the verbs.

Pass --dry-run with --all to preview which tasks WOULD be started
without actually flipping any state. Writes nothing to disk; the
.bak chain stays untouched. Useful for previewing a tag/priority
filter before committing to the bulk-start ("does this match what
I think it matches?").

Pass --json with --dry-run to emit a machine-readable preview
(stable schema) for scripted pipelines — same shape as pause
--all --dry-run --json so both bulk verbs feed identical pipes.

Examples:
  tsk start 3
  tsk start 3 5 7                          # several at once
  tsk start 3 --reset                      # bump started: even if already started
  tsk start --all --tag standup            # start every open standup task
  tsk start --all --priority urgent        # start every open urgent task
  tsk start --all --tag work --priority high
  tsk start --all --strict-and-tag work,p0 # intersection: tasks carrying BOTH tags
  tsk start --all --tag standup --dry-run  # preview without stamping
  tsk start --all --tag standup --dry-run --json | jq '.would_start[].id'
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if all {
				if len(args) > 0 {
					return usageErrorf("--all takes no positional ids (resolves the filtered set internally)")
				}
				return runStartAll(cmd, startTag, startStrictAndTag, startPrio, reset, dryRun, asJSON)
			}
			if dryRun {
				return usageErrorf("--dry-run only applies to --all (single-id start is already explicit)")
			}
			if asJSON {
				return usageErrorf("--json only applies to --all --dry-run (the preview path)")
			}
			if startTag != "" || startStrictAndTag != "" || startPrio != "" {
				return usageErrorf("--tag/--strict-and-tag/--priority only apply to --all (single-id start is already explicit)")
			}
			if len(args) == 0 {
				return usageErrorf("missing <id> (or pass --all with --tag/--strict-and-tag/--priority)")
			}
			return runStartStop(true, &reset)(cmd, args)
		},
	}
	cmd.Flags().BoolVar(&reset, "reset", false, "bump started: to now even if the task is already started")
	cmd.Flags().BoolVar(&all, "all", false, "start every open task matching --tag, --strict-and-tag, and/or --priority")
	cmd.Flags().StringVar(&startTag, "tag", "", "for --all: only start tasks carrying this tag (case-insensitive)")
	cmd.Flags().StringVar(&startStrictAndTag, "strict-and-tag", "", "for --all: only start open tasks carrying ALL listed tags (CSV; intersection). Sister of --tag's union-style single-tag filter: --tag work narrows to tasks carrying 'work'; --strict-and-tag work,p0 narrows to tasks carrying BOTH 'work' AND 'p0'. Mutually exclusive with --tag (each is a different selector axis). Composes with --priority as AND. Mirrors `tsk pause --all --strict-and-tag` and `tsk depend --pending --strict-and-tag` so the three bulk-verb tag-axis intersection filters read symmetrically.")
	cmd.Flags().StringVar(&startPrio, "priority", "", "for --all: only start tasks at this priority (low/medium/high/urgent)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "for --all: print which tasks would be started without writing")
	cmd.Flags().BoolVar(&asJSON, "json", false, "for --all --dry-run: emit JSON preview (stable schema for scripted pipelines)")
	return cmd
}

// runStartAll resolves the open-task set matching the requested
// filter scope and dispatches to runStartStop for the actual
// transition. Keeps the runStartStop body the single source of
// truth for how a start happens; this function is purely the
// id-resolution + dispatch layer.
//
// The filter scope shape mirrors pause --all (added in tick #25)
// and the depend --pending notification queue, so the three bulk-
// action verbs surface the same selector axes and read identically.
// Required: at least one of --tag, --strict-and-tag, or --priority
// must be set; see the function-level newStartCmd doc comment
// for why. The empty-set case ("no open tasks match") is a clean
// no-op so a typo in --tag exits 0 with a clear message rather
// than firing a non-zero exit that could trip a wrapper script.
//
// --strict-and-tag is the CSV intersection-style sister of --tag's
// union-style single-tag filter: --tag work narrows by ONE tag,
// --strict-and-tag work,p0 narrows by ALL listed tags
// (intersection). The two are mutually exclusive (each is a
// different selector axis). Mirrors the same flag
// `tsk pause --all --strict-and-tag` and `tsk depend --pending
// --strict-and-tag` expose, so the three bulk-verb tag-axis
// intersection filters read symmetrically.
//
// --dry-run short-circuits BEFORE the runStartStop dispatch: it
// prints the would-be-started ids and exits without writing.
// Critical invariant: the .bak chain is untouched (no Save called).
// Dry-run on an empty filter result reports the empty case the
// same way the non-dry path does — same wording so the two paths
// answer the "what would this do?" question identically.
//
// --json (only meaningful with --dry-run) emits a stable schema
// (would_start[], total_count, filter, tag, strict_and_tag,
// priority, reset) so scripted pipelines can pluck ids without
// parsing the human preview. Sister of pause --all --dry-run --json
// — same envelope shape so the two bulk-verb previews share jq
// pipelines.
func runStartAll(cmd *cobra.Command, tag, strictAndTagsRaw, prioRaw string, reset, dryRun, asJSON bool) error {
	tag = strings.TrimSpace(tag)
	prio, prioActive, err := parsePendingPriority(prioRaw)
	if err != nil {
		return err
	}
	strictAndTags := splitTagCSV(strictAndTagsRaw)
	if tag != "" && len(strictAndTags) > 0 {
		return usageErrorf("--tag and --strict-and-tag are mutually exclusive (each is a different tag-selector axis; --tag is single-tag, --strict-and-tag is intersection over a CSV)")
	}
	if tag == "" && len(strictAndTags) == 0 && !prioActive {
		return usageErrorf("--all requires --tag, --strict-and-tag, and/or --priority (refusing to start every open task in the store)")
	}
	if asJSON && !dryRun {
		return usageErrorf("--json only applies to --all --dry-run (the preview path)")
	}
	s, err := resolveStore(cmd, true)
	if err != nil {
		return err
	}
	ids := filterStartAllIDs(s.Tasks, tag, strictAndTags, prio, prioActive)
	if len(ids) == 0 {
		if dryRun && asJSON {
			return emitStartAllDryRunJSON(cmd.OutOrStdout(), s, nil, tag, strictAndTagsRaw, prioRaw, prioActive, reset)
		}
		filters := buildStartAllFilterSummary(tag, strictAndTagsRaw, prioRaw, prioActive)
		pf(cmd.OutOrStdout(), "no open tasks match (%s)\n", filters)
		return nil
	}
	if dryRun {
		// Compute the would-flip ids: in the dry-run path we
		// further partition into "would-actually-start" (Started
		// is nil OR reset is set) so the preview matches what
		// runStartStop would actually do. This mirrors the
		// idempotent-skip semantics so the preview reads truthfully
		// instead of just listing the filter-matched set.
		wouldStart := make([]int, 0, len(ids))
		for _, id := range ids {
			t := s.ByID(id)
			if t == nil {
				continue
			}
			if t.Started == nil || reset {
				wouldStart = append(wouldStart, id)
			}
		}
		if asJSON {
			return emitStartAllDryRunJSON(cmd.OutOrStdout(), s, wouldStart, tag, strictAndTagsRaw, prioRaw, prioActive, reset)
		}
		filters := buildStartAllFilterSummary(tag, strictAndTagsRaw, prioRaw, prioActive)
		if len(wouldStart) == 0 {
			pf(cmd.OutOrStdout(), "[dry-run] no tasks would be started (%d matched %s but all are already in-progress)\n",
				len(ids), filters)
			return nil
		}
		pf(cmd.OutOrStdout(), "[dry-run] would start %d task(s) (%s):\n",
			len(wouldStart), filters)
		for _, id := range wouldStart {
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
	return runStartStop(true, &reset)(cmd, args)
}

// startAllDryRunRow is the per-task entry in the JSON preview.
// Stable schema: id + title. Mirrors pauseAllDryRunRow so the
// two bulk-verb previews can share jq pipelines.
type startAllDryRunRow struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
}

// startAllDryRunDoc is the top-level JSON envelope. Stable shape:
// would_start[] (per-task rows) + counts + filter summary + reset
// flag. Empty result emits would_start: [] (not null) so consumers
// iterating the array don't crash. The reset field is exposed so
// scripted pipelines can branch on "this preview reflects --reset
// semantics" vs the default skip-already-started behavior.
//
// StrictAndTag is the CSV intersection-style sister of Tag (when
// used). When both are empty the field omits entirely; when set
// it serializes as the raw CSV string the user passed so the JSON
// preview echoes back exactly what filter they asked for. Mirrors
// pauseAllDryRunDoc.StrictAndTag so the two bulk-verb previews
// share the same surface.
type startAllDryRunDoc struct {
	WouldStart   []startAllDryRunRow `json:"would_start"`
	TotalCount   int                 `json:"total_count"`
	Filter       string              `json:"filter,omitempty"`
	Tag          string              `json:"tag,omitempty"`
	StrictAndTag string              `json:"strict_and_tag,omitempty"`
	Priority     string              `json:"priority,omitempty"`
	Reset        bool                `json:"reset"`
}

// emitStartAllDryRunJSON renders the stable preview shape for the
// start --all --dry-run --json path. Empty would_start is rendered
// as an empty array, not null, so jq pipelines that iterate don't
// crash on a no-match case. Filter fields are omitted when not set
// so the JSON stays minimal; reset is always emitted because false
// is the meaningful default a script needs to see.
//
// strictAndTagsRaw is the user-supplied CSV (already-trimmed) for
// the intersection filter. Empty means the filter isn't in use;
// the field is omitempty so the JSON stays minimal in that case.
// When set, the field surfaces in BOTH the structured
// "strict_and_tag" key and the human-readable "filter" summary
// (rendered as "tag=a&b") so scripted pipelines have both axes.
func emitStartAllDryRunJSON(w io.Writer, s *store.Store, ids []int, tag, strictAndTagsRaw, prioRaw string, prioActive, reset bool) error {
	rows := make([]startAllDryRunRow, 0, len(ids))
	for _, id := range ids {
		t := s.ByID(id)
		title := ""
		if t != nil {
			title = t.Title
		}
		rows = append(rows, startAllDryRunRow{ID: id, Title: title})
	}
	doc := startAllDryRunDoc{
		WouldStart:   rows,
		TotalCount:   len(rows),
		Filter:       buildStartAllFilterSummary(tag, strictAndTagsRaw, prioRaw, prioActive),
		Tag:          tag,
		StrictAndTag: strictAndTagsRaw,
		Reset:        reset,
	}
	if prioActive {
		doc.Priority = strings.ToLower(strings.TrimSpace(prioRaw))
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}

// filterStartAllIDs returns the sorted-ascending ids of every OPEN
// task matching the filter. Done tasks are excluded (start/done
// is meaningless); already-started tasks STAY in the set so the
// idempotent-skip in runStartStop covers them with the standard
// "no change" message (no special-casing here).
//
// strictAndTags is the CSV intersection filter: when non-empty, a
// task must carry ALL listed tags to qualify (taskHasAllTags
// short-circuit). Mutually exclusive with tag (the caller already
// validated this; the function just applies whichever is set).
func filterStartAllIDs(tasks []model.Task, tag string, strictAndTags []string, prio model.Priority, prioActive bool) []int {
	ids := make([]int, 0)
	for _, t := range tasks {
		if t.Done {
			continue
		}
		if tag != "" && !t.HasTag(tag) {
			continue
		}
		if len(strictAndTags) > 0 && !taskHasAllTags(&t, strictAndTags) {
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

// buildStartAllFilterSummary mirrors the depend --pending filter
// summary shape so the two bulk-action verbs read the same way
// when they print an empty-result line. Deterministic ordering:
// tag first, then strict-and-tag, then priority.
//
// strict-and-tag renders as "tag=a&b" (the &-separated form) so a
// scan-by-eye distinguishes "tag=a" (union, single) from
// "tag=a&b" (intersection, CSV) without checking the original
// flag name — same disambiguation marker `tsk pause
// --all --strict-and-tag` and `tsk depend --pending
// --strict-and-tag` use, so the surfaces read symmetrically.
func buildStartAllFilterSummary(tag, strictAndTagsRaw, prioRaw string, prioActive bool) string {
	parts := make([]string, 0, 3)
	if tag != "" {
		parts = append(parts, "tag="+tag)
	}
	if strict := splitTagCSV(strictAndTagsRaw); len(strict) > 0 {
		parts = append(parts, "tag="+strings.Join(strict, "&"))
	}
	if prioActive {
		parts = append(parts, "priority="+strings.ToLower(strings.TrimSpace(prioRaw)))
	}
	return strings.Join(parts, ", ")
}

// newStopCmd implements `tsk stop <id>`: clear the started: timestamp.
// Pure inverse of start, with the same multi-id + idempotent shape.
func newStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <id>...",
		Short: "Mark tasks as no longer in-progress (clear started:)",
		Long: `Mark one or more tasks as no longer in-progress. Clears the
started: meta key.

Use this when you've put a task down and don't expect to pick it up
again soon. (For "done", use 'tsk done'; for "later", 'tsk snooze'
or 'tsk wait'.)

Examples:
  tsk stop 3
  tsk stop 3 5 7
`,
		Args: cobra.MinimumNArgs(1),
		RunE: runStartStop(false, nil),
	}
}

// runStartStop is the shared body of start/stop. Mirrors the
// pin/unpin / freeze/thaw idempotency contract: every requested id
// must exist (validated up-front so no partial state lands), then
// flip whatever's different. Reports "no change" when nothing
// happened.
func runStartStop(starting bool, reset *bool) func(*cobra.Command, []string) error {
	verb := "started"
	if !starting {
		verb = "stopped"
	}
	return func(cmd *cobra.Command, args []string) error {
		ids, err := parseTaskIDs(args)
		if err != nil {
			return err
		}
		s, err := resolveStore(cmd, true)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if s.ByID(id) == nil {
				return fmt.Errorf("no task with id %d in %s", id, s.Path)
			}
		}
		now := time.Now()
		changed := 0
		for _, id := range ids {
			t := s.ByID(id)
			if t.Done {
				// Done tasks aren't in-progress candidates; surfacing
				// the conflict here is more helpful than silently
				// flipping a meta key on a completed task.
				return usageErrorf("#%d is already done — start/stop don't apply (use `tsk reopen %d` first)", id, id)
			}
			if starting {
				if t.Started != nil && (reset == nil || !*reset) {
					continue
				}
				ts := now
				t.Started = &ts
				changed++
				continue
			}
			// stop
			if t.Started == nil {
				continue
			}
			t.Started = nil
			changed++
		}
		if changed == 0 {
			pf(cmd.OutOrStdout(), "no change (%d already %s)\n", len(ids), verb)
			return nil
		}
		if err := s.Save(); err != nil {
			return err
		}
		pf(cmd.OutOrStdout(), "%s %d task(s)\n", verb, changed)
		return nil
	}
}

// newInProgressCmd implements `tsk in-progress`: list every task
// currently being worked on (Started != nil && !Done), sorted by
// most-recent start first. Designed for "what am I in the middle
// of?" answers. Aliased `wip` for muscle memory.
//
// --stale <duration> narrows the list to tasks whose elapsed time
// is GREATER than the threshold (e.g. --stale 24h surfaces only
// the tasks that have been sitting in-progress for over a day).
// The "I've been working on this too long" alert mode. Reuses the
// same duration parser as `tsk log --since`/`tsk depend --pending
// --since` (7d, 2w, 1h30m, 72h, etc.). Composes with --json so
// scripted alerts can flag stale WIP without parsing humanized
// strings. Zero/negative duration is a usage error — the threshold
// MUST be positive to define a "stale-er than this" filter.
//
// --priority <p> narrows the list to in-progress tasks at exactly
// the named priority (low/medium/high/urgent, short forms accepted
// via model.ParsePriority). Sister of --stale on the filtering
// axis — both narrow what surfaces in the WIP list without
// changing the underlying state. Mirrors `tsk depend --pending
// --priority` and `tsk start --all --priority` for symmetry across
// the verbs that already accept a priority filter.
//
// --tag <t> narrows the list to in-progress tasks carrying the
// named tag (case-insensitive, single tag — same semantics
// `tsk ls --tag` and `tsk depend --pending --tag` use).
// Sister of --priority on the filtering axis.
//
// --strict-and-tag <CSV> narrows the list to in-progress tasks
// carrying ALL listed tags (intersection-style; sister of --tag's
// union-style single-tag filter — --tag work narrows to one tag,
// --strict-and-tag work,p0 narrows to tasks carrying BOTH). Mirrors
// the same flag on `tsk start --all`, `tsk pause --all`, and
// `tsk depend --pending` so the four bulk-verb-adjacent tag-axis
// intersection filters read symmetrically across the verbs.
// Mutually exclusive with --tag (each is a different selector axis).
// All filters compose as AND: --stale 24h --priority urgent --tag
// work narrows the list to in-progress tasks running over a day,
// at exactly urgent priority, and carrying the 'work' tag.
func newInProgressCmd() *cobra.Command {
	var (
		asJSON          bool
		staleRaw        string
		wipPrio         string
		wipTag          string
		wipStrictAndTag string
	)
	cmd := &cobra.Command{
		Use:     "in-progress",
		Aliases: []string{"wip", "inprogress"},
		Short:   "List tasks currently in-progress (started: set, not done)",
		Long: `List every task currently marked in-progress. Sorted by most-recent
started: timestamp first, with the elapsed time shown so you can see
which tasks are getting stale.

Pass --stale <duration> to narrow the list to tasks running LONGER
than the threshold — the "what's been sitting too long?" alert. The
elapsed-time filter compares against (now - started:); anything at
or below the threshold is dropped, only the stale-er rows surface.
Useful in cron / pre-commit / standup loops to surface neglected
work without manually scanning the full WIP list:

  tsk wip --stale 24h          # only WIP running over a day
  tsk wip --stale 4h --json    # scripted alert for half-day stale

Pass --priority <p> to narrow to in-progress tasks at exactly the
named priority (low/medium/high/urgent). Sister of --stale on the
filtering axis. Pass --tag <t> for a single-tag filter, or
--strict-and-tag <a,b> for a multi-tag intersection (all-of). Tag
filters mirror the same flags on ` + "`tsk pause --all`" + `,
` + "`tsk start --all`" + `, and ` + "`tsk depend --pending`" + ` so
the four verb surfaces read symmetrically. All filters compose as
AND.

Examples:
  tsk in-progress
  tsk wip                       # alias
  tsk in-progress --json
  tsk wip --stale 1d            # only tasks running > 1 day
  tsk wip --stale 4h --json     # scripted half-day stale alert
  tsk wip --priority urgent     # only urgent in-progress
  tsk wip --tag work            # only WIP tagged 'work'
  tsk wip --strict-and-tag work,p0  # WIP carrying BOTH tags
  tsk wip --stale 24h --priority urgent --tag work  # composed AND
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// --stale validation up-front so a typo doesn't waste
			// the store-load cost. Empty value means no filter
			// (defensive against an unset shell variable).
			var staleDur time.Duration
			staleActive := false
			if strings.TrimSpace(staleRaw) != "" {
				d, err := parseDurationLocal(strings.TrimSpace(staleRaw))
				if err != nil {
					return usageErrorf("invalid --stale %q: %v", staleRaw, err)
				}
				if d <= 0 {
					return usageErrorf("--stale must be a positive duration, got %q", staleRaw)
				}
				staleDur = d
				staleActive = true
			}
			// --priority parsing: empty = no filter (defensive
			// against unset shell vars; matches depend --pending's
			// stance). Reuses model.ParsePriority for case-
			// insensitive short/long-form acceptance ("u" / "urgent"
			// both resolve to Urgent).
			wipPrioTrim := strings.TrimSpace(wipPrio)
			var prio model.Priority
			prioActive := false
			if wipPrioTrim != "" {
				p, err := model.ParsePriority(wipPrioTrim)
				if err != nil {
					return usageErrorf("invalid --priority %q: %v", wipPrio, err)
				}
				prio = p
				prioActive = true
			}
			// --tag + --strict-and-tag tag-axis selectors. They are
			// mutually exclusive (each is a different filter shape:
			// single-tag union vs multi-tag intersection), matching
			// the rejection contract `tsk start --all` /
			// `tsk pause --all` / `tsk depend --pending` use.
			wipTagTrim := strings.TrimSpace(wipTag)
			strictAndTags := splitTagCSV(wipStrictAndTag)
			if wipTagTrim != "" && len(strictAndTags) > 0 {
				return usageErrorf("--tag and --strict-and-tag are mutually exclusive (each is a different tag-selector axis; --tag is single-tag, --strict-and-tag is intersection over a CSV)")
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			now := time.Now()
			cutoff := now.Add(-staleDur) // tasks with Started.Before(cutoff) are "stale"
			out := make([]model.Task, 0)
			for _, t := range s.Tasks {
				if !t.IsInProgress() {
					continue
				}
				if staleActive && !t.Started.Before(cutoff) {
					// The task started AT or AFTER the cutoff →
					// elapsed is <= threshold → not stale enough.
					continue
				}
				if prioActive && t.Priority != prio {
					continue
				}
				if wipTagTrim != "" && !t.HasTag(wipTagTrim) {
					continue
				}
				if len(strictAndTags) > 0 && !taskHasAllTags(&t, strictAndTags) {
					continue
				}
				out = append(out, t)
			}
			sort.SliceStable(out, func(i, j int) bool {
				return out[i].Started.After(*out[j].Started)
			})
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if out == nil {
					out = []model.Task{}
				}
				return enc.Encode(out)
			}
			if len(out) == 0 {
				switch {
				case staleActive && prioActive:
					pf(cmd.OutOrStdout(), "no in-progress tasks running longer than %s at priority %s\n", humanizeDuration(staleDur), prio.String())
				case staleActive:
					pf(cmd.OutOrStdout(), "no in-progress tasks running longer than %s\n", humanizeDuration(staleDur))
				case prioActive:
					pf(cmd.OutOrStdout(), "no in-progress tasks at priority %s\n", prio.String())
				case wipTagTrim != "":
					pf(cmd.OutOrStdout(), "no in-progress tasks with tag %s\n", wipTagTrim)
				case len(strictAndTags) > 0:
					pf(cmd.OutOrStdout(), "no in-progress tasks with tags %s\n", strings.Join(strictAndTags, "&"))
				default:
					pln(cmd.OutOrStdout(), "no in-progress tasks")
				}
				return nil
			}
			filters := buildWipFilterSummary(staleActive, staleDur, prioActive, prio, wipTagTrim, strictAndTags)
			if filters != "" {
				pf(cmd.OutOrStdout(), "in-progress (filter: %s): %d task(s)\n", filters, len(out))
			}
			for _, t := range out {
				elapsed := humanizeElapsed(now.Sub(*t.Started))
				pf(cmd.OutOrStdout(), "#%d [%s] %s  (started %s ago)\n",
					t.ID, t.Priority.Short(), t.Title, elapsed)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON array of in-progress tasks")
	cmd.Flags().StringVar(&staleRaw, "stale", "", "filter to tasks running LONGER than this duration (e.g. 24h, 2d, 1w, 1h30m). Same duration parser as `tsk log --since` and `tsk depend --pending --since`. The 'I've been working on this too long' alert mode: pair with `--json` for scripted standup/cron-driven stale-WIP notifications without parsing humanized strings. Composes with `--json` for machine-readable output. Empty (default) = no filter (every in-progress task is listed).")
	cmd.Flags().StringVar(&wipPrio, "priority", "", "filter to in-progress tasks at exactly this priority (low/medium/high/urgent, short forms accepted). Sister of --stale on the filtering axis. Mirrors the --priority filter on `tsk depend --pending` / `tsk start --all` / `tsk pause --all`. Composes with --stale, --tag, and --strict-and-tag as AND. Empty (default) = no filter.")
	cmd.Flags().StringVar(&wipTag, "tag", "", "filter to in-progress tasks carrying this tag (case-insensitive, single tag). Sister of --priority. Mirrors `tsk depend --pending --tag` / `tsk ls --tag`. Mutually exclusive with --strict-and-tag (each is a different tag-selector axis). Composes with --stale and --priority as AND. Empty (default) = no filter.")
	cmd.Flags().StringVar(&wipStrictAndTag, "strict-and-tag", "", "filter to in-progress tasks carrying ALL listed tags (CSV; intersection). Sister of --tag's union-style single-tag filter: --tag work narrows to tasks carrying 'work'; --strict-and-tag work,p0 narrows to tasks carrying BOTH 'work' AND 'p0'. Mutually exclusive with --tag. Mirrors `tsk pause --all --strict-and-tag`, `tsk start --all --strict-and-tag`, and `tsk depend --pending --strict-and-tag` so the four tag-axis intersection filters read symmetrically. Composes with --stale and --priority as AND.")
	return cmd
}

// buildWipFilterSummary renders a single-line filter summary for the
// wip header line ("in-progress (filter: ...): N task(s)"). Sister
// of buildPendingFilterSummary / buildPauseAllFilterSummary so the
// three verb surfaces produce a recognizable summary shape. Order is
// stable: stale, then tag/strict-and-tag (whichever is active), then
// priority — same ordering depend --pending and pause --all use.
func buildWipFilterSummary(staleActive bool, staleDur time.Duration, prioActive bool, prio model.Priority, tag string, strictAndTags []string) string {
	parts := make([]string, 0, 4)
	if staleActive {
		parts = append(parts, "stale>"+humanizeDuration(staleDur))
	}
	if tag != "" {
		parts = append(parts, "tag="+tag)
	}
	if len(strictAndTags) > 0 {
		parts = append(parts, "tag="+strings.Join(strictAndTags, "&"))
	}
	if prioActive {
		parts = append(parts, "priority="+prio.String())
	}
	return strings.Join(parts, " ")
}

// humanizeElapsed renders a duration as the largest non-zero unit
// (NNNd, NNNh, NNNm). Anything under a minute reads "<1m" because
// "0m ago" looks broken. Used only for the in-progress display;
// kept local so it doesn't grow into a util pile.
func humanizeElapsed(d time.Duration) string {
	if d < time.Minute {
		return "<1m"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
