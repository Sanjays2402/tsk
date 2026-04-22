package tui

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Sanjays2402/tsk/internal/dateparse"
	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	tuiLocOnce sync.Once
	tuiLocVal  *time.Location
)

// pacificLoc returns the cached America/Los_Angeles location, falling back to
// time.Local when zoneinfo data is unavailable.
func pacificLoc() *time.Location {
	tuiLocOnce.Do(func() {
		if l, err := time.LoadLocation("America/Los_Angeles"); err == nil {
			tuiLocVal = l
			return
		}
		tuiLocVal = time.Local
	})
	return tuiLocVal
}

// App is the bubbletea Model for tsk's interactive UI.
type App struct {
	store     *store.Store
	pal       Palette
	keys      Keymap
	now       time.Time
	width     int
	height    int
	selection int
	collapsed map[sectionKind]bool
	form      formMode
	inputCur  inputBox
	editing   int
	confirm   int
	status    string
	showHelp  bool
	filter    string
	sortMode  string
}

// inputBox is a tiny stand-in that abstracts textinput to avoid importing the
// whole bubbles package in this model struct (keeps test helpers simple).
type inputBox struct {
	label string
	value string
	focus bool
}

func (b inputBox) View() string {
	caret := "█"
	if !b.focus {
		caret = ""
	}
	return fmt.Sprintf("%s: %s%s", b.label, b.value, caret)
}

// New constructs a new TUI app wrapped around a loaded store.
func New(s *store.Store) *App {
	return &App{
		store:     s,
		pal:       NewPalette(),
		keys:      DefaultKeymap(),
		now:       time.Now(),
		collapsed: map[sectionKind]bool{sectionDone: true},
		sortMode:  "priority",
	}
}

// Init satisfies tea.Model.
func (a *App) Init() tea.Cmd { return nil }

// Update handles keypresses and window events.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = m.Width, m.Height
		return a, nil
	case tea.KeyMsg:
		return a.handleKey(m)
	}
	return a, nil
}

func (a *App) handleKey(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.form != formNone {
		return a.handleFormKey(m)
	}
	if a.confirm != 0 {
		return a.handleConfirmKey(m)
	}
	if handled, model, cmd := a.handleGlobalKey(m); handled {
		return model, cmd
	}
	a.handleNavKey(m)
	return a, nil
}

func (a *App) handleGlobalKey(m tea.KeyMsg) (bool, tea.Model, tea.Cmd) {
	switch {
	case matches(m, a.keys.Quit):
		return true, a, tea.Quit
	case matches(m, a.keys.Help):
		a.showHelp = !a.showHelp
		return true, a, nil
	}
	return false, a, nil
}

func (a *App) handleNavKey(m tea.KeyMsg) {
	switch {
	case matches(m, a.keys.Down):
		a.moveSelection(1)
	case matches(m, a.keys.Up):
		a.moveSelection(-1)
	case matches(m, a.keys.Toggle):
		a.toggleCurrent()
	case matches(m, a.keys.Add):
		a.form = formAdd
		a.inputCur = inputBox{label: "new task", focus: true}
	case matches(m, a.keys.Edit):
		a.startEditTitle()
	case matches(m, a.keys.Delete):
		if id := a.currentID(); id != 0 {
			a.confirm = id
		}
	case matches(m, a.keys.PriorityCycle):
		a.cyclePriority()
	case matches(m, a.keys.TagEdit):
		a.startEditTags()
	case matches(m, a.keys.DueEdit):
		a.startEditDue()
	case matches(m, a.keys.Search):
		a.form = formSearch
		a.inputCur = inputBox{label: "search", value: a.filter, focus: true}
	case matches(m, a.keys.SortMenu):
		a.form = formSort
		a.inputCur = inputBox{label: "sort (priority|due|created|id)", value: a.sortMode, focus: true}
	case matches(m, a.keys.Section):
		a.toggleSection()
	}
}

func (a *App) startEditTitle() {
	id := a.currentID()
	if id == 0 {
		return
	}
	a.editing = id
	a.form = formEditTitle
	t := a.store.ByID(id)
	a.inputCur = inputBox{label: "edit title", value: t.Title, focus: true}
}

func (a *App) startEditTags() {
	id := a.currentID()
	if id == 0 {
		return
	}
	a.editing = id
	a.form = formTags
	t := a.store.ByID(id)
	a.inputCur = inputBox{label: "tags (comma-sep)", value: strings.Join(t.Tags, ","), focus: true}
}

func (a *App) startEditDue() {
	id := a.currentID()
	if id == 0 {
		return
	}
	a.editing = id
	a.form = formDue
	t := a.store.ByID(id)
	cur := ""
	if t != nil && t.Due != nil {
		cur = t.Due.Format(model.DateLayout)
	}
	a.inputCur = inputBox{label: "due (YYYY-MM-DD, tomorrow, fri, in 3d, eow; empty to clear)", value: cur, focus: true}
}

