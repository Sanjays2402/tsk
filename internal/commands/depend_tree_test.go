package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDependTreeRendersChain: A → B → C should render with B and C
// indented under A, deepest leaf last.
func TestDependTreeRendersChain(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"deep", "middle", "top"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	// 3 depends on 2; 2 depends on 1. Tree from id=3 should walk down.
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend 3: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "3", "--tree")
	if err != nil {
		t.Fatalf("depend tree: %v", err)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d:\n%s", len(lines), stdout)
	}
	// Root is flush-left; descendants are indented.
	if strings.HasPrefix(lines[0], " ") {
		t.Fatalf("root should be flush-left, got: %q", lines[0])
	}
	if !strings.HasPrefix(lines[1], "└─ ") {
		t.Fatalf("depth-1 child should start with '└─ ', got: %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "  └─ ") {
		t.Fatalf("depth-2 grandchild should start with '  └─ ', got: %q", lines[2])
	}
	// Verify the actual task references are correct.
	if !strings.Contains(lines[0], "#3") || !strings.Contains(lines[1], "#2") || !strings.Contains(lines[2], "#1") {
		t.Fatalf("expected 3→2→1 chain, got:\n%s", stdout)
	}
}

// TestDependTreeShowsDoneState: completed prereqs render with `[x]`
// so the user can see what's done vs open inside the chain.
func TestDependTreeShowsDoneState(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "2", "--tree")
	if err != nil {
		t.Fatalf("depend tree: %v", err)
	}
	// Task 2 is undone "[ ]", task 1 is done "[x]" and indented.
	if !strings.Contains(stdout, "#2 [ ]") {
		t.Fatalf("expected '#2 [ ]' for open root, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "└─ #1 [x]") {
		t.Fatalf("expected '└─ #1 [x]' for done prereq, got:\n%s", stdout)
	}
}

// TestDependTreeMissingDep: a dep id with no task should be marked
// "(missing)" rather than crashing or pretending it doesn't exist.
func TestDependTreeMissingDep(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"keeper", "future"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	// Hand-edit the file to add a dangling dep (id 99).
	path := filepath.Join(dir, ".tsk.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Splice in another dep id by adding 99 onto task 2's depends list.
	body = []byte(strings.Replace(string(body), "depends:1", "depends:1,99", 1))
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "2", "--tree")
	if err != nil {
		t.Fatalf("depend tree: %v", err)
	}
	if !strings.Contains(stdout, "#99 (missing") {
		t.Fatalf("expected '#99 (missing...)' marker, got:\n%s", stdout)
	}
}

// TestDependTreeCycleSafe: a deep cycle (which the writer doesn't
// prevent on hand-edits) must not loop the renderer. We construct
// A → B → A via direct file mutation since the write path would
// reject it.
func TestDependTreeCycleSafe(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"alpha", "beta"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2->1: %v", err)
	}
	// Hand-splice the inverse dep so a cycle exists in the file.
	path := filepath.Join(dir, ".tsk.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	// Add depends:2 to task 1 by replacing its meta closing -->
	// pattern with depends:2 -->. Only touches task 1.
	lines := strings.Split(string(body), "\n")
	for i, l := range lines {
		if strings.Contains(l, "id:1 ") && !strings.Contains(l, "depends:") {
			lines[i] = strings.Replace(l, "-->", "depends:2 -->", 1)
		}
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "1", "--tree")
	if err != nil {
		t.Fatalf("depend tree on cycle: %v", err)
	}
	if !strings.Contains(stdout, "(cycle)") {
		t.Fatalf("expected '(cycle)' marker, got:\n%s", stdout)
	}
}

// TestDependTreeJSON: --tree --json emits a recursive object with the
// expected shape and an empty dependencies array for leaves.
func TestDependTreeJSON(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "2", "--tree", "--json")
	if err != nil {
		t.Fatalf("depend tree json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if int(doc["id"].(float64)) != 2 {
		t.Fatalf("expected id=2 at root, got %v", doc["id"])
	}
	deps := doc["dependencies"].([]any)
	if len(deps) != 1 {
		t.Fatalf("expected 1 child, got %d", len(deps))
	}
	leaf := deps[0].(map[string]any)
	if int(leaf["id"].(float64)) != 1 {
		t.Fatalf("expected leaf id=1, got %v", leaf["id"])
	}
	// Leaf dependencies array must be present (not null) and empty.
	leafDeps, ok := leaf["dependencies"].([]any)
	if !ok {
		t.Fatalf("leaf dependencies should be array, got %T", leaf["dependencies"])
	}
	if len(leafDeps) != 0 {
		t.Fatalf("leaf dependencies should be empty, got %v", leafDeps)
	}
}

// TestDependTreeNoDeps: a task with zero deps tree-renders as just
// itself, no error.
func TestDependTreeNoDeps(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "lonely"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "1", "--tree")
	if err != nil {
		t.Fatalf("depend tree: %v", err)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected exactly 1 line, got %d:\n%s", len(lines), stdout)
	}
	if !strings.Contains(lines[0], "#1") {
		t.Fatalf("expected '#1' in single-line output, got %q", lines[0])
	}
}

// TestDependTreeRequiresID: --tree without an id should fail with a
// usage error (exit 2), not crash.
func TestDependTreeRequiresID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "depend", "--tree")
	if err == nil {
		t.Fatal("expected error for --tree with no id")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

// TestDependTreeRejectsMutationFlags: --tree is read-only; combining
// it with --on/--add etc must fail with a usage error.
func TestDependTreeRejectsMutationFlags(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	_, _, err := runCmd(t, dir, "depend", "1", "--tree", "--on", "2")
	if err == nil {
		t.Fatal("expected error for --tree + --on combo")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

// TestDependTreeDeterministicOrder: when a task has multiple deps,
// they render sorted by id so the output is reproducible (not slice-
// insertion-order dependent).
func TestDependTreeDeterministicOrder(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c", "d"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// 4 depends on 3,1,2 (deliberately unsorted on the CLI).
	if _, _, err := runCmd(t, dir, "depend", "4", "--on", "3,1,2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "depend", "4", "--tree")
	if err != nil {
		t.Fatalf("depend tree: %v", err)
	}
	// Find the index of each child id in the output; they must be in
	// ascending id order (1 then 2 then 3).
	i1 := strings.Index(stdout, "#1")
	i2 := strings.Index(stdout, "#2")
	i3 := strings.Index(stdout, "#3")
	if !(i1 > 0 && i2 > i1 && i3 > i2) {
		t.Fatalf("expected children sorted #1<#2<#3, got positions (1=%d, 2=%d, 3=%d):\n%s",
			i1, i2, i3, stdout)
	}
}
