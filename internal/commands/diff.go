package commands

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// newDiffCmd implements `tsk diff`: show what `tsk undo-last` would
// revert by diffing the live .tsk.md against its .bak snapshot.
//
// The .bak snapshot is the file as it was BEFORE the most recent save
// (add/done/edit/etc all write a snapshot first, then the change).
// So `tsk diff` shows the most recent change. Pair with `tsk undo-last`
// when you want to inspect before reverting.
//
// Output is a unified diff with "BEFORE" (= .bak) and "AFTER" (= live):
//
//	--- /path/.tsk.md.bak (snapshot)
//	+++ /path/.tsk.md (current)
//	@@ -3,1 +3,2 @@
//	 - [ ] something <!-- id:1 prio:medium -->
//	+- [ ] new thing <!-- id:2 prio:high -->
//
// Designed for the very common "wait, what did that command actually
// change?" workflow. No external `diff` binary required — uses a
// minimal LCS implementation inlined here so it works on stripped
// containers, BSD systems, anywhere tsk runs.
func newDiffCmd() *cobra.Command {
	var (
		context  int
		stat     bool
		nameOnly bool
	)
	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Show what `tsk undo-last` would revert (live vs .bak)",
		Long: `Show the most recent change to the task file as a unified diff.

Compares the live .tsk.md against the .bak snapshot tsk writes before
every save. The diff direction is BEFORE -> AFTER:
  - lines marked '+' were added by the most recent save
  - lines marked '-' were removed by it

  --context N   how many surrounding lines to show (default 3)
  --stat        only print "+A -B" line counts, not the full diff
  --name-only   only print the path if there's any difference

Exit codes:
  0  no differences (snapshot equals live)
  1  differences found (printed)
  2  no snapshot exists (.bak missing) — nothing to compare

Examples:
  tsk done 3
  tsk diff               # see what done 3 changed
  tsk diff --stat        # just the count
  tsk undo-last          # revert it
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if context < 0 {
				return usageErrorf("--context must be >= 0, got %d", context)
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			path := s.Path
			bak := path + ".bak"

			if _, err := os.Stat(bak); errors.Is(err, os.ErrNotExist) {
				return usageErrorf("no snapshot at %s — nothing to diff", bak)
			} else if err != nil {
				return fmt.Errorf("stat %s: %w", bak, err)
			}

			liveBytes, err := os.ReadFile(path)
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("read live: %w", err)
			}
			bakBytes, err := os.ReadFile(bak)
			if err != nil {
				return fmt.Errorf("read snapshot: %w", err)
			}

			before := splitLines(string(bakBytes))
			after := splitLines(string(liveBytes))

			added, removed := lineCounts(before, after)

			out := cmd.OutOrStdout()
			if added == 0 && removed == 0 {
				if !nameOnly && !stat {
					pln(out, "no changes")
				}
				return nil
			}

			switch {
			case nameOnly:
				pln(out, path)
			case stat:
				pf(out, "%s: +%d -%d\n", path, added, removed)
			default:
				pf(out, "--- %s (snapshot)\n", bak)
				pf(out, "+++ %s (current)\n", path)
				writeUnifiedDiff(out, before, after, context)
			}
			// Non-zero exit to mirror `git diff --exit-code` so scripts
			// can branch on "is there a pending change?" without parsing.
			// SilentExitCoder so main.go doesn't print "error: " (the
			// diff itself is the output; the non-zero is just a signal).
			return silentExit{code: 1}
		},
	}
	cmd.Flags().IntVar(&context, "context", 3, "lines of unchanged context around each hunk")
	cmd.Flags().BoolVar(&stat, "stat", false, "only show +A -B count, not the full diff")
	cmd.Flags().BoolVar(&nameOnly, "name-only", false, "only print the path if anything differs")
	return cmd
}

// splitLines splits s into lines. Preserves the empty trailing element
// only if the source ends with newline — important so diff line counts
// match what a user sees in their editor.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	// Trailing newline produces a "" element — drop it so line counts
	// match wc -l semantics.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// lineCounts returns (added, removed) computed via the same LCS
// edit script the diff writer uses. Walking the script once would
// be more efficient but keeping these split for readability — diffs
// here are O(hundreds of lines) at most.
func lineCounts(before, after []string) (added, removed int) {
	ops := diffScript(before, after)
	for _, op := range ops {
		switch op.kind {
		case opAdd:
			added++
		case opDel:
			removed++
		}
	}
	return added, removed
}

// opKind tags each step of the edit script.
type opKind int

const (
	opEq opKind = iota
	opAdd
	opDel
)

// diffOp is a single step in the LCS edit script.
type diffOp struct {
	kind opKind
	text string
	// Source/destination indices for hunk header math (0-based).
	srcIdx, dstIdx int
}

// diffScript produces a Myers-like LCS edit script for two line slices.
// Uses DP table — for the small inputs we expect (a .tsk.md file, hundreds
// of lines at most) the O(n*m) memory is fine and avoids the complexity
// of a true Myers implementation.
func diffScript(a, b []string) []diffOp {
	m, n := len(a), len(b)
	// dp[i][j] = LCS length of a[i:] and b[j:].
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := m - 1; i >= 0; i-- {
		for j := n - 1; j >= 0; j-- {
			if a[i] == b[j] {
				dp[i][j] = dp[i+1][j+1] + 1
			} else if dp[i+1][j] >= dp[i][j+1] {
				dp[i][j] = dp[i+1][j]
			} else {
				dp[i][j] = dp[i][j+1]
			}
		}
	}
	ops := make([]diffOp, 0, m+n)
	i, j := 0, 0
	for i < m && j < n {
		switch {
		case a[i] == b[j]:
			ops = append(ops, diffOp{kind: opEq, text: a[i], srcIdx: i, dstIdx: j})
			i++
			j++
		case dp[i+1][j] >= dp[i][j+1]:
			ops = append(ops, diffOp{kind: opDel, text: a[i], srcIdx: i, dstIdx: j})
			i++
		default:
			ops = append(ops, diffOp{kind: opAdd, text: b[j], srcIdx: i, dstIdx: j})
			j++
		}
	}
	for ; i < m; i++ {
		ops = append(ops, diffOp{kind: opDel, text: a[i], srcIdx: i, dstIdx: j})
	}
	for ; j < n; j++ {
		ops = append(ops, diffOp{kind: opAdd, text: b[j], srcIdx: i, dstIdx: j})
	}
	return ops
}

// writeUnifiedDiff emits the ops as classic unified-diff hunks. Each
// hunk is a contiguous run of changes plus `context` lines of context
// on each side; adjacent runs whose context overlaps are merged.
func writeUnifiedDiff(w io.Writer, before, after []string, context int) {
	ops := diffScript(before, after)
	hunks := groupHunks(ops, context)
	for _, h := range hunks {
		writeHunk(w, h)
	}
}

// hunk is a chunk of ops with computed source/dest start lines.
type hunk struct {
	ops                []diffOp
	srcStart, dstStart int
	srcLen, dstLen     int
}

// groupHunks walks the op script and splits it into hunks separated by
// runs of unchanged context longer than 2*context. Returns hunks with
// up to `context` equal-lines on each side of each change.
func groupHunks(ops []diffOp, context int) []hunk {
	if context < 0 {
		context = 0
	}
	// First, find indices of changed ops.
	changed := []int{}
	for i, op := range ops {
		if op.kind != opEq {
			changed = append(changed, i)
		}
	}
	if len(changed) == 0 {
		return nil
	}
	// Group changed indices into clusters that share context.
	type cluster struct{ first, last int }
	clusters := []cluster{}
	cur := cluster{first: changed[0], last: changed[0]}
	for k := 1; k < len(changed); k++ {
		if changed[k]-cur.last <= 2*context+1 {
			cur.last = changed[k]
		} else {
			clusters = append(clusters, cur)
			cur = cluster{first: changed[k], last: changed[k]}
		}
	}
	clusters = append(clusters, cur)
	// Convert each cluster to a hunk by expanding context bounds.
	out := make([]hunk, 0, len(clusters))
	for _, c := range clusters {
		start := c.first - context
		if start < 0 {
			start = 0
		}
		end := c.last + context
		if end >= len(ops) {
			end = len(ops) - 1
		}
		h := hunk{}
		h.ops = append(h.ops, ops[start:end+1]...)
		// Derive 1-based hunk header counts.
		h.srcStart = ops[start].srcIdx + 1
		h.dstStart = ops[start].dstIdx + 1
		for _, op := range h.ops {
			switch op.kind {
			case opEq:
				h.srcLen++
				h.dstLen++
			case opDel:
				h.srcLen++
			case opAdd:
				h.dstLen++
			}
		}
		// If the hunk starts purely with a delete or add (no eq lead-in),
		// the srcStart/dstStart from the op may be a step too low for
		// add-only hunks (which inherit the previous src position).
		// Special case: empty source segments get a 0 start per diff
		// convention.
		if h.srcLen == 0 {
			h.srcStart = ops[start].srcIdx
		}
		if h.dstLen == 0 {
			h.dstStart = ops[start].dstIdx
		}
		out = append(out, h)
	}
	return out
}

// writeHunk renders one hunk in unified-diff format.
func writeHunk(w io.Writer, h hunk) {
	pf(w, "@@ -%d,%d +%d,%d @@\n", h.srcStart, h.srcLen, h.dstStart, h.dstLen)
	for _, op := range h.ops {
		switch op.kind {
		case opEq:
			pf(w, " %s\n", op.text)
		case opAdd:
			pf(w, "+%s\n", op.text)
		case opDel:
			pf(w, "-%s\n", op.text)
		}
	}
}
