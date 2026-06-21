package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
)

// newElapsedCmd implements `tsk elapsed [id]`: show "started Nm/h/d ago"
// for one or every in-progress task. The script-friendly cousin of
// `tsk in-progress`.
//
// Why this and not just `tsk in-progress`?
//
//   - in-progress is the list view: id, priority, title, humanized "ago"
//   - elapsed is the focus view: target one task OR get raw seconds for
//     scripts ("how long has this been open?" pipelines)
//
// Two modes:
//
//   - `tsk elapsed`      — list every in-progress task with its elapsed
//     time, oldest-start first (the opposite of
//     in-progress, which leads with most-recent —
//     because for "what's been sitting?" the
//     stale tasks are the interesting ones)
//   - `tsk elapsed <id>` — single task; errors if it's not in-progress
//
// `--json` emits a stable schema with elapsed_seconds so consumers can
// build alerts ("anything in-progress over 24h?") without parsing the
// humanized string.
//
// Done tasks are an error in the positional form (they aren't in-progress
// by definition) — surfacing the conflict beats silently returning a
// past Completed timestamp the user didn't ask for.
func newElapsedCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "elapsed [id]",
		Short: "Show how long an in-progress task has been started",
		Long: `Show how long a task (or every in-progress task) has been started.

The list form (no id) is the staleness view — sorted OLDEST-start first
so the tasks that have been sitting in-progress longest float to the
top. Pair with 'tsk stop <id>' or 'tsk done <id>' to clear them.

With --json you get elapsed_seconds for each entry — useful for shell
pipelines that want to alert on stale work.

Examples:
  tsk elapsed                        # every in-progress task, stalest first
  tsk elapsed 3                      # just task 3
  tsk elapsed --json                 # stable JSON for scripts
  tsk elapsed --json | jq -r '.[] | select(.elapsed_seconds > 86400) | .id'
`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			now := time.Now()
			if len(args) == 1 {
				id, err := parseSingleID(args[0])
				if err != nil {
					return err
				}
				return emitElapsedSingle(cmd.OutOrStdout(), s.ByID(id), id, s.Path, now, asJSON)
			}
			return emitElapsedAll(cmd.OutOrStdout(), s.Tasks, now, asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit stable JSON with elapsed_seconds")
	return cmd
}

// elapsedEntry is the per-task report shape used for both plain and
// JSON output. ElapsedSeconds is the int the user actually wants in a
// shell pipeline (no string parsing); Elapsed is the human form for
// the plain view.
type elapsedEntry struct {
	ID             int    `json:"id"`
	Title          string `json:"title"`
	StartedAt      string `json:"started_at"` // RFC3339
	ElapsedSeconds int64  `json:"elapsed_seconds"`
	Elapsed        string `json:"elapsed"` // humanized
}

// emitElapsedSingle handles `tsk elapsed <id>`. Errors if the task
// doesn't exist or isn't in-progress (a done or never-started task
// has no \"elapsed\" answer).
func emitElapsedSingle(w io.Writer, t *model.Task, id int, path string, now time.Time, asJSON bool) error {
	if t == nil {
		return fmt.Errorf("no task with id %d in %s", id, path)
	}
	if !t.IsInProgress() {
		return usageErrorf("#%d is not in-progress (run `tsk start %d` first)", id, id)
	}
	entry := newElapsedEntry(*t, now)
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(entry)
	}
	pf(w, "#%d  %s\n", entry.ID, entry.Title)
	pf(w, "  started:  %s\n", entry.StartedAt)
	pf(w, "  elapsed:  %s (%ds)\n", entry.Elapsed, entry.ElapsedSeconds)
	return nil
}

// emitElapsedAll handles the no-arg form. Sort oldest-start first so
// stale work surfaces; tie-break on lower id for stable output.
func emitElapsedAll(w io.Writer, tasks []model.Task, now time.Time, asJSON bool) error {
	entries := make([]elapsedEntry, 0)
	for _, t := range tasks {
		if !t.IsInProgress() {
			continue
		}
		entries = append(entries, newElapsedEntry(t, now))
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].ElapsedSeconds != entries[j].ElapsedSeconds {
			return entries[i].ElapsedSeconds > entries[j].ElapsedSeconds
		}
		return entries[i].ID < entries[j].ID
	})
	if asJSON {
		if entries == nil {
			entries = []elapsedEntry{}
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}
	if len(entries) == 0 {
		pln(w, "no in-progress tasks")
		return nil
	}
	for _, e := range entries {
		pf(w, "#%d  %s  (elapsed %s)\n", e.ID, e.Title, e.Elapsed)
	}
	return nil
}

// newElapsedEntry projects one task into the report struct. The task
// MUST be in-progress (Started != nil) — caller guarantees this.
func newElapsedEntry(t model.Task, now time.Time) elapsedEntry {
	d := now.Sub(*t.Started)
	if d < 0 {
		// A started: in the future shouldn't happen, but clamp to 0
		// so the JSON consumer never sees a negative duration.
		d = 0
	}
	return elapsedEntry{
		ID:             t.ID,
		Title:          t.Title,
		StartedAt:      t.Started.Format(time.RFC3339),
		ElapsedSeconds: int64(d.Seconds()),
		Elapsed:        humanizeElapsed(d),
	}
}
