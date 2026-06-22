package commands

import (
	"fmt"

	"github.com/spf13/cobra"
)

// newReachableCmd implements `tsk reachable <id>`: a top-level
// discoverable verb for the subgraph reachable from one task via
// DependsOn edges.
//
// This is a deliberate top-level command (not an alias on `graph`)
// for the same reason `blocked` is a top-level command instead of
// a `depend` alias: aliases bury the command in help output. With a
// dedicated entry, `tsk --help` lists it directly and shell
// completion offers it.
//
// Surface mirrors `tsk graph --reachable <id>` exactly:
//
//	tsk reachable <id>                    # ASCII adjacency listing
//	tsk reachable <id> --format dot       # GraphViz DOT
//	tsk reachable <id> --open             # filter out done prereqs
//
// Implementation: delegate to runGraphReachable, the same code path
// `graph --reachable` uses, so output cannot drift between the two
// surfaces (one-line forwarder, no logic duplicated).
//
// Why a positional id (not a flag like `graph --reachable <id>`):
// when a command's purpose is keyed to one id, positional is the
// shape users reach for — e.g. `tsk show 7`, `tsk depend 7 --tree`.
// `--reachable` was a flag on `graph` because `graph` is whole-store
// by default; here, the entire command IS the per-id view.
func newReachableCmd() *cobra.Command {
	var (
		format string
		open   bool
	)
	cmd := &cobra.Command{
		Use:   "reachable <id>",
		Short: "Print the dependency subgraph reachable from a task",
		Long: `Print the dependency subgraph reachable from one task via DependsOn
edges — the same view as ` + "`tsk graph --reachable <id>`" + `, surfaced as
a discoverable top-level command.

The subgraph is the transitive prereqs of the root plus the root
itself: every node and edge you could reach by walking DependsOn
starting from <id>. Useful for "what's actually involved in
shipping this one task?" without the rest of the store's noise.

Output formats:
  --format ascii   adjacency listing (default; one line per task)
  --format dot     GraphViz DOT source for piping to ` + "`dot -Tpng`" + `

Composable filters:
  --open           skip done tasks and edges to done prereqs

Examples:
  tsk reachable 7                       # ASCII subgraph at #7
  tsk reachable 7 --open                # …filtered to active prereqs
  tsk reachable 7 --format dot | dot -Tpng -o sub.png
`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseSingleID(args[0])
			if err != nil {
				return err
			}
			fmtChoice, err := resolveGraphFormat(format)
			if err != nil {
				return err
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			if s.ByID(id) == nil {
				return fmt.Errorf("no task with id %d in %s", id, s.Path)
			}
			edges := collectGraphEdges(s, open)
			edges = filterReachableEdges(s, edges, id)
			return emitGraph(cmd.OutOrStdout(), s, edges, fmtChoice, id, nil)
		},
	}
	cmd.Flags().StringVar(&format, "format", "ascii", "output format: ascii or dot")
	cmd.Flags().BoolVar(&open, "open", false, "only include open tasks and the open deps that block them")
	return cmd
}
