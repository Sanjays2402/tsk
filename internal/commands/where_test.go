package commands

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestWherePrintsExplicitFile(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "where")
	if err != nil {
		t.Fatalf("where: %v", err)
	}
	want := filepath.Join(dir, ".tsk.md")
	if !strings.Contains(stdout, "path:    "+want) {
		t.Fatalf("expected path %q, got:\n%s", want, stdout)
	}
	if !strings.Contains(stdout, "method:  flag") {
		t.Fatalf("expected method:flag (test always passes --file), got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "exists:  yes") {
		t.Fatalf("expected exists:yes after init, got:\n%s", stdout)
	}
}

func TestWhereMissingFileReportsNo(t *testing.T) {
	dir := t.TempDir()
	stdout, _, err := runCmd(t, dir, "where")
	if err != nil {
		t.Fatalf("where: %v", err)
	}
	if !strings.Contains(stdout, "exists:  no") {
		t.Fatalf("expected exists:no for fresh tmp dir, got:\n%s", stdout)
	}
	if !strings.Contains(stdout, "tsk init") {
		t.Fatalf("expected init hint, got:\n%s", stdout)
	}
}

func TestWhereJSONShape(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "where", "--json")
	if err != nil {
		t.Fatalf("where --json: %v", err)
	}
	var info map[string]any
	if err := json.Unmarshal([]byte(stdout), &info); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	for _, key := range []string{"path", "method", "exists", "timezone", "tz_source"} {
		if _, ok := info[key]; !ok {
			t.Fatalf("missing key %q in JSON: %s", key, stdout)
		}
	}
	if !info["exists"].(bool) {
		t.Fatalf("exists should be true after init, got %v", info["exists"])
	}
	if info["method"].(string) != "flag" {
		t.Fatalf("expected method=flag, got %v", info["method"])
	}
}

func TestWhereTZSourceReflectsEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TSK_TZ", "Europe/London")
	ResetTZForTest()
	defer ResetTZForTest()
	stdout, _, err := runCmd(t, dir, "where")
	if err != nil {
		t.Fatalf("where: %v", err)
	}
	if !strings.Contains(stdout, "tz:      Europe/London (TSK_TZ)") {
		t.Fatalf("expected tz line citing TSK_TZ, got:\n%s", stdout)
	}
}

func TestWhereTZSourceSystemWhenUnset(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TSK_TZ", "")
	t.Setenv("TZ", "")
	ResetTZForTest()
	defer ResetTZForTest()
	stdout, _, err := runCmd(t, dir, "where", "--json")
	if err != nil {
		t.Fatalf("where: %v", err)
	}
	var info map[string]any
	if err := json.Unmarshal([]byte(stdout), &info); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if info["tz_source"].(string) != "system" {
		t.Fatalf("expected tz_source=system, got %v", info["tz_source"])
	}
}
