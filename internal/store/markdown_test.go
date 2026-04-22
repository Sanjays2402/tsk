package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
)

func TestParseRoundTrip(t *testing.T) {
	input := `# My Tasks

- [ ] Buy milk <!-- id:1 prio:medium due:2026-04-25 tags:errand,home created:2026-04-21T19:20:00-07:00 -->
      pick organic
      and oat too
- [x] Pay rent <!-- id:2 prio:high completed:2026-04-20T09:00:00-07:00 -->
- [ ] No meta task
`
	s := &Store{}
	if err := s.parse([]byte(input)); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(s.Tasks) != 3 {
		t.Fatalf("got %d tasks, want 3", len(s.Tasks))
	}
	if s.Tasks[0].Title != "Buy milk" || s.Tasks[0].ID != 1 {
		t.Errorf("tasks[0] bad: %+v", s.Tasks[0])
	}
	if s.Tasks[0].Priority != model.PriorityMedium {
		t.Errorf("priority: %v", s.Tasks[0].Priority)
	}
	if len(s.Tasks[0].Tags) != 2 {
		t.Errorf("tags: %v", s.Tasks[0].Tags)
	}
	if !strings.Contains(s.Tasks[0].Notes, "pick organic") || !strings.Contains(s.Tasks[0].Notes, "oat too") {
		t.Errorf("notes: %q", s.Tasks[0].Notes)
	}
	if !s.Tasks[1].Done {
		t.Errorf("task[1] should be done")
	}
	if s.Tasks[2].ID != 0 {
		t.Errorf("task[2] should have no id: %d", s.Tasks[2].ID)
	}

	// Round trip
	s.assignIDs()
	out := s.render()
	s2 := &Store{}
	if err := s2.parse(out); err != nil {
		t.Fatalf("reparse: %v", err)
	}
	if len(s2.Tasks) != 3 {
		t.Fatalf("round-trip tasks %d", len(s2.Tasks))
	}
	if s2.Tasks[0].Notes != s.Tasks[0].Notes {
		t.Errorf("notes lost: %q vs %q", s2.Tasks[0].Notes, s.Tasks[0].Notes)
	}
}

func TestParseHandEdits(t *testing.T) {
	cases := []string{
		"- [ ] plain\n",
		"- [X] big X marker\n",
		"- [ ] weird <!-- prio:bogus due:not-a-date extra:stuff -->\n",
		"- [ ] tabs in notes\n      line one\n      \n      line two\n",
		"- [ ] tags empty <!-- tags: -->\n",
	}
	for _, tc := range cases {
		s := &Store{}
		if err := s.parse([]byte(tc)); err != nil {
			t.Errorf("parse %q: %v", tc, err)
		}
		if len(s.Tasks) == 0 {
			t.Errorf("no task parsed from %q", tc)
		}
	}
}

func TestAssignIDs(t *testing.T) {
	s := &Store{Tasks: []model.Task{
		{Title: "a", ID: 2},
		{Title: "b"},
		{Title: "c", ID: 5},
		{Title: "d"},
	}}
	s.assignIDs()
	seen := map[int]bool{}
	for _, task := range s.Tasks {
		if task.ID == 0 {
			t.Errorf("unset ID: %+v", task)
		}
		if seen[task.ID] {
			t.Errorf("dup ID: %d", task.ID)
		}
		seen[task.ID] = true
	}
}

func TestAddAndSave(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".tsk.md")
	s, err := Load(p)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	due := time.Date(2026, 4, 25, 0, 0, 0, 0, time.UTC)
	s.Add(model.Task{Title: "first", Priority: model.PriorityHigh, Due: &due, Tags: []string{"x"}})
	s.Add(model.Task{Title: "second"})
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "- [ ] first") {
		t.Errorf("missing first: %s", data)
	}
	if !strings.Contains(string(data), "id:1") || !strings.Contains(string(data), "id:2") {
		t.Errorf("ids missing: %s", data)
	}

	s2, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(s2.Tasks) != 2 {
		t.Fatalf("expected 2 on reload")
	}
}

func TestResolveWalksUp(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, ".tsk.md")
	if err := os.WriteFile(p, []byte("# tsk\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, ok := Resolve(nested)
	if !ok {
		t.Fatalf("expected to find %s got %s", p, got)
	}
	if got != p {
		t.Errorf("got %q want %q", got, p)
	}
}

func TestResolveFallback(t *testing.T) {
	root := t.TempDir()
	_, ok := Resolve(root)
	if ok {
		t.Errorf("expected not found in empty dir")
	}
}

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.md")
	if err := AtomicWriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil || string(b) != "hello" {
		t.Fatalf("read got %q err=%v", b, err)
	}
	// no tempfile leftovers
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tsk-tmp-") {
			t.Errorf("leftover temp: %s", e.Name())
		}
	}
}

func TestRemoveAndSetDone(t *testing.T) {
	s := &Store{Tasks: []model.Task{{ID: 1, Title: "a"}, {ID: 2, Title: "b"}}}
	if !s.SetDone(1, true) {
		t.Fatal("SetDone failed")
	}
	if !s.Tasks[0].Done || s.Tasks[0].Completed == nil {
		t.Error("not marked done")
	}
	if !s.Remove(2) {
		t.Error("remove failed")
	}
	if len(s.Tasks) != 1 {
		t.Error("remove not applied")
	}
	if s.Remove(999) {
		t.Error("remove nonexistent returned true")
	}
}
