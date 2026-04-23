package commands

import (
	"testing"
	"time"
)

func TestResolveTZRespectsTSKTZ(t *testing.T) {
	ResetTZForTest()
	t.Setenv("TSK_TZ", "America/New_York")
	t.Setenv("TZ", "Asia/Tokyo") // must lose to TSK_TZ
	loc := ResolveTZ()
	if loc.String() != "America/New_York" {
		t.Fatalf("expected TSK_TZ to win, got %q", loc.String())
	}
	t.Cleanup(ResetTZForTest)
}

func TestResolveTZFallsBackToTZ(t *testing.T) {
	ResetTZForTest()
	t.Setenv("TSK_TZ", "")
	t.Setenv("TZ", "America/New_York")
	loc := ResolveTZ()
	if loc.String() != "America/New_York" {
		t.Fatalf("expected TZ fallback, got %q", loc.String())
	}
	t.Cleanup(ResetTZForTest)
}

func TestResolveTZIgnoresInvalidZone(t *testing.T) {
	ResetTZForTest()
	t.Setenv("TSK_TZ", "Invalid/Not_A_Zone")
	t.Setenv("TZ", "")
	loc := ResolveTZ()
	// Must silently fall through to the next strategy rather than crash.
	if loc == nil {
		t.Fatal("ResolveTZ returned nil for invalid TSK_TZ")
	}
	t.Cleanup(ResetTZForTest)
}

func TestResolveTZUsesLocalWhenEnvEmpty(t *testing.T) {
	ResetTZForTest()
	t.Setenv("TSK_TZ", "")
	t.Setenv("TZ", "")
	loc := ResolveTZ()
	if loc == nil {
		t.Fatal("ResolveTZ returned nil")
	}
	// On most dev hosts time.Local is not UTC; just assert we got something
	// non-nil without crashing. Specific zone name depends on the host.
	_ = loc
	t.Cleanup(ResetTZForTest)
}

func TestResolveTZCachesFirstCall(t *testing.T) {
	ResetTZForTest()
	t.Setenv("TSK_TZ", "America/New_York")
	first := ResolveTZ()
	// Change env; second call must return the cached value.
	t.Setenv("TSK_TZ", "Asia/Tokyo")
	second := ResolveTZ()
	if first != second {
		t.Fatalf("expected cached location, got %v then %v", first, second)
	}
	t.Cleanup(ResetTZForTest)
}

// Sanity: the resolved location actually works with time.Now().
func TestResolveTZProducesUsableLocation(t *testing.T) {
	ResetTZForTest()
	t.Setenv("TSK_TZ", "America/New_York")
	loc := ResolveTZ()
	n := time.Now().In(loc)
	if n.Location() != loc {
		t.Fatalf("time.Now().In did not adopt the location: got %v", n.Location())
	}
	t.Cleanup(ResetTZForTest)
}
