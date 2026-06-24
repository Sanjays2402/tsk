package store

import (
	"testing"

	"github.com/Sanjays2402/tsk/internal/model"
)

// orderIDs returns the current task ids in slice (file) order.
func orderIDs(s *Store) []int {
	out := make([]int, 0, len(s.Tasks))
	for _, t := range s.Tasks {
		out = append(out, t.ID)
	}
	return out
}

// seedOrdered builds a store with tasks carrying ids 1..n in order.
func seedOrdered(n int) *Store {
	s := &Store{}
	for i := 1; i <= n; i++ {
		s.Tasks = append(s.Tasks, model.Task{ID: i, Title: "t", Priority: model.PriorityMedium})
	}
	return s
}

func eqInts(a, b []int) bool {
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

func TestMoveBeforeEarlierTask(t *testing.T) {
	s := seedOrdered(5) // [1 2 3 4 5]
	// Drag #4 to sit before #2 -> [1 4 2 3 5]
	if !s.Move(4, 2) {
		t.Fatal("Move(4,2) returned false")
	}
	if got := orderIDs(s); !eqInts(got, []int{1, 4, 2, 3, 5}) {
		t.Fatalf("order = %v, want [1 4 2 3 5]", got)
	}
}

func TestMoveBeforeLaterTask(t *testing.T) {
	s := seedOrdered(5) // [1 2 3 4 5]
	// Drag #2 to sit before #5 -> [1 3 4 2 5]
	if !s.Move(2, 5) {
		t.Fatal("Move(2,5) returned false")
	}
	if got := orderIDs(s); !eqInts(got, []int{1, 3, 4, 2, 5}) {
		t.Fatalf("order = %v, want [1 3 4 2 5]", got)
	}
}

func TestMoveToEndWithZero(t *testing.T) {
	s := seedOrdered(4) // [1 2 3 4]
	// before == 0 means "drop at the very end".
	if !s.Move(2, 0) {
		t.Fatal("Move(2,0) returned false")
	}
	if got := orderIDs(s); !eqInts(got, []int{1, 3, 4, 2}) {
		t.Fatalf("order = %v, want [1 3 4 2]", got)
	}
}

func TestMoveToFront(t *testing.T) {
	s := seedOrdered(4) // [1 2 3 4]
	// Drag #4 before #1 -> [4 1 2 3]
	if !s.Move(4, 1) {
		t.Fatal("Move(4,1) returned false")
	}
	if got := orderIDs(s); !eqInts(got, []int{4, 1, 2, 3}) {
		t.Fatalf("order = %v, want [4 1 2 3]", got)
	}
}

func TestMoveBeforeSelfIsNoop(t *testing.T) {
	s := seedOrdered(3) // [1 2 3]
	if !s.Move(2, 2) {
		t.Fatal("Move(2,2) should be a successful no-op")
	}
	if got := orderIDs(s); !eqInts(got, []int{1, 2, 3}) {
		t.Fatalf("order = %v, want [1 2 3] (unchanged)", got)
	}
}

func TestMoveUnknownMovedReturnsFalse(t *testing.T) {
	s := seedOrdered(3)
	if s.Move(99, 1) {
		t.Fatal("Move with unknown moved id should return false")
	}
	if got := orderIDs(s); !eqInts(got, []int{1, 2, 3}) {
		t.Fatalf("order changed on failed move: %v", got)
	}
}

func TestMoveUnknownBeforeReturnsFalse(t *testing.T) {
	s := seedOrdered(3)
	if s.Move(1, 99) {
		t.Fatal("Move with unknown before id should return false")
	}
	if got := orderIDs(s); !eqInts(got, []int{1, 2, 3}) {
		t.Fatalf("order changed on failed move: %v", got)
	}
}

// TestMovePersistsThroughSave verifies the new order survives a render/parse
// round-trip — i.e. the .tsk.md is actually rewritten in the dragged order.
func TestMovePersistsThroughSave(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/.tsk.md"
	s := &Store{Path: path}
	s.Add(model.Task{Title: "alpha", Priority: model.PriorityMedium})
	s.Add(model.Task{Title: "bravo", Priority: model.PriorityMedium})
	s.Add(model.Task{Title: "charlie", Priority: model.PriorityMedium})
	if err := s.Save(); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Drag charlie (#3) to the front (before #1).
	if !s.Move(3, 1) {
		t.Fatal("Move(3,1) returned false")
	}
	if err := s.Save(); err != nil {
		t.Fatalf("save 2: %v", err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	gotTitles := make([]string, 0, len(reloaded.Tasks))
	for _, tk := range reloaded.Tasks {
		gotTitles = append(gotTitles, tk.Title)
	}
	want := []string{"charlie", "alpha", "bravo"}
	if len(gotTitles) != 3 || gotTitles[0] != want[0] || gotTitles[1] != want[1] || gotTitles[2] != want[2] {
		t.Fatalf("reordered titles = %v, want %v", gotTitles, want)
	}
}
