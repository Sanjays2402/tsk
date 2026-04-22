package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sanjays2402/tsk/internal/store"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create a .tsk.md in the current directory",
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, _ := cmd.Flags().GetString("file")
			if path == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return err
				}
				path = filepath.Join(cwd, store.FileName)
			}
			if _, err := os.Stat(path); err == nil {
				return fmt.Errorf("%s already exists", path)
			}
			if err := store.AtomicWriteFile(path, []byte("# tsk\n\n"), 0o644); err != nil {
				return err
			}
			pf(cmd.OutOrStdout(), "created %s\n", path)
			return nil
		},
	}
}
