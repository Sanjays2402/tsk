package tui

import (
	"testing"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
	tea "github.com/charmbracelet/bubbletea"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Load(dir + "/.tsk.md")
	if err != nil {
		t.Fatal(err)
	}
	s.Add(model.Task{Title: "a", Priority: model.PriorityHigh})
	s.Add(model.Task{Title: "b", Priority: model.PriorityLow})
	if err := s.Save(); err != nil {
		t.Fatal(err)
	}
	return New(s)
}

func TestAppUpdateNoPanic(t *testing.T) {
	app := newTestApp(t)
	msgs := []tea.Msg{
		tea.WindowSizeMsg{Width: 80, Height: 24},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")},
		tea.KeyMsg{Type: tea.KeyEsc},
	}
	var m tea.Model = app
	for _, msg := range msgs {
		m, _ = m.Update(msg)
	}
	view := m.(*App).View()
	if view == "" {
		t.Error("empty view")
	}
}

func TestProgressBar(t *testing.T) {
	s := progressBar(3, 10, 10)
	if s == "" {
		t.Error("empty progress bar")
	}
	empty := progressBar(0, 0, 5)
	if empty == "" {
		t.Error("empty total progress bar empty")
	}
}

func TestGroupedTasks(t *testing.T) {
	app := newTestApp(t)
	groups := groupedTasks(app.store.Tasks, app.now)
	if len(groups[sectionNoDue]) != 2 {
		t.Errorf("expected 2 in No Due, got %d", len(groups[sectionNoDue]))
	}
}
