package util

import "testing"

func TestFuzzyEmpty(t *testing.T) {
	items := []string{"buy milk", "pay rent", "walk dog"}
	out := Fuzzy(items, "")
	if len(out) != 3 {
		t.Errorf("empty query should return all: got %d", len(out))
	}
}

func TestFuzzyRanks(t *testing.T) {
	items := []string{"buy milk", "pay rent", "walk dog"}
	out := Fuzzy(items, "py")
	if len(out) == 0 || out[0].Index != 1 {
		t.Errorf("expected 'pay rent' first, got %v", out)
	}
}
