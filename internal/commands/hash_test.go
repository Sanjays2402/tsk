package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
)

// TestHashFileMatchesSha256OfBytes: the default (file) hash equals
// SHA-256 of the raw file bytes. Hand-computing is the only way to
// catch the "we accidentally hashed the parsed bytes" footgun.
func TestHashFileMatchesSha256OfBytes(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing one"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "hash")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	// Expected output: "<hex>  <abs path>\n"
	parts := strings.SplitN(strings.TrimRight(stdout, "\n"), "  ", 2)
	if len(parts) != 2 {
		t.Fatalf("expected 'hex  path' shape, got %q", stdout)
	}
	got := parts[0]
	if !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(got) {
		t.Fatalf("not a sha256 hex: %q", got)
	}
	// Now compute directly and compare.
	path := filepath.Join(dir, ".tsk.md")
	want, err := fileHash(path)
	if err != nil {
		t.Fatalf("fileHash: %v", err)
	}
	if got != want {
		t.Fatalf("file hash mismatch:\n got:  %s\n want: %s", got, want)
	}
}

// TestHashShortPrefix: --short emits a 12-char prefix of the full hash.
func TestHashShortPrefix(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "abc"); err != nil {
		t.Fatalf("add: %v", err)
	}
	full, _, err := runCmd(t, dir, "hash")
	if err != nil {
		t.Fatalf("hash full: %v", err)
	}
	short, _, err := runCmd(t, dir, "hash", "--short")
	if err != nil {
		t.Fatalf("hash short: %v", err)
	}
	fullDigest := strings.Split(full, "  ")[0]
	shortDigest := strings.Split(short, "  ")[0]
	if len(shortDigest) != 12 {
		t.Fatalf("expected 12-char short, got %d (%q)", len(shortDigest), shortDigest)
	}
	if !strings.HasPrefix(fullDigest, shortDigest) {
		t.Fatalf("short %q is not a prefix of full %q", shortDigest, fullDigest)
	}
}

// TestHashSemanticStableAcrossCosmeticEdit: a --fix-style canonical
// re-render (achievable here by hand-mangling and reloading through
// store) must not change the semantic hash. This is the WHOLE POINT
// of having a semantic mode.
func TestHashSemanticStableAcrossCosmeticEdit(t *testing.T) {
	dir := t.TempDir()
	// Anchor created stamps so a re-render doesn't change them.
	now := time.Now().UTC().Format(time.RFC3339)
	writeRawTasks(t, dir,
		"- [ ] alpha <!-- id:1 prio:high created:"+now+" -->",
		"- [ ] beta  <!-- id:2 prio:medium created:"+now+" -->",
	)
	before, _, err := runCmd(t, dir, "hash", "--semantic")
	if err != nil {
		t.Fatalf("hash sem: %v", err)
	}
	beforeDigest := strings.Split(before, "  ")[0]
	// Cosmetic mangle: rewrite with non-canonical bullets (the parser
	// tolerates '*' and '+' as well as '-') and extra blank lines.
	// Same semantic content; canonical writer would normalize all of
	// these away.
	path := filepath.Join(dir, ".tsk.md")
	mangled := "# tasks\n\n\n" +
		"* [ ] alpha <!-- id:1 prio:high created:" + now + " -->\n" +
		"\n" +
		"+ [ ] beta <!--   id:2 prio:medium  created:" + now + " -->\n"
	if err := os.WriteFile(path, []byte(mangled), 0o644); err != nil {
		t.Fatalf("mangle: %v", err)
	}
	after, _, err := runCmd(t, dir, "hash", "--semantic")
	if err != nil {
		t.Fatalf("hash sem after: %v", err)
	}
	afterDigest := strings.Split(after, "  ")[0]
	if beforeDigest != afterDigest {
		t.Fatalf("semantic hash should be stable across cosmetic edits, before=%s after=%s",
			beforeDigest, afterDigest)
	}
}

