package commands

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
)

// newLastCmd implements `tsk last`: surface the most recently mutated
// task — the task whose Created or Completed timestamp is the latest.
//
// This is the "wait, what was I just doing?" command. Pair with
// 'tsk diff' to see what changed in the file, with 'tsk show <id>'
// to see the task in full.
//
// Tie-break: if both Created and Completed exist for a task, we use
// the LATER of the two as "last touched". Across tasks, the task with
// the latest touched-time wins; ties (same timestamp at second
// granularity) fall back to the LARGER id (most recently added).
func newLastCmd() *cobra.Command {
	var (
		asJSON bool
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "last [N]",
		Short: "Show the most recently mutated task (or the last N)",
		Long: `Show the most recently mutated task — the task whose Created or
Completed timestamp is the most recent.

Optionally pass N (or --n) to see the most recent N tasks instead.

The "mutated" timestamp is max(Created, Completed) per task: an old task
that was just marked done floats above a brand-new task added a minute
earlier, because completion is the more recent event.

Examples:
  tsk last                # 1 most-recent task
  tsk last 5              # 5 most-recent tasks
  tsk last --json         # stable JSON for scripts
  tsk last 1 --json | jq -r '.[0].Title'
`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n, err := resolveLastLimit(args, limit)
			if err != nil {
				return err
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			tasks := mostRecentlyMutated(s.Tasks, n)
			return emitLastResults(cmd.OutOrStdout(), tasks, asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	cmd.Flags().IntVar(&limit, "n", 0, "number of recent tasks (overrides positional N)")
	return cmd
}

// resolveLastLimit picks the limit from --n, then the positional arg,
// then defaults to 1. Returns an error if both are set to different
// values (avoid the ambiguity).
func resolveLastLimit(args []string, flagN int) (int, error) {
	pos := -1
	if len(args) == 1 {
		raw := strings.TrimSpace(args[0])
		if raw == "" {
			pos = 1
		} else {
			n, err := strconvAtoiPos(raw)
			if err != nil {
				return 0, err
			}
			pos = n
		}
	}
	switch {
	case flagN > 0 && pos > 0 && flagN != pos:
		return 0, usageErrorf("--n=%d and positional N=%d disagree", flagN, pos)
	case flagN > 0:
		return flagN, nil
	case pos > 0:
		return pos, nil
	case flagN == 0 && pos == -1:
		return 1, nil
	default:
		// pos == 0 → "0" was explicitly passed; treat as 1 to avoid empty
		// output (an explicit zero feels like a footgun here).
		return 1, nil
	}
}

// strconvAtoiPos parses a positive int; rejects negatives and non-numeric.
func strconvAtoiPos(raw string) (int, error) {
	n := 0
	for _, r := range raw {
		if r < '0' || r > '9' {
			return 0, usageErrorf("N must be a positive integer, got %q", raw)
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

// scoredTask pairs a task with its computed "last touched" timestamp.
type scoredTask struct {
	t  model.Task
	ts time.Time
}

// mostRecentlyMutated returns the top-n tasks sorted by max(Created,
// Completed) descending, then by ID descending. Tasks with no
// timestamps at all are skipped (nothing meaningful to surface for an
// edited-by-hand entry with no created stamp).
func mostRecentlyMutated(in []model.Task, n int) []model.Task {
	scoredAll := make([]scoredTask, 0, len(in))
	for _, t := range in {
		ts := lastTouched(t)
		if ts.IsZero() {
			continue
		}
		scoredAll = append(scoredAll, scoredTask{t: t, ts: ts})
	}
	sortByLatest(scoredAll)
	if n > 0 && len(scoredAll) > n {
		scoredAll = scoredAll[:n]
	}
	out := make([]model.Task, len(scoredAll))
	for i, s := range scoredAll {
		out[i] = s.t
	}
	return out
}

// lastTouched returns max(Created, Completed). Zero if both are unset.
func lastTouched(t model.Task) time.Time {
	ts := t.Created
	if t.Completed != nil && t.Completed.After(ts) {
		ts = *t.Completed
	}
	return ts
}

// sortByLatest sorts by ts desc, then id desc. Stable so equal-timestamp
// runs preserve insertion order as a final fallback.
func sortByLatest(in []scoredTask) {
	// Manual swap-based insertion sort to avoid importing sort.
	// O(n^2) is fine — `tsk last` runs on at most a few hundred tasks.
	for i := 1; i < len(in); i++ {
		j := i
		for j > 0 {
			a, b := in[j-1], in[j]
			if a.ts.Equal(b.ts) {
				if a.t.ID >= b.t.ID {
					break
				}
			} else if a.ts.After(b.ts) {
				break
			}
			in[j-1], in[j] = in[j], in[j-1]
			j--
		}
	}
}

// emitLastResults dispatches to JSON or plain.
func emitLastResults(w iWriter, tasks []model.Task, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		// Always emit an array (even for the default n=1) so consumers
		// don't have to branch on the input. Empty arrays are []
		// (not null) for the same reason.
		if tasks == nil {
			tasks = []model.Task{}
		}
		return enc.Encode(tasks)
	}
	if len(tasks) == 0 {
		pln(w, "no tasks with timestamps")
		return nil
	}
	for _, t := range tasks {
		printLastLine(w, t)
	}
	return nil
}

// iWriter is a tiny alias so this file doesn't pull io.Writer; the
// commands package conventionally uses cmd.OutOrStdout() which is an
// io.Writer at runtime.
type iWriter interface {
	Write(p []byte) (int, error)
}

// printLastLine renders one task with its touched-at timestamp and the
// reason (created vs completed) so the user can see WHY it's "last".
func printLastLine(w iWriter, t model.Task) {
	ts := lastTouched(t)
	reason := "created"
	if t.Completed != nil && t.Completed.Equal(ts) {
		reason = "completed"
	}
	check := " "
	if t.Done {
		check = "x"
	}
	line := fmt.Sprintf("[%s] #%d %s  (%s %s)",
		check, t.ID, t.Title, reason, ts.Format("2006-01-02 15:04:05 -0700"))
	pln(w, line)
}