func (a *App) handleFormKey(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.Type {
	case tea.KeyEsc:
		a.form = formNone
		a.editing = 0
		return a, nil
	case tea.KeyEnter:
		return a.commitForm()
	case tea.KeyBackspace:
		if len(a.inputCur.value) > 0 {
			a.inputCur.value = a.inputCur.value[:len(a.inputCur.value)-1]
		}
	case tea.KeyRunes, tea.KeySpace:
		a.inputCur.value += string(m.Runes)
	}
	return a, nil
}

func (a *App) commitForm() (tea.Model, tea.Cmd) {
	val := strings.TrimSpace(a.inputCur.value)
	switch a.form {
	case formAdd:
		if val != "" {
			a.store.Add(model.Task{Title: val, Priority: model.PriorityMedium, Created: time.Now()})
			_ = a.store.Save()
			a.status = "added"
		}
	case formEditTitle:
		if t := a.store.ByID(a.editing); t != nil && val != "" {
			t.Title = val
			_ = a.store.Save()
			a.status = "edited"
		}
	case formTags:
		if t := a.store.ByID(a.editing); t != nil {
			t.Tags = splitTags(val)
			t.NormalizeTags()
			_ = a.store.Save()
			a.status = "tags updated"
		}
	case formDue:
		a.commitDue(val)
	case formSearch:
		a.filter = val
	case formSort:
		if val == "priority" || val == "due" || val == "created" || val == "id" {
			a.sortMode = val
		}
	}
	a.form = formNone
	a.editing = 0
	return a, nil
}

// commitDue applies the parsed due-date form value to the current task, or
// clears the due date if the input is empty.
func (a *App) commitDue(val string) {
	t := a.store.ByID(a.editing)
	if t == nil {
		return
	}
	if val == "" {
		t.Due = nil
		_ = a.store.Save()
		a.status = "due cleared"
		return
	}
	loc := pacificLoc()
	due, err := dateparse.Parse(val, time.Now().In(loc), loc)
	if err != nil {
		a.status = err.Error()
		return
	}
	t.Due = &due
	_ = a.store.Save()
	a.status = "due updated"
}

func (a *App) handleConfirmKey(m tea.KeyMsg) (tea.Model, tea.Cmd) {
	if matches(m, a.keys.Confirm) {
		a.store.Remove(a.confirm)
		_ = a.store.Save()
		a.status = "deleted"
	}
	a.confirm = 0
	return a, nil
}

