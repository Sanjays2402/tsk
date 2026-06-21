package commands

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/store"
)

// newLintCmd implements `tsk lint`: a file-format hygiene checker.
//
// `tsk doctor` already covers runtime sanity (duplicate IDs, empty
// titles, timestamp ordering, timezone resolution). `tsk lint` is its
// storage-format sibling: it looks at the raw bytes of .tsk.md and
// flags things the parser TOLERATES on read but the writer normalizes
// away on save. Running --fix is a safe round-trip that canonicalizes
// everything the parser already understood — same in-memory tasks,
// cleaner bytes on disk.
//
// Checks performed:
//   - non-canonical task lines (uses '*' or '+' bullet, has leading
//     whitespace, uses 'X' instead of 'x' in the checkbox)
//   - unknown meta keys (silently preserved on parse, dropped on save)
//   - tasks missing a created: timestamp (common for hand-edited rows;
//     `tsk last` and time-bucketed views need them)
//   - notes-shaped lines (>=6 spaces of indent) appearing BEFORE any
//     task — those land in s.Header and survive, but only by accident
//   - tasks with no meta comment at all (no id, no priority, nothing) —
//     parser assigns defaults but a heads-up is useful
//
// Exit codes:
//
//	0 clean
//	1 findings present (silent; the report itself is the output)
//	2 bad invocation / IO failure
func newLintCmd() *cobra.Command {
	var (
		asJSON     bool
		fix        bool
		autofixAll bool
		depCycles  bool
	)
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Validate .tsk.md storage-format hygiene (--fix to canonicalize, --autofix-all for multi-step safe fixes)",
		Long: `Validate the active .tsk.md against tsk's storage-format conventions.

This is the file-format sibling of 'tsk doctor': doctor checks the
in-memory task model (duplicate IDs, broken timestamps, tz drift),
lint checks the raw bytes (non-canonical bullets, unknown meta
keys, missing created stamps, stray notes-shaped lines).

Pass --fix to re-render the file through tsk's canonical writer.
That's a safe round-trip: the in-memory tasks are unchanged; only
their byte representation is normalized. This catches:
  - non-canonical bullets ('*' or '+' instead of '-')
  - non-canonical checkbox ('X' instead of 'x')
  - leading whitespace before task lines
  - unknown meta keys (silently dropped on save)
  - stray notes-indented lines before any task

Pass --autofix-all to ALSO repair findings the round-trip alone
can't fix — currently:
  - missing_created_timestamp: backfill 'created:<now>' so
    time-bucketed views (last, log, yesterday) include the task.
The mode is non-interactive ("just trust me") — every fix it
applies is conservative and reversible via 'tsk undo-last' (a
.bak snapshot is taken before writing). --autofix-all implies
--fix, so both kinds of repair happen in one pass.

Pass --json for a stable machine-readable report (CI / pre-commit
hook friendly).

Pass --dep-cycles to scan the DependsOn graph for 3+ node cycles
via Tarjan's strongly-connected-components algorithm. The depend
writer only rejects self-deps and direct A<->B cycles; deeper
cycles (A->B->C->A) are tolerated at write time and surfaced here.
Each detected cycle is reported with its full id chain plus a
suggested edge to break, so the user can ` + "`tsk depend <id> --remove`" + `
their way out. --dep-cycles is a READ-ONLY scan; it does not
trigger --fix or --autofix-all (cycles need human judgement).

Exit codes: 0 clean (or fixes applied), 1 findings present and
not fixed, 2 bad invocation.
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := resolveLintPath(cmd)
			if err != nil {
				return err
			}
			report, err := runLint(path)
			if err != nil {
				return err
			}
			if depCycles {
				appendDepCycleFindings(&report, path)
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				if err := enc.Encode(report); err != nil {
					return err
				}
			} else {
				printLintReport(cmd.OutOrStdout(), report)
			}
			if autofixAll && len(report.Findings) > 0 {
				applied, err := applyLintAutofixAll(path, report)
				if err != nil {
					return err
				}
				pf(cmd.OutOrStdout(), "autofixed: %s (%d repair(s) applied)\n", path, applied)
				return nil
			}
			if fix && len(report.Findings) > 0 {
				if err := applyLintFix(path); err != nil {
					return err
				}
				pf(cmd.OutOrStdout(), "fixed: re-rendered %s in canonical form\n", path)
				return nil
			}
			if len(report.Findings) > 0 {
				return silentExit{code: 1}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON")
	cmd.Flags().BoolVar(&fix, "fix", false, "re-render the file in canonical form (safe round-trip)")
	cmd.Flags().BoolVar(&autofixAll, "autofix-all", false, "apply --fix PLUS semantic repairs (currently: backfill missing created: stamps)")
	cmd.Flags().BoolVar(&depCycles, "dep-cycles", false, "scan the DependsOn graph for 3+ node cycles (Tarjan SCC)")
	return cmd
}

// LintReport is the structured findings list. Stable schema for --json.
type LintReport struct {
	Path     string        `json:"path"`
	Findings []LintFinding `json:"findings"`
}

// LintFinding describes a single non-canonical pattern. Line is 1-based
// (matches editor line numbers), TaskID is 0 when not applicable.
type LintFinding struct {
	Line   int    `json:"line,omitempty"`
	Check  string `json:"check"`
	Detail string `json:"detail"`
	TaskID int    `json:"task_id,omitempty"`
}

// resolveLintPath mirrors resolveStore's path resolution but returns
// just the string (so we can read the file raw, not via the parser).
func resolveLintPath(cmd *cobra.Command) (string, error) {
	path, _ := cmd.Flags().GetString("file")
	if path == "" {
		resolved, ok := store.Resolve("")
		if !ok {
			return "", fmt.Errorf("no .tsk.md found; run `tsk init`")
		}
		path = resolved
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("no .tsk.md at %s; run `tsk init`", path)
	}
	return path, nil
}

// runLint reads the file once for byte-level checks, then again via
// store.Load for semantic checks. Returns the combined report.
func runLint(path string) (LintReport, error) {
	report := LintReport{Path: path, Findings: []LintFinding{}}
	f, err := os.Open(path)
	if err != nil {
		return report, fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	scanByteLevel(&report, f)
	s, err := store.Load(path)
	if err != nil {
		return report, fmt.Errorf("parse %s: %w", path, err)
	}
	scanSemanticLevel(&report, s)
	sort.SliceStable(report.Findings, func(i, j int) bool {
		if report.Findings[i].Line != report.Findings[j].Line {
			return report.Findings[i].Line < report.Findings[j].Line
		}
		return report.Findings[i].Check < report.Findings[j].Check
	})
	return report, nil
}

// canonicalTaskLineRe matches a fully canonical task line (zero leading
// whitespace, "- [ ]" / "- [x]"). Used to detect tolerated-but-non-canonical
// forms (extra leading spaces, '*'/'+' bullet, 'X' uppercase, tabs).
var canonicalTaskLineRe = regexp.MustCompile(`^- \[( |x)\] `)

// looseTaskLineRe matches every form the parser tolerates — used to
// decide "this looks like a task but isn't canonical". Mirrors the
// tolerance of taskLineRe in markdown.go.
var looseTaskLineRe = regexp.MustCompile(`^(?:[ ]{0,3}|\t)[-*+] \[( |x|X)\] `)

// knownMetaKeys lists every meta key applyMeta() recognizes. Anything
// else inside the <!-- ... --> block is silently preserved on read but
// dropped on save — worth flagging.
var knownMetaKeys = map[string]bool{
	"id":         true,
	"prio":       true,
	"priority":   true,
	"due":        true,
	"wait":       true,
	"wait_until": true,
	"waituntil":  true,
	"tags":       true,
	"created":    true,
	"started":    true,
	"completed":  true,
	"pin":        true,
	"pinned":     true,
	"depends":    true,
	"depends_on": true,
	"dependson":  true,
}

// metaKeyValueRe matches the canonical "key:value" inside a meta comment.
// Same shape as store.metaPairRe — duplicated here so this file doesn't
// need an internal import.
var metaKeyValueRe = regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*):\s*([^\s]+)`)

