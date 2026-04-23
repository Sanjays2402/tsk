package commands

import (
	"bytes"
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
