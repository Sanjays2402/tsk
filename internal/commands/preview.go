package commands

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/store"
)

// newPreviewCmd implements `tsk preview`: render a .tsk.md payload
// from STDIN (or a --from path) using tsk's parser and the standard
// ls renderer, WITHOUT touching the active store and WITHOUT
// creating a .bak snapshot.
//
// User story this serves:
//
//   - "I want to see what `tsk diff`'s pre-change snapshot looks
//     like rendered, without actually undoing or reading from the
//     active store." Pipe in `cat .tsk.md.bak | tsk preview` for
//     that.
//
//   - "I'm scripting a CI step that synthesizes a hypothetical store
//     and wants to run tsk's queries against it without writing to
//     disk." `echo "$markdown" | tsk preview --json` returns the
//     parsed task list as JSON.
//
//   - "I want to validate that a hand-edited markdown blob parses
//     cleanly before I rename it to .tsk.md." `tsk preview --from
//     scratch.md` parses and renders — non-zero exit + clear error
//     if the parser chokes.
//
// Pipeline contract: preview NEVER reads .tsk.md (no resolveStore
// call), NEVER calls store.Save (no .bak chain side effects). It
// uses store.LoadBytes — a fresh parse against an in-memory payload.
//
// Filters/format: --json, --format plain|table reuse the same lsFilters
// flag surface so users don't learn a second filter set. Default is
// plain. State filters (--done, --all, --today, --overdue, --upcoming,
// --tag, --priority, --include-waiting, --respect-deps) all compose
// the same way they do in `tsk ls`. Because preview operates on a
// detached snapshot (no resolveStore), --respect-deps walks the
// detached store's own DependsOn graph for blocker checks — exactly
// what users want for "test this snapshot in isolation".
func newPreviewCmd() *cobra.Command {
	f := lsFilters{}
	var from string
	cmd := &cobra.Command{
		Use:   "preview",
		Short: "Render a .tsk.md payload from stdin or --from without touching the active store",
		Long: `Render a .tsk.md payload from STDIN (or --from <path>) using tsk's
parser and the standard ` + "`tsk ls`" + ` renderer, WITHOUT touching the
active store and WITHOUT creating a .bak snapshot.

Useful for:
  - inspecting a snapshot pipe (` + "`cat .tsk.md.bak | tsk preview`" + `)
  - running tsk's queries against a synthesized store in CI without
    writing to disk
  - validating that a hand-edited markdown blob parses cleanly
    before committing it as the active .tsk.md

Every state filter from ` + "`tsk ls`" + ` (--done, --all, --today, --overdue,
--upcoming, --tag, --priority, --include-waiting, --respect-deps) works
here too — preview is "ls against an in-memory store".

Examples:
  cat .tsk.md.bak | tsk preview              # what was the previous state?
  tsk preview --from /tmp/scratch.md         # render a file without registering it
  echo "$markdown" | tsk preview --json      # parse + emit structured output
  tsk preview --from snapshot.md --respect-deps --tag work
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := readPreviewBytes(cmd.InOrStdin(), from)
			if err != nil {
				return err
			}
			s, err := store.LoadBytes(data)
			if err != nil {
				return fmt.Errorf("parse preview input: %w", err)
			}
			tasks, err := applyFilters(s.Tasks, f)
			if err != nil {
				return err
			}
			if f.respectDeps {
				tasks = filterBlockedTasks(s, tasks)
			}
			format, err := resolveLsFormat(f.format, f.asJSON)
			if err != nil {
				return err
			}
			return printTasks(cmd.OutOrStdout(), tasks, format)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "read input from this file path instead of stdin")
	cmd.Flags().BoolVar(&f.done, "done", false, "only show done tasks")
	cmd.Flags().BoolVar(&f.all, "all", false, "show all tasks (done + undone + waiting)")
	cmd.Flags().BoolVar(&f.today, "today", false, "only show tasks due today")
	cmd.Flags().BoolVar(&f.overdue, "overdue", false, "only show overdue tasks")
	cmd.Flags().BoolVar(&f.upcoming, "upcoming", false, "only show tasks due in the future")
	cmd.Flags().BoolVar(&f.includeWaiting, "include-waiting", false, "include tasks with wait:<future date>")
	cmd.Flags().BoolVar(&f.respectDeps, "respect-deps", false, "skip tasks blocked by an open prerequisite")
	cmd.Flags().StringVar(&f.tag, "tag", "", "only show tasks with this tag")
	cmd.Flags().StringVar(&f.priorityStr, "priority", "", "only show tasks with this priority")
	cmd.Flags().BoolVar(&f.asJSON, "json", false, "emit JSON (shortcut for --format=json)")
	cmd.Flags().StringVar(&f.format, "format", "", "output format: plain, table, or json")
	return cmd
}

// readPreviewBytes resolves the input source for preview. Priority:
//  1. --from <path>: read that file in full (must exist).
//  2. stdin: read until EOF.
//
// Refuses an empty input — a zero-byte preview is a usage error
// (likely a misconfigured pipe or forgotten --from path). The user
// can always pass `echo "" | tsk preview` if they really want to
// see "no tasks", but the silent-empty default is a footgun.
//
// Cap on payload size: 4 MiB. Anything larger almost certainly
// indicates the user accidentally piped in a non-.tsk.md file
// (binary, log, etc), and a malicious payload could exhaust memory
// otherwise. The active store path doesn't have this cap because
// the file is trusted (user wrote it), but stdin is by definition
// less trusted.
func readPreviewBytes(in io.Reader, from string) ([]byte, error) {
	const maxBytes = 4 * 1024 * 1024
	if from != "" {
		data, err := os.ReadFile(from)
		if err != nil {
			return nil, fmt.Errorf("read --from %s: %w", from, err)
		}
		if len(data) > maxBytes {
			return nil, usageErrorf("--from %s: payload is %d bytes (cap is %d); refusing as a safety guard",
				from, len(data), maxBytes)
		}
		if len(data) == 0 {
			return nil, usageErrorf("--from %s: file is empty", from)
		}
		return data, nil
	}
	data, err := io.ReadAll(io.LimitReader(in, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read stdin: %w", err)
	}
	if len(data) > maxBytes {
		return nil, usageErrorf("stdin payload exceeds %d bytes cap (likely not a .tsk.md file)", maxBytes)
	}
	if len(data) == 0 {
		return nil, usageErrorf("no input on stdin (pipe a .tsk.md payload in, or use --from <path>)")
	}
	return data, nil
}
