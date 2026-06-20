package commands

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestShowPlainOutputHasAllFields(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "thing to do",
		"-p", "high",
		"-t", "dev",
		"-t", "urgent-tag",
		"-n", "first note line\nsecond note line",
		"-d", "2099-12-31"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "show", "1")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	want := []string{
		"id:        1",
		"title:     thing to do",
		"status:    open",
		"priority:  high",
		"due:       2099-12-31",
		"tags:      #dev #urgent-tag",
		"notes:",
		"  first note line",
		"  second note line",
	}
	for _, w := range want {
		if !strings.Contains(stdout, w) {
			t.Fatalf("show output missing %q:\n%s", w, stdout)
		}
	}
}

func TestShowMissingFieldsAreDashed(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "bare"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "show", "1")
	if err != nil {
		t.Fatalf("show: %v", err)
	}
	for _, w := range []string{"due:       -", "tags:      -"} {
		if !strings.Contains(stdout, w) {
			t.Fatalf("missing dash placeholder %q:\n%s", w, stdout)
		}
	}
	if strings.Contains(stdout, "notes:") {
		t.Fatalf("notes section should be absent when empty:\n%s", stdout)
	}
}

func TestShowJSONIsValidAndScoped(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "a", "-p", "urgent"); err != nil {
		t.Fatalf("add a: %v", err)
	}
	if _, _, err := runCmd(t, dir, "add", "b"); err != nil {
		t.Fatalf("add b: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "show", "1", "--json")
	if err != nil {
		t.Fatalf("show --json: %v", err)
	}
	var task map[string]any
	if err := json.Unmarshal([]byte(stdout), &task); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	// JSON should be a single task object, not an array, and must be id 1.
	if got, _ := task["ID"].(float64); int(got) != 1 {
		t.Fatalf("expected ID 1, got %v", task["ID"])
	}
	if got, _ := task["Title"].(string); got != "a" {
		t.Fatalf("expected Title a, got %v", task["Title"])
	}
	if strings.Contains(stdout, `"Title": "b"`) {
		t.Fatalf("show --json 1 should not contain task 2:\n%s", stdout)
	}
}

func TestShowUnknownID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "show", "999")
	if err == nil {
		t.Fatal("expected error for unknown id")
	}
}

func TestShowAcceptsHashPrefix(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "show", "#1")
	if err != nil {
		t.Fatalf("show #1: %v", err)
	}
	if !strings.Contains(stdout, "id:        1") {
		t.Fatalf("expected to find task 1, got:\n%s", stdout)
	}
}

func TestShowRejectsBadID(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	_, _, err := runCmd(t, dir, "show", "abc")
	if err == nil {
		t.Fatal("expected error for non-numeric id")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 2 {
		t.Fatalf("expected exit code 2 for usage error, got %v", err)
	}
}
