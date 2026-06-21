package commands

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestEnvPlainHasAllSections(t *testing.T) {
	dir := t.TempDir()
	// Make sure NO_COLOR / TSK_TZ are set so we exercise the populated branches.
	t.Setenv("NO_COLOR", "1")
	t.Setenv("TSK_TZ", "America/Los_Angeles")
	t.Setenv("TSK_FORCE_TEST", "yes") // exercise TSK_* var collection
	ResetTZForTest()
	defer ResetTZForTest()
	stdout, _, err := runCmd(t, dir, "env")
	if err != nil {
		t.Fatalf("env: %v", err)
	}
	for _, want := range []string{
		"[files]", "[timezone]", "[editor]", "[color]", "[runtime]",
		"TSK_TZ:   America/Los_Angeles",
		"NO_COLOR:", "TSK_FORCE_TEST=yes",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected %q in plain env, got:\n%s", want, stdout)
		}
	}
}

func TestEnvJSONStableSchema(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TSK_TZ", "America/Los_Angeles")
	t.Setenv("EDITOR", "vim-test")
	t.Setenv("VISUAL", "")
	t.Setenv("NO_COLOR", "")
	ResetTZForTest()
	defer ResetTZForTest()
	stdout, _, err := runCmd(t, dir, "env", "--json")
	if err != nil {
		t.Fatalf("env --json: %v", err)
	}
	var info EnvInfo
	if err := json.Unmarshal([]byte(stdout), &info); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if info.Timezone.Resolved == "" || info.Timezone.Source != "TSK_TZ" {
		t.Fatalf("expected TSK_TZ source, got: %+v", info.Timezone)
	}
	if info.Editor.Source != "EDITOR" || info.Editor.Resolved != "vim-test" {
		t.Fatalf("expected EDITOR vim-test, got: %+v", info.Editor)
	}
	if info.Color.Disabled {
		t.Fatalf("color disabled with empty NO_COLOR")
	}
	if info.Runtime.GoVersion == "" {
		t.Fatalf("missing go version: %+v", info.Runtime)
	}
}

func TestEnvVISUALBeatsEDITOR(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("EDITOR", "should-lose")
	t.Setenv("VISUAL", "should-win")
	stdout, _, err := runCmd(t, dir, "env", "--json")
	if err != nil {
		t.Fatalf("env --json: %v", err)
	}
	var info EnvInfo
	if err := json.Unmarshal([]byte(stdout), &info); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if info.Editor.Resolved != "should-win" || info.Editor.Source != "VISUAL" {
		t.Fatalf("VISUAL should beat EDITOR, got: %+v", info.Editor)
	}
}

func TestEnvNoColorDisablesOnNonEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("NO_COLOR", "anything")
	stdout, _, err := runCmd(t, dir, "env", "--json")
	if err != nil {
		t.Fatalf("env --json: %v", err)
	}
	var info EnvInfo
	if err := json.Unmarshal([]byte(stdout), &info); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if !info.Color.Disabled {
		t.Fatalf("expected color disabled with NO_COLOR=anything: %+v", info.Color)
	}
}

func TestEnvNoColorEmptyDoesNotDisable(t *testing.T) {
	dir := t.TempDir()
	// Explicitly unset NO_COLOR so an outer env doesn't leak in.
	if err := os.Unsetenv("NO_COLOR"); err != nil {
		t.Fatalf("unsetenv: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "env", "--json")
	if err != nil {
		t.Fatalf("env --json: %v", err)
	}
	var info EnvInfo
	if err := json.Unmarshal([]byte(stdout), &info); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if info.Color.Disabled {
		t.Fatalf("color must NOT be disabled when NO_COLOR is unset: %+v", info.Color)
	}
	if info.Color.NOCOLOR != "(unset)" {
		t.Fatalf("expected '(unset)' marker, got %q", info.Color.NOCOLOR)
	}
}

func TestEnvFilesFlagWins(t *testing.T) {
	dir := t.TempDir()
	customFile := dir + "/custom-tsk.md"
	stdout, _, err := runCmd(t, dir, "--file", customFile, "env", "--json")
	if err != nil {
		t.Fatalf("env --json: %v", err)
	}
	var info EnvInfo
	if err := json.Unmarshal([]byte(stdout), &info); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if info.Files.Method != "flag" {
		t.Fatalf("expected method=flag, got %q", info.Files.Method)
	}
	if info.Files.Resolved != customFile {
		t.Fatalf("expected resolved=%q, got %q", customFile, info.Files.Resolved)
	}
	if info.Files.Exists {
		t.Fatalf("custom file shouldn't exist: %+v", info.Files)
	}
}

func TestEnvUnsetVarsMarked(t *testing.T) {
	dir := t.TempDir()
	if err := os.Unsetenv("TSK_TZ"); err != nil {
		t.Fatalf("unsetenv: %v", err)
	}
	if err := os.Unsetenv("EDITOR"); err != nil {
		t.Fatalf("unsetenv: %v", err)
	}
	if err := os.Unsetenv("VISUAL"); err != nil {
		t.Fatalf("unsetenv: %v", err)
	}
	stdout, _, err := runCmd(t, dir, "env", "--json")
	if err != nil {
		t.Fatalf("env --json: %v", err)
	}
	var info EnvInfo
	if err := json.Unmarshal([]byte(stdout), &info); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	if info.Timezone.TSKTZ != "(unset)" {
		t.Fatalf("expected '(unset)' for TSK_TZ, got %q", info.Timezone.TSKTZ)
	}
	if info.Editor.Resolved != "(none)" {
		t.Fatalf("expected '(none)' when no editor env, got %q", info.Editor.Resolved)
	}
	if info.Editor.Source != "fallback" {
		t.Fatalf("expected fallback source, got %q", info.Editor.Source)
	}
}

func TestEnvCollectsTSKPrefixedVars(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TSK_FOO", "1")
	t.Setenv("TSK_BAR", "2")
	t.Setenv("OTHER_NOT_TSK", "ignore")
	stdout, _, err := runCmd(t, dir, "env", "--json")
	if err != nil {
		t.Fatalf("env --json: %v", err)
	}
	var info EnvInfo
	if err := json.Unmarshal([]byte(stdout), &info); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout)
	}
	hasFoo, hasBar, hasOther := false, false, false
	for _, kv := range info.Env {
		if kv == "TSK_FOO=1" {
			hasFoo = true
		}
		if kv == "TSK_BAR=2" {
			hasBar = true
		}
		if strings.HasPrefix(kv, "OTHER_NOT_TSK") {
			hasOther = true
		}
	}
	if !hasFoo || !hasBar {
		t.Fatalf("expected TSK_FOO + TSK_BAR in env, got %v", info.Env)
	}
	if hasOther {
		t.Fatalf("non-TSK var leaked into output: %v", info.Env)
	}
}
