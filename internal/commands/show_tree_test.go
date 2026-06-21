package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestShowTreeRendersChain: --tree appends the recursive prereq chain
// under the normal snapshot. The snapshot lines must still be there
// (we're additive, not replacing), and the chain follows after a
// blank line + "dependencies:" header.
func TestShowTreeRendersChain(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"deep", "middle", "top"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "2"); err != nil {
		t.Fatalf("depend 3: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "show", "3", "--tree")
	if err != nil {
		t.Fatalf("show --tree: %v", err)
	}
	// Snapshot fields still present.
	if !strings.Contains(stdout, "id:        3") {
		t.Fatalf("expected snapshot id:3 line, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "title:     top") {
		t.Fatalf("expected snapshot title, got:\n%s", stdout)
	}
	// The depends: meta line is rendered by printTaskDetail.
	if !strings.Contains(stdout, "depends:") {
		t.Fatalf("expected 'depends:' meta line, got:\n%s", stdout)
	}
	// Tree section appears.
	if !strings.Contains(stdout, "dependencies:") {
		t.Fatalf("expected 'dependencies:' header for tree section, got:\n%s", stdout)
	}
	// And the chain runs 3 -> 2 -> 1.
	for _, expect := range []string{"#3 [ ] top", "└─ #2", "└─ #1"} {
		if !strings.Contains(stdout, expect) {
			t.Fatalf("expected %q in tree output, got:\n%s", expect, stdout)
		}
	}
}

// TestShowTreeSuppressesWhenNoDeps: a task with no dependencies should
// produce identical output between `show` and `show --tree`. We don't
// want an empty "dependencies:" header dangling.
func TestShowTreeSuppressesWhenNoDeps(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "lonely"); err != nil {
		t.Fatalf("add: %v", err)
	}
	plain, _, err := runCmd(t, dir, "show", "1")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	treeOut, _, err := runCmd(t, dir, "show", "1", "--tree")
	if err != nil {
		t.Fatalf("show --tree: %v", err)
	}
	if plain != treeOut {
		t.Fatalf("show vs show --tree must be identical on a leaf task.\nplain:\n%s\n--tree:\n%s", plain, treeOut)
	}
	if strings.Contains(treeOut, "dependencies:") {
		t.Fatalf("no-dep task must not render 'dependencies:' header, got:\n%s", treeOut)
	}
}

// TestShowTreeJSONAddsDependencyTreeField: --tree --json adds a
// `dependency_tree` field to the task object containing the nested
// shape. Plain --json (without --tree) must NOT include that key so
// the schema is stable for downstream parsers.
func TestShowTreeJSONAddsDependencyTreeField(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	// Plain --json: no dependency_tree key.
	plain, _, err := runCmd(t, dir, "show", "2", "--json")
	if err != nil {
		t.Fatalf("show --json: %v", err)
	}
	var plainDoc map[string]any
	if err := json.Unmarshal([]byte(plain), &plainDoc); err != nil {
		t.Fatalf("plain --json invalid: %v\n%s", err, plain)
	}
	if _, ok := plainDoc["dependency_tree"]; ok {
		t.Fatalf("plain --json must NOT have dependency_tree, got:\n%s", plain)
	}
	// --tree --json: dependency_tree present with the expected nested shape.
	treed, _, err := runCmd(t, dir, "show", "2", "--tree", "--json")
	if err != nil {
		t.Fatalf("show --tree --json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(treed), &doc); err != nil {
		t.Fatalf("--tree --json invalid: %v\n%s", err, treed)
	}
	tree, ok := doc["dependency_tree"].(map[string]any)
	if !ok {
		t.Fatalf("expected dependency_tree object, got %T:\n%s", doc["dependency_tree"], treed)
	}
	if id, _ := tree["id"].(float64); int(id) != 2 {
		t.Fatalf("expected tree root id=2, got %v", tree["id"])
	}
	deps, _ := tree["dependencies"].([]any)
	if len(deps) != 1 {
		t.Fatalf("expected one dependency under root, got %d:\n%s", len(deps), treed)
	}
	child, _ := deps[0].(map[string]any)
	if id, _ := child["id"].(float64); int(id) != 1 {
		t.Fatalf("expected child id=1, got %v", child["id"])
	}
	// And the task fields are still present alongside dependency_tree.
	if id, _ := doc["ID"].(float64); int(id) != 2 {
		t.Fatalf("expected ID=2 in JSON, got %v", doc["ID"])
	}
}

// TestShowTreeJSONNoDepsOmitsTreeField: just like the plain-text path
// suppresses the section, JSON should not include the dependency_tree
// key on a leaf task. Schema parity with `tsk show --json`.
func TestShowTreeJSONNoDepsOmitsTreeField(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "leaf"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "show", "1", "--tree", "--json")
	if err != nil {
		t.Fatalf("show --tree --json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if _, ok := doc["dependency_tree"]; ok {
		t.Fatalf("leaf task must omit dependency_tree key, got:\n%s", stdout)
	}
}

// TestShowTreeRendersDoneInChain: prereqs that are already done should
// render with [x] inside the tree so the user sees completion state.
func TestShowTreeRendersDoneInChain(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"prereq", "blocked"} {
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
	stdout, _, err := runCmd(t, dir, "show", "2", "--tree")
	if err != nil {
		t.Fatalf("show --tree: %v", err)
	}
	// Root is open, child is done.
	if !strings.Contains(stdout, "#2 [ ]") {
		t.Fatalf("expected '#2 [ ]' for open root, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "└─ #1 [x]") {
		t.Fatalf("expected '└─ #1 [x]' for done child, got:\n%s", stdout)
	}
}
