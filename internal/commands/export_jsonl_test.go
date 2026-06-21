package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestExportJSONLOneTaskPerLine(t *testing.T) {
	dir := t.TempDir()
	mustAdd(t, dir, "alpha", "-p", "high")
	mustAdd(t, dir, "beta", "-p", "low")
	mustAdd(t, dir, "gamma")
	stdout, _, err := runCmd(t, dir, "export", "--jsonl")
	if err != nil {
		t.Fatalf("export --jsonl: %v", err)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d:\n%s", len(lines), stdout)
	}
	for i, line := range lines {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Fatalf("line %d not valid JSON: %v\n%s", i, err, line)
		}
		// Critical contract: no embedded newlines in any line (would
		// break the streaming property).
		if strings.Contains(line, "\n") {
			t.Fatalf("line %d contains newline:\n%s", i, line)
		}
	}
}

func TestExportJSONLSchemaMatchesJSON(t *testing.T) {
	dir := t.TempDir()
	mustAdd(t, dir, "ship release", "-p", "high", "-t", "dev")
	stdout, _, err := runCmd(t, dir, "export", "--jsonl")
	if err != nil {
		t.Fatalf("export --jsonl: %v", err)
	}
	line := strings.TrimSpace(stdout)
	var rec map[string]any
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatalf("invalid JSONL: %v\n%s", err, line)
	}
	// Same field shape as a single element from --json array.
	jsonOut, _, err := runCmd(t, dir, "export", "--json")
	if err != nil {
		t.Fatalf("export --json: %v", err)
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(jsonOut), &arr); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, jsonOut)
	}
	if len(arr) != 1 {
		t.Fatalf("expected 1 element in --json, got %d", len(arr))
	}
	// All fields in --json should be present in --jsonl (schemas match).
	for k := range arr[0] {
		if _, ok := rec[k]; !ok {
			t.Fatalf("--jsonl missing field %q present in --json: %v", k, rec)
		}
	}
}

func TestExportJSONLEmptyIsBlank(t *testing.T) {
	dir := t.TempDir()
	// Init a file with no tasks (mustAdd would add one).
	if _, _, err := runCmd(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "export", "--jsonl")
	if err != nil {
		t.Fatalf("export --jsonl: %v", err)
	}
	if stdout != "" {
		t.Fatalf("expected empty stdout for 0 tasks, got %q", stdout)
	}
}

func TestExportFormatJSONLAlias(t *testing.T) {
	dir := t.TempDir()
	mustAdd(t, dir, "x")
	// --format=jsonl should work identically to --jsonl.
	a, _, err := runCmd(t, dir, "export", "--format", "jsonl")
	if err != nil {
		t.Fatalf("export --format jsonl: %v", err)
	}
	b, _, err := runCmd(t, dir, "export", "--jsonl")
	if err != nil {
		t.Fatalf("export --jsonl: %v", err)
	}
	if a != b {
		t.Fatalf("--format jsonl and --jsonl must produce identical output\na:%s\nb:%s", a, b)
	}
}

func TestExportFormatNDJSONAlias(t *testing.T) {
	dir := t.TempDir()
	mustAdd(t, dir, "x")
	stdout, _, err := runCmd(t, dir, "export", "--format", "ndjson")
	if err != nil {
		t.Fatalf("export --format ndjson: %v", err)
	}
	if !strings.HasSuffix(stdout, "\n") {
		t.Fatalf("expected newline-terminated, got %q", stdout)
	}
	if strings.Contains(strings.TrimRight(stdout, "\n"), "\n") {
		t.Fatalf("ndjson must be one line per task, got %q", stdout)
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &rec); err != nil {
		t.Fatalf("invalid: %v\n%s", err, stdout)
	}
}

func TestExportJSONLAndJSONMutuallyExclusive(t *testing.T) {
	dir := t.TempDir()
	mustAdd(t, dir, "x")
	_, _, err := runCmd(t, dir, "export", "--json", "--jsonl")
	if err == nil {
		t.Fatal("expected error when both --json and --jsonl specified")
	}
}

func TestExportUnknownFormatRejected(t *testing.T) {
	dir := t.TempDir()
	mustAdd(t, dir, "x")
	_, _, err := runCmd(t, dir, "export", "--format", "yaml")
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
}
