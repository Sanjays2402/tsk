// Package tui implements tsk's interactive bubbletea interface.
package tui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
)

// noColor reports whether colored output is disabled (NO_COLOR per https://no-color.org).
func noColor() bool {
	v, ok := os.LookupEnv("NO_COLOR")
	return ok && v != ""
}

func color(light, dark string) lipgloss.TerminalColor {
	if noColor() {
		return lipgloss.NoColor{}
	}
	return lipgloss.AdaptiveColor{Light: light, Dark: dark}
}

// Palette bundles the color styles used across the TUI.
type Palette struct {
	Primary   lipgloss.Style
	Accent    lipgloss.Style
	Muted     lipgloss.Style
	Urgent    lipgloss.Style
	High      lipgloss.Style
	Medium    lipgloss.Style
	Low       lipgloss.Style
	Done      lipgloss.Style
	Section   lipgloss.Style
	Help      lipgloss.Style
	Selection lipgloss.Style
}

// NewPalette builds the default amber/gold Palette, honoring NO_COLOR.
func NewPalette() Palette {
	return Palette{
		Primary:   lipgloss.NewStyle().Foreground(color("#B45309", "#F59E0B")).Bold(true),
		Accent:    lipgloss.NewStyle().Foreground(color("#92400E", "#FBBF24")),
		Muted:     lipgloss.NewStyle().Foreground(color("#64748B", "#94A3B8")),
		Urgent:    lipgloss.NewStyle().Foreground(color("#B91C1C", "#EF4444")).Bold(true),
		High:      lipgloss.NewStyle().Foreground(color("#C2410C", "#F97316")),
		Medium:    lipgloss.NewStyle().Foreground(color("#A16207", "#EAB308")),
		Low:       lipgloss.NewStyle().Foreground(color("#475569", "#64748B")),
		Done:      lipgloss.NewStyle().Foreground(color("#64748B", "#64748B")).Strikethrough(true),
		Section:   lipgloss.NewStyle().Foreground(color("#B45309", "#FBBF24")).Bold(true).Underline(true),
		Help:      lipgloss.NewStyle().Foreground(color("#64748B", "#94A3B8")).Italic(true),
		Selection: lipgloss.NewStyle().Foreground(color("#B45309", "#F59E0B")).Bold(true),
	}
}
