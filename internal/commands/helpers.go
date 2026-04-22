package commands

import (
	"fmt"
	"io"
	"os"

	"github.com/Sanjays2402/tsk/internal/store"
	"github.com/spf13/cobra"
)

// pf is a tolerant Fprintf: errors writing to user-facing output are ignored.
func pf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

// pln is a tolerant Fprintln.
func pln(w io.Writer, args ...any) {
	_, _ = fmt.Fprintln(w, args...)
}

// resolveStore opens (and returns) the store at --file, nearest .tsk.md, or a cwd fallback.
// If requireExisting is true, an error is returned when no file is found.
func resolveStore(cmd *cobra.Command, requireExisting bool) (*store.Store, error) {
	path, _ := cmd.Flags().GetString("file")
	if path == "" {
		if requireExisting {
			resolved, ok := store.Resolve("")
			if !ok {
				return nil, fmt.Errorf("no .tsk.md found; run `tsk init`")
			}
			path = resolved
		} else {
			path = store.ResolveOrCreate("")
		}
	}
	if requireExisting {
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("no .tsk.md at %s; run `tsk init`", path)
		}
	}
	return store.Load(path)
}
