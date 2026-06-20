package commands

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

// newCompletionCmd implements `tsk completion {bash|zsh|fish|powershell}`.
//
// Without flags it prints the script to stdout (cobra default). Pass
// --install to write the script to a well-known location for that
// shell so completion starts working without manual rcfile editing.
//
// Install destinations (per shell):
//
//	bash: ~/.local/share/bash-completion/completions/tsk
//	zsh:  the first $fpath dir under the user's home, fallback ~/.zsh/completions/_tsk
//	fish: ~/.config/fish/completions/tsk.fish
//	powershell: ~/.config/powershell/Microsoft.PowerShell_profile.ps1 (appended once, idempotent)
//
// All destinations are user-owned (no sudo), which is the polite default;
// system-wide installs are out of scope for this command.
func newCompletionCmd() *cobra.Command {
	var (
		install bool
		printTo bool
	)
	cmd := &cobra.Command{
		Use:                   "completion {bash|zsh|fish|powershell}",
		Short:                 "Generate (or install) a shell completion script",
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		Long: `Generate a shell completion script for tsk.

Default: print to stdout. Source it in your shell init file, or pass
--install to drop the script into the standard completion directory for
your shell — no manual rcfile editing required for most setups.

Examples:
  tsk completion bash > /usr/local/etc/bash_completion.d/tsk
  tsk completion zsh --install
  tsk completion fish --install
  tsk completion powershell --install

The --install destinations are user-owned (no sudo). Re-running --install
is idempotent: existing files are replaced; the PowerShell profile is
edited once and never appended twice.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			shell := args[0]
			if printTo && install {
				return usageErrorf("--install and printing to stdout are mutually exclusive")
			}
			if !install {
				return writeCompletionScript(cmd.Root(), cmd.OutOrStdout(), shell)
			}
			return installCompletionScript(cmd, shell)
		},
	}
	cmd.Flags().BoolVar(&install, "install", false, "write the script into the standard completion directory")
	cmd.Flags().BoolVar(&printTo, "stdout", false, "explicitly request stdout (default behavior)")
	return cmd
}

// writeCompletionScript dispatches to cobra's per-shell generator and
// writes the result to w.
func writeCompletionScript(root *cobra.Command, w writeable, shell string) error {
	switch shell {
	case "bash":
		return root.GenBashCompletionV2(w, true)
	case "zsh":
		return root.GenZshCompletion(w)
	case "fish":
		return root.GenFishCompletion(w, true)
	case "powershell":
		return root.GenPowerShellCompletionWithDesc(w)
	}
	return fmt.Errorf("unsupported shell %q", shell)
}

// writeable is the io.Writer interface restated locally so this file
// stays self-contained next to the rest of the commands package.
type writeable interface {
	Write(p []byte) (int, error)
}

// installCompletionScript renders the script and writes it to the
// per-shell standard location. Reports the destination so the user
// can verify (or grep their rc files for it).
func installCompletionScript(cmd *cobra.Command, shell string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}
	var buf bytes.Buffer
	if err := writeCompletionScript(cmd.Root(), &buf, shell); err != nil {
		return err
	}
	dest, msg, err := installDest(shell, home)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(dest), err)
	}
	switch shell {
	case "powershell":
		if err := appendPowerShellOnce(dest, buf.String()); err != nil {
			return err
		}
	default:
		if err := os.WriteFile(dest, buf.Bytes(), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", dest, err)
		}
	}
	pf(cmd.OutOrStdout(), "installed %s completion: %s\n", shell, dest)
	if msg != "" {
		pf(cmd.OutOrStdout(), "  %s\n", msg)
	}
	return nil
}

// installDest returns the standard install path for a shell. The
// second return is an optional follow-up message (e.g. "restart your
// shell") to print after a successful write.
func installDest(shell, home string) (path, hint string, err error) {
	switch shell {
	case "bash":
		// XDG-blessed location; works on macOS Homebrew bash-completion v2
		// and modern Linux distros.
		return filepath.Join(home, ".local", "share", "bash-completion", "completions", "tsk"),
			"restart your shell or `source` the file to activate", nil
	case "zsh":
		// Drop the canonical _tsk file under ~/.zsh/completions; that
		// directory is conventional and easy to add to $fpath.
		dst := filepath.Join(home, ".zsh", "completions", "_tsk")
		hint := "add `fpath=(~/.zsh/completions $fpath)` to ~/.zshrc above `compinit` if not already present"
		return dst, hint, nil
	case "fish":
		return filepath.Join(home, ".config", "fish", "completions", "tsk.fish"),
			"fish picks this up automatically — no rc edits needed", nil
	case "powershell":
		// On Unix-like systems, PowerShell profiles live under ~/.config/powershell;
		// on Windows they live under Documents/PowerShell. Pick the
		// platform-appropriate default.
		if runtime.GOOS == "windows" {
			return filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"),
				"appended to your PowerShell profile — restart pwsh", nil
		}
		return filepath.Join(home, ".config", "powershell", "Microsoft.PowerShell_profile.ps1"),
			"appended to your PowerShell profile — restart pwsh", nil
	}
	return "", "", fmt.Errorf("unsupported shell %q", shell)
}

// appendPowerShellOnce edits the PowerShell profile to include the tsk
// completion block — sentinel-guarded so re-running is idempotent.
// Reads the existing file, looks for the start sentinel; if present,
// replaces the block between the start and end sentinels; otherwise
// appends a fresh block.
func appendPowerShellOnce(dest, script string) error {
	const startSentinel = "# >>> tsk completion >>>"
	const endSentinel = "# <<< tsk completion <<<"
	block := fmt.Sprintf("\n%s\n%s\n%s\n", startSentinel, strings.TrimSpace(script), endSentinel)

	existing, err := os.ReadFile(dest)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", dest, err)
	}
	body := string(existing)
	if start := strings.Index(body, startSentinel); start != -1 {
		end := strings.Index(body[start:], endSentinel)
		if end != -1 {
			end = start + end + len(endSentinel)
			// Replace existing block.
			body = body[:start] + strings.TrimPrefix(block, "\n") + body[end:]
			return os.WriteFile(dest, []byte(body), 0o644)
		}
	}
	// Append fresh block.
	body += block
	return os.WriteFile(dest, []byte(body), 0o644)
}
