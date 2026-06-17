package commands

import (
	"strings"
	"testing"
)

// TestExportMarkdownRendersNotes verifies that task notes are emitted as
// indented blockquote lines beneath the task, including multi-line notes.
func TestExportMarkdownRendersNotes(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "task with notes", "-n", "first line\nsecond line"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "export", "--format", "markdown")
	if err != nil {
		t.Fatalf("export markdown: %v", err)
	}
	if !strings.Contains(stdout, "  > first line") {
		t.Fatalf("expected first note line as indented blockquote, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "  > second line") {
		t.Fatalf("expected second note line as indented blockquote, got:\n%s", stdout)
	}
}

// TestExportMarkdownRendersDue verifies the "(due YYYY-MM-DD)" annotation is
// appended when a task has a due date.
func TestExportMarkdownRendersDue(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "dated task", "-d", "2030-01-15"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "export", "-f", "markdown")
	if err != nil {
		t.Fatalf("export markdown: %v", err)
	}
	if !strings.Contains(stdout, "dated task (due 2030-01-15)") {
		t.Fatalf("expected due-date annotation, got:\n%s", stdout)
	}
}

// TestExportMarkdownPriorityGlyphs verifies each priority level maps to its
// ASCII glyph in the markdown output.
func TestExportMarkdownPriorityGlyphs(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		prio  string
		glyph string
	}{
		{"urgent", "[!]"},
		{"high", "[H]"},
		{"medium", "[M]"},
		{"low", "[L]"},
	}
	for _, c := range cases {
		if _, _, err := runCmd(t, dir, "add", "p-"+c.prio, "-p", c.prio); err != nil {
			t.Fatalf("add %s: %v", c.prio, err)
		}
	}
	stdout, _, err := runCmd(t, dir, "export", "--format", "markdown")
	if err != nil {
		t.Fatalf("export markdown: %v", err)
	}
	for _, c := range cases {
		want := c.glyph + " p-" + c.prio
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %q for priority %s, got:\n%s", want, c.prio, stdout)
		}
	}
}

// TestExportMarkdownEmptyStore confirms exporting an empty file still produces
// the top-level heading and does not error.
func TestExportMarkdownEmptyStore(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "export", "--format", "markdown")
	if err != nil {
		t.Fatalf("export markdown on empty store: %v", err)
	}
	if !strings.Contains(stdout, "# Tasks") {
		t.Fatalf("expected '# Tasks' heading even when empty, got:\n%s", stdout)
	}
}
