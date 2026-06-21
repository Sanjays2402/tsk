package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCompletionProducesUsableScripts asserts that all four supported shells
// generate a script that (a) parses without error and (b) names the tsk root
// command. This catches future breakage if cobra versions change how scripts
// are emitted, or if a case is accidentally dropped from the switch.
func TestCompletionProducesUsableScripts(t *testing.T) {
	shells := []string{"bash", "zsh", "fish", "powershell"}
	for _, shell := range shells {
		shell := shell
		t.Run(shell, func(t *testing.T) {
			root := NewRoot()
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&out)
			root.SetArgs([]string{"completion", shell})
			if err := root.Execute(); err != nil {
				t.Fatalf("completion %s: %v", shell, err)
			}
			script := out.String()
			if len(script) < 100 {
				t.Fatalf("%s script suspiciously short (%d bytes)", shell, len(script))
			}
			// Every supported shell's output references the root command by
			// some form of its name. Bash/zsh emit literal "tsk", fish emits
			// "__fish_tsk", powershell emits "Register-ArgumentCompleter tsk".
			if !strings.Contains(strings.ToLower(script), "tsk") {
				end := 400
				if end > len(script) {
					end = len(script)
				}
				t.Fatalf("%s script does not mention 'tsk':\n%s", shell, script[:end])
			}
		})
	}
}

func TestCompletionRejectsUnsupportedShell(t *testing.T) {
	root := NewRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	// Silence cobra's default error trailing so the test output is clean.
	root.SilenceUsage = true
	root.SilenceErrors = true
	root.SetArgs([]string{"completion", "tcsh"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error for unsupported shell, got nil")
	}
}

func TestCompletionRequiresExactlyOneArg(t *testing.T) {
	root := NewRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SilenceUsage = true
	root.SilenceErrors = true
	root.SetArgs([]string{"completion"})
	if err := root.Execute(); err == nil {
		t.Fatal("expected error when shell arg is missing")
	}
}

// TestCompletionInstallWritesScript checks that --install drops the
// generated script at the expected per-shell location under a sandboxed
// HOME directory.
func TestCompletionInstallWritesScript(t *testing.T) {
	cases := []struct {
		shell   string
		relPath string
	}{
		{"bash", ".local/share/bash-completion/completions/tsk"},
		{"zsh", ".zsh/completions/_tsk"},
		{"fish", ".config/fish/completions/tsk.fish"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.shell, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			stdout, _, err := runCmd(t, t.TempDir(), "completion", tc.shell, "--install")
			if err != nil {
				t.Fatalf("install %s: %v", tc.shell, err)
			}
			target := filepath.Join(home, tc.relPath)
			info, err := os.Stat(target)
			if err != nil {
				t.Fatalf("expected file at %s: %v", target, err)
			}
			if info.Size() < 100 {
				t.Fatalf("file at %s suspiciously small (%d bytes)", target, info.Size())
			}
			if !strings.Contains(stdout, "installed "+tc.shell) {
				t.Fatalf("expected confirmation message, got %q", stdout)
			}
		})
	}
}

// TestCompletionInstallPowerShellIdempotent verifies that re-running
// the PowerShell install appends the block exactly once.
func TestCompletionInstallPowerShellIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Pre-seed the profile with some unrelated content to ensure the
	// install preserves it.
	profile := filepath.Join(home, ".config", "powershell", "Microsoft.PowerShell_profile.ps1")
	if err := os.MkdirAll(filepath.Dir(profile), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	preexisting := "# my custom prompt\nfunction prompt { \"PS> \" }\n"
	if err := os.WriteFile(profile, []byte(preexisting), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, _, err := runCmd(t, t.TempDir(), "completion", "powershell", "--install"); err != nil {
		t.Fatalf("first install: %v", err)
	}
	body, err := os.ReadFile(profile)
	if err != nil {
		t.Fatalf("read 1: %v", err)
	}
	if !strings.Contains(string(body), preexisting) {
		t.Fatalf("preexisting content lost:\n%s", body)
	}
	if !strings.Contains(string(body), "# >>> tsk completion >>>") {
		t.Fatalf("start sentinel missing:\n%s", body)
	}
	// Re-run; should still have exactly one sentinel pair.
	if _, _, err := runCmd(t, t.TempDir(), "completion", "powershell", "--install"); err != nil {
		t.Fatalf("second install: %v", err)
	}
	body, err = os.ReadFile(profile)
	if err != nil {
		t.Fatalf("read 2: %v", err)
	}
	starts := strings.Count(string(body), "# >>> tsk completion >>>")
	ends := strings.Count(string(body), "# <<< tsk completion <<<")
	if starts != 1 || ends != 1 {
		t.Fatalf("expected exactly one sentinel pair, got start=%d end=%d:\n%s", starts, ends, body)
	}
	// Custom content still present.
	if !strings.Contains(string(body), "my custom prompt") {
		t.Fatalf("custom content lost on second install:\n%s", body)
	}
}

// TestCompletionInstallRejectsStdoutMix ensures --install and --stdout
// can't both be set.
func TestCompletionInstallRejectsStdoutMix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	_, _, err := runCmd(t, t.TempDir(), "completion", "bash", "--install", "--stdout")
	if err == nil {
		t.Fatal("expected error for --install + --stdout")
	}
}
