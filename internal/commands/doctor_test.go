package commands

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDoctorCleanStore verifies a healthy .tsk.md gets "all checks passed".
func TestDoctorCleanStore(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "ship feature"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "doctor")
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	if !strings.Contains(stdout, "all checks passed") {
		t.Errorf("expected all checks passed, got:\n%s", stdout)
	}
}

// TestDoctorJSONOutput verifies --json emits a parseable DoctorReport.
func TestDoctorJSONOutput(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := runCmd(t, dir, "add", "ship feature"); err != nil {
		t.Fatalf("add: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "doctor", "--json")
	if err != nil {
		t.Fatalf("doctor --json: %v", err)
	}
	var report DoctorReport
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("invalid JSON: %v\noutput:\n%s", err, stdout)
	}
	if report.TaskCnt != 1 {
		t.Errorf("expected task_count=1, got %d", report.TaskCnt)
	}
	if len(report.Errors) != 0 {
		t.Errorf("expected no errors on clean store, got: %+v", report.Errors)
	}
}

// TestDoctorDuplicateIDs verifies duplicate IDs are flagged as errors.
func TestDoctorDuplicateIDs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".tsk.md")
	content := "# Tasks\n\n" +
		"- [ ] first <!-- id:1 created:2026-05-11T00:00:00Z -->\n" +
		"- [ ] second <!-- id:1 created:2026-05-11T00:00:00Z -->\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "doctor", "--json")
	if err == nil {
		t.Fatal("expected non-zero exit on duplicate IDs")
	}
	var ec ExitCoder
	if !asExitCoder(err, &ec) || ec.ExitCode() != 1 {
		t.Errorf("expected exit code 1, got %v", err)
	}
	var report DoctorReport
	if jerr := json.Unmarshal([]byte(stdout), &report); jerr != nil {
		t.Fatalf("invalid JSON: %v\noutput:\n%s", jerr, stdout)
	}
	if len(report.Errors) == 0 {
		t.Errorf("expected duplicate ID error, got none")
	}
	found := false
	for _, e := range report.Errors {
		if e.Check == "unique_ids" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected unique_ids check failure, got: %+v", report.Errors)
	}
}

// TestDoctorMissingFile verifies a missing file is reported as an error.
func TestDoctorMissingFile(t *testing.T) {
	dir := t.TempDir()
	stdout, _, err := runCmd(t, dir, "doctor", "--json")
	if err == nil {
		t.Fatal("expected non-zero exit when no .tsk.md exists")
	}
	var report DoctorReport
	if jerr := json.Unmarshal([]byte(stdout), &report); jerr != nil {
		t.Fatalf("invalid JSON: %v\noutput:\n%s", jerr, stdout)
	}
	if len(report.Errors) == 0 {
		t.Errorf("expected missing-file error, got none")
	}
}

// TestDoctorSilentExitNoErrPrefix verifies the error returned for the
// non-zero exit path is a SilentExitCoder, so main.go skips the "error:"
// prefix. We can't intercept main.go in tests, but we CAN verify the
// returned error is the right type.
func TestDoctorSilentExitNoErrPrefix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".tsk.md")
	content := "# Tasks\n\n" +
		"- [ ] first <!-- id:1 created:2026-05-11T00:00:00Z -->\n" +
		"- [ ] second <!-- id:1 created:2026-05-11T00:00:00Z -->\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, _, err := runCmd(t, dir, "doctor")
	if err == nil {
		t.Fatal("expected error")
	}
	if _, ok := err.(SilentExitCoder); !ok {
		t.Errorf("expected SilentExitCoder, got %T: %v", err, err)
	}
	if err.Error() != "" {
		t.Errorf("expected empty Error() string for silent exit, got %q", err.Error())
	}
}
