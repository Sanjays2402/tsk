package tui

import (
	"github.com/Sanjays2402/tsk/internal/store"
	"github.com/spf13/cobra"

	tea "github.com/charmbracelet/bubbletea"
)

// Run launches the TUI for the given cobra command (honoring --file).
func Run(cmd *cobra.Command) error {
	path, _ := cmd.Flags().GetString("file")
	if path == "" {
		path = store.ResolveOrCreate("")
	}
	s, err := store.Load(path)
	if err != nil {
		return err
	}
	app := New(s)
	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
