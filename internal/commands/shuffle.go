package commands

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
)

// newShuffleCmd implements `tsk shuffle [N]`: pick N undone tasks at
// random and print them.
//
// This is the decision-paralysis breaker: when `tsk top` returns 12
// equally-prioritized tasks and you can't decide, `tsk shuffle` flips
// a coin (or rolls a die). The default `N=1` answers "give me ONE
// thing to do" — `tsk shuffle 5` answers "stop being precious about
// picking, here's a candidate list".
//
// Behavior:
//
//   - excludes done and waiting (frozen, snoozed past today) tasks by
//     default — the same default scope as `tsk top` and `tsk next`,
//     because that's the user-meaningful set of "things I could
//     actually work on right now"
//   - --all relaxes the exclusion to include done + waiting
//   - --tag X / --priority P narrow the candidate pool (compose with
//     each other via AND)
//   - --seed N makes the pick deterministic for tests / scripts
//   - --json emits a stable array of task objects
//
// Implementation note: we sample WITHOUT replacement (Fisher-Yates
// partial shuffle on the index slice). Sampling with replacement
// would let the same task appear twice in a 5-pick, which would feel
// broken — the user asked for 5 tasks, not 5 picks from N.
//
// If N > pool, we cap at pool size and print a heads-up line so the
// user knows. Erroring would be too pedantic (your intent is clear).
func newShuffleCmd() *cobra.Command {
	var (
		showAll   bool
		filterTag string
		filterPri string
		seed      int64
		asJSON    bool
	)
	cmd := &cobra.Command{
		Use:   "shuffle [N]",
		Short: "Pick N undone tasks at random (decision-paralysis breaker)",
		Long: `Pick N undone tasks uniformly at random from the candidate pool.

Default scope mirrors 'tsk top' / 'tsk next': undone, not-waiting.
--all expands to done + waiting. --tag and --priority narrow the
candidate pool (compose via AND).

Sampling is WITHOUT replacement so the same task never appears twice
in a single pick. If N exceeds the pool, the pool size is returned
with a heads-up note.

For tests / reproducible scripts, --seed makes the pick deterministic.

Examples:
  tsk shuffle                       # one random task
  tsk shuffle 5                     # five
  tsk shuffle 3 --tag dev           # three random dev tasks
  tsk shuffle --priority high       # one high-prio task
  tsk shuffle 5 --seed 42 --json    # reproducible JSON
`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n := 1
			if len(args) == 1 {
				parsed, err := strconvAtoiPos(args[0])
				if err != nil {
					return err
				}
				if parsed == 0 {
					return usageErrorf("N must be >= 1")
				}
				n = parsed
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			pri, err := parseShufflePriority(filterPri)
			if err != nil {
				return err
			}
			pool := shufflePool(s.Tasks, time.Now(), showAll, filterTag, pri)
			picked, capped := samplePool(pool, n, seed)
			return emitShuffle(cmd.OutOrStdout(), picked, capped, asJSON)
		},
	}
	cmd.Flags().BoolVar(&showAll, "all", false, "include done and waiting tasks in the pool")
	cmd.Flags().StringVar(&filterTag, "tag", "", "only consider tasks with this tag")
	cmd.Flags().StringVar(&filterPri, "priority", "", "only consider tasks with this priority (low/medium/high/urgent)")
	cmd.Flags().Int64Var(&seed, "seed", 0, "seed the RNG for deterministic picks (0 = time-based)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON array of picked tasks")
	return cmd
}

// shufflePool builds the candidate set under the given scope and filters.
// Done + waiting are excluded by default (mirrors top/next).
func shufflePool(tasks []model.Task, now time.Time, showAll bool, tag string, pri *model.Priority) []model.Task {
	out := make([]model.Task, 0, len(tasks))
	for _, t := range tasks {
		if !showAll {
			if t.Done {
				continue
			}
			if t.IsWaiting(now) {
				continue
			}
		}
		if tag != "" && !t.HasTag(tag) {
			continue
		}
		if pri != nil && t.Priority != *pri {
			continue
		}
		out = append(out, t)
	}
	return out
}

// parseShufflePriority turns "" into nil, anything else into a
// validated Priority pointer. Errors propagate to the user via the
// exit-code-2 usage path. (A different signature is used by `bulk`'s
// parseOptionalPriority, which carries a flag-name annotation —
// shuffle's usage error already mentions --priority, so the simpler
// form fits.)
func parseShufflePriority(raw string) (*model.Priority, error) {
	if raw == "" {
		return nil, nil
	}
	p, err := model.ParsePriority(raw)
	if err != nil {
		return nil, usageErrorf("%s", err.Error())
	}
	return &p, nil
}

// samplePool draws min(n, len(pool)) elements without replacement.
// Uses a partial Fisher-Yates on an index slice so we don't have to
// copy + mutate the pool itself. Returns the picked tasks IN PICK
// ORDER (not file order) so the user sees the actual roll of the
// dice. `capped` is true when n > pool — caller uses it for the
// heads-up note.
func samplePool(pool []model.Task, n int, seed int64) ([]model.Task, bool) {
	if len(pool) == 0 {
		return nil, false
	}
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	r := rand.New(rand.NewSource(seed))
	indices := make([]int, len(pool))
	for i := range indices {
		indices[i] = i
	}
	capped := false
	if n > len(pool) {
		n = len(pool)
		capped = true
	}
	for i := 0; i < n; i++ {
		j := i + r.Intn(len(indices)-i)
		indices[i], indices[j] = indices[j], indices[i]
	}
	out := make([]model.Task, n)
	for i := 0; i < n; i++ {
		out[i] = pool[indices[i]]
	}
	return out, capped
}

// emitShuffle dispatches to JSON or plain output.
func emitShuffle(w iWriter, picked []model.Task, capped, asJSON bool) error {
	if asJSON {
		if picked == nil {
			picked = []model.Task{}
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(picked)
	}
	if len(picked) == 0 {
		_, _ = fmt.Fprintln(w, "no tasks to pick from")
		return nil
	}
	if capped {
		_, _ = fmt.Fprintf(w, "(only %d task(s) in the pool — showing all)\n", len(picked))
	}
	for _, t := range picked {
		marker := " "
		if t.Done {
			marker = "x"
		}
		_, _ = fmt.Fprintf(w, "[%s] #%d  [%s]  %s\n", marker, t.ID, t.Priority.Short(), t.Title)
	}
	return nil
}
