package commands

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
)

// newHashCmd implements `tsk hash`: print a stable content hash of the
// store. Two flavours, both deterministic:
//
//   - file mode (default): SHA-256 of the raw .tsk.md bytes. Cheap, but
//     sensitive to cosmetic whitespace differences a `--fix` lint pass
//     would normalize away. Use this for "did the file change at all?"
//     pre-commit checks.
//   - semantic mode (--semantic / -s): SHA-256 of a canonical projection
//     of the parsed task model — id, title, state, priority, due, wait,
//     tags, pin, notes, created, completed — in id order. Stable across
//     cosmetic edits (extra blank lines, bullet style, key order inside
//     the meta comment). Use this for CI: "did any TASK actually change?"
//
// Output is the 64-char hex digest plus the file path (`<hash>  <path>`
// shape, sha256sum-shaped so you can pipe it through `sha256sum -c`
// when the user picked file mode). With --json, both fields plus the
// mode are emitted in a stable schema. With --short, only the first 12
// hex characters are printed (git-style abbreviation) — handy for
// shell prompts and CI tags.
//
// Exit codes:
//
//	0 always (unless IO failure)
//
// Hash never changes for a re-issued semantic hash on identical task
// data even if the file was --fix-ed in between: the whole point.
func newHashCmd() *cobra.Command {
	var (
		semantic bool
		asJSON   bool
		short    bool
	)
	cmd := &cobra.Command{
		Use:   "hash",
		Short: "Print a stable content hash of the active .tsk.md (file or semantic)",
		Long: `Print a deterministic content hash of the active .tsk.md file.

Two modes:
  default     SHA-256 of the raw file bytes. Cheap, but changes for any
              cosmetic edit (bullet style, whitespace, key order).
  --semantic  SHA-256 of a canonical projection of the parsed tasks (id,
              title, state, priority, due, wait, tags, pin, notes,
              created, completed) in id order. Stable across cosmetic
              edits — the right choice for CI "did anything actually
              change?" checks.

Output:
  <64-char hex>  <path>            (default — sha256sum-shaped)
  <12-char hex>  <path>            (with --short)
  {"hash":"...", "path":"...",     (with --json)
   "mode":"file|semantic"}

Examples:
  tsk hash                          # raw file hash
  tsk hash --semantic               # task-content hash (CI-friendly)
  tsk hash --short                  # 12-char abbreviation
  tsk hash --json | jq -r '.hash'

Pair with cron / CI:
  before=$(tsk hash --semantic --short)
  ... time passes, edits happen ...
  after=$(tsk hash --semantic --short)
  [ "$before" != "$after" ] && echo "tasks changed"
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := resolveHashPath(cmd)
			if err != nil {
				return err
			}
			digest, err := computeHash(path, semantic)
			if err != nil {
				return err
			}
			mode := "file"
			if semantic {
				mode = "semantic"
			}
			emitted := digest
			if short {
				emitted = digest[:12]
			}
			return emitHashResult(cmd.OutOrStdout(), emitted, path, mode, asJSON)
		},
	}
	cmd.Flags().BoolVarP(&semantic, "semantic", "s", false, "hash the parsed task model instead of the raw file bytes")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit JSON {hash, path, mode}")
	cmd.Flags().BoolVar(&short, "short", false, "print a 12-character abbreviation (git-style)")
	return cmd
}

// resolveHashPath uses the same lookup as resolveStore but allows the
// raw file path (so we can SHA the bytes directly in file mode without
// the parser ever running).
func resolveHashPath(cmd *cobra.Command) (string, error) {
	path, _ := cmd.Flags().GetString("file")
	if path == "" {
		resolved, ok := store.Resolve("")
		if !ok {
			return "", fmt.Errorf("no .tsk.md found; run `tsk init`")
		}
		path = resolved
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("no .tsk.md at %s; run `tsk init`", path)
	}
	return path, nil
}

// computeHash branches on the requested mode.
func computeHash(path string, semantic bool) (string, error) {
	if semantic {
		s, err := store.Load(path)
		if err != nil {
			return "", fmt.Errorf("parse %s: %w", path, err)
		}
		return semanticHash(s.Tasks), nil
	}
	return fileHash(path)
}

// fileHash returns the lowercase-hex SHA-256 of the raw bytes.
func fileHash(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// semanticHash projects every task onto a canonical string form, joins
// the lines with '\n', and hashes the result. Each field is prefixed
// with a stable key so adding new fields later doesn't shift the bytes
// of unrelated keys. Tasks are emitted in ID order so file-order edits
// don't change the hash either.
//
// The canonical form is INTENTIONALLY different from store's render
// format: no leading `- [x]` markup, no `<!--` comment wrapper, no
// optional fields suppressed when empty (we always emit the key with
// an explicit "-" placeholder so adding a value never collapses two
// keys into one byte sequence by accident).
func semanticHash(tasks []model.Task) string {
	ids := make([]int, 0, len(tasks))
	byID := make(map[int]model.Task, len(tasks))
	for _, t := range tasks {
		ids = append(ids, t.ID)
		byID[t.ID] = t
	}
	// Hand-rolled in-place sort to avoid pulling sort into the file
	// (also the task counts here are small).
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j-1] > ids[j]; j-- {
			ids[j-1], ids[j] = ids[j], ids[j-1]
		}
	}
	var sb strings.Builder
	for _, id := range ids {
		t := byID[id]
		sb.WriteString(canonicalTaskLine(t))
		sb.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:])
}

// canonicalTaskLine emits one task as a stable, single-line projection.
// Newlines in notes are escaped so the line stays one record. Each
// key is always emitted; missing values become "-".
func canonicalTaskLine(t model.Task) string {
	done := "0"
	if t.Done {
		done = "1"
	}
	pinned := "0"
	if t.Pinned {
		pinned = "1"
	}
	due := "-"
	if t.Due != nil {
		due = t.Due.Format(model.DateLayout)
	}
	wait := "-"
	if t.WaitUntil != nil {
		wait = t.WaitUntil.Format(model.DateLayout)
	}
	created := "-"
	if !t.Created.IsZero() {
		// UTC + RFC3339 keeps the hash invariant under timezone-only
		// reinterpretations of the same instant.
		created = t.Created.UTC().Format(store.TimeLayout)
	}
	completed := "-"
	if t.Completed != nil {
		completed = t.Completed.UTC().Format(store.TimeLayout)
	}
	tags := "-"
	if len(t.Tags) > 0 {
		// NormalizeTags already sorts; defensive sort here in case a
		// caller hand-built a Task literal without normalizing.
		sorted := append([]string(nil), t.Tags...)
		for i := 1; i < len(sorted); i++ {
			for j := i; j > 0 && sorted[j-1] > sorted[j]; j-- {
				sorted[j-1], sorted[j] = sorted[j], sorted[j-1]
			}
		}
		tags = strings.Join(sorted, ",")
	}
	notes := escapeNotesForHash(t.Notes)
	return fmt.Sprintf(
		"id=%d\ttitle=%s\tdone=%s\tprio=%s\tdue=%s\twait=%s\tpin=%s\ttags=%s\tcreated=%s\tcompleted=%s\tnotes=%s",
		t.ID, t.Title, done, t.Priority.String(),
		due, wait, pinned, tags, created, completed, notes,
	)
}

// escapeNotesForHash escapes \n / \t / \\ so the canonical record stays
// on one line and embedded tabs can't accidentally collide with a
// field separator.
func escapeNotesForHash(s string) string {
	if s == "" {
		return "-"
	}
	var sb strings.Builder
	sb.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\\':
			sb.WriteString(`\\`)
		case '\n':
			sb.WriteString(`\n`)
		case '\t':
			sb.WriteString(`\t`)
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// emitHashResult dispatches between human and JSON output.
func emitHashResult(w io.Writer, digest, path, mode string, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Hash string `json:"hash"`
			Path string `json:"path"`
			Mode string `json:"mode"`
		}{Hash: digest, Path: path, Mode: mode})
	}
	// sha256sum shape so `tsk hash | sha256sum -c` works when mode=file.
	pf(w, "%s  %s\n", digest, path)
	return nil
}