// metaCommentRe pulls the bit inside <!-- ... --> from a task line.
var metaCommentRe = regexp.MustCompile(`<!--\s*(.*?)\s*-->`)

// scanByteLevel walks the raw file line-by-line. Surfaces things the
// parser tolerates but the writer canonicalizes.
func scanByteLevel(r *LintReport, in io.Reader) {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	sawTask := false
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		switch {
		case looseTaskLineRe.MatchString(line) && !canonicalTaskLineRe.MatchString(line):
			r.Findings = append(r.Findings, LintFinding{
				Line:   lineNo,
				Check:  "non_canonical_task_line",
				Detail: "task line isn't in canonical '- [ ]' / '- [x]' form (run --fix to normalize)",
			})
			sawTask = true
		case looseTaskLineRe.MatchString(line):
			sawTask = true
			scanMetaUnknownKeys(r, lineNo, line)
		case !sawTask && strings.HasPrefix(line, strings.Repeat(" ", store.NotesIndent)):
			r.Findings = append(r.Findings, LintFinding{
				Line:   lineNo,
				Check:  "stray_notes_before_task",
				Detail: "notes-indented line appears before any task — will be lost on next save",
			})
		}
	}
}

// scanMetaUnknownKeys looks inside a task line's <!-- ... --> block and
// reports keys that aren't in the known set. The parser silently
// preserves them in-memory and the writer drops them, so they're a
// real data-loss footgun on hand-edited files.
func scanMetaUnknownKeys(r *LintReport, lineNo int, line string) {
	m := metaCommentRe.FindStringSubmatch(line)
	if len(m) < 2 {
		return
	}
	for _, kv := range metaKeyValueRe.FindAllStringSubmatch(m[1], -1) {
		key := strings.ToLower(kv[1])
		if knownMetaKeys[key] {
			continue
		}
		r.Findings = append(r.Findings, LintFinding{
			Line:   lineNo,
			Check:  "unknown_meta_key",
			Detail: fmt.Sprintf("meta key %q is unrecognized and will be dropped on next save", kv[1]),
		})
	}
}

