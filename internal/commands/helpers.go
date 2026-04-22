package commands

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/Sanjays2402/tsk/internal/store"
	"github.com/spf13/cobra"
)

// ExitCoder is an error that carries a preferred process exit code. Main uses
// it to exit with 2 on user-input errors (e.g. bad --due), distinguishing them
// from unexpected failures that exit 1.
type ExitCoder interface {
	error
	ExitCode() int
}

type exitErr struct {
	msg  string
	code int
}

func (e *exitErr) Error() string { return e.msg }

func (e *exitErr) ExitCode() int { return e.code }

// usageErrorf returns an error that should exit with code 2.
func usageErrorf(format string, args ...any) error {
	return &exitErr{msg: fmt.Sprintf(format, args...), code: 2}
}

var (
	locOnce sync.Once
	locVal  *time.Location
)

// PacificLoc returns the cached America/Los_Angeles location, falling back to
// time.Local if the zoneinfo database is unavailable (rare but possible on
// stripped containers).
func PacificLoc() *time.Location {
	locOnce.Do(func() {
		if l, err := time.LoadLocation("America/Los_Angeles"); err == nil {
			locVal = l
			return
		}
		locVal = time.Local
	})
	return locVal
}

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
