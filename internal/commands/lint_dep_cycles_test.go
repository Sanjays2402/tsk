package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// spliceDeps hand-edits the .tsk.md file to inject a `depends:CSV`
// meta value on the row carrying id:N — bypassing the depend
// validator's direct-cycle / self-dep refusal so tests can craft
// 3+ node cycles the writer would otherwise reject.
func spliceDeps(t *testing.T, dir string, id int, csv string) {
	t.Helper()
	path := filepath.Join(dir, ".tsk.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	marker := "id:" + itoa(id) + " "
	lines := strings.Split(string(body), "\n")
	hit := false
	for i, l := range lines {
		if !strings.Contains(l, marker) {
			continue
		}
		if strings.Contains(l, "depends:") {
			// Replace existing depends value.
			start := strings.Index(l, "depends:")
			rest := l[start+len("depends:"):]
			end := strings.IndexAny(rest, " ")
			if end < 0 {
				t.Fatalf("malformed depends on line %d: %q", i+1, l)
			}
			lines[i] = l[:start+len("depends:")] + csv + l[start+len("depends:")+end:]
		} else {
			lines[i] = strings.Replace(l, "-->", "depends:"+csv+" -->", 1)
		}
		hit = true
		break
	}
	if !hit {
		t.Fatalf("could not find row with id:%d in %s", id, path)
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

// itoa is a tiny local helper so tests don't pull in strconv just for
// integer formatting.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// TestLintDepCyclesDetectsThreeNodeCycle: hand-edit a 1->2->3->1
// cycle into the file. `tsk lint --dep-cycles` must surface it
// even though the depend writer never rejected each individual
// edge (each one is a 2-node case the writer doesn't catch unless
// the back edge already exists).
func TestLintDepCyclesDetectsThreeNodeCycle(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"alpha", "beta", "gamma"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// Build the cycle by hand-editing every node — using `tsk depend`
	// for the second back-edge would trip the direct-cycle validator
	// for the 1->2, 2->3 chain when we try to add 3->1 in the
	// presence of 1->...->3. We splice all three at once.
	spliceDeps(t, dir, 1, "2")
	spliceDeps(t, dir, 2, "3")
	spliceDeps(t, dir, 3, "1")

	stdout, _, err := runCmd(t, dir, "lint", "--dep-cycles")
	// findings present → exit 1 (silentExit), surfaced as error
	if err == nil {
		t.Fatalf("expected exit-1 with findings, got nil\noutput:\n%s", stdout)
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 1 {
		t.Fatalf("expected exit 1 with findings, got %v", err)
	}
	if !strings.Contains(stdout, "dependency_cycle") {
		t.Fatalf("expected dependency_cycle finding, got:\n%s", stdout)
	}
	// Canonical chain: starts at #1 (smallest id), 1 -> 2 -> 3 -> 1.
	if !strings.Contains(stdout, "#1 -> #2 -> #3 -> #1") {
		t.Fatalf("expected '#1 -> #2 -> #3 -> #1' chain, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "tsk depend") {
		t.Fatalf("expected suggested fix command, got:\n%s", stdout)
	}
}

// TestLintDepCyclesNoFalsePositiveOnDAG: a normal chain 3->2->1
// has no cycle and must NOT trip the detector.
func TestLintDepCyclesNoFalsePositiveOnDAG(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2->1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend 3->2: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "lint", "--dep-cycles")
	if err != nil {
		t.Fatalf("lint should be clean for a DAG, got err=%v\n%s", err, stdout)
	}
	if strings.Contains(stdout, "dependency_cycle") {
		t.Fatalf("DAG must not surface as a cycle, got:\n%s", stdout)
	}
}

// TestLintDepCyclesIgnoresDanglingEdges: a dep pointing at a missing
// id is tolerated everywhere else in tsk (unmetBlockers, topo, etc).
// The cycle scan must apply the same policy — a dangling edge is
// not a cycle endpoint.
func TestLintDepCyclesIgnoresDanglingEdges(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "real"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Splice a dangling dep (id 99 doesn't exist).
	spliceDeps(t, dir, 1, "99")
	stdout, _, err := runCmd(t, dir, "lint", "--dep-cycles")
	if err != nil {
		t.Fatalf("dangling dep should not be a cycle, err=%v\n%s", err, stdout)
	}
	if strings.Contains(stdout, "dependency_cycle") {
		t.Fatalf("dangling dep must not produce a cycle finding, got:\n%s", stdout)
	}
}

// TestLintDepCyclesJSONShape: --json must carry the cycle finding
// in a structured form using the existing LintFinding schema.
func TestLintDepCyclesJSONShape(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	spliceDeps(t, dir, 1, "2")
	spliceDeps(t, dir, 2, "3")
	spliceDeps(t, dir, 3, "1")
	stdout, _, err := runCmd(t, dir, "lint", "--dep-cycles", "--json")
	if err == nil {
		t.Fatalf("expected exit 1 with cycle finding\n%s", stdout)
	}
	var doc map[string]any
	if jerr := json.Unmarshal([]byte(stdout), &doc); jerr != nil {
		t.Fatalf("invalid JSON: %v\n%s", jerr, stdout)
	}
	findings, ok := doc["findings"].([]any)
	if !ok {
		t.Fatalf("findings should be array, got %T", doc["findings"])
	}
	hit := false
	for _, raw := range findings {
		obj, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if check, _ := obj["check"].(string); check == "dependency_cycle" {
			hit = true
			detail, _ := obj["detail"].(string)
			if !strings.Contains(detail, "#1") || !strings.Contains(detail, "#2") || !strings.Contains(detail, "#3") {
				t.Fatalf("detail should include all cycle ids, got %q", detail)
			}
		}
	}
	if !hit {
		t.Fatalf("expected at least one dependency_cycle finding, got:\n%s", stdout)
	}
}

// TestLintDepCyclesFlagOptIn: WITHOUT --dep-cycles the scan must not
// run even if cycles exist — keeps default `tsk lint` fast on big
// stores and matches the documented opt-in contract.
func TestLintDepCyclesFlagOptIn(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	spliceDeps(t, dir, 1, "2")
	spliceDeps(t, dir, 2, "3")
	spliceDeps(t, dir, 3, "1")
	stdout, _, err := runCmd(t, dir, "lint")
	if err != nil {
		// Other findings (e.g. missing_created_timestamp on the
		// spliced rows) could still trip exit 1. Tolerate that; we
		// only care that the cycle finding is ABSENT.
		_ = err
	}
	if strings.Contains(stdout, "dependency_cycle") {
		t.Fatalf("dependency_cycle should not appear without --dep-cycles, got:\n%s", stdout)
	}
}

// TestLintDepCyclesMultipleSCCs: two independent cycles in the same
// store must each surface as their own finding.
func TestLintDepCyclesMultipleSCCs(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d", "e", "f"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// Cycle 1: 1 -> 2 -> 3 -> 1
	spliceDeps(t, dir, 1, "2")
	spliceDeps(t, dir, 2, "3")
	spliceDeps(t, dir, 3, "1")
	// Cycle 2: 4 -> 5 -> 6 -> 4
	spliceDeps(t, dir, 4, "5")
	spliceDeps(t, dir, 5, "6")
	spliceDeps(t, dir, 6, "4")

	stdout, _, err := runCmd(t, dir, "lint", "--dep-cycles")
	if err == nil {
		t.Fatalf("expected exit 1\n%s", stdout)
	}
	c1 := strings.Contains(stdout, "#1 -> #2 -> #3 -> #1")
	c2 := strings.Contains(stdout, "#4 -> #5 -> #6 -> #4")
	if !c1 || !c2 {
		t.Fatalf("expected both cycles; #1-chain=%v #4-chain=%v\noutput:\n%s",
			c1, c2, stdout)
	}
	// Cycles must be reported in ascending first-id order.
	i1 := strings.Index(stdout, "#1 -> #2 -> #3")
	i2 := strings.Index(stdout, "#4 -> #5 -> #6")
	if !(i1 < i2) {
		t.Fatalf("expected #1-cycle before #4-cycle, got positions (%d, %d)", i1, i2)
	}
}