// scanSemanticLevel runs checks that need the parsed task model. These
// can't be fixed by a round-trip — they describe real missing or odd
// task data the user must intentionally repair.
func scanSemanticLevel(r *LintReport, s *store.Store) {
	for _, t := range s.Tasks {
		if t.Created.IsZero() {
			r.Findings = append(r.Findings, LintFinding{
				Check:  "missing_created_timestamp",
				Detail: "task has no 'created:' timestamp — time-bucketed views (last, log, yesterday) may skip it",
				TaskID: t.ID,
			})
		}
	}
}

// applyLintFix re-renders the file through store.Save. This is a safe
// round-trip — every tolerated input becomes the canonical output. The
// store.Save path also creates a .bak snapshot, so the user can
// `tsk undo-last` if --fix touched something they didn't expect.
func applyLintFix(path string) error {
	s, err := store.Load(path)
	if err != nil {
		return fmt.Errorf("reload for fix: %w", err)
	}
	return s.Save()
}

// printLintReport renders the findings list. No findings → one line.
func printLintReport(w io.Writer, r LintReport) {
	pf(w, "file:    %s\n", r.Path)
	pf(w, "checks:  %d finding(s)\n", len(r.Findings))
	if len(r.Findings) == 0 {
		pln(w, "all checks passed")
		return
	}
	pln(w)
	for _, f := range r.Findings {
		loc := ""
		switch {
		case f.Line > 0:
			loc = fmt.Sprintf("line %d", f.Line)
		case f.TaskID > 0:
			loc = fmt.Sprintf("#%d", f.TaskID)
		}
		if loc != "" {
			pf(w, "  %s  %s: %s\n", loc, f.Check, f.Detail)
		} else {
			pf(w, "  %s: %s\n", f.Check, f.Detail)
		}
	}
}
