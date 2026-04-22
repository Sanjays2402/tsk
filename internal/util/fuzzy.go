package util

import (
	"sort"
	"strings"

	"github.com/sahilm/fuzzy"
)

// Searchable is the interface for types indexed by Fuzzy.
type Searchable interface {
	SearchKey() string
}

// Match is a fuzzy-match result containing the original index and score.
type Match struct {
	Index int
	Score int
}

// Fuzzy returns the indices of items whose search keys match the query, ordered by score.
func Fuzzy(items []string, query string) []Match {
	query = strings.TrimSpace(query)
	if query == "" {
		out := make([]Match, len(items))
		for i := range items {
			out[i] = Match{Index: i}
		}
		return out
	}
	matches := fuzzy.Find(query, items)
	out := make([]Match, 0, len(matches))
	for _, m := range matches {
		out = append(out, Match{Index: m.Index, Score: m.Score})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}
