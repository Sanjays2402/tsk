package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTagsCountsUndoneByDefault(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "c", "-t", "home"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "tags")
	if err != nil {
		t.Fatalf("tags: %v", err)
	}
	if !strings.Contains(stdout, "#work") || !strings.Contains(stdout, "#home") {
		t.Fatalf("expected both tags listed, got:\n%s", stdout)
	}
	// work should appear before home (count 2 > 1).
	workIdx := strings.Index(stdout, "#work")
	homeIdx := strings.Index(stdout, "#home")
	if workIdx == -1 || homeIdx == -1 || workIdx > homeIdx {
		t.Fatalf("expected #work before #home (count-desc), got:\n%s", stdout)
	}
}

func TestTagsHidesDoneTasksByDefault(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "open", "-t", "alpha"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "closed", "-t", "beta"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "tags")
	if err != nil {
		t.Fatalf("tags: %v", err)
	}
	if !strings.Contains(stdout, "#alpha") {
		t.Fatalf("expected #alpha (undone), got:\n%s", stdout)
	}
	if strings.Contains(stdout, "#beta") {
		t.Fatalf("#beta is on a done task, should be hidden by default, got:\n%s", stdout)
	}
}

func TestTagsAllIncludesDone(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "open", "-t", "alpha"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "closed", "-t", "beta"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "tags", "--all")
	if err != nil {
		t.Fatalf("tags --all: %v", err)
	}
	if !strings.Contains(stdout, "#alpha") || !strings.Contains(stdout, "#beta") {
		t.Fatalf("--all should include both, got:\n%s", stdout)
	}
}

func TestTagsDoneOnly(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "open", "-t", "alpha"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "closed", "-t", "beta"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "2"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "tags", "--done")
	if err != nil {
		t.Fatalf("tags --done: %v", err)
	}
	if !strings.Contains(stdout, "#beta") {
		t.Fatalf("--done should include #beta, got:\n%s", stdout)
	}
	if strings.Contains(stdout, "#alpha") {
		t.Fatalf("--done should exclude open #alpha, got:\n%s", stdout)
	}
}

func TestTagsAllAndDoneMutuallyExclusive(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x", "-t", "y"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "tags", "--all", "--done")
	if err == nil {
		t.Fatal("expected error for --all + --done")
	}
}

func TestTagsJSONShape(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "tags", "--json")
	if err != nil {
		t.Fatalf("tags --json: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if len(rows) != 1 || rows[0]["tag"] != "work" || rows[0]["count"].(float64) != 2 {
		t.Fatalf("unexpected JSON: %v", rows)
	}
}

func TestTagsJSONEmitsEmptyArrayNotNull(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "untagged"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "tags", "--json")
	if err != nil {
		t.Fatalf("tags --json: %v", err)
	}
	if strings.TrimSpace(stdout) != "[]" {
		t.Fatalf("empty result should be [], got %q", stdout)
	}
}

func TestTagsLimitCaps(t *testing.T) {
	dir := t.TempDir()
	for _, tg := range []string{"a", "b", "c", "d", "e"} {
		if _, _, err := runCmd(t, dir, "add", "task", "-t", tg); err != nil {
			t.Fatalf("add %s: %v", tg, err)
		}
	}
	stdout, _, err := runCmd(t, dir, "tags", "--limit", "2")
	if err != nil {
		t.Fatalf("tags --limit: %v", err)
	}
	// Each line is one tag; expect 2.
	if n := strings.Count(stdout, "\n"); n != 2 {
		t.Fatalf("expected 2 lines from --limit 2, got %d:\n%s", n, stdout)
	}
}

func TestTagsMinHidesRareTags(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 3; i++ {
		if _, _, err := runCmd(t, dir, "add", "task", "-t", "common"); err != nil {
			t.Fatalf("add common: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "add", "task", "-t", "rare"); err != nil {
		t.Fatalf("add rare: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "tags", "--min", "2")
	if err != nil {
		t.Fatalf("tags --min: %v", err)
	}
	if !strings.Contains(stdout, "#common") {
		t.Fatalf("expected #common (count 3), got:\n%s", stdout)
	}
	if strings.Contains(stdout, "#rare") {
		t.Fatalf("#rare (count 1) should be hidden under --min 2, got:\n%s", stdout)
	}
}

func TestTagsSortAlpha(t *testing.T) {
	dir := t.TempDir()
	// Make 'home' the most-used so default order is home, work.
	for i := 0; i < 3; i++ {
		if _, _, err := runCmd(t, dir, "add", "task", "-t", "home"); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "add", "task", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "tags", "--sort", "alpha")
	if err != nil {
		t.Fatalf("tags --sort: %v", err)
	}
	homeIdx := strings.Index(stdout, "#home")
	workIdx := strings.Index(stdout, "#work")
	if homeIdx == -1 || workIdx == -1 || homeIdx > workIdx {
		t.Fatalf("alpha sort should put #home before #work, got:\n%s", stdout)
	}
}

func TestTagsEmptyMessage(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "untagged"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "tags")
	if err != nil {
		t.Fatalf("tags: %v", err)
	}
	if !strings.Contains(stdout, "no tags") {
		t.Fatalf("expected 'no tags' message, got:\n%s", stdout)
	}
}

func TestTagsRejectsNegativeLimit(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x", "-t", "y"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "tags", "--limit", "-1")
	if err == nil {
		t.Fatal("expected error for negative limit")
	}
}

func TestTagsRejectsNegativeMin(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x", "-t", "y"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "tags", "--min", "-1")
	if err == nil {
		t.Fatal("expected error for negative --min")
	}
}

func TestTagsAlias(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing", "-t", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "taglist"); err != nil {
		t.Fatalf("taglist alias: %v", err)
	}
}

func TestPickTagScope(t *testing.T) {
	cases := []struct {
		all, done bool
		want      tagScope
		wantErr   bool
	}{
		{false, false, tagScopeUndone, false},
		{true, false, tagScopeAll, false},
		{false, true, tagScopeDone, false},
		{true, true, 0, true},
	}
	for _, tc := range cases {
		got, err := pickTagScope(tc.all, tc.done)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("all=%v done=%v: expected error", tc.all, tc.done)
			}
			continue
		}
		if err != nil {
			t.Fatalf("all=%v done=%v: unexpected error %v", tc.all, tc.done, err)
		}
		if got != tc.want {
			t.Fatalf("all=%v done=%v: got %d want %d", tc.all, tc.done, got, tc.want)
		}
	}
}
