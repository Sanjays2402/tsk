package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
)

// newNoteCmd implements `tsk note <id> [text...]`: add or edit the notes
// block on a single task without dropping into the full-file editor.
//
// Three input modes (mutually exclusive):
//
//  1. Positional text:   tsk note 3 picked up the contract
//     The text is joined with spaces and used verbatim.
//
//  2. Editor flow:       tsk note 3
//     Opens $EDITOR (vi if unset) on a temp file pre-populated with
//     the current notes, then saves whatever you wrote back to the
//     task. This is the default when no text arg is given.
//
//  3. Stdin:             echo "context" | tsk note 3 --stdin
//     Reads the new notes from stdin. Composes nicely with pipes.
//
// Optional flags:
//
//	--append          add to existing notes instead of replacing
//	                  (joined with a blank line for readability)
//	--clear           remove the notes block entirely (cannot combine
//	                  with text args, --append, or --stdin)
//
// The implementation is split so the editor invocation is a single
// indirection (the noteEditor variable) — tests can swap it for a
// shim without needing to set $EDITOR or fork a subprocess.
func newNoteCmd() *cobra.Command {
	var (
		appendMode bool
		clear      bool
		fromStdin  bool
	)
	cmd := &cobra.Command{
		Use:     "note <id> [text...]",
		Aliases: []string{"notes"},
		Short:   "Add, edit, or clear the notes block on a single task",
		Long: `Add, edit, or clear the notes block on a single task.

Three input modes (mutually exclusive):

  tsk note 3 picked up the contract     # positional text (joined with spaces)
  tsk note 3                            # opens $EDITOR on a temp file
  echo "context" | tsk note 3 --stdin   # reads from stdin

Flags:
  --append       add to existing notes instead of replacing
  --clear        remove the notes block entirely

The editor flow pre-populates the temp file with the current notes so
you can edit-in-place. Empty/whitespace-only content in any mode is
treated as a clear (with a confirming message).

Examples:
  tsk note 3 found the source
  tsk note 3                          # opens vim/etc on the notes
  tsk note 3 --append "also: ping legal"
  cat thinking.md | tsk note 3 --stdin
  tsk note 3 --clear
`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseSingleID(args[0])
			if err != nil {
				return err
			}
			mode, err := pickNoteMode(args[1:], appendMode, clear, fromStdin)
			if err != nil {
				return err
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			t := s.ByID(id)
			if t == nil {
				return fmt.Errorf("no task with id %d in %s", id, s.Path)
			}
			return runNote(cmd, s, t, mode)
		},
	}
	cmd.Flags().BoolVar(&appendMode, "append", false, "add to existing notes instead of replacing")
	cmd.Flags().BoolVar(&clear, "clear", false, "remove the notes block entirely")
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "read notes from stdin (composes with pipes)")
	return cmd
}

// noteMode is one of: positional text, editor, stdin, clear.
type noteMode struct {
	kind   string // "text" | "editor" | "stdin" | "clear"
	text   string // populated when kind == "text"
	append bool   // valid for text/editor/stdin (not clear)
}

// pickNoteMode validates the flag combination and picks the source of
// the new note content.
func pickNoteMode(extraArgs []string, appendMode, clear, fromStdin bool) (noteMode, error) {
	hasText := len(extraArgs) > 0
	if clear {
		if hasText || appendMode || fromStdin {
			return noteMode{}, usageErrorf(
				"--clear cannot be combined with text args, --append, or --stdin",
			)
		}
		return noteMode{kind: "clear"}, nil
	}
	if hasText && fromStdin {
		return noteMode{}, usageErrorf("text args and --stdin are mutually exclusive")
	}
	if hasText {
		text := strings.TrimRight(strings.Join(extraArgs, " "), " \t\n")
		return noteMode{kind: "text", text: text, append: appendMode}, nil
	}
	if fromStdin {
		return noteMode{kind: "stdin", append: appendMode}, nil
	}
	return noteMode{kind: "editor", append: appendMode}, nil
}

