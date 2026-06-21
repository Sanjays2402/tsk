package commands

import (
	"encoding/json"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/store"
)

// newEnvCmd implements `tsk env`: dump the effective configuration
// tsk is operating under right now — which file, which timezone,
// which editor will open on `tsk note`, whether color is suppressed,
// etc.
//
// Why it exists: when something behaves unexpectedly the first
// debug question is always "what state were you in?". `where` covers
// the file half; `env` covers everything else (TZ, EDITOR, NO_COLOR,
// runtime version, home dir). Together they answer "why did tsk do
// THAT" without running strace.
//
// Output is grouped by category in plain mode and a flat JSON object
// keyed identically in --json mode. The JSON schema is stable so
// shell prompts / dashboards can pull individual fields with jq.
func newEnvCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Print the effective env tsk is running under (TZ, EDITOR, paths, ...)",
		Long: `Print the effective configuration tsk is operating under.

Groups: file paths, timezone, editor, color, runtime. Together with
` + "`tsk where`" + ` this is the canonical "why did tsk do that?" debug snapshot.

Pass --json for a stable schema scriptable by jq:
  tsk env --json | jq -r '.editor.resolved'

Empty env vars are reported explicitly as "(unset)" so a missing value
is distinguishable from an empty one.
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			info := buildEnvInfo(cmd)
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(info)
			}
			printEnvInfo(cmd.OutOrStdout(), info)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	return cmd
}

// EnvFiles groups path-related env (mirrors what `where` reports but
// also surfaces home dir + global file location for completeness).
type EnvFiles struct {
	FlagPath   string `json:"flag_path"`   // --file value, empty if unset
	Resolved   string `json:"resolved"`    // actual file tsk would open
	Method     string `json:"method"`      // flag, nearest, global, cwd
	Exists     bool   `json:"exists"`      // does the resolved file exist?
	GlobalPath string `json:"global_path"` // ~/.tsk/global.md
	HomeDir    string `json:"home_dir"`    // os.UserHomeDir() result
}

// EnvTimezone reports which TZ tsk parses dates in and why.
type EnvTimezone struct {
	Resolved string `json:"resolved"` // IANA name (e.g. "America/Los_Angeles")
	Source   string `json:"source"`   // TSK_TZ | TZ | system
	TSKTZ    string `json:"tsk_tz"`   // raw env var value or "(unset)"
	TZ       string `json:"tz"`       // raw env var value or "(unset)"
}

// EnvEditor reports what `tsk note` / `tsk edit` will open.
type EnvEditor struct {
	Resolved string `json:"resolved"` // the chosen editor or "(none)"
	Source   string `json:"source"`   // VISUAL | EDITOR | fallback
	VISUAL   string `json:"visual"`   // raw or "(unset)"
	EDITOR   string `json:"editor"`   // raw or "(unset)"
}

// EnvColor reports whether tsk's colored output is suppressed.
type EnvColor struct {
	Disabled bool   `json:"disabled"` // true if NO_COLOR is set non-empty
	NOCOLOR  string `json:"no_color"` // raw or "(unset)"
}

// EnvRuntime reports the Go runtime + tsk version.
type EnvRuntime struct {
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	Date      string `json:"date"`
}

// EnvInfo is the JSON-stable schema returned by --json.
type EnvInfo struct {
	Files    EnvFiles    `json:"files"`
	Timezone EnvTimezone `json:"timezone"`
	Editor   EnvEditor   `json:"editor"`
	Color    EnvColor    `json:"color"`
	Runtime  EnvRuntime  `json:"runtime"`
	Env      []string    `json:"env"` // sorted TSK_* env vars (key=value)
}

// buildEnvInfo collects everything env-related into one struct so plain
// + JSON paths share the source of truth.
func buildEnvInfo(cmd *cobra.Command) EnvInfo {
	home, _ := os.UserHomeDir()

	flagPath, _ := cmd.Flags().GetString("file")
	info := EnvInfo{
		Files: EnvFiles{
			FlagPath:   flagPath,
			GlobalPath: globalPath(home),
			HomeDir:    home,
		},
		Timezone: EnvTimezone{
			Resolved: ResolveTZ().String(),
			Source:   detectTZSource(),
			TSKTZ:    orUnset(os.Getenv("TSK_TZ")),
			TZ:       orUnset(os.Getenv("TZ")),
		},
		Editor:  buildEditorInfo(),
		Color:   buildColorInfo(),
		Runtime: buildRuntimeInfo(),
		Env:     collectTSKEnvVars(),
	}
	resolveEnvFilePath(&info, flagPath)
	return info
}

// resolveEnvFilePath runs the same file-resolution logic the rest of
// tsk uses so `tsk env` agrees with what `tsk add` would actually edit.
func resolveEnvFilePath(info *EnvInfo, flagPath string) {
	if strings.TrimSpace(flagPath) != "" {
		info.Files.Resolved = flagPath
		info.Files.Method = "flag"
	} else {
		resolved, found := store.Resolve("")
		info.Files.Resolved = resolved
		switch {
		case found:
			info.Files.Method = "nearest"
		case isUnderHomeDotTsk(resolved):
			info.Files.Method = "global"
		default:
			info.Files.Method = "cwd"
		}
	}
	if fi, err := os.Stat(info.Files.Resolved); err == nil && !fi.IsDir() {
		info.Files.Exists = true
	}
}

// buildEditorInfo replicates the editor-resolution logic note/edit use:
// VISUAL beats EDITOR beats system fallback. Tracks the source so users
// can see which env var won.
func buildEditorInfo() EnvEditor {
	e := EnvEditor{
		Resolved: "(none)",
		Source:   "fallback",
		VISUAL:   orUnset(os.Getenv("VISUAL")),
		EDITOR:   orUnset(os.Getenv("EDITOR")),
	}
	if v := strings.TrimSpace(os.Getenv("VISUAL")); v != "" {
		e.Resolved = v
		e.Source = "VISUAL"
		return e
	}
	if v := strings.TrimSpace(os.Getenv("EDITOR")); v != "" {
		e.Resolved = v
		e.Source = "EDITOR"
		return e
	}
	return e
}

// buildColorInfo reports the NO_COLOR convention (no_color.org): if the
// env var is set to a non-empty value, colored output is disabled.
func buildColorInfo() EnvColor {
	raw := os.Getenv("NO_COLOR")
	return EnvColor{
		Disabled: strings.TrimSpace(raw) != "",
		NOCOLOR:  orUnset(raw),
	}
}

// buildRuntimeInfo reports the Go runtime + build-time tsk version.
func buildRuntimeInfo() EnvRuntime {
	return EnvRuntime{
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Version:   buildVersion,
		Commit:    buildCommit,
		Date:      buildDate,
	}
}

// collectTSKEnvVars returns every TSK_*-prefixed env var (sorted, in
// KEY=VALUE form) so tsk's surface area is visible at a glance. Useful
// when adding new TSK_* knobs — users see them immediately on `tsk env`.
func collectTSKEnvVars() []string {
	out := []string{}
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "TSK_") {
			out = append(out, e)
		}
	}
	sort.Strings(out)
	if out == nil {
		out = []string{}
	}
	return out
}

// orUnset replaces empty strings with "(unset)" so JSON/plain output
// distinguishes unset env vars from empty ones.
func orUnset(v string) string {
	if v == "" {
		return "(unset)"
	}
	return v
}

// globalPath returns where tsk would put the global fallback file
// (~/.tsk/global.md). Empty if home dir can't be determined.
func globalPath(home string) string {
	if home == "" {
		return ""
	}
	return home + "/.tsk/global.md"
}

// printEnvInfo renders the human-readable grouped form.
func printEnvInfo(w io.Writer, info EnvInfo) {
	pln(w, "[files]")
	pf(w, "  resolved: %s\n", info.Files.Resolved)
	pf(w, "  method:   %s\n", info.Files.Method)
	pf(w, "  exists:   %s\n", yesNo(info.Files.Exists))
	pf(w, "  flag:     %s\n", orDash(info.Files.FlagPath))
	pf(w, "  global:   %s\n", orDash(info.Files.GlobalPath))
	pf(w, "  home:     %s\n", orDash(info.Files.HomeDir))

	pln(w, "")
	pln(w, "[timezone]")
	pf(w, "  resolved: %s\n", info.Timezone.Resolved)
	pf(w, "  source:   %s\n", info.Timezone.Source)
	pf(w, "  TSK_TZ:   %s\n", info.Timezone.TSKTZ)
	pf(w, "  TZ:       %s\n", info.Timezone.TZ)

	pln(w, "")
	pln(w, "[editor]")
	pf(w, "  resolved: %s\n", info.Editor.Resolved)
	pf(w, "  source:   %s\n", info.Editor.Source)
	pf(w, "  VISUAL:   %s\n", info.Editor.VISUAL)
	pf(w, "  EDITOR:   %s\n", info.Editor.EDITOR)

	pln(w, "")
	pln(w, "[color]")
	pf(w, "  disabled: %s\n", yesNo(info.Color.Disabled))
	pf(w, "  NO_COLOR: %s\n", info.Color.NOCOLOR)

	pln(w, "")
	pln(w, "[runtime]")
	pf(w, "  go:       %s (%s/%s)\n", info.Runtime.GoVersion, info.Runtime.OS, info.Runtime.Arch)
	pf(w, "  version:  %s\n", info.Runtime.Version)
	pf(w, "  commit:   %s\n", info.Runtime.Commit)
	pf(w, "  date:     %s\n", info.Runtime.Date)

	if len(info.Env) > 0 {
		pln(w, "")
		pln(w, "[TSK_* env]")
		for _, kv := range info.Env {
			pf(w, "  %s\n", kv)
		}
	}
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func orDash(v string) string {
	if v == "" {
		return "-"
	}
	return v
}
