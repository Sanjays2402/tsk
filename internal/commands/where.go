package commands

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/store"
)

// newWhereCmd implements `tsk where`: print which .tsk.md tsk is going to
// operate on, how it found that file, and which timezone it's using to
// interpret natural-language dates. Designed for:
//   - shell prompts (PS1, starship) that want to surface the active list
//   - debugging "wait, which file did that edit?" surprises
//   - scripts: with --json the output is a stable schema
//
// The command does NOT require the file to exist — if it doesn't, the
// `exists` field reports false and `method` explains the fallback chosen.
// That lets a fresh shell discover where `tsk init` would create the file.
func newWhereCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "where",
		Short: "Print the resolved .tsk.md path, resolution method, and timezone",
		Long: `Print the resolved .tsk.md path, how tsk found it, and the
timezone used for date parsing.

Resolution methods (in priority order):
  flag       - --file <path> on the command line (or persistent)
  nearest    - walked up from cwd and found a .tsk.md
  global     - no .tsk.md walked-up; falls back to ~/.tsk/global.md
  cwd        - no walked-up file AND no home dir; falls back to ./.tsk.md

Pass --json for a stable schema scriptable by jq.

Examples:
  tsk where
  tsk where --json | jq -r '.path'
  tsk where --file ./other/.tsk.md
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			info, err := buildWhereInfo(cmd)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}
			printWhere(cmd.OutOrStdout(), info)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	return cmd
}

// WhereInfo is the stable struct emitted by `tsk where --json`.
type WhereInfo struct {
	Path     string `json:"path"`
	Method   string `json:"method"`
	Exists   bool   `json:"exists"`
	Timezone string `json:"timezone"`
	TZSource string `json:"tz_source"`
}

// buildWhereInfo runs the same resolution logic CLI commands do.
func buildWhereInfo(cmd *cobra.Command) (WhereInfo, error) {
	info := WhereInfo{Timezone: ResolveTZ().String(), TZSource: detectTZSource()}
	flagPath, _ := cmd.Flags().GetString("file")
	if strings.TrimSpace(flagPath) != "" {
		abs, err := filepath.Abs(flagPath)
		if err != nil {
			return info, err
		}
		info.Path = abs
		info.Method = "flag"
	} else {
		resolved, found := store.Resolve("")
		info.Path = resolved
		switch {
		case found:
			info.Method = "nearest"
		case isUnderHomeDotTsk(resolved):
			info.Method = "global"
		default:
			info.Method = "cwd"
		}
	}
	if fi, err := os.Stat(info.Path); err == nil && !fi.IsDir() {
		info.Exists = true
	}
	return info, nil
}

// detectTZSource reports which env var (if any) supplied the active timezone.
func detectTZSource() string {
	if v := strings.TrimSpace(os.Getenv("TSK_TZ")); v != "" {
		return "TSK_TZ"
	}
	if v := strings.TrimSpace(os.Getenv("TZ")); v != "" {
		return "TZ"
	}
	return "system"
}

// isUnderHomeDotTsk reports whether p is under ~/.tsk (where store.Resolve
// places its "no .tsk.md found" fallback).
func isUnderHomeDotTsk(p string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	prefix := filepath.Join(home, ".tsk") + string(filepath.Separator)
	return strings.HasPrefix(p, prefix)
}

// printWhere renders the labelled human form.
func printWhere(w io.Writer, info WhereInfo) {
	exists := "yes"
	if !info.Exists {
		exists = "no (run `tsk init` to create)"
	}
	pf(w, "path:    %s\n", info.Path)
	pf(w, "method:  %s\n", info.Method)
	pf(w, "exists:  %s\n", exists)
	pf(w, "tz:      %s (%s)\n", info.Timezone, info.TZSource)
}
