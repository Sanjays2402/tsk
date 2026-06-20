package commands

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestTagAddsAndRemovesInOneShot(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing", "-t", "old"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "tag", "1", "+new", "-old", "+urgent")
	if err != nil {
		t.Fatalf("tag: %v", err)
	}
	if !strings.Contains(stdout, "#1 tags ") {
		t.Fatalf("expected transition line, got: %q", stdout)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(content, "tags:new,urgent") {
		t.Fatalf("expected sorted tags new,urgent on disk, got:\n%s", content)
	}
}

func TestTagBareNameMeansAdd(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "tag", "1", "work"); err != nil {
		t.Fatalf("tag shorthand: %v", err)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	if !strings.Contains(content, "tags:work") {
		t.Fatalf("expected work tag added via shorthand, got:\n%s", content)
	}
}

func TestTagIsCaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing", "-t", "Work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "tag", "1", "+WORK"); err != nil {
		t.Fatalf("tag dup-add: %v", err)
	}
	content := readFile(t, filepath.Join(dir, ".tsk.md"))
	// Should not duplicate — Work and WORK normalize to the same.
	if strings.Count(content, "work") != 1 {
		t.Fatalf("expected exactly one 'work' tag, got:\n%s", content)
	}
	// Then remove it case-mismatched.
	if _, _, err := runCmd(t, dir, "tag", "1", "-Work"); err != nil {
		t.Fatalf("tag rm: %v", err)
	}
	content = readFile(t, filepath.Join(dir, ".tsk.md"))
	if strings.Contains(content, "tags:") {
		t.Fatalf("expected no tags after removal, got:\n%s", content)
	}
}

func TestTagNoChangeIsNoop(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing", "-t", "work"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "tag", "1", "+work", "-missing")
	if err != nil {
		t.Fatalf("tag noop: %v", err)
	}
	if !strings.Contains(stdout, "tags unchanged") {
		t.Fatalf("expected unchanged notice, got: %q", stdout)
	}
}

func TestTagRejectsConflict(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "tag", "1", "+a", "-a")
	if err == nil {
		t.Fatal("expected conflict error")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit 2, got %v", err)
	}
}

func TestTagRejectsEmptyName(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "tag", "1", "+")
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestTagNeedsAtLeastOneOp(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "tag", "1")
	if err == nil {
		t.Fatal("expected error when no tag op given")
	}
}

func TestParseTagOpsTable(t *testing.T) {
	cases := []struct {
		name        string
		in          []string
		wantAdds    []string
		wantRemoves []string
		wantErr     bool
	}{
		{name: "single add", in: []string{"+a"}, wantAdds: []string{"a"}},
		{name: "single rm", in: []string{"-a"}, wantRemoves: []string{"a"}},
		{name: "bare is add", in: []string{"a"}, wantAdds: []string{"a"}},
		{name: "mixed", in: []string{"+a", "-b", "c"}, wantAdds: []string{"a", "c"}, wantRemoves: []string{"b"}},
		{name: "dedup adds", in: []string{"+a", "+A"}, wantAdds: []string{"a"}},
		{name: "conflict", in: []string{"+a", "-A"}, wantErr: true},
		{name: "empty", in: []string{""}, wantErr: true},
		{name: "lone plus", in: []string{"+"}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			adds, removes, err := parseTagOps(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !equalStrings(adds, tc.wantAdds) {
				t.Fatalf("adds: got %v want %v", adds, tc.wantAdds)
			}
			if !equalStrings(removes, tc.wantRemoves) {
				t.Fatalf("removes: got %v want %v", removes, tc.wantRemoves)
			}
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
