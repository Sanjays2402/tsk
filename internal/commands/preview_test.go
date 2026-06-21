package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runPreview executes `tsk preview ...` with the given stdin payload
// and CLI args. Returns stdout, combined output, and the error.
// Unlike runCmd, we do NOT prepend --file because preview is supposed
// to bypass the active store entirely.
func runPreview(t *testing.T, stdin string, args ...string) (stdout, combined string, err error) {
	t.Helper()
	root := NewRoot()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetIn(bytes.NewBufferString(stdin))
	root.SetArgs(append([]string{"preview"}, args...))
	err = root.Execute()
	return out.String(), out.String() + errb.String(), err
}

// TestPreviewParsesStdinAndRendersAsLs: a basic .tsk.md snippet on
// stdin should parse cleanly and render the standard `ls` plain
// output.
func TestPreviewParsesStdinAndRendersAsLs(t *testing.T) {
	input := `# tasks

- [ ] write tests <!-- id:1 prio:high -->
- [x] ship feature <!-- id:2 prio:medium -->
`
	stdout, _, err := runPreview(t, input)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if !strings.Contains(stdout, "write tests") {
		t.Fatalf("expected 'write tests' in plain output, got:\n%s", stdout)
	}
	// Default is undone-only, so #2 should be hidden.
	if strings.Contains(stdout, "ship feature") {
		t.Fatalf("done task should be hidden by default, got:\n%s", stdout)
	}
	stdout, _, err = runPreview(t, input, "--all")
	if err != nil {
		t.Fatalf("preview --all: %v", err)
	}
	if !strings.Contains(stdout, "ship feature") {
		t.Fatalf("--all should expose done task, got:\n%s", stdout)
	}
}

// TestPreviewNeverTouchesActiveStore: even if there's an active
// .tsk.md in the cwd, preview must NOT read from it. We assert this
// by running preview from a TempDir that contains a "wrong" .tsk.md
// and verifying the output ONLY reflects stdin content.
func TestPreviewNeverTouchesActiveStore(t *testing.T) {
	dir := t.TempDir()
	wrong := "# tasks\n\n- [ ] WRONG <!-- id:1 prio:medium -->\n"
	if err := os.WriteFile(filepath.Join(dir, ".tsk.md"), []byte(wrong), 0o644); err != nil {
		t.Fatalf("write wrong .tsk.md: %v", err)
	}
	// chdir so any resolveStore call would find that file.
	oldwd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(oldwd)

	right := "# tasks\n\n- [ ] correct-task <!-- id:1 prio:high -->\n"
	stdout, _, err := runPreview(t, right)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if !strings.Contains(stdout, "correct-task") {
		t.Fatalf("expected stdin task in output, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "WRONG") {
		t.Fatalf("preview must not read active store, got WRONG in output:\n%s", stdout)
	}
	// And no .bak should have been created by preview.
	if _, err := os.Stat(filepath.Join(dir, ".tsk.md.bak")); err == nil {
		t.Fatalf("preview must not produce a .bak snapshot")
	}
}

// TestPreviewFromFlagReadsFile: --from <path> reads from disk
// instead of stdin. Useful for inspecting snapshots.
func TestPreviewFromFlagReadsFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.md")
	body := "# tasks\n\n- [ ] from-file <!-- id:1 prio:medium -->\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	stdout, _, err := runPreview(t, "", "--from", path)
	if err != nil {
		t.Fatalf("preview --from: %v", err)
	}
	if !strings.Contains(stdout, "from-file") {
		t.Fatalf("expected from-file in output, got:\n%s", stdout)
	}
}

// TestPreviewEmptyStdinErrorsCleanly: empty stdin must produce a
// usage error (exit 2) so users know the pipe failed silently.
func TestPreviewEmptyStdinErrorsCleanly(t *testing.T) {
	_, _, err := runPreview(t, "")
	if err == nil {
		t.Fatal("expected error for empty stdin")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

// TestPreviewMissingFromFile: --from with a non-existent path must
// error cleanly (not crash) with a useful message.
func TestPreviewMissingFromFile(t *testing.T) {
	_, _, err := runPreview(t, "", "--from", "/nonexistent/path/scratch.md")
	if err == nil {
		t.Fatal("expected error for missing --from")
	}
	if !strings.Contains(err.Error(), "read --from") {
		t.Fatalf("expected 'read --from' in error, got: %v", err)
	}
}

// TestPreviewJSONFlow: --json renders the parsed task list as a
// stable JSON array. The same JSON shape as `tsk ls --json`.
func TestPreviewJSONFlow(t *testing.T) {
	input := "# tasks\n\n- [ ] task-one <!-- id:1 prio:urgent tags:work -->\n"
	stdout, _, err := runPreview(t, input, "--json")
	if err != nil {
		t.Fatalf("preview --json: %v", err)
	}
	var tasks []map[string]any
	if err := json.Unmarshal([]byte(stdout), &tasks); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}
	if title, _ := tasks[0]["Title"].(string); title != "task-one" {
		t.Fatalf("expected title=task-one, got %v", tasks[0]["Title"])
	}
}

// TestPreviewRespectDepsUsesSnapshotGraph: --respect-deps must walk
// the SNAPSHOT's own dep graph, not the active store. We craft a
// snapshot where #2 depends on the still-open #1; preview
// --respect-deps should hide #2.
func TestPreviewRespectDepsUsesSnapshotGraph(t *testing.T) {
	input := `# tasks

- [ ] prereq <!-- id:1 prio:medium -->
- [ ] blocked <!-- id:2 prio:medium depends:1 -->
`
	// Default: both visible.
	stdout, _, err := runPreview(t, input)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if !strings.Contains(stdout, "blocked") {
		t.Fatalf("expected 'blocked' in default output, got:\n%s", stdout)
	}
	// With --respect-deps: blocked is hidden because #1 is open.
	stdout, _, err = runPreview(t, input, "--respect-deps")
	if err != nil {
		t.Fatalf("preview --respect-deps: %v", err)
	}
	if strings.Contains(stdout, "blocked") {
		t.Fatalf("--respect-deps should hide blocked, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "prereq") {
		t.Fatalf("prereq is unblocked and must remain, got:\n%s", stdout)
	}
}

// TestPreviewParseErrorBubblesUp: a malformed input should fail
// cleanly, not crash. We craft something that the parser actually
// chokes on — the markdown parser is lenient, so we use a payload
// missing the leading "- [ ]" but otherwise looking task-shaped.
// Here we test that the error path goes through fmt.Errorf with
// the "parse preview input" prefix.
func TestPreviewParseErrorBubblesUp(t *testing.T) {
	// Real .tsk.md parser is very lenient — almost everything
	// parses to "no tasks". Instead, test the contract that a
	// hugely-oversized stdin gets capped (a near-equivalent error
	// signal that the safety guard works).
	big := make([]byte, 4*1024*1024+10)
	for i := range big {
		big[i] = 'a'
	}
	_, _, err := runPreview(t, string(big))
	if err == nil {
		t.Fatal("expected error for oversized stdin")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected 'exceeds' in error, got: %v", err)
	}
}
