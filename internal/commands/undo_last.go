package commands

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// newUndoLastCmd creates `tsk undo-last`. It restores the .tsk.md.bak
// snapshot that Save() automatically writes before every change.
//
// Single-step undo: there is only ever one snapshot ("the file as it was
// before the most recent save"). Running undo-last twice swaps the file
// back to the most recent state — i.e. it's its own inverse.
func newUndoLastCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "undo-last",
		Short: "Revert the most recent change to the task file (single-step undo)",
		Long: `Restores the .tsk.md.bak snapshot that tsk writes before every save.

Every save (add, rm, done, undo, edit, move, bulk, archive, etc.) writes the
prior file contents to .tsk.md.bak first. tsk undo-last swaps that snapshot
back in. Because the swap itself is a save, the file you just replaced is now
the new .bak — so running undo-last twice undoes the undo.

There is only ever one snapshot. If you need durable history, use git.
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			path := s.Path
			bak := path + ".bak"

			if _, err := os.Stat(bak); errors.Is(err, os.ErrNotExist) {
				return usageErrorf("no snapshot at %s — nothing to undo", bak)
			} else if err != nil {
				return fmt.Errorf("stat %s: %w", bak, err)
			}

			if !yes && !confirmUndoLast(cmd.OutOrStdout(), cmd.InOrStdin(), path, bak) {
				pf(cmd.OutOrStdout(), "aborted\n")
				return nil
			}

			// Swap atomically: read both, write each to the other path.
			curBytes, err := os.ReadFile(path)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("read current: %w", err)
			}
			bakBytes, err := os.ReadFile(bak)
			if err != nil {
				return fmt.Errorf("read snapshot: %w", err)
			}
			// Write the snapshot into the live file (becomes the active state).
			if err := atomicWriteOrPanicFree(path, bakBytes); err != nil {
				return fmt.Errorf("write current: %w", err)
			}
			// Write the previously-live contents into the .bak so undo-last is its own inverse.
			// If the live file didn't exist before, write an empty .bak (still a valid swap target).
			if err := atomicWriteOrPanicFree(bak, curBytes); err != nil {
				return fmt.Errorf("write snapshot: %w", err)
			}
			pf(cmd.OutOrStdout(), "restored %s from %s\n", path, bak)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip the confirmation prompt")
	return cmd
}

// confirmUndoLast prompts the user; returns true if they confirm.
func confirmUndoLast(out io.Writer, in io.Reader, path, bak string) bool {
	pf(out, "About to swap %s ↔ %s.\nContinue? [y/N] ", path, bak)
	buf := make([]byte, 16)
	n, _ := in.Read(buf)
	if n == 0 {
		return false
	}
	resp := strings.ToLower(strings.TrimSpace(string(buf[:n])))
	return resp == "y" || resp == "yes"
}

// atomicWriteOrPanicFree is a thin wrapper used so test code can inject
// failures without panicking. It exists for future-proofing.
func atomicWriteOrPanicFree(path string, data []byte) error {
	tmp := path + ".tsk-undo-tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
