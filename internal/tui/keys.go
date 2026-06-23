package tui

import "github.com/charmbracelet/bubbles/key"

// Keymap holds the bindable keys the TUI listens for.
type Keymap struct {
	Up, Down          key.Binding
	Top, Bottom       key.Binding
	Toggle            key.Binding
	Add, Edit, Delete key.Binding
	PriorityCycle     key.Binding
	TagEdit           key.Binding
	DueEdit           key.Binding
	Search            key.Binding
	SortMenu          key.Binding
	Help              key.Binding
	Quit              key.Binding
	Confirm, Cancel   key.Binding
	Section           key.Binding
	Reload            key.Binding
	ReloadClear       key.Binding
	Clone             key.Binding
	JumpNext          key.Binding
	FocusPinned       key.Binding
	ArchiveCurrent    key.Binding
	TogglePin         key.Binding
}

// DefaultKeymap returns the default tsk TUI keybindings.
func DefaultKeymap() Keymap {
	return Keymap{
		Up:             key.NewBinding(key.WithKeys("k", "up"), key.WithHelp("k/↑", "up")),
		Down:           key.NewBinding(key.WithKeys("j", "down"), key.WithHelp("j/↓", "down")),
		Top:            key.NewBinding(key.WithKeys("g", "home"), key.WithHelp("g", "top")),
		Bottom:         key.NewBinding(key.WithKeys("G", "end"), key.WithHelp("G", "bottom")),
		Toggle:         key.NewBinding(key.WithKeys("enter", " "), key.WithHelp("␣/⏎", "toggle done")),
		Add:            key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add")),
		Edit:           key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		Delete:         key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		PriorityCycle:  key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "priority")),
		TagEdit:        key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "tags")),
		DueEdit:        key.NewBinding(key.WithKeys("D"), key.WithHelp("D", "due")),
		Search:         key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
		SortMenu:       key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "sort")),
		Help:           key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:           key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Confirm:        key.NewBinding(key.WithKeys("y")),
		Cancel:         key.NewBinding(key.WithKeys("esc", "n")),
		Section:        key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "collapse section")),
		Reload:         key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "reload")),
		ReloadClear:    key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "reload+clear filter")),
		Clone:          key.NewBinding(key.WithKeys("C"), key.WithHelp("C", "clone")),
		JumpNext:       key.NewBinding(key.WithKeys("N"), key.WithHelp("N", "jump to next-unblocked")),
		FocusPinned:    key.NewBinding(key.WithKeys("F"), key.WithHelp("F", "focus pinned only")),
		ArchiveCurrent: key.NewBinding(key.WithKeys("X"), key.WithHelp("X", "archive (done only)")),
		TogglePin:      key.NewBinding(key.WithKeys("*"), key.WithHelp("*", "toggle pin")),
	}
}
