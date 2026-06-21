package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestJustifyTopLevelMatchesDependJustify: `tsk justify <id>` must
// produce byte-identical output to `tsk depend <id> --justify`. The
// top-level verb is a forwarder; if these ever diverge it's a real bug.
func TestJustifyTopLevelMatchesDependJustify(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq", "blocked"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	viaSubflag, _, err := runCmd(t, dir, "depend", "2", "--justify")
	if err != nil {
		t.Fatalf("depend --justify: %v", err)
	}
	viaTopLevel, _, err := runCmd(t, dir, "justify", "2")
	if err != nil {
		t.Fatalf("justify 2: %v", err)
	}
	if viaSubflag != viaTopLevel {
		t.Fatalf("top-level justify must match depend --justify byte-for-byte.\nsubflag:\n%s\ntoplevel:\n%s",
			viaSubflag, viaTopLevel)
	}
}

// TestJustifyAllEmitsEveryBlockedChain: --all walks every open
// blocked task in id order, prefixing each chain with a header so
// the boundaries are scannable.
func TestJustifyAllEmitsEveryBlockedChain(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq-a", "prereq-b", "blocked-a", "blocked-b", "free"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	// #3 depends on #1, #4 depends on #2. #5 has no deps.
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1"); err != nil {
		t.Fatalf("depend 3: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "2"); err != nil {
		t.Fatalf("depend 4: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "justify", "--all")
	if err != nil {
		t.Fatalf("justify --all: %v", err)
	}
	// Both blocked roots appear with their headers.
	for _, want := range []string{"=== #3 blocked-a ===", "=== #4 blocked-b ==="} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected header %q, got:\n%s", want, stdout)
		}
	}
	// Free task #5 is NOT in the output.
	if strings.Contains(stdout, "=== #5") {
		t.Fatalf("#5 has no deps and must not appear in --all, got:\n%s", stdout)
	}
	// And the per-chain content (the "blocked by" hop) is present.
	if !strings.Contains(stdout, "blocked by:") {
		t.Fatalf("expected chain content, got:\n%s", stdout)
	}
	// Order: #3 header before #4 header (id-ascending).
	i3 := strings.Index(stdout, "=== #3")
	i4 := strings.Index(stdout, "=== #4")
	if i3 < 0 || i4 < 0 || i3 > i4 {
		t.Fatalf("expected #3 header before #4 header, got positions (3=%d, 4=%d):\n%s", i3, i4, stdout)
	}
}

// TestJustifyAllEmptyMessageWhenNothingBlocked: when no task is
// blocked, the plain path emits a clear message instead of staying
// silent (silent output would be ambiguous: "did the command run?").
func TestJustifyAllEmptyMessageWhenNothingBlocked(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "lonely"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "justify", "--all")
	if err != nil {
		t.Fatalf("justify --all: %v", err)
	}
	if !strings.Contains(stdout, "no blocked tasks") {
		t.Fatalf("expected 'no blocked tasks' message, got:\n%s", stdout)
	}
}

// TestJustifyAllJSONMapShape: --all --json emits an object keyed by
// task id (string) with chain arrays as values. Empty input gives
// `{}`, never null.
func TestJustifyAllJSONMapShape(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq", "blocked"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "justify", "--all", "--json")
	if err != nil {
		t.Fatalf("justify --all --json: %v", err)
	}
	var doc map[string][]map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	chain, ok := doc["2"]
	if !ok {
		t.Fatalf("expected key \"2\" in map, got keys: %v\n%s", keysOf(doc), stdout)
	}
	if len(chain) != 2 {
		t.Fatalf("expected chain of length 2, got %d:\n%s", len(chain), stdout)
	}
	if status, _ := chain[0]["status"].(string); status != "blocked" {
		t.Fatalf("expected root status=blocked, got %v", chain[0])
	}
	if status, _ := chain[1]["status"].(string); status != "open-leaf" {
		t.Fatalf("expected leaf status=open-leaf, got %v", chain[1])
	}
}

// TestJustifyAllJSONEmptyObject: zero blocked tasks must emit `{}`
// (not `null`), so jq `keys[]` and `to_entries` work without crashing.
func TestJustifyAllJSONEmptyObject(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "lonely"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "justify", "--all", "--json")
	if err != nil {
		t.Fatalf("justify --all --json: %v", err)
	}
	trimmed := strings.TrimSpace(stdout)
	if trimmed != "{}" {
		t.Fatalf("expected empty object {}, got %q", trimmed)
	}
}

// TestJustifyRequiresIDOrAll: with no positional id AND no --all,
// the command must error (otherwise what would it even justify?).
func TestJustifyRequiresIDOrAll(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "justify")
	if err == nil {
		t.Fatal("expected error: justify with no id and no --all")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2 (usage error), got %v", err)
	}
}

// TestJustifyAllRejectsPositionalID: --all is whole-store; combining
// it with an id is contradictory.
func TestJustifyAllRejectsPositionalID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "justify", "1", "--all")
	if err == nil {
		t.Fatal("expected error: justify <id> --all is contradictory")
	}
}

// TestJustifyOnlyBlockedRoots: --all skips done tasks, waiting
// tasks, and tasks with no deps. Only OPEN tasks with at least one
// unmet prereq qualify as "blocked roots".
func TestJustifyOnlyBlockedRoots(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq", "done-dependent", "blocked-dependent", "free-task"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1"); err != nil {
		t.Fatalf("depend 3: %v", err)
	}
	// Mark #1 done -> #2 should become done-eligible -> mark it.
	if _, _, err := runCmd(t, dir, "done", "1", "2"); err != nil {
		t.Fatalf("done 1,2: %v", err)
	}
	// Re-undo #1 so #3 is once again blocked. #2 is still done.
	if _, _, err := runCmd(t, dir, "undo", "1"); err != nil {
		t.Fatalf("undo: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "justify", "--all")
	if err != nil {
		t.Fatalf("justify --all: %v", err)
	}
	// Only #3 should be in the output (open + still blocked).
	if !strings.Contains(stdout, "=== #3") {
		t.Fatalf("expected #3 header (blocked-dependent), got:\n%s", stdout)
	}
	if strings.Contains(stdout, "=== #2") {
		t.Fatalf("done #2 must not appear, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "=== #4") {
		t.Fatalf("free #4 must not appear, got:\n%s", stdout)
	}
}

// keysOf is a tiny test helper for surfacing map keys in error
// messages so failures point at the missing key precisely.
func keysOf(m map[string][]map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
