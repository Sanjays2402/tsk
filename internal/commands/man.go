package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

// newManCmd implements `tsk man`: generate manpages (one per subcommand)
// from cobra's help tree. The default output directory is ./man (cwd) so
// the user can preview without committing to any system install — pass
// --dir to send them elsewhere, or `--install` for the standard
// /usr/local/share/man/man1 location (with the user's confirmation
// signaled by --yes since that path typically needs sudo).
//
// Pairs naturally with 'tsk completion --install' (next feature): both
// generate something cobra knows how to emit and put it in a place
// shells/manpath will find without further work.
func newManCmd() *cobra.Command {
	var (
		dir     string
		install bool
		yes     bool
		section string
	)
	cmd := &cobra.Command{
		Use:   "man",
		Short: "Generate manpages from the command tree",
		Long: `Generate manpages (one per subcommand) from cobra's help tree.

By default, manpages are written to ./man in the current directory so
you can inspect them without elevated privileges. Pass --install to
write to the standard system location (/usr/local/share/man/man1).
That target usually requires write access — re-run with sudo if needed.

Manpages are section-1 ('user commands') by default; override with --section.

Examples:
  tsk man                         # writes ./man/tsk.1, ./man/tsk-add.1, ...
  tsk man --dir ~/.local/share/man/man1
  sudo tsk man --install --yes    # writes to /usr/local/share/man/man1
  man -l ./man/tsk.1              # preview without installing
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			outDir, err := resolveManOutDir(dir, install, yes)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return fmt.Errorf("mkdir %s: %w", outDir, err)
			}
			header := &doc.GenManHeader{
				Title:   "TSK",
				Section: strings.TrimSpace(section),
				Source:  fmt.Sprintf("tsk %s", buildVersion),
				Manual:  "tsk Manual",
			}
			root := cmd.Root()
			if err := doc.GenManTree(root, header, outDir); err != nil {
				return fmt.Errorf("generate manpages: %w", err)
			}
			count, err := countManFiles(outDir, header.Section)
			if err != nil {
				return err
			}
			pf(cmd.OutOrStdout(), "wrote %d manpage(s) to %s\n", count, outDir)
			pf(cmd.OutOrStdout(), "preview: man -l %s/tsk.%s\n", outDir, header.Section)
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "output directory (default: ./man)")
	cmd.Flags().BoolVar(&install, "install", false, "write to /usr/local/share/man/man1 (requires --yes)")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm system-wide install")
	cmd.Flags().StringVar(&section, "section", "1", "manpage section (1=user, 8=admin)")
	return cmd
}

// resolveManOutDir picks the output directory from --dir, then --install,
// then defaults to ./man. --install requires --yes to avoid surprising
// system-wide writes from a tab-complete typo.
func resolveManOutDir(dir string, install, yes bool) (string, error) {
	dir = strings.TrimSpace(dir)
	switch {
	case dir != "" && install:
		return "", usageErrorf("--dir and --install are mutually exclusive")
	case install && !yes:
		return "", usageErrorf("--install writes system-wide; re-run with --yes to confirm")
	case install:
		return "/usr/local/share/man/man1", nil
	case dir != "":
		abs, err := filepath.Abs(dir)
		if err != nil {
			return "", err
		}
		return abs, nil
	default:
		return "./man", nil
	}
}

// countManFiles tallies how many *.<section> files cobra dropped, so
// the success line reports an accurate number even if cobra changes
// its naming convention in a future release.
func countManFiles(dir, section string) (int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("read %s: %w", dir, err)
	}
	suffix := "." + section
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), suffix) {
			n++
		}
	}
	return n, nil
}
