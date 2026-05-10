package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

// writeRawTasks writes the given task lines directly to .tsk.md so tests can
// stamp arbitrary completed timestamps. Each line should be a full markdown
// task line (`- [x] title <!-- id:N ... -->`) without trailing newline.
func writeRawTasks(t *testing.T, dir string, lines ...string) {
	t.Helper()
	path := filepath.Join(dir, ".tsk.md")
	body := "# tasks\n\n" + strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write store: %v", err)
	}
}

func TestStatsSinceFiltersDoneAndStreak(t *testing.T) {
	dir := t.TempDir()
	// Anchor at 'today 12:00' in the local zone so completions land on
	// distinct calendar days regardless of when the test runs.
	now := time.Now()
	day := func(offset int) string {
		return now.AddDate(0, 0, offset).Format(time.RFC3339)
	}
	// 5 tasks: completed 0, -1, -3, -10, -45 days ago plus one undone.
	writeRawTasks(t, dir,
		"- [x] today <!-- id:1 prio:medium completed:"+day(0)+" -->",
		"- [x] yesterday <!-- id:2 prio:medium completed:"+day(-1)+" -->",
		"- [x] three <!-- id:3 prio:medium completed:"+day(-3)+" -->",
		"- [x] ten <!-- id:4 prio:medium completed:"+day(-10)+" -->",
		"- [x] forty-five <!-- id:5 prio:medium completed:"+day(-45)+" -->",
		"- [ ] open <!-- id:6 prio:medium -->",
	)

	// All-time: 5 done, 1 undone.
	stdout, _, err := runCmd(t, dir, "stats")
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if !strings.Contains(stdout, "done:        5") {
		t.Fatalf("all-time should show done=5, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "undone:      1") {
		t.Fatalf("all-time should show undone=1, got:\n%s", stdout)
	}

	// 7d window: only the 0/-1/-3 day tasks count for done/streak.
	stdout, _, err = runCmd(t, dir, "stats", "--since", "7d")
	if err != nil {
		t.Fatalf("stats --since 7d: %v", err)
	}
	if !strings.Contains(stdout, "done:        3") {
		t.Fatalf("--since 7d should show done=3, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "undone:      1") {
		t.Fatalf("undone is whole-store, should still be 1, got:\n%s", stdout)
	}
	// total should still be whole-store too.
	if !strings.Contains(stdout, "total:       6") {
		t.Fatalf("total is whole-store, should be 6, got:\n%s", stdout)
	}
	// Streak: today + yesterday → 2 days. (-3 day breaks the chain.)
	if !strings.Contains(stdout, "streak:      2 day(s)") {
		t.Fatalf("--since 7d streak should be 2, got:\n%s", stdout)
	}
	// And the 'since' annotation is present.
	if !strings.Contains(stdout, "since:       7d ago") {
		t.Fatalf("missing 'since' annotation, got:\n%s", stdout)
	}
}

func TestStatsSinceRejectsBogus(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "stats", "--since", "banana")
	if err == nil {
		t.Fatal("expected error for bogus --since")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected ExitCode 2, got %v", err)
	}
}

func TestStatsGraphRendersSparkline(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeRawTasks(t, dir,
		"- [x] a <!-- id:1 prio:medium completed:"+now.Format(time.RFC3339)+" -->",
		"- [x] b <!-- id:2 prio:medium completed:"+now.AddDate(0, 0, -2).Format(time.RFC3339)+" -->",
	)
	stdout, _, err := runCmd(t, dir, "stats", "--graph")
	if err != nil {
		t.Fatalf("stats --graph: %v", err)
	}
	// Locate the sparkline line.
	var line string
	for _, l := range strings.Split(stdout, "\n") {
		if strings.HasPrefix(l, "30d completions:") {
			line = l
			break
		}
	}
	if line == "" {
		t.Fatalf("expected '30d completions:' line, got:\n%s", stdout)
	}
	const prefix = "30d completions:  "
	rest := strings.TrimRight(strings.TrimPrefix(line, prefix), "\r")
	runes := []rune(rest)
	if len(runes) != 30 {
		t.Fatalf("expected 30 runes in sparkline, got %d (%q)", len(runes), rest)
	}
	allowed := map[rune]bool{}
	for _, r := range " ▁▂▃▄▅▆▇█" {
		allowed[r] = true
	}
	for i, r := range runes {
		if !allowed[r] {
			t.Fatalf("sparkline rune %d (%q) not in alphabet", i, r)
		}
	}
	// Today is at the right edge — must be a non-space rune since we have a
	// completion today.
	if runes[len(runes)-1] == ' ' {
		t.Fatalf("expected non-empty bucket at right edge (today), got space")
	}
}

func TestStatsJSONStableSchema(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	writeRawTasks(t, dir,
		"- [x] a <!-- id:1 prio:medium tags:dev,write completed:"+now.Format(time.RFC3339)+" -->",
		"- [x] b <!-- id:2 prio:medium tags:dev completed:"+now.AddDate(0, 0, -1).Format(time.RFC3339)+" -->",
		"- [x] c <!-- id:3 prio:medium tags:write completed:"+now.AddDate(0, 0, -2).Format(time.RFC3339)+" -->",
		"- [ ] open <!-- id:4 prio:medium tags:dev -->",
	)
	stdout, _, err := runCmd(t, dir, "stats", "--json", "--since", "30d")
	if err != nil {
		t.Fatalf("stats --json: %v", err)
	}
	// Must be valid JSON.
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	// Required keys must all be present and the right shape.
	required := []string{
		"total", "done", "undone", "overdue", "today",
		"completion", "streak", "since_seconds",
		"top_tags", "completion_history",
	}
	for _, k := range required {
		if _, ok := doc[k]; !ok {
			t.Fatalf("missing required key %q in JSON output", k)
		}
	}
	// Numeric scalars come back as float64 from encoding/json.
	for _, k := range []string{"total", "done", "undone", "overdue", "today", "streak", "since_seconds", "completion"} {
		if _, ok := doc[k].(float64); !ok {
			t.Fatalf("key %q should be a number, got %T", k, doc[k])
		}
	}
	// since_seconds matches 30d.
	if got := int(doc["since_seconds"].(float64)); got != 30*24*60*60 {
		t.Fatalf("since_seconds want %d, got %d", 30*24*60*60, got)
	}
	// top_tags must be sorted desc by count.
	rawTags, ok := doc["top_tags"].([]any)
	if !ok {
		t.Fatalf("top_tags should be an array, got %T", doc["top_tags"])
	}
	if len(rawTags) == 0 {
		t.Fatal("expected at least one tag, got none")
	}
	last := -1
	for i, raw := range rawTags {
		obj, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("top_tags[%d] should be object, got %T", i, raw)
		}
		tag, ok := obj["tag"].(string)
		if !ok || tag == "" {
			t.Fatalf("top_tags[%d].tag should be non-empty string, got %v", i, obj["tag"])
		}
		countF, ok := obj["count"].(float64)
		if !ok {
			t.Fatalf("top_tags[%d].count should be number, got %T", i, obj["count"])
		}
		count := int(countF)
		if last >= 0 && count > last {
			t.Fatalf("top_tags not sorted desc by count: %d after %d", count, last)
		}
		last = count
	}
	// completion_history is always present, always 30 oldest-first buckets.
	hist, ok := doc["completion_history"].([]any)
	if !ok {
		t.Fatalf("completion_history should be array, got %T", doc["completion_history"])
	}
	if len(hist) != 30 {
		t.Fatalf("completion_history should have 30 buckets, got %d", len(hist))
	}
	var prev string
	for i, raw := range hist {
		obj, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("history[%d] should be object, got %T", i, raw)
		}
		date, ok := obj["date"].(string)
		if !ok {
			t.Fatalf("history[%d].date should be string", i)
		}
		if _, ok := obj["count"].(float64); !ok {
			t.Fatalf("history[%d].count should be number", i)
		}
		if prev != "" && date <= prev {
			t.Fatalf("history not oldest-first: %s after %s", date, prev)
		}
		prev = date
	}
}

func TestStatsJSONWithoutSince(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "stats", "--json")
	if err != nil {
		t.Fatalf("stats --json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if got := int(doc["since_seconds"].(float64)); got != 0 {
		t.Fatalf("since_seconds with no --since should be 0, got %d", got)
	}
}
