package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestGraphJSONIncludeAllTurnsOnEveryField: --include-all flips
// every opt-in node field on at once. Equivalent to passing all
// five --include-* flags individually.
func TestGraphJSONIncludeAllTurnsOnEveryField(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "in-flight", "-p", "urgent", "-t", "ship", "-d", "2026-08-15"); err != nil {
		t.Fatalf("add a: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "shipped-prereq", "-p", "low", "-t", "later", "-d", "2026-12-01"); err != nil {
		t.Fatalf("add b: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "1", "--on", "2"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--include-all")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v\nbody: %s", err, stdout)
	}
	gotPrios := map[int]string{}
	gotTags := map[int][]string{}
	gotDues := map[int]string{}
	gotCompl := map[int]string{}
	gotStarted := map[int]string{}
	for _, n := range doc.Nodes {
		gotPrios[n.ID] = n.Priority
		if n.Tags != nil {
			gotTags[n.ID] = *n.Tags
		}
		gotDues[n.ID] = n.Due
		gotCompl[n.ID] = n.Completed
		gotStarted[n.ID] = n.Started
	}
	if gotPrios[1] != "urgent" {
		t.Errorf("node #1 priority: want urgent, got %q", gotPrios[1])
	}
	if len(gotTags[1]) != 1 || gotTags[1][0] != "ship" {
		t.Errorf("node #1 tags: want [ship], got %v", gotTags[1])
	}
	if gotDues[1] != "2026-08-15" {
		t.Errorf("node #1 due: want 2026-08-15, got %q", gotDues[1])
	}
	if gotStarted[1] == "" {
		t.Errorf("node #1 (started) should carry started ts, got empty")
	}
	if gotCompl[2] == "" {
		t.Errorf("node #2 (done) should carry completed ts, got empty")
	}
}

// TestGraphJSONIncludeAllEquivalentToIndividualFlags: setting
// --include-all should produce the SAME envelope bytes as
// setting all five individual --include-* flags together.
func TestGraphJSONIncludeAllEquivalentToIndividualFlags(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-p", "high", "-t", "x,y", "-d", "2026-09-01"); err != nil {
		t.Fatalf("add a: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b"); err != nil {
		t.Fatalf("add b: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	gotIndividual, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json",
		"--include-priority", "--include-tags", "--include-due", "--include-completed", "--include-started")
	if err != nil {
		t.Fatalf("graph individual: %v", err)
	}
	gotAll, _, err := runCmd(t, dir, "graph", "--reachable", "2", "--json", "--include-all")
	if err != nil {
		t.Fatalf("graph --include-all: %v", err)
	}
	if gotIndividual != gotAll {
		t.Errorf("--include-all should produce same bytes as all individual flags\n--all:\n%s\n--individual:\n%s",
			gotAll, gotIndividual)
	}
}

// TestGraphJSONIncludeAllRequiresJSON: --include-all without
// --json is rejected at the usage layer (exit 2).
func TestGraphJSONIncludeAllRequiresJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "graph", "--include-all")
	if err == nil {
		t.Fatal("expected error for --include-all without --json")
	}
	if !strings.Contains(err.Error(), "--include-all") {
		t.Fatalf("expected error to mention --include-all, got %v", err)
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2, got %v", err)
	}
}

// TestGraphJSONIncludeAllIdempotentWithIndividualFlags: setting
// --include-all alongside one of the individual flags doesn't
// double-emit or otherwise diverge from --include-all alone.
func TestGraphJSONIncludeAllIdempotentWithIndividualFlags(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-p", "high"); err != nil {
		t.Fatalf("add: %v", err)
	}
	out1, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--include-all")
	if err != nil {
		t.Fatalf("graph 1: %v", err)
	}
	out2, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--include-all", "--include-priority")
	if err != nil {
		t.Fatalf("graph 2: %v", err)
	}
	if out1 != out2 {
		t.Errorf("--include-all + --include-priority should equal --include-all alone\nbare:\n%s\ncombined:\n%s",
			out1, out2)
	}
}

