package commands

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sanjays2402/tsk/internal/model"
)

// TestDedupeExactCatchesCaseAndWhitespace: the default (exact) mode
// must catch "Pay rent" vs "pay rent" vs "pay  rent".
func TestDedupeExactCatchesCaseAndWhitespace(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"Pay rent", "pay rent", "pay  rent", "completely different"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	stdout, _, err := runCmd(t, dir, "dedupe")
	// Expected: ExitCode 1 (dupes found is signal, not error).
	if err == nil {
		t.Fatal("expected non-nil error (silentExit 1) when dupes present")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 1 {
		t.Fatalf("expected ExitCode 1, got %v", err)
	}
	for _, want := range []string{"Pay rent", "pay rent", "pay  rent"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %q in dedupe output:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "completely different") {
		t.Fatalf("non-dupe should NOT appear:\n%s", stdout)
	}
}

// TestDedupePunctuationNormalized: trailing/embedded punctuation is
// stripped, so "buy milk." and "buy milk" collide.
func TestDedupePunctuationNormalized(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"buy milk", "buy milk.", "buy milk!"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add %s: %v", title, err)
		}
	}
	stdout, _, err := runCmd(t, dir, "dedupe")
	if err == nil {
		t.Fatal("expected ExitCode 1")
	}
	if !strings.Contains(stdout, "buy milk.") {
		t.Fatalf("expected punctuated variant in output:\n%s", stdout)
	}
}

// TestDedupeNoDupesReturns0: a clean store exits 0 with the explicit
// "no duplicates" message (so a user can't mistake silence for error).
func TestDedupeNoDupesReturns0(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"alpha", "beta", "gamma"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	stdout, _, err := runCmd(t, dir, "dedupe")
	if err != nil {
		t.Fatalf("expected exit 0 on no dupes, got %v", err)
	}
	if !strings.Contains(stdout, "no duplicates") {
		t.Fatalf("expected 'no duplicates' message, got:\n%s", stdout)
	}
}

// TestDedupeNearFlag: --near catches one-edit typos that exact mode
// misses.
func TestDedupeNearFlag(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"deploy infra", "dpeloy infra", "unrelated thing"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	// Exact mode shouldn't surface them (different normalized strings).
	stdout, _, _ := runCmd(t, dir, "dedupe")
	if strings.Contains(stdout, "dpeloy") {
		t.Fatalf("exact mode should NOT catch transposition, got:\n%s", stdout)
	}
	// --near should.
	stdout, _, err := runCmd(t, dir, "dedupe", "--near")
	if err == nil {
		t.Fatal("expected ExitCode 1 with --near")
	}
	if !strings.Contains(stdout, "dpeloy infra") {
		t.Fatalf("--near should catch transposition, got:\n%s", stdout)
	}
}

// TestDedupeNearStrict: --near=1 must NOT pull in distance-2 pairs
// (the bound is real, not just a hint).
func TestDedupeNearStrict(t *testing.T) {
	dir := t.TempDir()
	// "abcde" vs "axcye" = distance 2 (substitute 2 chars). With
	// --near=1, these should NOT be grouped.
	for _, title := range []string{"abcde", "axcye"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	stdout, _, err := runCmd(t, dir, "dedupe", "--near=1")
	if err != nil {
		t.Fatalf("expected exit 0 (no dupes within distance 1), got %v", err)
	}
	if !strings.Contains(stdout, "no duplicates") {
		t.Fatalf("expected no dupes at distance 1:\n%s", stdout)
	}
}

// TestDedupeDoneOnlyExcludesUndone: --done flips the scope.
func TestDedupeDoneOnlyExcludesUndone(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "dup-a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "dup-a"); err != nil {
		t.Fatalf("add: %v", err)
	}
	// Default scope (undone): both surface as dupes.
	if _, _, err := runCmd(t, dir, "dedupe"); err == nil {
		t.Fatal("expected ExitCode 1 in default scope")
	}
	// Now finish one. With --done, only the finished one is in scope,
	// so no dupes.
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "dedupe", "--done")
	if err != nil {
		t.Fatalf("expected exit 0 (only 1 done task), got %v", err)
	}
	if !strings.Contains(stdout, "no duplicates") {
		t.Fatalf("expected no dupes when only 1 done task:\n%s", stdout)
	}
}

