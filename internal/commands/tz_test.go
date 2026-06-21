package commands

import (
	"testing"
)

// TestResolveTZPrefersTSKTZ verifies $TSK_TZ wins over $TZ and the system
// default. Uses ResetTZForTest to clear the sync.Once cache between cases.
func TestResolveTZPrefersTSKTZ(t *testing.T) {
	ResetTZForTest()
	t.Cleanup(ResetTZForTest)

	t.Setenv("TSK_TZ", "America/New_York")
	t.Setenv("TZ", "Europe/London")

	loc := ResolveTZ()
	if loc == nil {
		t.Fatal("ResolveTZ returned nil")
	}
	if loc.String() != "America/New_York" {
		t.Fatalf("TSK_TZ should win; want America/New_York, got %s", loc)
	}
}

// TestResolveTZFallsBackToTZ verifies $TZ is used when $TSK_TZ is unset.
func TestResolveTZFallsBackToTZ(t *testing.T) {
	ResetTZForTest()
	t.Cleanup(ResetTZForTest)

	t.Setenv("TSK_TZ", "")
	t.Setenv("TZ", "Asia/Tokyo")

	loc := ResolveTZ()
	if loc == nil {
		t.Fatal("ResolveTZ returned nil")
	}
	if loc.String() != "Asia/Tokyo" {
		t.Fatalf("TZ should be used; want Asia/Tokyo, got %s", loc)
	}
}

// TestResolveTZIgnoresInvalidZone verifies an unparseable $TSK_TZ is skipped in
// favor of the next candidate ($TZ) rather than erroring.
func TestResolveTZIgnoresInvalidZone(t *testing.T) {
	ResetTZForTest()
	t.Cleanup(ResetTZForTest)

	t.Setenv("TSK_TZ", "Not/ARealZone")
	t.Setenv("TZ", "America/Los_Angeles")

	loc := ResolveTZ()
	if loc == nil {
		t.Fatal("ResolveTZ returned nil")
	}
	if loc.String() != "America/Los_Angeles" {
		t.Fatalf("invalid TSK_TZ should fall through to TZ; want America/Los_Angeles, got %s", loc)
	}
}

// TestResolveTZCachesFirstResult verifies the sync.Once cache: once resolved,
// later env changes within the same process are ignored until ResetTZForTest.
func TestResolveTZCachesFirstResult(t *testing.T) {
	ResetTZForTest()
	t.Cleanup(ResetTZForTest)

	t.Setenv("TSK_TZ", "America/New_York")
	first := ResolveTZ()
	if first.String() != "America/New_York" {
		t.Fatalf("setup: want America/New_York, got %s", first)
	}

	// Change the env; without a reset the cached value must persist.
	t.Setenv("TSK_TZ", "Asia/Tokyo")
	second := ResolveTZ()
	if second.String() != "America/New_York" {
		t.Fatalf("cache should hold first result; want America/New_York, got %s", second)
	}
}

// TestPacificLocDelegatesToResolveTZ confirms the backward-compat alias returns
// the same location as ResolveTZ.
func TestPacificLocDelegatesToResolveTZ(t *testing.T) {
	ResetTZForTest()
	t.Cleanup(ResetTZForTest)

	t.Setenv("TSK_TZ", "Europe/Berlin")
	if PacificLoc().String() != ResolveTZ().String() {
		t.Fatalf("PacificLoc should delegate to ResolveTZ")
	}
}
