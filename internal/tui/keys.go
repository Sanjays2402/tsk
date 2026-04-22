package tui

import "github.com/charmbracelet/bubbles/key"

// Keymap holds the bindable keys the TUI listens for.
type Keymap struct {
	Up, Down          key.Binding
	Toggle            key.Binding
	Add, Edit, Delete key.Binding
	PriorityCycle     key.Binding
	TagEdit           key.Binding
	Search            key.Binding
	SortMenu          key.Binding
	Help              key.Binding
	Quit              key.Binding
	Confirm, Cancel   key.Binding
	Section           key.Binding
}

// DefaultKeymap returns the default tsk TUI keybindings.
func DefaultKeymap() Keymap {
	return Keymap{
		Up:            key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
		Down:          key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
		Toggle:        key.NewBinding(key.WithKeys("enter", " "), key.WithHelp("␣/⏎", "toggle done")),
		Add:           key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
		Edit:          key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		Delete:        key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		PriorityCycle: key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "priority")),
		TagEdit:       key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "tags")),
		Search:        key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		SortMenu:      key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sort")),
		Help:          key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:          key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Confirm:       key.NewBinding(key.WithKeys("y")),
		Cancel:        key.NewBinding(key.WithKeys("esc", "n")),
		Section:       key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "collapse section")),
	}
}