// TestDedupeAllUnion: --all walks done + undone.
func TestDedupeAllUnion(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "dup"); err != nil {
		t.Fatalf("add 1: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "dup"); err != nil {
		t.Fatalf("add 2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	// In default scope only the undone task is in scope, so no dupes.
	if _, _, err := runCmd(t, dir, "dedupe"); err != nil {
		t.Fatalf("expected exit 0 in default scope post-done, got %v", err)
	}
	// --all reintroduces both and the dupe surfaces.
	if _, _, err := runCmd(t, dir, "dedupe", "--all"); err == nil {
		t.Fatal("expected ExitCode 1 with --all")
	}
}

// TestDedupeJSONShape: stable schema.
func TestDedupeJSONShape(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"x", "x"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	stdout, _, err := runCmd(t, dir, "dedupe", "--json")
	if err == nil {
		t.Fatal("expected ExitCode 1")
	}
	var doc struct {
		Groups []struct {
			Distance int            `json:"distance"`
			Tasks    []model.Task   `json:"tasks"`
			Anything map[string]any `json:"-"`
		} `json:"groups"`
	}
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(doc.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(doc.Groups))
	}
	if len(doc.Groups[0].Tasks) != 2 {
		t.Fatalf("expected 2 tasks in group, got %d", len(doc.Groups[0].Tasks))
	}
	if doc.Groups[0].Distance != 0 {
		t.Fatalf("expected distance 0 for exact dupes, got %d", doc.Groups[0].Distance)
	}
}

// TestDedupeFilesOnly: --files-only prints just IDs, groups separated
// by blank lines.
func TestDedupeFilesOnly(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"a", "a", "b", "b"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	stdout, _, err := runCmd(t, dir, "dedupe", "--files-only")
	if err == nil {
		t.Fatal("expected ExitCode 1")
	}
	got := strings.TrimSpace(stdout)
	want := "1\n2\n\n3\n4"
	if got != want {
		t.Fatalf("--files-only output\n got: %q\nwant: %q", got, want)
	}
}

// TestDedupeMutexes: --done + --all and --files-only + --json are both
// usage errors.
func TestDedupeMutexes(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	for _, args := range [][]string{
		{"dedupe", "--done", "--all"},
		{"dedupe", "--files-only", "--json"},
	} {
		_, _, err := runCmd(t, dir, args...)
		if err == nil {
			t.Fatalf("expected error for %v", args)
		}
		var ec ExitCoder
		if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
			t.Fatalf("expected ExitCode 2 for %v, got %v", args, err)
		}
	}
}

// TestDedupeNegativeNearRejected.
func TestDedupeNegativeNearRejected(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "dedupe", "--near=-1")
	if err == nil {
		t.Fatal("expected error for negative --near")
	}
}

// TestNormalizeTitleVariants: direct unit test of the title normalizer
// so refactors keep the same semantics.
func TestNormalizeTitleVariants(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"  Pay  rent  ", "pay rent"},
		{"Buy milk.", "buy milk"},
		{"PR #42!!!", "pr 42"},
		{"  ", ""},
		{"hello\tworld", "hello world"},
	}
	for _, c := range cases {
		got := normalizeTitle(c.in)
		if got != c.want {
			t.Fatalf("normalize(%q) = %q want %q", c.in, got, c.want)
		}
	}
}

// TestBoundedEditDistanceBasic + early termination behaves correctly.
func TestBoundedEditDistanceBasic(t *testing.T) {
	cases := []struct {
		a, b string
		cap  int
		want int
	}{
		{"kitten", "sitting", 5, 3},        // canonical
		{"abc", "abc", 1, 0},               // identical
		{"abc", "axc", 1, 1},               // substitute
		{"abc", "abcd", 1, 1},              // insert
		{"abcd", "abc", 1, 1},              // delete
		{"abcd", "wxyz", 1, 2},             // cap+1 (true distance >1)
		{"abcdefghij", "qqqqqqqqqq", 2, 3}, // bail early on length skew?
	}
	for _, c := range cases {
		got := boundedEditDistance(c.a, c.b, c.cap)
		if c.want > c.cap {
			if got <= c.cap {
				t.Fatalf("dist(%q,%q,cap=%d)=%d, expected >%d (capped)", c.a, c.b, c.cap, got, c.cap)
			}
			continue
		}
		if got != c.want {
			t.Fatalf("dist(%q,%q,cap=%d)=%d want %d", c.a, c.b, c.cap, got, c.want)
		}
	}
}

// TestFindNearDupeGroupsClustering: synthetic, asserts the greedy
// clusterer doesn't pull unrelated items together at the chosen
// distance and that each cluster keeps file order.
func TestFindNearDupeGroupsClustering(t *testing.T) {
	tasks := []model.Task{
		{ID: 1, Title: "deploy infra"},
		{ID: 2, Title: "dpeloy infra"},
		{ID: 3, Title: "deploy infra2"},
		{ID: 4, Title: "buy milk"},
		{ID: 5, Title: "by milk"},
	}
	groups := findNearDupeGroups(tasks, 2)
	if len(groups) != 2 {
		t.Fatalf("expected 2 clusters, got %d", len(groups))
	}
	// First cluster should be the deploy family (ids 1,2,3).
	gotIDs := []int{}
	for _, t := range groups[0].Tasks {
		gotIDs = append(gotIDs, t.ID)
	}
	if len(gotIDs) != 3 || gotIDs[0] != 1 || gotIDs[1] != 2 || gotIDs[2] != 3 {
		t.Fatalf("first cluster ids = %v, want [1,2,3]", gotIDs)
	}
	// Second cluster should be the milk pair (ids 4,5).
	gotIDs = nil
	for _, t := range groups[1].Tasks {
		gotIDs = append(gotIDs, t.ID)
	}
	if len(gotIDs) != 2 || gotIDs[0] != 4 || gotIDs[1] != 5 {
		t.Fatalf("second cluster ids = %v, want [4,5]", gotIDs)
	}
}
