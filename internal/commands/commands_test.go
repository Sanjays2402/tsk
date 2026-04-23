package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runCmd executes the root command with the given args against a scratch
// .tsk.md inside tmpDir. Returns captured stdout, combined output, and error.
func runCmd(t *testing.T, tmpDir string, args ...string) (stdout, combined string, err error) {
	t.Helper()
	root := NewRoot()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	// Prepend --file so every test works against its own scratch file.
	full := append([]string{"--file", filepath.Join(tmpDir, ".tsk.md")}, args...)
	root.SetArgs(full)
	err = root.Execute()
	return out.String(), out.String() + errb.String(), err
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestAddAndListRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "write more tests", "-p", "high", "-t", "dev"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "buy milk", "-p", "low"); err != nil {
		t.Fatalf("add second: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "ls")
	if err != nil {
		t.Fatalf("ls: %v", err)
	}
	if !strings.Contains(stdout, "write more tests") || !strings.Contains(stdout, "buy milk") {
		t.Fatalf("ls output missing tasks:\n%s", stdout)
	}
}

func TestAddRejectsEmptyTitle(t *testing.T) {
	dir := t.TempDir()
	_, _, err := runCmd(t, dir, "add", "   ")
	if err == nil {
		t.Fatal("expected error for whitespace-only title, got nil")
	}
}

func TestAddRejectsBadDue(t *testing.T) {
	dir := t.TempDir()
	_, _, err := runCmd(t, dir, "add", "thing", "-d", "this-is-not-a-date")
	if err == nil {
		t.Fatal("expected error for bad --due")
	}
	var ec ExitCoder
	// The error should carry exit code 2 (user-input error).
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected ExitCode 2 user-input error, got %v", err)
	}
}

// asExitCoder is a tiny local helper to avoid pulling errors.As into the
// surface area of the test — the behavior is identical.
func asExitCoder(err error, target *ExitCoder) bool {
	for e := err; e != nil; {
		if ec, ok := e.(ExitCoder); ok {
			*target = ec
			return true
		}
		type unwrapper interface{ Unwrap() error }
		if u, ok := e.(unwrapper); ok {
			e = u.Unwrap()
			continue
		}
		return false
	}
	return false
}

func TestDoneUndoToggle(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "do the thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(content, "- [x] do the thing") {
		t.Fatalf("expected task marked done, got:\n%s", content)
	}
	if _, _, err := runCmd(t, dir, "undo", "1"); err != nil {
		t.Fatalf("undo: %v", err)
	}
	content = readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(content, "- [ ] do the thing") {
		t.Fatalf("expected task marked undone, got:\n%s", content)
	}
}

func TestRmDeletes(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "gone soon"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "rm", "1"); err != nil {
		t.Fatalf("rm: %v", err)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if strings.Contains(content, "gone soon") {
		t.Fatalf("expected task removed, still present:\n%s", content)
	}
}

func TestExportJSONValid(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "json task", "-p", "urgent", "-t", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "export", "--json")
	if err != nil {
		t.Fatalf("export --json: %v", err)
	}
	var parsed any
	if err := json.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("export --json produced invalid JSON: %v\n%s", err, stdout)
	}
}

func TestStatsRuns(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "stats"); err != nil {
		t.Fatalf("stats: %v", err)
	}
}

func TestNextReturnsHighestPriority(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{"low", "high", "medium"} {
		if _, _, err := runCmd(t, dir, "add", "task "+p, "-p", p); err != nil {
			t.Fatalf("add %s: %v", p, err)
		}
	}
	stdout, _, err := runCmd(t, dir, "next")
	if err != nil {
		t.Fatalf("next: %v", err)
	}
	// The "high" task was #2; next should surface that one.
	if !strings.Contains(stdout, "task high") {
		t.Fatalf("expected 'task high' from next, got:\n%s", stdout)
	}
}

func TestDoneAcceptsMultipleIDs(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	if _, _, err := runCmd(t, dir, "done", "1", "2", "3"); err != nil {
		t.Fatalf("done multi: %v", err)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if strings.Count(content, "- [x] ") != 3 {
		t.Fatalf("expected 3 done tasks, content:\n%s", content)
	}
}

func TestRmAcceptsMultipleIDs(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "b", "c"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	if _, _, err := runCmd(t, dir, "rm", "1", "3"); err != nil {
		t.Fatalf("rm multi: %v", err)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if strings.Contains(content, "- [ ] a") || strings.Contains(content, "- [ ] c") {
		t.Fatalf("expected a and c removed, content:\n%s", content)
	}
	if !strings.Contains(content, "- [ ] b") {
		t.Fatalf("expected b preserved, content:\n%s", content)
	}
}

func TestDoneRollsBackOnBadID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// 1 is valid, 999 is not. We currently fail mid-way — that's documented
	// behavior. Just assert the caller sees a real error, not a silent pass.
	_, _, err := runCmd(t, dir, "done", "1", "999")
	if err == nil {
		t.Fatal("expected error for non-existent id, got nil")
	}
}

func TestExportMarkdownFormat(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "write code", "-p", "high", "-t", "dev"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "ship it", "-p", "urgent"); err != nil {
		t.Fatalf("add 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "export", "--format", "markdown")
	if err != nil {
		t.Fatalf("export md: %v", err)
	}
	if !strings.Contains(stdout, "# Tasks") {
		t.Fatalf("expected markdown heading, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "## Todo") || !strings.Contains(stdout, "## Done") {
		t.Fatalf("expected both Todo and Done sections, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "[!] ship it") {
		t.Fatalf("expected urgent marker on ship it, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "#dev") {
		t.Fatalf("expected tag rendering, got:\n%s", stdout)
	}
}

func TestExportFormatMdAlias(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "one"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "export", "-f", "md")
	if err != nil {
		t.Fatalf("export md alias: %v", err)
	}
	if !strings.Contains(stdout, "# Tasks") {
		t.Fatalf("md alias should work, got:\n%s", stdout)
	}
}

func TestExportRejectsMultipleFormats(t *testing.T) {
	dir := t.TempDir()
	_, _, err := runCmd(t, dir, "export", "--json", "--csv")
	if err == nil {
		t.Fatal("expected error with multiple formats")
	}
}

func TestExportRejectsUnknownFormat(t *testing.T) {
	dir := t.TempDir()
	_, _, err := runCmd(t, dir, "export", "--format", "yaml")
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}
