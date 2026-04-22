package commands

import (
	"fmt"
	"os"

	"github.com/Sanjays2402/tsk/internal/store"
	"github.com/spf13/cobra"
)

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
