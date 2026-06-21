package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
)

// newJustifyCmd implements `tsk justify [<id>]`: a top-level
// discoverable verb that walks the dependency chain for one task
// (or, with --all, every blocked task in the store) and prints
// the chain of reasons that gates each one.
//
// Single-task mode is byte-identical to `tsk depend <id> --justify`
// — it delegates straight into runDependJustify. That's the same
// pattern `tsk blocked` and `tsk reachable` use: top-level entries
// for sub-flag views so they appear in `tsk --help` and shell
// completion. Aliases on `depend` would bury this verb in
// `tsk depend --help` only.
//
// --all is the new capability: emit a justify chain for EVERY open
// blocked task in the store, in id order. It's the "what's gating
// everything?" review screen — when you've been away from the
// project for a while and want to see all the chokepoints at once
// without N separate invocations of `tsk depend <id> --justify`.
//
// Output for --all:
//   - plain: each task's chain prefixed with a "=== #N ===" header
//     plus a blank line between, so the boundaries are visible at a
//     glance
//   - JSON: an object keyed by id, value = the same chain shape
//     `tsk depend --justify --json` emits per-task, so `jq` filters
//     compose ("which tasks are blocked by #7?"
//     -> `jq 'to_entries | map(select(.value[] | .blocked_by == 7))'`)
//
// Empty --all (no blocked tasks): plain "no blocked tasks" message,
// JSON empty-object `{}`. NEVER null — consumers iterating with
// `keys[]` would crash.
//
// Why a JSON OBJECT (not array) for --all: every chain has a known
// root id, which makes a map the natural shape — "give me #7's chain
// without scanning the array". Per-task chains stay arrays internally
// (same shape as single-task --json) so existing consumers can pull
// one chain out and pass it to the same jq filter they wrote for
// single-task mode.
func newJustifyCmd() *cobra.Command {
	var (
		asJSON bool
		all    bool
	)
	cmd := &cobra.Command{
		Use:   "justify [<id>]",
		Short: "Plain-English chain of reasons gating one task (or every blocked task with --all)",
		Long: `Walk the dependency chain for a task and print the chain of reasons
that's gating it. Useful for "why is this stuck?" debugging.

Single-task mode is identical to ` + "`tsk depend <id> --justify`" + ` —
this is just the discoverable top-level verb. Use --all to emit a
chain for EVERY open blocked task in the store at once (review
screen for "what's gating everything?" after time away).

Examples:
  tsk justify 7                # chain of reasons gating #7
  tsk justify 7 --json         # structured chain for scripts
  tsk justify --all            # every blocked task at once
  tsk justify --all --json     # CI: dump every chain for review

The chain follows the LOWEST-id open prereq at each step — the
result is reproducible across runs. For "what should I work on
next?", use ` + "`tsk next --respect-deps`" + `; for the structural view,
use ` + "`tsk depend <id> --tree`" + `.
`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateJustifyFlags(args, all); err != nil {
				return err
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			if all {
				return runJustifyAll(cmd.OutOrStdout(), s, asJSON)
			}
			id, err := parseSingleID(args[0])
			if err != nil {
				return err
			}
			t := s.ByID(id)
			if t == nil {
				return fmt.Errorf("no task with id %d in %s", id, s.Path)
			}
			return runDependJustify(cmd.OutOrStdout(), s, t, asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	cmd.Flags().BoolVar(&all, "all", false, "emit a chain for every blocked task in the store")
	return cmd
}

// validateJustifyFlags rejects nonsensical invocations up-front so
// the user sees a precise error instead of a confusing fallback.
func validateJustifyFlags(args []string, all bool) error {
	if all && len(args) > 0 {
		return usageErrorf("--all takes no positional id (it covers every blocked task)")
	}
	if !all && len(args) == 0 {
		return usageErrorf("missing <id> (or pass --all for the whole-store view)")
	}
	return nil
}

// runJustifyAll walks every OPEN BLOCKED task in id order and emits
// its justify chain. \"Blocked\" here matches `tsk blocked` exactly:
// at least one unmet prerequisite per unmetBlockers' policy.
//
// Done tasks are excluded (they're not blocked by definition).
// Waiting tasks are excluded (they're hidden in default views;
// surfacing them in a \"what's stuck?\" review would suggest work
// that's deliberately deferred). Tasks with no DependsOn are
// excluded (they're not blocked).
func runJustifyAll(w io.Writer, s *store.Store, asJSON bool) error {
	blocked := collectBlockedRoots(s)
	if asJSON {
		return emitJustifyAllJSON(w, s, blocked)
	}
	return emitJustifyAllPlain(w, s, blocked)
}

// collectBlockedRoots returns every open task with at least one
// unmet prerequisite, sorted by id ascending. The output is a slice
// of value-typed model.Task copies so callers can pass &task to
// runDependJustify-shaped APIs without aliasing into the store.
func collectBlockedRoots(s *store.Store) []model.Task {
	out := make([]model.Task, 0)
	for _, t := range s.Tasks {
		if t.Done {
			continue
		}
		t := t
		if len(unmetBlockers(s, &t, nil)) == 0 {
			continue
		}
		out = append(out, t)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// emitJustifyAllPlain renders every blocked task's chain with a
// "=== #N ===" header so the boundaries are obvious. A blank line
// separates chains so the output stays scannable in a terminal.
//
// Empty input: one-line "no blocked tasks" so the user knows the
// command ran successfully — silent output here would be ambiguous
// ("did it fail? did it find nothing?").
func emitJustifyAllPlain(w io.Writer, s *store.Store, blocked []model.Task) error {
	if len(blocked) == 0 {
		pln(w, "no blocked tasks")
		return nil
	}
	for i, t := range blocked {
		if i > 0 {
			pln(w)
		}
		pf(w, "=== #%d %s ===\n", t.ID, t.Title)
		t := t
		if err := runDependJustify(w, s, &t, false); err != nil {
			return err
		}
	}
	return nil
}

// emitJustifyAllJSON renders a {id_string: [step,...]} map so
// callers can look up one chain by id without scanning. The inner
// chain shape matches single-task `--json` exactly (same justifyStep
// array) so jq filters compose between modes.
//
// Empty input: emit `{}` (NOT null) so consumers iterating with
// `keys[]` don't crash on an unexpected nil.
func emitJustifyAllJSON(w io.Writer, s *store.Store, blocked []model.Task) error {
	out := make(map[string][]justifyStep, len(blocked))
	for _, t := range blocked {
		t := t
		chain := buildJustifyChain(s, &t)
		out[fmt.Sprintf("%d", t.ID)] = chain
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