func splitTags(s string) []string {
	out := []string{}
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func matches(m tea.KeyMsg, binding interface{ Keys() []string }) bool {
	s := m.String()
	for _, k := range binding.Keys() {
		if k == s {
			return true
		}
	}
	return false
}

// visibleTasks returns the flat ordered list of tasks rendered to the user.
func (a *App) visibleTasks() []model.Task {
	groups := groupedTasks(a.store.Tasks, a.now)
	var out []model.Task
	order := []sectionKind{sectionOverdue, sectionToday, sectionUpcoming, sectionNoDue, sectionDone}
	for _, k := range order {
		if a.collapsed[k] {
			continue
		}
		g := groups[k]
		if a.sortMode != "" {
			model.SortBy(g, a.sortMode)
		}
		if a.filter != "" {
			filtered := g[:0]
			q := strings.ToLower(a.filter)
			for _, t := range g {
				if strings.Contains(strings.ToLower(t.Title), q) {
					filtered = append(filtered, t)
				}
			}
			g = filtered
		}
		out = append(out, g...)
	}
	return out
}

func (a *App) currentID() int {
	vt := a.visibleTasks()
	if a.selection < 0 || a.selection >= len(vt) {
		return 0
	}
	return vt[a.selection].ID
}

func (a *App) moveSelection(d int) {
	n := len(a.visibleTasks())
	if n == 0 {
		a.selection = 0
		return
	}
	a.selection = (a.selection + d + n) % n
}

func (a *App) toggleCurrent() {
	id := a.currentID()
	if id == 0 {
		return
	}
	t := a.store.ByID(id)
	a.store.SetDone(id, !t.Done)
	_ = a.store.Save()
}

func (a *App) cyclePriority() {
	id := a.currentID()
	if id == 0 {
		return
	}
	t := a.store.ByID(id)
	t.Priority = model.Priority((int(t.Priority) + 1) % 4)
	_ = a.store.Save()
}

func (a *App) toggleSection() {
	vt := a.visibleTasks()
	if a.selection >= len(vt) {
		return
	}
	// Find which section the current selection belongs to and toggle it.
	id := a.currentID()
	t := a.store.ByID(id)
	if t == nil {
		return
	}
	switch {
	case t.Done:
		a.collapsed[sectionDone] = !a.collapsed[sectionDone]
	case t.IsOverdue(a.now):
		a.collapsed[sectionOverdue] = !a.collapsed[sectionOverdue]
	case t.IsDueToday(a.now):
		a.collapsed[sectionToday] = !a.collapsed[sectionToday]
	case t.IsUpcoming(a.now):
		a.collapsed[sectionUpcoming] = !a.collapsed[sectionUpcoming]
	default:
		a.collapsed[sectionNoDue] = !a.collapsed[sectionNoDue]
	}
}

// View renders the UI as a string.
func (a *App) View() string {
	var b strings.Builder
	b.WriteString(a.pal.Primary.Render("tsk"))
	b.WriteString("  ")
	b.WriteString(a.pal.Muted.Render(a.store.Path))
	b.WriteByte('\n')
	b.WriteByte('\n')

	groups := groupedTasks(a.store.Tasks, a.now)
	order := []sectionKind{sectionOverdue, sectionToday, sectionUpcoming, sectionNoDue, sectionDone}
	cursor := 0
	done, total := 0, 0
	for _, t := range a.store.Tasks {
		if t.Done {
			done++
		}
		total++
	}

	for _, k := range order {
		g := groups[k]
		if a.sortMode != "" {
			model.SortBy(g, a.sortMode)
		}
		if a.filter != "" {
			filtered := g[:0]
			q := strings.ToLower(a.filter)
			for _, t := range g {
				if strings.Contains(strings.ToLower(t.Title), q) {
					filtered = append(filtered, t)
				}
			}
			g = filtered
		}
		marker := "▾"
		if a.collapsed[k] {
			marker = "▸"
		}
		b.WriteString(a.pal.Section.Render(fmt.Sprintf("%s %s (%d)", marker, k.label(), len(g))))
		b.WriteByte('\n')
		if a.collapsed[k] {
			continue
		}
		for _, t := range g {
			line := a.renderTaskLine(t, cursor == a.selection)
			b.WriteString(line)
			b.WriteByte('\n')
			cursor++
		}
	}
	b.WriteByte('\n')
	b.WriteString(a.pal.Accent.Render(progressBar(done, total, 24)))
	b.WriteByte('\n')

	if a.form != formNone {
		b.WriteByte('\n')
		b.WriteString(a.inputCur.View())
	}
	if a.confirm != 0 {
		b.WriteByte('\n')
		b.WriteString(a.pal.Urgent.Render(fmt.Sprintf("delete #%d? (y/n)", a.confirm)))
	}
	if a.status != "" {
		b.WriteByte('\n')
		b.WriteString(a.pal.Muted.Render(a.status))
	}
	if a.showHelp {
		b.WriteByte('\n')
		b.WriteString(a.helpView())
	} else {
		b.WriteByte('\n')
		b.WriteString(a.pal.Help.Render("j/k move · ␣ toggle · a add · e edit · d delete · D due · p prio · t tags · / search · s sort · tab collapse · ? help · q quit"))
	}
	return b.String()
}

func (a *App) renderTaskLine(t model.Task, selected bool) string {
	prefix := "  "
	if selected {
		prefix = a.pal.Primary.Render("▸ ")
	}
	check := checkbox(t.Done)
	prio := priorityLabel(t.Priority, a.pal)
	title := t.Title
	style := lipgloss.NewStyle()
	if t.Done {
		style = a.pal.Done
	}
	meta := ""
	if t.Due != nil {
		meta = a.pal.Muted.Render("  " + t.Due.Format(model.DateLayout))
	}
	if len(t.Tags) > 0 {
		sorted := append([]string(nil), t.Tags...)
		sort.Strings(sorted)
		meta += a.pal.Accent.Render("  #" + strings.Join(sorted, " #"))
	}
	return fmt.Sprintf("%s%s %s %s%s", prefix, check, prio, style.Render(title), meta)
}

func (a *App) helpView() string {
	rows := [][2]string{
		{"j/k", "move selection"},
		{"⏎/␣", "toggle done"},
		{"a", "add task"},
		{"e", "edit title"},
		{"t", "edit tags"},
		{"D", "set due date"},
		{"p", "cycle priority"},
		{"d", "delete (confirm)"},
		{"/", "fuzzy filter"},
		{"s", "sort: priority|due|created|id"},
		{"tab", "collapse current section"},
		{"?", "toggle help"},
		{"q", "quit"},
	}
	var b strings.Builder
	b.WriteString(a.pal.Section.Render("Help"))
	b.WriteByte('\n')
	for _, r := range rows {
		fmt.Fprintf(&b, "  %-5s  %s\n", r[0], r[1])
	}
	return b.String()
}
