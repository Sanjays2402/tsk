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

// ResolveTZ returns the time.Location tsk should interpret natural-language
// dates in. It resolves in priority order:
//
//  1. $TSK_TZ, if set to a valid IANA zone name (e.g. "America/New_York")
//  2. $TZ, if set to a valid IANA zone (standard *nix convention)
//  3. time.Local (the system default)
//  4. America/Los_Angeles, as a last-resort fallback if time.Local resolves
//     to UTC on a container with no zoneinfo (which would silently make
//     "tomorrow" mean "UTC tomorrow").
//
// The result is cached — first call wins, later env changes are ignored
// within the same process. Tests that need to override should call
// ResetTZForTest.
func ResolveTZ() *time.Location {
	locOnce.Do(func() {
		for _, candidate := range []string{os.Getenv("TSK_TZ"), os.Getenv("TZ")} {
			if candidate == "" {
				continue
			}
			if l, err := time.LoadLocation(candidate); err == nil {
				locVal = l
				return
			}
		}
		// Prefer system local over a blind LA default; only fall back to LA
		// when time.Local is UTC (typical on stripped containers without
		// /etc/localtime) to avoid the "tomorrow = UTC tomorrow" footgun.
		if time.Local != time.UTC {
			locVal = time.Local
			return
		}
		if l, err := time.LoadLocation("America/Los_Angeles"); err == nil {
			locVal = l
			return
		}
		locVal = time.Local
	})
	return locVal
}

// ResetTZForTest clears the cached location so tests can re-resolve under a
// different $TSK_TZ. Must not be called from production code paths.
func ResetTZForTest() {
	locOnce = sync.Once{}
	locVal = nil
}

// PacificLoc is retained for backward compatibility with existing callers.
// New code should prefer ResolveTZ.
func PacificLoc() *time.Location {
	return ResolveTZ()
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
