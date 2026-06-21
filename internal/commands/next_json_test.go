package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestNextJSONPicksHighestPriority: the JSON path returns the same
// task the human-readable path would, with a stable schema. This is
// the script-friendly composability test — `tsk next --json | jq -r
// '.id'` is the canonical pipeline shape we want to enable.
func TestNextJSONPicksHighestPriority(t *testing.T) {
	dir := t.TempDir()
	for _, p := range []string{"low", "high", "medium"} {
		if _, _, err := runCmd(t, dir, "add", "task "+p, "-p", p); err != nil {
			t.Fatalf("add %s: %v", p, err)
		}
	}
	stdout, _, err := runCmd(t, dir, "next", "--json")
	if err != nil {
		t.Fatalf("next --json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if got, _ := doc["title"].(string); got != "task high" {
		t.Fatalf("expected title 'task high', got %v", doc["title"])
	}
	if got, _ := doc["priority"].(string); got != "high" {
		t.Fatalf("expected priority=high, got %v", doc["priority"])
	}
	// Empty store sentinel must NOT be set on a successful pick.
	if v, _ := doc["empty"].(bool); v {
		t.Fatalf("empty should be false on successful pick, got %v", doc["empty"])
	}
}

// TestNextJSONEmptyStore: the all-caught-up sentinel must encode as
// {"empty": true} so scripts can branch on a real boolean rather
// than string-matching "all caught up".
func TestNextJSONEmptyStore(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "x"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "done", "1"); err != nil {
		t.Fatalf("done: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "next", "--json")
	if err != nil {
		t.Fatalf("next --json empty: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if v, _ := doc["empty"].(bool); !v {
		t.Fatalf("expected empty=true on caught-up store, got %v", doc)
	}
	if _, ok := doc["id"]; ok {
		t.Fatalf("id must not appear on empty result, got %v", doc)
	}
}

// TestNextJSONRespectsBlockedFallback: with --respect-deps, when
// every open task is blocked, the fallback must surface the blocked
// best with blocked=true and blocked_by populated — matching the
// "(blocked by ...)" annotation that human output appends.
func TestNextJSONRespectsBlockedFallback(t *testing.T) {
	dir := t.TempDir()
	for _, title := range []string{"alpha", "beta", "gamma"} {
		if _, _, err := runCmd(t, dir, "add", title); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	if _, _, err := runCmd(t, dir, "depend", "1", "--on", "2"); err != nil {
		t.Fatalf("depend 1->2: %v", err)
	}
	if _, _, err := runCmd(t, dir, "depend", "2", "--on", "3"); err != nil {
		t.Fatalf("depend 2->3: %v", err)
	}
	path := filepath.Join(dir, ".tsk.md")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(string(body), "\n")
	for i, l := range lines {
		if strings.Contains(l, "id:3 ") && !strings.Contains(l, "depends:") {
			lines[i] = strings.Replace(l, "-->", "depends:1 -->", 1)
		}
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "next", "--respect-deps", "--json")
	if err != nil {
		t.Fatalf("next --respect-deps --json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if v, _ := doc["blocked"].(bool); !v {
		t.Fatalf("expected blocked=true in fallback, got %v", doc)
	}
	by, ok := doc["blocked_by"].([]any)
	if !ok || len(by) == 0 {
		t.Fatalf("expected non-empty blocked_by, got %v (%T)", doc["blocked_by"], doc["blocked_by"])
	}
}

// TestNextJSONSchemaFields: the schema includes id/title/priority/
// pinned plus optional due/tags/blocked/blocked_by. Pinned task with
// a due date and tags should round-trip every key.
func TestNextJSONSchemaFields(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "pin me", "-p", "urgent", "-t", "alpha", "-t", "beta", "-d", "2099-01-15"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if _, _, err := runCmd(t, dir, "pin", "1"); err != nil {
		t.Fatalf("pin: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "next", "--json")
	if err != nil {
		t.Fatalf("next --json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if got := int(doc["id"].(float64)); got != 1 {
		t.Fatalf("expected id=1, got %v", doc["id"])
	}
	if got, _ := doc["priority"].(string); got != "urgent" {
		t.Fatalf("expected priority=urgent, got %v", doc["priority"])
	}
	if got, _ := doc["due"].(string); got != "2099-01-15" {
		t.Fatalf("expected due=2099-01-15, got %v", doc["due"])
	}
	if v, _ := doc["pinned"].(bool); !v {
		t.Fatalf("expected pinned=true, got %v", doc["pinned"])
	}
	tags, ok := doc["tags"].([]any)
	if !ok || len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %v", doc["tags"])
	}
}

// TestNextJSONFallbackPathConsistent: the fallback (no fallback,
// fully empty case) returns just {"empty":true}, while the not-empty
// path NEVER emits empty=true. Documents the omitempty contract.
func TestNextJSONFallbackPathConsistent(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "only"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "next", "--json")
	if err != nil {
		t.Fatalf("next --json: %v", err)
	}
	if strings.Contains(stdout, `"empty"`) {
		t.Fatalf("empty key must not appear when a task is returned, got:\n%s", stdout)
	}
}
