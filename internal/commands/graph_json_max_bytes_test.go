package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGraphJSONMaxBytesEvictsOldLinesUntilUnderCap: with
// --max-bytes set below the current size, the oldest lines are
// evicted FIFO until the file fits under the cap.
func TestGraphJSONMaxBytesEvictsOldLinesUntilUnderCap(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	snap := filepath.Join(dir, "snap.jsonl")
	// Build up a few records: each compact envelope is ~70 bytes.
	for i := 0; i < 10; i++ {
		if _, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", snap, "--append"); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	preBody, _ := os.ReadFile(snap)
	preLines := nonEmptyLineCount(string(preBody))
	if preLines != 10 {
		t.Fatalf("expected 10 records before max-bytes; got %d", preLines)
	}

	// Cap at 200 bytes — ~3 records max.
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", snap, "--append", "--max-bytes", "200")
	if err != nil {
		t.Fatalf("append with max-bytes: %v", err)
	}
	if !strings.Contains(stdout, "byte-cap evicted") {
		t.Errorf("expected byte-cap eviction message; got: %s", stdout)
	}
	body, _ := os.ReadFile(snap)
	if int64(len(body)) > 200 {
		t.Errorf("expected body under 200 bytes; got %d bytes", len(body))
	}
	postLines := nonEmptyLineCount(string(body))
	if postLines == 0 {
		t.Errorf("expected at least the new record retained; got 0 lines")
	}
	if postLines >= preLines+1 {
		t.Errorf("expected eviction (less than %d records); got %d", preLines+1, postLines)
	}
}

// TestGraphJSONMaxBytesNoOpUnderCap: when the file is already
// under cap after the new append, no eviction happens.
func TestGraphJSONMaxBytesNoOpUnderCap(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	snap := filepath.Join(dir, "snap.jsonl")
	// One record then cap at 10 MiB — way above the single
	// envelope size.
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", snap, "--append", "--max-bytes", "10485760")
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if strings.Contains(stdout, "byte-cap evicted") {
		t.Errorf("expected NO byte-cap eviction under cap; got: %s", stdout)
	}
	if !strings.Contains(stdout, "format=jsonl") {
		t.Errorf("expected jsonl tag; got: %s", stdout)
	}
}

// TestGraphJSONMaxBytesRequiresAppend: setting --max-bytes without
// --append is a usage error — the byte cap only makes sense for
// the streaming path (overwrite is a single record by definition).
func TestGraphJSONMaxBytesRequiresAppend(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	snap := filepath.Join(dir, "snap.jsonl")
	_, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", snap, "--max-bytes", "1024")
	if err == nil {
		t.Fatal("expected error for --max-bytes without --append")
	}
	if !strings.Contains(err.Error(), "--max-bytes requires --append") {
		t.Errorf("expected '--max-bytes requires --append' error; got: %v", err)
	}
}

// TestGraphJSONMaxBytesNegativeRejected: a negative byte cap is a
// usage error.
func TestGraphJSONMaxBytesNegativeRejected(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	snap := filepath.Join(dir, "snap.jsonl")
	_, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", snap, "--append", "--max-bytes", "-1")
	if err == nil {
		t.Fatal("expected error for negative --max-bytes")
	}
	if !strings.Contains(err.Error(), "--max-bytes must be >= 0") {
		t.Errorf("expected '--max-bytes must be >= 0' error; got: %v", err)
	}
}

// TestGraphJSONMaxBytesZeroDisablesCap: --max-bytes 0 explicitly
// disables the byte cap (no eviction, no message).
func TestGraphJSONMaxBytesZeroDisablesCap(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	snap := filepath.Join(dir, "snap.jsonl")
	for i := 0; i < 5; i++ {
		if _, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", snap, "--append"); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", snap, "--append", "--max-bytes", "0")
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if strings.Contains(stdout, "byte-cap evicted") {
		t.Errorf("expected no eviction for --max-bytes 0; got: %s", stdout)
	}
	body, _ := os.ReadFile(snap)
	if nonEmptyLineCount(string(body)) != 6 {
		t.Errorf("expected 6 records (all kept); got %d", nonEmptyLineCount(string(body)))
	}
}

// TestGraphJSONMaxBytesComposesWithRotate: when BOTH --max-bytes
// and --rotate are set, both caps apply on the same append and
// the message reflects both eviction counts when both fire.
func TestGraphJSONMaxBytesComposesWithRotate(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	snap := filepath.Join(dir, "snap.jsonl")
	// 8 records, then cap at 5 by count AND ~150 bytes (well
	// under 5 records' worth) — bytes cap should fire after the
	// count cap fires, evicting additional lines.
	for i := 0; i < 8; i++ {
		if _, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", snap, "--append"); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json",
		"--output", snap, "--append", "--rotate", "5", "--max-bytes", "150")
	if err != nil {
		t.Fatalf("append with both caps: %v", err)
	}
	body, _ := os.ReadFile(snap)
	if int64(len(body)) > 150 {
		t.Errorf("expected body under 150 bytes; got %d bytes", len(body))
	}
	postLines := nonEmptyLineCount(string(body))
	if postLines >= 5 {
		t.Errorf("expected fewer than 5 records (byte cap stricter); got %d", postLines)
	}
	if !strings.Contains(stdout, "byte cap") && !strings.Contains(stdout, "byte-cap") {
		t.Errorf("expected combined or byte-cap eviction message; got: %s", stdout)
	}
}

// TestGraphJSONMaxBytesPreservesLastRecord: even when the most-
// recent record is larger than the byte cap, the latest snapshot
// is always preserved (we never drop the last line).
func TestGraphJSONMaxBytesPreservesLastRecord(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	snap := filepath.Join(dir, "snap.jsonl")
	// Cap at 1 byte — guarantees the single record is "oversized".
	stdout, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", snap, "--append", "--max-bytes", "1")
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	body, _ := os.ReadFile(snap)
	if nonEmptyLineCount(string(body)) != 1 {
		t.Errorf("expected exactly 1 record preserved (the last); got %d records\nbody: %s", nonEmptyLineCount(string(body)), body)
	}
	// No eviction message because there was nothing OLDER to evict.
	if strings.Contains(stdout, "byte-cap evicted") {
		t.Errorf("expected no eviction message for single-record file; got: %s", stdout)
	}
}

// TestGraphJSONMaxBytesHelpMention: the --help output mentions the
// new flag and documents its --append requirement.
func TestGraphJSONMaxBytesHelpMention(t *testing.T) {
	dir := t.TempDir()
	_, combined, err := runCmd(t, dir, "graph", "--help")
	if err != nil {
		t.Fatalf("help: %v", err)
	}
	if !strings.Contains(combined, "--max-bytes") {
		t.Errorf("expected --max-bytes in help text; got:\n%s", combined)
	}
}

// TestRotateJSONLFileByBytesDirectUnit: direct unit test for the
// helper, mirrors the rotateJSONLFile direct test pattern.
func TestRotateJSONLFileByBytesDirectUnit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snap.jsonl")

	// Empty/missing file: no-op.
	if dropped, err := rotateJSONLFileByBytes(path, 100); err != nil {
		t.Fatalf("missing file should not error: %v", err)
	} else if dropped != 0 {
		t.Errorf("expected 0 dropped on missing file; got %d", dropped)
	}

	// Write 5 lines of ~20 bytes each (~100 bytes total including newlines).
	body := "aaaaaaaaaaaaaaaaaaa\nbbbbbbbbbbbbbbbbbbb\nccccccccccccccccccc\nddddddddddddddddddd\neeeeeeeeeeeeeeeeeee\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Cap at 50 bytes — should evict the oldest lines until under.
	dropped, err := rotateJSONLFileByBytes(path, 50)
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if dropped == 0 {
		t.Errorf("expected at least one eviction; got 0")
	}
	post, _ := os.ReadFile(path)
	if int64(len(post)) > 50 {
		t.Errorf("expected post-rotate body under 50 bytes; got %d", len(post))
	}
	// Last line MUST be retained.
	if !strings.HasSuffix(string(post), "eeeeeeeeeeeeeeeeeee\n") {
		t.Errorf("expected last line preserved after rotation; got body:\n%s", post)
	}

	// Cap of 0 is no-op (the "disabled" branch).
	preBody, _ := os.ReadFile(path)
	if dropped, err := rotateJSONLFileByBytes(path, 0); err != nil {
		t.Fatalf("zero cap should not error: %v", err)
	} else if dropped != 0 {
		t.Errorf("expected 0 dropped on cap=0; got %d", dropped)
	}
	postBody, _ := os.ReadFile(path)
	if string(preBody) != string(postBody) {
		t.Errorf("expected zero cap to leave body unchanged; pre/post diverged")
	}
}

// TestGraphJSONMaxBytesAtomicTmpCleanup: a successful rotation
// leaves no orphan .maxbytes.tmp behind in the output directory.
func TestGraphJSONMaxBytesAtomicTmpCleanup(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "root"); err != nil {
		t.Fatalf("add: %v", err)
	}
	snap := filepath.Join(dir, "snap.jsonl")
	for i := 0; i < 6; i++ {
		if _, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", snap, "--append"); err != nil {
			t.Fatalf("append: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "graph", "--reachable", "1", "--json", "--output", snap, "--append", "--max-bytes", "100"); err != nil {
		t.Fatalf("append with cap: %v", err)
	}
	// No .maxbytes.tmp left.
	if _, err := os.Stat(snap + ".maxbytes.tmp"); !os.IsNotExist(err) {
		t.Errorf("expected no orphan .maxbytes.tmp file; stat err=%v", err)
	}
}

// nonEmptyLineCount is a tiny test helper: returns the count of
// non-empty newline-separated lines. Used for JSONL record counting
// without depending on json decoding (which would re-test a path
// already covered).
func nonEmptyLineCount(s string) int {
	n := 0
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			n++
		}
	}
	return n
}
