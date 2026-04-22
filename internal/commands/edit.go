package commands

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func newEditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open .tsk.md in $EDITOR",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}
			c := exec.Command(editor, s.Path) //nolint:gosec // editor is user-controlled by design
			c.Stdin = os.Stdin
			c.Stdout = cmd.OutOrStdout()
			c.Stderr = cmd.ErrOrStderr()
			if err := c.Run(); err != nil {
				return fmt.Errorf("editor: %w", err)
			}
			return nil
		},
	}
}
