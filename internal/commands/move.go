package commands

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
)

// moveOpts captures the parsed flags for `tsk move`.
type moveOpts struct {
	destAbs string
	ids     []int
	dryRun  bool
	keepIDs bool
}

// newMoveCmd implements `tsk move <id...> --to <path>`. It removes the
// task(s) from the source file (current --file or nearest .tsk.md) and
// appends them to the destination file. The destination is created if it
// doesn't exist. IDs are re-assigned in the destination using its next-ID
// counter so the move can never produce a collision.
//
// This is the natural workflow for scratch → project promotion: you triage
// in ~/.tsk.md, then `tsk move 3 5 7 --to ~/projects/foo/.tsk.md`.
func newMoveCmd() *cobra.Command {
	var (
		dest    string
		dryRun  bool
		keepIDs bool
	)
	cmd := &cobra.Command{
		Use:   "move <id> [<id>...]",
		Short: "Move task(s) to another .tsk.md file",
		Long: `Move one or more tasks from the active .tsk.md to a destination file.

The destination is created if it doesn't exist. IDs are re-assigned in
the destination by default to avoid collisions; pass --keep-ids to
preserve original IDs (will fail if the destination already has them).

Examples:
  tsk move 3 --to ~/projects/foo/.tsk.md
  tsk move 3 5 7 --to ~/projects/foo/.tsk.md
  tsk move 12 --to ../other/.tsk.md --dry-run
`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := parseMoveOpts(args, dest, dryRun, keepIDs)
			if err != nil {
				return err
			}
			src, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			return runMove(cmd.OutOrStdout(), src, opts)
		},
	}
	cmd.Flags().StringVar(&dest, "to", "", "destination .tsk.md path (required)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview the move without modifying files")
	cmd.Flags().BoolVar(&keepIDs, "keep-ids", false, "preserve original task IDs (fails on collision)")
	if err := cmd.MarkFlagRequired("to"); err != nil {
		// Should never fail at construction time.
		panic(fmt.Errorf("mark --to required: %w", err))
	}
	return cmd
}

// parseMoveOpts validates the raw flags into a moveOpts.
func parseMoveOpts(args []string, dest string, dryRun, keepIDs bool) (moveOpts, error) {
	var o moveOpts
	if strings.TrimSpace(dest) == "" {
		return o, usageErrorf("move requires --to <path>")
	}
	destAbs, err := filepath.Abs(dest)
	if err != nil {
		return o, usageErrorf("invalid --to path: %v", err)
	}
	ids, err := parseTaskIDs(args)
	if err != nil {
		return o, err
	}
	o.destAbs = destAbs
	o.ids = ids
	o.dryRun = dryRun
	o.keepIDs = keepIDs
	return o, nil
}

// runMove is the high-level orchestrator: resolve source, dest, then commit.
func runMove(out io.Writer, src *store.Store, opts moveOpts) error {
	if err := refuseSameFile(src.Path, opts.destAbs); err != nil {
		return err
	}
	toMove, err := gatherTasksToMove(src, opts.ids)
	if err != nil {
		return err
	}
	dst, err := store.Load(opts.destAbs)
	if err != nil {
		return fmt.Errorf("load destination %s: %w", opts.destAbs, err)
	}
	if opts.keepIDs {
		if err := checkNoIDCollision(dst, toMove); err != nil {
			return err
		}
	}
	if opts.dryRun {
		pln(out, planMoveSummary(src.Path, opts.destAbs, toMove, opts.keepIDs))
		return nil
	}
	if err := appendToDestAndSave(dst, toMove, opts.keepIDs); err != nil {
		return err
	}
	if err := removeFromSrcAndSave(src, opts.ids); err != nil {
		return err
	}
	pf(out, "moved %d task(s) → %s\n", len(toMove), opts.destAbs)
	return nil
}

// refuseSameFile errors if source and destination resolve to the same path.
func refuseSameFile(srcPath, destAbs string) error {
	srcAbs, err := filepath.Abs(srcPath)
	if err == nil && srcAbs == destAbs {
		return usageErrorf("source and destination are the same file: %s", srcAbs)
	}
	return nil
}

// gatherTasksToMove looks up every requested ID in src; errors if any miss.
func gatherTasksToMove(src *store.Store, ids []int) ([]model.Task, error) {
	out := make([]model.Task, 0, len(ids))
	for _, id := range ids {
		t := src.ByID(id)
		if t == nil {
			return nil, fmt.Errorf("no task with id %d in %s", id, src.Path)
		}
		out = append(out, *t)
	}
	return out, nil
}

// checkNoIDCollision verifies the destination doesn't already contain any
// of the IDs we're about to insert (used only when --keep-ids is set).
func checkNoIDCollision(dst *store.Store, toMove []model.Task) error {
	existing := make(map[int]bool, len(dst.Tasks))
	for _, t := range dst.Tasks {
		existing[t.ID] = true
	}
	for _, t := range toMove {
		if existing[t.ID] {
			return fmt.Errorf("destination already has id %d; drop --keep-ids to re-assign", t.ID)
		}
	}
	return nil
}

// appendToDestAndSave appends toMove to dst, optionally clearing IDs first.
func appendToDestAndSave(dst *store.Store, toMove []model.Task, keepIDs bool) error {
	for _, t := range toMove {
		if !keepIDs {
			t.ID = 0 // Add() will assign next
		}
		dst.Add(t)
	}
	if err := dst.Save(); err != nil {
		return fmt.Errorf("save destination: %w", err)
	}
	return nil
}

// removeFromSrcAndSave removes the moved IDs from src and saves it.
func removeFromSrcAndSave(src *store.Store, ids []int) error {
	for _, id := range ids {
		if !src.Remove(id) {
			// Shouldn't happen — we checked above — but be paranoid.
			return fmt.Errorf("internal: failed to remove id %d from source after destination saved", id)
		}
	}
	if err := src.Save(); err != nil {
		return fmt.Errorf("save source (destination already updated): %w", err)
	}
	return nil
}

// parseTaskIDs converts positional args into a deduped sorted slice of ints.
func parseTaskIDs(args []string) ([]int, error) {
	seen := make(map[int]bool, len(args))
	out := make([]int, 0, len(args))
	for _, a := range args {
		n, err := strconv.Atoi(strings.TrimPrefix(a, "#"))
		if err != nil || n <= 0 {
			return nil, usageErrorf("invalid task id %q", a)
		}
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out, nil
}

// planMoveSummary builds the --dry-run preview text.
func planMoveSummary(srcPath, dstPath string, tasks []model.Task, keepIDs bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "DRY RUN: would move %d task(s) %s → %s\n", len(tasks), srcPath, dstPath)
	if !keepIDs {
		b.WriteString("  (IDs will be re-assigned in destination)\n")
	}
	for _, t := range tasks {
		fmt.Fprintf(&b, "  #%d %s\n", t.ID, t.Title)
	}
	return b.String()
}
