package tui

import (
	"github.com/charmbracelet/bubbles/textinput"
)

// formMode identifies which inline form is currently active.
type formMode int

// Form modes displayed above the list.
const (
	formNone formMode = iota
	formAdd
	formEditTitle
	formTags
	formSearch
	formSort
)

func newTextInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.CharLimit = 256
	ti.Width = 60
	return ti
}
