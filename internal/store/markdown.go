package store

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
)

// FileName is the markdown filename tsk reads from and writes to.
const FileName = ".tsk.md"

// NotesIndent is the number of leading spaces a continuation (notes) line must have.
const NotesIndent = 6

// TimeLayout is the RFC3339 layout used for created/completed timestamps.
const TimeLayout = time.RFC3339

var (
	taskLineRe = regexp.MustCompile(`^- \[( |x|X)\] (.*?)(?:\s*<!--\s*(.*?)\s*-->)?\s*$`)
	metaPairRe = regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*):\s*([^\s]+)`)
)

// Store is a markdown-backed task list.
type Store struct {
	// Path is the absolute path to the .tsk.md file (may not exist yet).
	Path string
	// Header holds any non-task content that appears before the first task line.
	Header string
	// Tasks is the in-memory task slice, in file order.
	Tasks []model.Task
}

// Load reads and parses a .tsk.md file. Missing file yields an empty Store.
func Load(path string) (*Store, error) {
	s := &Store{Path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if err := s.parse(data); err != nil {
		return nil, err
	}
	return s, nil
}

// Save atomically writes the store back to disk.
func (s *Store) Save() error {
	s.assignIDs()
	data := s.render()
	return AtomicWriteFile(s.Path, data, 0o644)
}

// Add appends a task to the store (without saving) and returns the assigned ID.
func (s *Store) Add(t model.Task) int {
	if t.Created.IsZero() {
		t.Created = time.Now()
	}
	if t.ID == 0 {
		t.ID = s.nextID()
	}
	t.NormalizeTags()
	s.Tasks = append(s.Tasks, t)
	return t.ID
}

// ByID returns a pointer to the task with the given ID, or nil.
func (s *Store) ByID(id int) *model.Task {
	for i := range s.Tasks {
		if s.Tasks[i].ID == id {
			return &s.Tasks[i]
		}
	}
	return nil
}

// Remove deletes the task with the given ID. Returns true if removed.
func (s *Store) Remove(id int) bool {
	for i := range s.Tasks {
		if s.Tasks[i].ID == id {
			s.Tasks = append(s.Tasks[:i], s.Tasks[i+1:]...)
			return true
		}
	}
	return false
}

// SetDone toggles the completion state of the task with the given ID.
func (s *Store) SetDone(id int, done bool) bool {
	t := s.ByID(id)
	if t == nil {
		return false
	}
	t.Done = done
	if done {
		now := time.Now()
		t.Completed = &now
	} else {
		t.Completed = nil
	}
	return true
}

func (s *Store) nextID() int {
	max := 0
	for _, t := range s.Tasks {
		if t.ID > max {
			max = t.ID
		}
	}
	return max + 1
}

func (s *Store) assignIDs() {
	used := make(map[int]bool, len(s.Tasks))
	for _, t := range s.Tasks {
		if t.ID > 0 {
			used[t.ID] = true
		}
	}
	next := 1
	for i := range s.Tasks {
		if s.Tasks[i].ID > 0 {
			continue
		}
		for used[next] {
			next++
		}
		s.Tasks[i].ID = next
		used[next] = true
	}
}

func (s *Store) parse(data []byte) error {
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var headerLines []string
	var tasks []model.Task
	sawTask := false
	var currentNotes []string

	flushNotes := func() {
		if len(tasks) == 0 {
			return
		}
		if len(currentNotes) == 0 {
			return
		}
		notes := strings.Join(currentNotes, "\n")
		tasks[len(tasks)-1].Notes = strings.TrimRight(notes, " \n\t")
		currentNotes = currentNotes[:0]
	}

	for sc.Scan() {
		line := sc.Text()
		if m := taskLineRe.FindStringSubmatch(line); m != nil {
			flushNotes()
			sawTask = true
			task := parseTaskLine(m)
			tasks = append(tasks, task)
			continue
		}
		if !sawTask {
			headerLines = append(headerLines, line)
			continue
		}
		// continuation notes (indented)
		if strings.HasPrefix(line, strings.Repeat(" ", NotesIndent)) {
			currentNotes = append(currentNotes, line[NotesIndent:])
			continue
		}
		if strings.TrimSpace(line) == "" {
			if len(currentNotes) > 0 {
				currentNotes = append(currentNotes, "")
			}
			continue
		}
		// Stray line after tasks section — tolerate by treating as notes for previous task.
		currentNotes = append(currentNotes, strings.TrimSpace(line))
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("scan: %w", err)
	}
	flushNotes()
	s.Header = strings.Join(headerLines, "\n")
	if len(headerLines) > 0 {
		s.Header += "\n"
	}
	s.Tasks = tasks
	return nil
}

func parseTaskLine(m []string) model.Task {
	t := model.Task{
		Done:     m[1] == "x" || m[1] == "X",
		Title:    strings.TrimSpace(m[2]),
		Priority: model.PriorityMedium,
	}
	meta := ""
	if len(m) >= 4 {
		meta = m[3]
	}
	applyMeta(&t, meta)
	t.NormalizeTags()
	return t
}

func applyMeta(t *model.Task, meta string) {
	if meta == "" {
		return
	}
	for _, match := range metaPairRe.FindAllStringSubmatch(meta, -1) {
		key, val := match[1], match[2]
		switch strings.ToLower(key) {
		case "id":
			if n, err := parseInt(val); err == nil {
				t.ID = n
			}
		case "prio", "priority":
			if p, err := model.ParsePriority(val); err == nil {
				t.Priority = p
			}
		case "due":
			if tm, err := time.Parse(model.DateLayout, val); err == nil {
				t.Due = &tm
			}
		case "tags":
			if val == "" {
				continue
			}
			t.Tags = append(t.Tags, strings.Split(val, ",")...)
		case "created":
			if tm, err := time.Parse(TimeLayout, val); err == nil {
				t.Created = tm
			}
		case "completed":
			if tm, err := time.Parse(TimeLayout, val); err == nil {
				t.Completed = &tm
			}
		}
	}
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

func (s *Store) render() []byte {
	var buf bytes.Buffer
	if s.Header != "" {
		buf.WriteString(s.Header)
		if !strings.HasSuffix(s.Header, "\n") {
			buf.WriteByte('\n')
		}
	}
	// Stable output: render in current Task slice order. Callers control order.
	for _, t := range s.Tasks {
		renderTask(&buf, t)
	}
	return buf.Bytes()
}

func renderTask(buf *bytes.Buffer, t model.Task) {
	box := " "
	if t.Done {
		box = "x"
	}
	fmt.Fprintf(buf, "- [%s] %s", box, t.Title)
	meta := renderMeta(t)
	if meta != "" {
		fmt.Fprintf(buf, " <!-- %s -->", meta)
	}
	buf.WriteByte('\n')
	if t.Notes != "" {
		for _, line := range strings.Split(t.Notes, "\n") {
			fmt.Fprintf(buf, "%s%s\n", strings.Repeat(" ", NotesIndent), line)
		}
	}
}

func renderMeta(t model.Task) string {
	parts := make([]string, 0, 6)
	if t.ID > 0 {
		parts = append(parts, fmt.Sprintf("id:%d", t.ID))
	}
	parts = append(parts, fmt.Sprintf("prio:%s", t.Priority))
	if t.Due != nil {
		parts = append(parts, fmt.Sprintf("due:%s", t.Due.Format(model.DateLayout)))
	}
	if len(t.Tags) > 0 {
		tags := append([]string(nil), t.Tags...)
		sort.Strings(tags)
		parts = append(parts, fmt.Sprintf("tags:%s", strings.Join(tags, ",")))
	}
	if !t.Created.IsZero() {
		parts = append(parts, fmt.Sprintf("created:%s", t.Created.Format(TimeLayout)))
	}
	if t.Completed != nil {
		parts = append(parts, fmt.Sprintf("completed:%s", t.Completed.Format(TimeLayout)))
	}
	return strings.Join(parts, " ")
}

// Resolve returns the path to the nearest .tsk.md starting at start and
// walking upward. If none is found it returns (fallback, false).
// The fallback is ~/.tsk/global.md.
func Resolve(start string) (path string, found bool) {
	if start == "" {
		if cwd, err := os.Getwd(); err == nil {
			start = cwd
		}
	}
	dir := start
	for {
		candidate := filepath.Join(dir, FileName)
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			return candidate, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(start, FileName), false
	}
	return filepath.Join(home, ".tsk", "global.md"), false
}

// ResolveOrCreate returns the nearest .tsk.md path. If none exists, it returns
// a path in cwd (never the global fallback) so `tsk init` and `tsk add` create
// in the user's working directory by default.
func ResolveOrCreate(start string) string {
	if path, ok := Resolve(start); ok {
		return path
	}
	if start == "" {
		if cwd, err := os.Getwd(); err == nil {
			start = cwd
		}
	}
	return filepath.Join(start, FileName)
}