// TestGraphJSONIncludeAllCompactJSON: composes with --compact-json
// — single-line record with all five fields inline.
func TestGraphJSONIncludeAllCompactJSON(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "everything", "-p", "urgent", "-t", "ship", "-d", "2026-07-04"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--compact-json", "--include-all")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	body := strings.TrimRight(stdout, "\n")
	if strings.Contains(body, "\n") {
		t.Fatalf("compact output should be single-line, got:\n%s", stdout)
	}
	// Every applicable opt-in field should appear inline.
	for _, want := range []string{"\"priority\":\"urgent\"", "\"tags\":[\"ship\"]", "\"due\":\"2026-07-04\"", "\"started\":\""} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q in compact body, got: %s", want, body)
		}
	}
}

// TestGraphJSONIncludeAllAppendModeStreaming: composes with
// --append — the JSONL stream carries all opt-in fields on every
// applicable record.
func TestGraphJSONIncludeAllAppendModeStreaming(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "ship", "-p", "high", "-t", "release", "-d", "2026-09-01"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	outPath := dir + "/history.jsonl"
	if _, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", outPath, "--append", "--include-all"); err != nil {
		t.Fatalf("graph append: %v", err)
	}
	body := readFile(t, outPath)
	line := strings.TrimRight(body, "\n")
	for _, want := range []string{"\"priority\":", "\"tags\":", "\"due\":", "\"started\":"} {
		if !strings.Contains(line, want) {
			t.Errorf("expected %q in compact append record, got: %s", want, line)
		}
	}
}

// TestGraphJSONIncludeAllUpstreamOf: works identically for
// --upstream-of (the inverse subgraph direction).
func TestGraphJSONIncludeAllUpstreamOf(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root", "-p", "urgent", "-t", "release", "-d", "2026-08-01"); err != nil {
		t.Fatalf("add root: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "downstream"); err != nil {
		t.Fatalf("add downstream: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "1"); err != nil {
		t.Fatalf("depend: %v", err)
	}
	if _, _, err := runCmd(t, dir, "start", "1"); err != nil {
		t.Fatalf("start: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--upstream-of", "1", "--json", "--include-all")
	if err != nil {
		t.Fatalf("graph --upstream-of --include-all: %v", err)
	}
	var doc subgraphDoc
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if doc.Direction != "upstream-of" {
		t.Errorf("direction: want upstream-of, got %q", doc.Direction)
	}
	// Find node #1 and confirm it has multiple opt-in fields populated.
	var n1 *subgraphNode
	for i := range doc.Nodes {
		if doc.Nodes[i].ID == 1 {
			n1 = &doc.Nodes[i]
			break
		}
	}
	if n1 == nil {
		t.Fatal("node #1 missing")
	}
	if n1.Priority != "urgent" {
		t.Errorf("priority: want urgent, got %q", n1.Priority)
	}
	if n1.Due != "2026-08-01" {
		t.Errorf("due: want 2026-08-01, got %q", n1.Due)
	}
	if n1.Started == "" {
		t.Errorf("started should be set, got empty")
	}
}

// TestGraphJSONIncludeAllDefaultIsNoOp: without --include-all (or
// any other --include-* flag), the historical minimal envelope
// shape is preserved.
func TestGraphJSONIncludeAllDefaultIsNoOp(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-p", "urgent", "-t", "ship"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json")
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	for _, forbidden := range []string{"\"priority\"", "\"tags\"", "\"due\"", "\"completed\"", "\"started\""} {
		if strings.Contains(stdout, forbidden) {
			t.Errorf("default envelope should NOT contain %s, got:\n%s", forbidden, stdout)
		}
	}
}

// TestGraphJSONIncludeAllFlagListedInHelp: the help text
// documents --include-all so users can discover the shortcut.
func TestGraphJSONIncludeAllFlagListedInHelp(t *testing.T) {
	dir := t.TempDir()
	stdout, _, err := runCmd(t, dir, "graph", "--help")
	if err != nil {
		t.Fatalf("graph --help: %v", err)
	}
	if !strings.Contains(stdout, "--include-all") {
		t.Errorf("expected --include-all in help text, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "include-all") || !strings.Contains(stdout, "every opt-in field") {
		t.Errorf("help text should describe what --include-all does, got:\n%s", stdout)
	}
}
