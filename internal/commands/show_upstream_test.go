package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestShowUpstreamRendersDependents: --upstream appends the list of
// tasks that depend on this one, plus state annotations. Snapshot is
// preserved verbatim above the new section.
func TestShowUpstreamRendersDependents(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"target", "depender-a", "depender-b"} {
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
	stdout, _, err := runCmd(t, dir, "show", "1", "--upstream")
	if err != nil {
		t.Fatalf("show --upstream: %v", err)
	}
	// Snapshot intact.
	if !strings.Contains(stdout, "id:        1") {
		t.Fatalf("expected snapshot id line, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "title:     target") {
		t.Fatalf("expected snapshot title, got:\n%s", stdout)
	}
	// Upstream section header and both rows present.
	if !strings.Contains(stdout, "upstream:") {
		t.Fatalf("expected 'upstream:' header, got:\n%s", stdout)
	}
	for _, want := range []string{"#2  depender-a  (unblocks)", "#3  depender-b  (unblocks)"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected upstream row %q, got:\n%s", want, stdout)
		}
	}
}

// TestShowUpstreamSuppressesWhenNoDependents: a task no one depends
// on must produce IDENTICAL plain output between `show` and
// `show --upstream` — no dangling empty "upstream:" header.
func TestShowUpstreamSuppressesWhenNoDependents(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "lonely"); err != nil {
		t.Fatalf("add: %v", err)
	}
	plain, _, err := runCmd(t, dir, "show", "1")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	upstreamOut, _, err := runCmd(t, dir, "show", "1", "--upstream")
	if err != nil {
		t.Fatalf("show --upstream: %v", err)
	}
	if plain != upstreamOut {
		t.Fatalf("show vs show --upstream must be identical with no dependents.\nplain:\n%s\n--upstream:\n%s",
			plain, upstreamOut)
	}
	if strings.Contains(upstreamOut, "upstream:") {
		t.Fatalf("no-dependent task must not render 'upstream:' header, got:\n%s", upstreamOut)
	}
}

// TestShowUpstreamJSONAddsField: --upstream --json adds a top-level
// `upstream` array to the task object. Plain --json (without --upstream)
// must NOT include the key so the schema is stable.
func TestShowUpstreamJSONAddsField(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"target", "depender"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	// Plain --json: no upstream key.
	plain, _, err := runCmd(t, dir, "show", "1", "--json")
	if err != nil {
		t.Fatalf("show --json: %v", err)
	}
	var plainDoc map[string]any
	if err := json.Unmarshal([]byte(plain), &plainDoc); err != nil {
		t.Fatalf("plain --json invalid: %v\n%s", err, plain)
	}
	if _, ok := plainDoc["upstream"]; ok {
		t.Fatalf("plain --json must NOT have upstream key, got:\n%s", plain)
	}
	// --upstream --json: upstream array with the dependent.
	withUp, _, err := runCmd(t, dir, "show", "1", "--upstream", "--json")
	if err != nil {
		t.Fatalf("show --upstream --json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(withUp), &doc); err != nil {
		t.Fatalf("--upstream --json invalid: %v\n%s", err, withUp)
	}
	rows, ok := doc["upstream"].([]any)
	if !ok {
		t.Fatalf("expected upstream array, got %T:\n%s", doc["upstream"], withUp)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one upstream row, got %d:\n%s", len(rows), withUp)
	}
	row, _ := rows[0].(map[string]any)
	if id, _ := row["id"].(float64); int(id) != 2 {
		t.Fatalf("expected upstream id=2, got %v", row["id"])
	}
	if status, _ := row["status"].(string); status != "unblocks" {
		t.Fatalf("expected status=unblocks, got %v", row["status"])
	}
	// Task fields still present alongside upstream.
	if id, _ := doc["ID"].(float64); int(id) != 1 {
		t.Fatalf("expected ID=1 in JSON, got %v", doc["ID"])
	}
}

// TestShowUpstreamJSONNoDependentsOmitsField: a task no one depends
// on must not include the upstream key, matching the plain-text
// suppression. Schema parity with `show --json` and with `show --tree
// --json` on a leaf.
func TestShowUpstreamJSONNoDependentsOmitsField(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "lonely"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "show", "1", "--upstream", "--json")
	if err != nil {
		t.Fatalf("show --upstream --json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if _, ok := doc["upstream"]; ok {
		t.Fatalf("lonely task must omit upstream key, got:\n%s", stdout)
	}
}

// TestShowTreeAndUpstreamMutex: --tree and --upstream are different
// relationships; combining would muddle the output. Must error.
func TestShowTreeAndUpstreamMutex(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "show", "1", "--tree", "--upstream")
	if err == nil {
		t.Fatal("expected error combining --tree and --upstream")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

// TestShowUpstreamPlainIncludesAnnotations: the state annotations
// match `tsk depend --upstream` exactly — unblocks / blocked / done
// based on what closing the queried task would do.
func TestShowUpstreamPlainIncludesAnnotations(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"target", "only-target", "target-and-open", "another-open"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// #2 depends ONLY on #1 (unblocks).
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend 2: %v", err)
	}
	// #3 depends on #1 AND #4 (still blocked even if we close #1).
	if _, _, err := runCmd(t, dir, "depend", "3", "--on", "1,4"); err != nil {
		t.Fatalf("depend 3: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "show", "1", "--upstream")
	if err != nil {
		t.Fatalf("show --upstream: %v", err)
	}
	if !strings.Contains(stdout, "#2  only-target  (unblocks)") {
		t.Fatalf("expected (unblocks) annotation on #2, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "#3  target-and-open  (blocked)") {
		t.Fatalf("expected (blocked) annotation on #3, got:\n%s", stdout)
	}
}