// runNote dispatches to the right input source, then applies the result.
func runNote(cmd *cobra.Command, s saver, t *model.Task, mode noteMode) error {
	if mode.kind == "clear" {
		return applyNoteClear(cmd, s, t)
	}
	newContent, err := loadNewNoteContent(cmd, t, mode)
	if err != nil {
		return err
	}
	return applyNoteContent(cmd, s, t, newContent, mode.append)
}

// loadNewNoteContent fetches the new note text from the selected source.
func loadNewNoteContent(cmd *cobra.Command, t *model.Task, mode noteMode) (string, error) {
	switch mode.kind {
	case "text":
		return mode.text, nil
	case "stdin":
		return readNoteFromStdin(cmd)
	case "editor":
		return readNoteFromEditor(cmd, t)
	}
	return "", fmt.Errorf("internal: unknown note mode %q", mode.kind)
}

// readNoteFromStdin slurps all of stdin and returns it with trailing
// whitespace trimmed (preserves intentional blank lines inside).
func readNoteFromStdin(cmd *cobra.Command) (string, error) {
	r := cmd.InOrStdin()
	buf := make([]byte, 0, 1024)
	tmp := make([]byte, 4096)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return strings.TrimRight(string(buf), " \t\n"), nil
}

// noteEditor is the function used to invoke the editor. It's a variable
// (rather than a direct call) so tests can substitute a deterministic
// shim without spawning a process or setting $EDITOR.
var noteEditor = invokeEditor

// readNoteFromEditor writes the current notes to a temp file, runs
// noteEditor on it, then reads the result back.
func readNoteFromEditor(cmd *cobra.Command, t *model.Task) (string, error) {
	tmp, err := os.CreateTemp("", fmt.Sprintf("tsk-note-%d-*.md", t.ID))
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	if _, err := tmp.WriteString(t.Notes); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close temp file: %w", err)
	}

	if err := noteEditor(cmd, tmpPath); err != nil {
		return "", fmt.Errorf("editor: %w", err)
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return "", fmt.Errorf("read edited file: %w", err)
	}
	return strings.TrimRight(string(data), " \t\n"), nil
}

// invokeEditor is the production implementation: $EDITOR <path>, with
// vi as fallback. Stdio is wired to the cobra command's streams so
// interactive editors paint correctly.
func invokeEditor(cmd *cobra.Command, path string) error {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	c := exec.Command(editor, path) //nolint:gosec // editor is user-controlled by design
	c.Stdin = os.Stdin
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	return c.Run()
}

// applyNoteContent stores the new content (replace or append), with
// empty content treated as a clear-equivalent so editor users get the
// same explicit feedback as `--clear`.
func applyNoteContent(cmd *cobra.Command, s saver, t *model.Task, newContent string, append bool) error {
	newContent = strings.TrimRight(newContent, " \t\n")
	// Empty input -> treat as clear (with explicit messaging) so quitting
	// the editor without typing doesn't silently wipe a task by accident
	// — we still tell the user what happened.
	if strings.TrimSpace(newContent) == "" {
		return applyNoteClear(cmd, s, t)
	}
	if append && strings.TrimSpace(t.Notes) != "" {
		newContent = strings.TrimRight(t.Notes, " \t\n") + "\n\n" + newContent
	}
	if t.Notes == newContent {
		pf(cmd.OutOrStdout(), "#%d notes unchanged\n", t.ID)
		return nil
	}
	t.Notes = newContent
	if err := s.Save(); err != nil {
		return err
	}
	pf(cmd.OutOrStdout(), "#%d notes updated (%d bytes)\n", t.ID, len(newContent))
	return nil
}

// applyNoteClear wipes the notes field. No-op when already empty.
func applyNoteClear(cmd *cobra.Command, s saver, t *model.Task) error {
	if strings.TrimSpace(t.Notes) == "" {
		pf(cmd.OutOrStdout(), "#%d already has no notes\n", t.ID)
		return nil
	}
	t.Notes = ""
	if err := s.Save(); err != nil {
		return err
	}
	pf(cmd.OutOrStdout(), "#%d notes cleared\n", t.ID)
	return nil
}