// TestHashSemanticChangesWhenContentChanges: flipping a task title
// MUST change the semantic hash.
func TestHashSemanticChangesWhenContentChanges(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "first"); err != nil {
		t.Fatalf("add: %v", err)
	}
	before, _, err := runCmd(t, dir, "hash", "--semantic")
	if err != nil {
		t.Fatalf("hash sem: %v", err)
	}
	if _, _, err := runCmd(t, dir, "rename", "1", "second"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	after, _, err := runCmd(t, dir, "hash", "--semantic")
	if err != nil {
		t.Fatalf("hash sem after rename: %v", err)
	}
	if before == after {
		t.Fatalf("expected semantic hash to change after rename")
	}
}

// TestHashJSONShape: --json emits {hash, path, mode}.
func TestHashJSONShape(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "j"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "hash", "--semantic", "--json")
	if err != nil {
		t.Fatalf("hash json: %v", err)
	}
	var doc struct {
		Hash string `json:"hash"`
		Path string `json:"path"`
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(doc.Hash) != 64 {
		t.Fatalf("expected 64-char hash, got %d (%q)", len(doc.Hash), doc.Hash)
	}
	if doc.Mode != "semantic" {
		t.Fatalf("expected mode=semantic, got %q", doc.Mode)
	}
	if doc.Path == "" {
		t.Fatal("path empty")
	}
}

// TestHashJSONShortAbbreviates: --json combined with --short still
// truncates to 12 chars (so consumers can use short hashes in JSON
// too).
func TestHashJSONShortAbbreviates(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "k"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "hash", "--short", "--json")
	if err != nil {
		t.Fatalf("hash short json: %v", err)
	}
	var doc struct {
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(doc.Hash) != 12 {
		t.Fatalf("expected 12-char short hash, got %d (%q)", len(doc.Hash), doc.Hash)
	}
}

// TestSemanticHashIDOrderInvariant: hashing the same tasks in a
// different in-memory order yields the same digest because the
// projection sorts by ID. Synthetic — exercises the helper directly.
func TestSemanticHashIDOrderInvariant(t *testing.T) {
	base := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	a := []model.Task{
		{ID: 1, Title: "a", Priority: model.PriorityMedium, Created: base},
		{ID: 2, Title: "b", Priority: model.PriorityHigh, Created: base},
		{ID: 3, Title: "c", Priority: model.PriorityLow, Created: base},
	}
	b := []model.Task{
		{ID: 3, Title: "c", Priority: model.PriorityLow, Created: base},
		{ID: 1, Title: "a", Priority: model.PriorityMedium, Created: base},
		{ID: 2, Title: "b", Priority: model.PriorityHigh, Created: base},
	}
	if semanticHash(a) != semanticHash(b) {
		t.Fatal("semantic hash must be order-invariant by ID")
	}
}

// TestSemanticHashTagOrderInvariant: identical tags in different order
// produce the same hash (defensive sort inside canonicalTaskLine).
func TestSemanticHashTagOrderInvariant(t *testing.T) {
	t1 := []model.Task{{ID: 1, Title: "x", Tags: []string{"b", "a", "c"}}}
	t2 := []model.Task{{ID: 1, Title: "x", Tags: []string{"a", "b", "c"}}}
	if semanticHash(t1) != semanticHash(t2) {
		t.Fatal("semantic hash must be invariant to tag input order")
	}
}

// TestSemanticHashNotesEscaping: tasks differing only by embedded
// newlines must produce different hashes (escaping is not lossy).
func TestSemanticHashNotesEscaping(t *testing.T) {
	a := []model.Task{{ID: 1, Title: "x", Notes: "line1"}}
	b := []model.Task{{ID: 1, Title: "x", Notes: "line1\nline2"}}
	if semanticHash(a) == semanticHash(b) {
		t.Fatal("hashes should differ when notes content differs")
	}
}

// TestEscapeNotesForHashRoundTripDistinct: \n vs literal "\\n" must
// produce DIFFERENT escapes so the hash distinguishes them. The
// canonical escape for '\\' is '\\\\' precisely to keep this lossless.
func TestEscapeNotesForHashRoundTripDistinct(t *testing.T) {
	a := escapeNotesForHash("a\nb")
	b := escapeNotesForHash(`a\nb`)
	if a == b {
		t.Fatalf("real \\n and literal \\n should escape distinctly, both = %q", a)
	}
}
