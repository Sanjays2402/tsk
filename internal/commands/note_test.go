package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// withNoteEditor swaps the noteEditor function for the test and restores
// it on cleanup. The fn receives the temp file path and can read/write it.
func withNoteEditor(t *testing.T, fn func(*cobra.Command, string) error) {
	t.Helper()
	orig := noteEditor
	noteEditor = fn
	t.Cleanup(func() { noteEditor = orig })
}

func TestNoteInlineTextReplacesNotes(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "note", "1", "found", "the", "source")
	if err != nil {
		t.Fatalf("note: %v", err)
	}
	if !strings.Contains(stdout, "#1 notes updated") {
		t.Fatalf("expected updated line, got: %q", stdout)
	}
	stdout, _, err = runCmd(t, dir, "show", "1")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(stdout, "found the source") {
		t.Fatalf("expected joined inline text, got:\n%s", stdout)
	}
}

func TestNoteAppendKeepsExisting(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing", "-n", "first line"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "note", "1", "--append", "second line"); err != nil {
		t.Fatalf("note append: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "show", "1")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(stdout, "first line") || !strings.Contains(stdout, "second line") {
		t.Fatalf("expected both lines after append, got:\n%s", stdout)
	}
}

func TestNoteAppendOnEmptyJustWrites(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "note", "1", "--append", "first"); err != nil {
		t.Fatalf("note append: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "show", "1")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(stdout, "first") {
		t.Fatalf("expected note written, got:\n%s", stdout)
	}
}

func TestNoteClearRemovesNotes(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing", "-n", "nope"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "note", "1", "--clear")
	if err != nil {
		t.Fatalf("note --clear: %v", err)
	}
	if !strings.Contains(stdout, "notes cleared") {
		t.Fatalf("expected cleared message, got: %q", stdout)
	}
	stdout, _, err = runCmd(t, dir, "show", "1")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if strings.Contains(stdout, "notes:") {
		t.Fatalf("show should not contain notes section after clear, got:\n%s", stdout)
	}
}

func TestNoteClearOnEmptyIsNoop(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "note", "1", "--clear")
	if err != nil {
		t.Fatalf("note --clear noop: %v", err)
	}
	if !strings.Contains(stdout, "already has no notes") {
		t.Fatalf("expected no-op message, got: %q", stdout)
	}
}

func TestNoteEditorFlowWritesContent(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Substitute the editor with an inline shim that writes to the temp file.
	withNoteEditor(t, func(_ *cobra.Command, path string) error {
		return os.WriteFile(path, []byte("edited via shim\n"), 0o644)
	})
	if _, _, err := runCmd(t, dir, "note", "1"); err != nil {
		t.Fatalf("note (editor): %v", err)
	}
	stdout, _, err := runCmd(t, dir, "show", "1")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(stdout, "edited via shim") {
		t.Fatalf("expected shim-written content, got:\n%s", stdout)
	}
}

func TestNoteEditorFlowPrePopulatesCurrentNotes(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing", "-n", "starting content"); err != nil {
		t.Fatalf("add: %v", err)
	}
	var seen string
	withNoteEditor(t, func(_ *cobra.Command, path string) error {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		seen = string(b)
		// Write something different back so we can also verify the update lands.
		return os.WriteFile(path, []byte("rewritten\n"), 0o644)
	})
	if _, _, err := runCmd(t, dir, "note", "1"); err != nil {
		t.Fatalf("note: %v", err)
	}
	if seen != "starting content" {
		t.Fatalf("editor should see existing notes, got %q", seen)
	}
	stdout, _, err := runCmd(t, dir, "show", "1")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(stdout, "rewritten") {
		t.Fatalf("expected rewritten content saved, got:\n%s", stdout)
	}
}

func TestNoteEditorEmptyClears(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing", "-n", "to be wiped"); err != nil {
		t.Fatalf("add: %v", err)
	}
	withNoteEditor(t, func(_ *cobra.Command, path string) error {
		// Simulate "user opened editor, deleted everything, saved".
		return os.WriteFile(path, []byte(""), 0o644)
	})
	stdout, _, err := runCmd(t, dir, "note", "1")
	if err != nil {
		t.Fatalf("note: %v", err)
	}
	if !strings.Contains(stdout, "cleared") {
		t.Fatalf("expected cleared message for empty editor save, got: %q", stdout)
	}
}

func TestNoteStdinReadsFromStdin(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Run with a custom stdin reader to deliver the piped content.
	root := NewRoot()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetIn(bytes.NewBufferString("from stdin pipe\nwith multiple lines"))
	root.SetArgs([]string{"--file", filepath.Join(dir, ".tsk.md"), "note", "1", "--stdin"})
	if err := root.Execute(); err != nil {
		t.Fatalf("note --stdin: %v\n%s", err, errb.String())
	}
	stdout, _, err := runCmd(t, dir, "show", "1")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	if !strings.Contains(stdout, "from stdin pipe") {
		t.Fatalf("expected stdin content, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "with multiple lines") {
		t.Fatalf("expected multi-line stdin preserved, got:\n%s", stdout)
	}
}

func TestNoteRejectsClearWithText(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "note", "1", "--clear", "extra")
	if err == nil {
		t.Fatal("expected error combining --clear with text")
	}
}

func TestNoteRejectsClearWithAppend(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "note", "1", "--clear", "--append")
	if err == nil {
		t.Fatal("expected error combining --clear with --append")
	}
}

func TestNoteRejectsStdinWithText(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "note", "1", "--stdin", "extra text")
	if err == nil {
		t.Fatal("expected error combining --stdin with text args")
	}
}

func TestNoteSameContentIsNoop(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing", "-n", "identical"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "note", "1", "identical")
	if err != nil {
		t.Fatalf("note same: %v", err)
	}
	if !strings.Contains(stdout, "notes unchanged") {
		t.Fatalf("expected unchanged notice, got: %q", stdout)
	}
}

func TestNoteAlias(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "notes", "1", "via", "alias"); err != nil {
		t.Fatalf("notes alias: %v", err)
	}
}

func TestNoteUnknownID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "note", "999", "text")
	if err == nil {
		t.Fatal("expected error for unknown id")
	}
}

func TestPickNoteModeTable(t *testing.T) {
	cases := []struct {
		name       string
		args       []string
		appendMode bool
		clear      bool
		fromStdin  bool
		wantKind   string
		wantText   string
		wantErr    bool
	}{
		{name: "text", args: []string{"hello", "world"}, wantKind: "text", wantText: "hello world"},
		{name: "text-append", args: []string{"more"}, appendMode: true, wantKind: "text", wantText: "more"},
		{name: "editor-default", wantKind: "editor"},
		{name: "stdin", fromStdin: true, wantKind: "stdin"},
		{name: "clear", clear: true, wantKind: "clear"},
		{name: "clear+text", args: []string{"x"}, clear: true, wantErr: true},
		{name: "clear+append", clear: true, appendMode: true, wantErr: true},
		{name: "clear+stdin", clear: true, fromStdin: true, wantErr: true},
		{name: "text+stdin", args: []string{"x"}, fromStdin: true, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := pickNoteMode(tc.args, tc.appendMode, tc.clear, tc.fromStdin)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if m.kind != tc.wantKind {
				t.Fatalf("kind: got %q want %q", m.kind, tc.wantKind)
			}
			if tc.wantText != "" && m.text != tc.wantText {
				t.Fatalf("text: got %q want %q", m.text, tc.wantText)
			}
		})
	}
}
