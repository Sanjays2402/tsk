package serve

import (
	"sort"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
)

// taskDTO is the JSON shape returned to the client. We expose plain types
// (no *time.Time, no enum int) so the SPA can render without translation.
type taskDTO struct {
	ID        int      `json:"id"`
	Title     string   `json:"title"`
	Done      bool     `json:"done"`
	Priority  string   `json:"priority"`
	Due       string   `json:"due,omitempty"` // YYYY-MM-DD or ""
	Tags      []string `json:"tags"`          // [] when none
	Notes     string   `json:"notes,omitempty"`
	Created   string   `json:"created,omitempty"`   // RFC3339
	Completed string   `json:"completed,omitempty"` // RFC3339
}

// taskInputDTO is the body shape for POST /api/tasks.
type taskInputDTO struct {
	Title    string   `json:"title"`
	Priority string   `json:"priority,omitempty"`
	Due      string   `json:"due,omitempty"` // natural language ok
	Tags     []string `json:"tags,omitempty"`
	Notes    string   `json:"notes,omitempty"`
}

// taskPatchDTO is the body shape for PATCH /api/tasks/:id. Every field is a
// pointer so omitted fields are left untouched; explicit "" for due clears it.
type taskPatchDTO struct {
	Title    *string   `json:"title,omitempty"`
	Priority *string   `json:"priority,omitempty"`
	Due      *string   `json:"due,omitempty"`
	Tags     *[]string `json:"tags,omitempty"`
	Notes    *string   `json:"notes,omitempty"`
	Done     *bool     `json:"done,omitempty"`
}

// statsDTO mirrors the CLI stats but as a stable JSON schema.
type statsDTO struct {
	Total      int           `json:"total"`
	Done       int           `json:"done"`
	Undone     int           `json:"undone"`
	Overdue    int           `json:"overdue"`
	Today      int           `json:"today"`
	Completion float64       `json:"completion"`
	Streak     int           `json:"streak"`
	TopTags    []tagCountDTO `json:"top_tags"`
}

type tagCountDTO struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

func taskToDTO(t model.Task) taskDTO {
	out := taskDTO{
		ID:       t.ID,
		Title:    t.Title,
		Done:     t.Done,
		Priority: t.Priority.String(),
		Tags:     append([]string{}, t.Tags...), // ensure non-nil for JSON
		Notes:    t.Notes,
	}
	if t.Due != nil {
		out.Due = t.Due.Format(model.DateLayout)
	}
	if !t.Created.IsZero() {
		out.Created = t.Created.Format(time.RFC3339)
	}
	if t.Completed != nil {
		out.Completed = t.Completed.Format(time.RFC3339)
	}
	return out
}

func tasksToDTO(in []model.Task) []taskDTO {
	out := make([]taskDTO, 0, len(in))
	for _, t := range in {
		out = append(out, taskToDTO(t))
	}
	return out
}

// computeStatsDTO mirrors the CLI's computeStats logic but emits a JSON-ready
// shape. Kept local to avoid an import cycle with internal/commands.
func computeStatsDTO(tasks []model.Task, now time.Time) statsDTO {
	var s statsDTO
	s.Total = len(tasks)
	s.TopTags = []tagCountDTO{}
	tagMap := map[string]int{}
	for _, t := range tasks {
		if t.Done {
			s.Done++
		} else {
			s.Undone++
		}
		if t.IsOverdue(now) {
			s.Overdue++
		}
		if t.IsDueToday(now) {
			s.Today++
		}
		for _, tag := range t.Tags {
			tagMap[tag]++
		}
	}
	if s.Total > 0 {
		s.Completion = float64(s.Done) / float64(s.Total) * 100
	}
	s.Streak = currentStreakDTO(tasks, now)

	tags := make([]tagCountDTO, 0, len(tagMap))
	for k, v := range tagMap {
		tags = append(tags, tagCountDTO{Tag: k, Count: v})
	}
	sort.Slice(tags, func(i, j int) bool {
		if tags[i].Count != tags[j].Count {
			return tags[i].Count > tags[j].Count
		}
		return tags[i].Tag < tags[j].Tag
	})
	if len(tags) > 5 {
		tags = tags[:5]
	}
	s.TopTags = tags
	return s
}

func currentStreakDTO(tasks []model.Task, now time.Time) int {
	days := map[string]bool{}
	for _, t := range tasks {
		if !t.Done || t.Completed == nil {
			continue
		}
		days[t.Completed.Format(model.DateLayout)] = true
	}
	streak := 0
	cur := now
	for {
		if days[cur.Format(model.DateLayout)] {
			streak++
			cur = cur.AddDate(0, 0, -1)
			continue
		}
		if streak == 0 && cur.Format(model.DateLayout) == now.Format(model.DateLayout) {
			cur = cur.AddDate(0, 0, -1)
			continue
		}
		break
	}
	return streak
}
