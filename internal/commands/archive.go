package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
	"github.com/spf13/cobra"
)

// archiveFileName is the sibling file that receives archived tasks. It lives
// in the same directory as the active .tsk.md so it travels with the project.
const archiveFileName = ".tsk.archive.md"

// archiveHeader is the one-line header written when the archive file is
// created for the first time.
const archiveHeader = "# tsk archive\n"

func newArchiveCmd() *cobra.Command {
	var (
		olderThan string
		sinceID   int
		all       bool
		dryRun    bool
		strategy  string
		mergeInto string
		bucketBy  string
		strictAnd bool
		asJSON    bool
	)
	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Move completed tasks to a sibling .tsk.archive.md file",
		Long: "Move Done tasks out of the active .tsk.md and into a sibling .tsk.archive.md.\n" +
			"Archived tasks get fresh sequential IDs in the archive file, continuing\n" +
			"from the archive's existing max ID. Active task IDs do not change.\n\n" +
			"--bucket-by selects a USER-DEFINED bucket axis instead of the time/id\n" +
			"axis --strategy uses. Mutually exclusive with --strategy (each defines\n" +
			"a different bucket axis). Supported keys:\n" +
			"  priority      one section per priority (urgent/high/medium/low). Sorted\n" +
			"                descending so urgent sections come first.\n" +
			"  tag           one section per FIRST tag of each archived task; untagged\n" +
			"                tasks fall into '## untagged'. One-task-one-bucket — picking\n" +
			"                the first tag is the most predictable interpretation when a\n" +
			"                task has multiple tags.\n" +
			"  tag:X         boolean partition: tasks tagged X land in '## tag:X' (sorted\n" +
			"                first), everything else lands in '## other'. Case-insensitive\n" +
			"                tag match. Use when you want to call out ONE tag in the\n" +
			"                archive without scattering into one section per distinct tag.\n" +
			"  tag:X,Y,Z     CSV variant of tag:X: tasks tagged ANY of X/Y/Z land in\n" +
			"                '## tag:X,Y,Z', everything else in '## other'. Same union\n" +
			"                semantics as `tsk graph --highlight-tag a,b`. Useful when\n" +
			"                you want to call out a logical SLICE of the archive (\"show\n" +
			"                what shipped tagged release OR p0\") without listing tags\n" +
			"                one-by-one or generating a section per distinct tag.\n" +
			"  tag:!X        INVERSE of tag:X: tasks NOT tagged X land in '## tag:!X',\n" +
			"                tasks tagged X land in '## other'. Useful for call-out\n" +
			"                slices like \"everything that wasn't tagged 'release'\" —\n" +
			"                same boolean-partition contract as tag:X, but flipped.\n" +
			"                Combines with the CSV form (\"tag:!a,!b\" for the NOT-ANY-OF\n" +
			"                variant); all tags in the CSV must share the same\n" +
			"                inversion sense (no mixed \"tag:!a,b\" — pick one direction).\n" +
			"  --strict-and  (for CSV-tag forms) flip the default union (any-of) to\n" +
			"                intersection (all-of). A task lands in the call-out\n" +
			"                bucket only if it carries ALL listed tags. The bucket\n" +
			"                label gains a '&' marker (\"## tag:&a,b\" for positive,\n" +
			"                \"## tag:!&a,b\" for inverse) so flat-text scans can\n" +
			"                distinguish union from intersection sections. Has no\n" +
			"                effect on single-tag forms (one tag has no union/\n" +
			"                intersection distinction).\n" +
			"  id-range:N    fixed-width id windows of size N: '1-N', 'N+1-2N', …\n" +
			"                Useful when id order doubles as creation order — sister\n" +
			"                of priority/tag for the id axis. N must be a positive\n" +
			"                integer. Tasks with id 0 (legacy ID-less tasks) collapse\n" +
			"                into '## id:0'.\n\n" +
			"--since-id <N> selects by ID instead of time: archive every Done task\n" +
			"with id < N. The 'id-axis' sister of --older-than's time-axis cutoff —\n" +
			"useful when you want to clean up a legacy block of work (e.g. 'everything\n" +
			"older than the v2 refactor at id #150') without worrying about completion\n" +
			"timestamps. Combines with --strategy and --merge-into the same way every\n" +
			"other selector does. Mutually exclusive with --all (intent overlap) and\n" +
			"with --older-than (two different axes; pick one to keep the selection\n" +
			"crisp). Tasks WITHOUT a Completed timestamp still qualify if their id\n" +
			"is below the cutoff — the whole point of an id-axis is to skip the\n" +
			"time check entirely.\n\n" +
			"--merge-into <file> writes to a non-default archive file instead of the\n" +
			"sibling .tsk.archive.md. Useful for per-project rollups (e.g. a yearly or\n" +
			"per-team archive that several .tsk.md files feed into). The target file is\n" +
			"created if missing, with the same '# tsk archive' header used by the\n" +
			"default sibling. Subsequent --merge-into calls to the same file continue\n" +
			"its id space (max+1, same as the default sibling). Bucketed strategies\n" +
			"layer correctly on top of merge-into so a 'tsk archive --strategy weekly\n" +
			"--merge-into ~/work.archive.md' run works.\n\n" +
			"--strategy controls how archived tasks are grouped inside the archive file:\n" +
			"  flat (default)  one growing list in the order they were archived\n" +
			"  daily           group into '## YYYY-MM-DD' sections by completion date,\n" +
			"                  in the task's recorded time zone. Finer-grained sibling\n" +
			"                  of weekly/monthly — useful when the archive churns and\n" +
			"                  you want day-level resolution for end-of-day rollups.\n" +
			"                  Same '## undated' bucket policy as weekly/monthly.\n" +
			"  weekly          group into '## YYYY-W##' sections by completion week (ISO),\n" +
			"                  so old archives have a scannable timeline. Tasks without a\n" +
			"                  Completed timestamp fall into '## undated' so they're not lost.\n" +
			"  monthly         group into '## YYYY-MM' sections by completion month — coarser\n" +
			"                  scannability for stores that accumulate hundreds of archived\n" +
			"                  rows. Same '## undated' bucket policy as weekly.\n" +
			"  quarterly       group into '## YYYY-Q#' sections (Q1=Jan-Mar, Q2=Apr-Jun,\n" +
			"                  Q3=Jul-Sep, Q4=Oct-Dec) — fiscal-quarter scannability for\n" +
			"                  retro reviews. Same '## undated' bucket policy.\n" +
			"  yearly          group into '## YYYY' sections — coarsest scannability for\n" +
			"                  multi-year stores where even quarterly produces too many\n" +
			"                  sections. Same '## undated' bucket policy.\n\n" +
			"daily/weekly/monthly/quarterly/yearly is purely a layout choice — IDs, content,\n" +
			"and round-trip semantics are identical; switching strategies later just changes\n" +
			"how the NEXT batch is placed in the file. Existing archive contents are\n" +
			"preserved verbatim (no re-bucketing of historical data).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			strategy = strings.ToLower(strings.TrimSpace(strategy))
			switch strategy {
			case "", "flat":
				strategy = "flat"
			case "daily":
				// ok
			case "weekly":
				// ok
			case "monthly":
				// ok
			case "quarterly":
				// ok
			case "yearly":
				// ok
			default:
				return usageErrorf("unknown --strategy %q (want flat, daily, weekly, monthly, quarterly, or yearly)", strategy)
			}
			// --bucket-by is a user-supplied non-time/id axis (e.g.
			// priority, tag) that's mutually exclusive with
			// --strategy: the two answer different organization
			// questions (when did it happen vs. what category).
			// Combining them would muddle the layout contract.
			bucketBy = strings.TrimSpace(bucketBy)
			if bucketBy != "" && strategy != "flat" {
				return usageErrorf("--bucket-by and --strategy are mutually exclusive (each defines a different bucket axis)")
			}
			// --strict-and is meaningful ONLY in combination with
			// the CSV-tag forms ("tag:X,Y" / "tag:!X,!Y"). For
			// every other bucket axis (priority, single-tag,
			// id-range, or no bucket-by at all) the flag has no
			// applicable semantic — flagging it loudly is the
			// right call so the user doesn't think it silently
			// changed something.
			if strictAnd {
				if bucketBy == "" {
					return usageErrorf("--strict-and requires --bucket-by tag:X,Y (CSV-tag variant); got no --bucket-by")
				}
				if !strings.HasPrefix(strings.ToLower(bucketBy), "tag:") {
					return usageErrorf("--strict-and only applies to --bucket-by tag:X,Y (CSV-tag variant); got --bucket-by=%q", bucketBy)
				}
			}
			bucketFunc, err := resolveBucketByKey(bucketBy, strictAnd)
			if err != nil {
				return err
			}
			// --since-id is mutually exclusive with --all and --older-than:
			// each is a different selection axis (id, all-of-them, time)
			// and combining them would muddle the contract.
			if sinceID > 0 {
				if all {
					return usageErrorf("--since-id and --all are mutually exclusive (each picks a different selection axis)")
				}
				// Detect an EXPLICIT --older-than: the default
				// value is "30d", so we can't just check for
				// non-empty. cobra's Changed() check is the
				// idiomatic "did the user pass this flag?"
				if cmd.Flags().Changed("older-than") {
					return usageErrorf("--since-id and --older-than are mutually exclusive (id-axis vs time-axis)")
				}
			}
			if sinceID < 0 {
				return usageErrorf("--since-id must be positive, got %d", sinceID)
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}

			cutoff, useCutoff, err := resolveCutoff(olderThan, all)
			if err != nil {
				return err
			}

			pred := func(t model.Task) bool {
				if !t.Done {
					return false
				}
				if sinceID > 0 {
					// id-axis: archive every Done task with id < cutoff.
					// Time check skipped entirely (the whole point of
					// --since-id).
					return t.ID < sinceID
				}
				if !useCutoff {
					return true // --all
				}
				// No completion timestamp on a done task → can't prove age,
				// be conservative and skip.
				if t.Completed == nil {
					return false
				}
				return t.Completed.Before(cutoff)
			}
			kept, archived := s.Partition(pred)

			out := cmd.OutOrStdout()
			archivePath, err := resolveArchivePath(s, mergeInto)
			if err != nil {
				return err
			}

			if len(archived) == 0 {
				if asJSON {
					return emitArchiveJSON(out, archivePath, strategy, bucketBy, strictAnd, dryRun, nil, kept, archived, nil)
				}
				pf(out, "no tasks to archive\n")
				return nil
			}

			if dryRun {
				if asJSON {
					// Dry-run JSON: simulate the archive-id assignment
					// so the envelope mirrors the real-run shape, but
					// don't touch disk. The archive store is loaded
					// only to read its current max id; nothing is
					// written.
					nextSim := 1
					if archForRead, loadErr := store.Load(archivePath); loadErr == nil {
						nextSim = maxTaskID(archForRead.Tasks) + 1
					}
					activeIDsSim := make([]int, len(archived))
					simulated := make([]model.Task, len(archived))
					for i, t := range archived {
						copyT := t
						activeIDsSim[i] = copyT.ID
						copyT.ID = nextSim + i
						simulated[i] = copyT
					}
					return emitArchiveJSON(out, archivePath, strategy, bucketBy, strictAnd, dryRun, bucketFunc, kept, simulated, activeIDsSim)
				}
				summary := strategy
				if bucketBy != "" {
					summary = fmt.Sprintf("bucket-by=%s", bucketBy)
					if strictAnd {
						summary += " (strict-and)"
					}
				}
				pf(out, "would archive %d task(s) → %s (%s)\n", len(archived), archivePath, summary)
				for _, t := range archived {
					pf(out, "  #%d %s\n", t.ID, t.Title)
				}
				return nil
			}

			// Load (or initialize) the archive file and continue IDs from
			// its current max.
			arch, err := store.Load(archivePath)
			if err != nil {
				return fmt.Errorf("load archive: %w", err)
			}
			if _, statErr := os.Stat(archivePath); os.IsNotExist(statErr) {
				arch.Header = archiveHeader
			}
			next := maxTaskID(arch.Tasks) + 1
			// Capture the active-store ids BEFORE the loop rewrites
			// them to archive ids — the JSON envelope needs both
			// (the user's mental model is "task #N became archive
			// #M") and we'd lose the active id otherwise.
			activeIDs := make([]int, len(archived))
			for i := range archived {
				activeIDs[i] = archived[i].ID
				archived[i].ID = next
				next++
			}
			switch strategy {
			case "weekly":
				if err := writeBucketedArchive(archivePath, arch, archived, bucketByISOWeek); err != nil {
					return fmt.Errorf("save weekly archive: %w", err)
				}
			case "monthly":
				if err := writeBucketedArchive(archivePath, arch, archived, bucketByMonth); err != nil {
					return fmt.Errorf("save monthly archive: %w", err)
				}
			case "daily":
				if err := writeBucketedArchive(archivePath, arch, archived, bucketByDay); err != nil {
					return fmt.Errorf("save daily archive: %w", err)
				}
			case "quarterly":
				if err := writeBucketedArchive(archivePath, arch, archived, bucketByQuarter); err != nil {
					return fmt.Errorf("save quarterly archive: %w", err)
				}
			case "yearly":
				if err := writeBucketedArchive(archivePath, arch, archived, bucketByYear); err != nil {
					return fmt.Errorf("save yearly archive: %w", err)
				}
			default:
				if bucketFunc != nil {
					if err := writeBucketedArchive(archivePath, arch, archived, bucketFunc); err != nil {
						return fmt.Errorf("save bucket-by archive: %w", err)
					}
					break
				}
				arch.ReplaceTasks(append(arch.Tasks, archived...))
				if err := arch.Save(); err != nil {
					return fmt.Errorf("save archive: %w", err)
				}
			}

			s.ReplaceTasks(kept)
			if err := s.Save(); err != nil {
				return fmt.Errorf("save active: %w", err)
			}

			if asJSON {
				return emitArchiveJSON(out, archivePath, strategy, bucketBy, strictAnd, dryRun, bucketFunc, kept, archived, activeIDs)
			}
			pf(out, "archived %d task(s) → %s (strategy=%s)\n", len(archived), archivePath, archiveStrategyLabel(strategy, bucketBy, strictAnd))
			pf(out, "active tasks: %d\n", len(kept))
			return nil
		},
	}
	cmd.Flags().StringVar(&olderThan, "older-than", "30d", "only archive tasks completed more than this ago (e.g. 7d, 2w, 1m)")
	cmd.Flags().IntVar(&sinceID, "since-id", 0, "archive every Done task with id < N (id-axis cutoff; sister of --older-than)")
	cmd.Flags().BoolVar(&all, "all", false, "archive every Done task regardless of age")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be archived without changing files")
	cmd.Flags().StringVar(&strategy, "strategy", "flat", "archive layout: flat | daily | weekly | monthly | quarterly | yearly (one bucket per calendar year)")
	cmd.Flags().StringVar(&mergeInto, "merge-into", "", "write to this archive file instead of the sibling .tsk.archive.md (~ expansion supported; created if missing)")
	cmd.Flags().StringVar(&bucketBy, "bucket-by", "", "user-supplied bucket axis: 'priority', 'tag', 'tag:X' (boolean partition by single tag), 'tag:!X' (inverse single-tag), 'tag:X,Y,Z' (multi-tag CSV union), 'tag:!X,!Y' (inverse CSV), or 'id-range:N' (fixed-width id windows). Mutually exclusive with --strategy.")
	cmd.Flags().BoolVar(&strictAnd, "strict-and", false, "for --bucket-by tag:X,Y (CSV-tag variant): require ALL listed tags on a task (intersection) instead of the default ANY (union). Combines with the inverse form (tag:!X,!Y --strict-and = NOT carrying ALL listed tags). The bucket label gains a '&' marker so flat-text archive scans can distinguish union vs intersection sections.")
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit a stable JSON envelope describing the archive run (which tasks landed in which buckets, per-task archive id assignments, the resolved archive path, and the strategy/bucket-by summary). Works with --dry-run too — the dry-run JSON simulates the archive-id assignment without writing anything. Useful for scripted CI gates and post-archive notifications that need a machine-readable manifest rather than parsing the plain-text summary.")
	return cmd
}

// archiveStrategyLabel formats the summary label for the success
// message: "strategy=X" for the standard time/id bucketing, or
// "bucket-by=Y" when the user opted into a custom axis. Single
// helper so the dry-run and success paths print the same shape.
//
// strictAnd appends "(strict-and)" when the user opted into the
// intersection (all-of) form for a CSV-tag bucket-by — keeps the
// summary self-documenting so scripts watching the output can tell
// union from intersection without checking the user's exact flags.
func archiveStrategyLabel(strategy, bucketBy string, strictAnd bool) string {
	if bucketBy != "" {
		s := fmt.Sprintf("bucket-by=%s", bucketBy)
		if strictAnd {
			s += " (strict-and)"
		}
		return s
	}
	return strategy
}

// resolveCutoff returns the time before which a task is "old enough" to
// archive, plus whether the cutoff should be applied at all (--all bypasses
// it). With --all set, useCutoff is false. Otherwise the duration string is
// parsed and subtracted from time.Now().
func resolveCutoff(olderThan string, all bool) (cutoff time.Time, useCutoff bool, err error) {
	if all {
		return time.Time{}, false, nil
	}
	d, err := store.ParseDuration(olderThan)
	if err != nil {
		return time.Time{}, false, usageErrorf("%s", err.Error())
	}
	return time.Now().Add(-d), true, nil
}

func maxTaskID(tasks []model.Task) int {
	m := 0
	for _, t := range tasks {
		if t.ID > m {
			m = t.ID
		}
	}
	return m
}

// resolveArchivePath picks the archive file to write to. The default
// (mergeInto == "") is the sibling .tsk.archive.md alongside the
// active .tsk.md — the long-standing tsk behaviour. A non-empty
// mergeInto overrides with a user-supplied path so several projects
// can roll into one shared archive.
//
// Path handling:
//   - "~" expansion via os.UserHomeDir (the standard go-stdlib
//     pattern; cobra doesn't auto-expand)
//   - relative paths resolve against the active store's directory
//     so a typo like "archive.md" doesn't accidentally write to the
//     CWD when the user clearly meant "alongside my .tsk.md"
//   - absolute paths pass through unchanged
//
// Validation: the target must not be the same path as the active
// store (would be self-archiving — the function would read the file,
// then overwrite it with the merged result, corrupting the active
// .tsk.md). Surfaced as a usage error so the user immediately
// understands the bug. Comparing canonical absolute paths is the
// only safe way (symlinks, relative ~. shenanigans).
//
// Why not normalize to the directory containing the active store
// for ALL relative paths? Because users typing
// "--merge-into team.archive.md" inside a project-local context
// almost always mean "in this project's dir, not my CWD". When
// they DO mean CWD, they can prefix "./" — explicit beats implicit
// for filesystem writes.
func resolveArchivePath(s *store.Store, mergeInto string) (string, error) {
	if mergeInto == "" {
		return filepath.Join(filepath.Dir(s.Path), archiveFileName), nil
	}
	expanded := mergeInto
	if strings.HasPrefix(expanded, "~/") || expanded == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("--merge-into: cannot expand ~: %w", err)
		}
		if expanded == "~" {
			expanded = home
		} else {
			expanded = filepath.Join(home, expanded[2:])
		}
	}
	if !filepath.IsAbs(expanded) {
		expanded = filepath.Join(filepath.Dir(s.Path), expanded)
	}
	// Refuse to archive into the active store — would corrupt the
	// active file on the second pass.
	srcAbs, _ := filepath.Abs(s.Path)
	dstAbs, _ := filepath.Abs(expanded)
	if srcAbs != "" && srcAbs == dstAbs {
		return "", usageErrorf("--merge-into %q resolves to the active store at %s; pick a different file", mergeInto, s.Path)
	}
	return expanded, nil
}

// bucketFn computes the section header key (e.g. "2026-W12" or
// "2026-05") and a stable sort key for a completed task. Tasks
// without a Completed timestamp return ("undated", 0) and are
// pushed to the tail of the section list. Pure function: same
// input → same output, no side effects.
type bucketFn func(t model.Task) (key string, sortKey int)

// bucketByISOWeek groups tasks by ISO week (Mon-first, year-
// boundary-safe via time.ISOWeek). The sortKey is year*100+week so
// chronological ascending sort works lexicographically too.
func bucketByISOWeek(t model.Task) (string, int) {
	if t.Completed == nil {
		return "undated", 0
	}
	y, w := t.Completed.ISOWeek()
	return fmt.Sprintf("%04d-W%02d", y, w), y*100 + w
}

// bucketByMonth groups tasks by calendar month (the local time
// zone of the Completed timestamp). ISO weeks split across month
// boundaries, so a sister "monthly" grouping is the natural
// coarser-grained sibling: scannable for stores that accumulate
// hundreds of archived rows over years. Year-boundary safe (Dec
// 2025 sorts before Jan 2026 because 202512 < 202601).
func bucketByMonth(t model.Task) (string, int) {
	if t.Completed == nil {
		return "undated", 0
	}
	y, m, _ := t.Completed.Date()
	return fmt.Sprintf("%04d-%02d", y, int(m)), y*100 + int(m)
}

// bucketByDay groups tasks by calendar day (in the time zone of
// the Completed timestamp). The finest-grained sibling of weekly
// and monthly — useful when a project archives multiple times a
// day or when you want day-by-day rollups (end-of-day summaries,
// daily standup logs). Section header is the same ISO date format
// the rest of tsk uses (model.DateLayout = "2006-01-02") so
// scanning is consistent with `tsk ls --due`, `tsk show`, etc.
//
// SortKey is year*10000 + month*100 + day — lexicographically
// safe (Dec 31 2025 = 20251231 < Jan 1 2026 = 20260101) and
// fits easily in int (year 2147 max). Year-boundary safe by
// construction.
//
// Why a separate sortKey rather than a string compare on the
// "YYYY-MM-DD" key? Two reasons: keeps the bucketFn contract
// (returning an int) consistent with bucketByISOWeek's "year*100+
// week" math, and avoids any subtle locale-string-compare gotchas
// that could surface if a future tweak to the key format breaks
// strict ASCII ordering.
func bucketByDay(t model.Task) (string, int) {
	if t.Completed == nil {
		return "undated", 0
	}
	y, m, d := t.Completed.Date()
	return fmt.Sprintf("%04d-%02d-%02d", y, int(m), d), y*10000 + int(m)*100 + d
}

// bucketByQuarter groups tasks by fiscal quarter (Q1 = Jan-Mar, Q2 =
// Apr-Jun, Q3 = Jul-Sep, Q4 = Oct-Dec) in the time zone of the
// Completed timestamp. The coarsest practical sibling of monthly:
// useful for retro reviews ("what did we ship last quarter?") and
// for stores that accumulate years of completed work where monthly
// would still be too many sections to scan.
//
// Section header is "YYYY-Q#" (e.g. "2026-Q2") — keeps the leading
// year so chronological grep/scan still works the same way as the
// other bucketed strategies. SortKey is year*10+quarter so 2025-Q4
// (20254) sorts before 2026-Q1 (20261); fits trivially in int.
//
// Quarter math: monthInt is 1..12, (monthInt-1)/3 gives 0..3, +1
// gives Q1..Q4. The boundary cases (March -> Q1, April -> Q2,
// December -> Q4) are covered by TestBucketByQuarterBoundaries.
func bucketByQuarter(t model.Task) (string, int) {
	if t.Completed == nil {
		return "undated", 0
	}
	y, m, _ := t.Completed.Date()
	q := (int(m)-1)/3 + 1
	return fmt.Sprintf("%04d-Q%d", y, q), y*10 + q
}

// bucketByYear groups tasks by calendar year (in the time zone of
// the Completed timestamp). The coarsest sibling of the bucketed
// strategies: one section per calendar year. Useful for multi-year
// stores where even quarterly produces too many sections — e.g. a
// long-running personal task file that wants "what did I get done
// in 2024?" / "what about 2025?" navigation.
//
// Section header is "YYYY" — no decoration, since the year IS the
// bucket key. SortKey is the year itself (already int, already
// sorts chronologically). Year-boundary safety is trivial because
// the bucket boundary IS the year.
func bucketByYear(t model.Task) (string, int) {
	if t.Completed == nil {
		return "undated", 0
	}
	y, _, _ := t.Completed.Date()
	return fmt.Sprintf("%04d", y), y
}

// bucketByPriority groups tasks by their priority value: "urgent",
// "high", "medium", "low". Useful for project-rollup archives where
// you want sections summarizing the IMPORTANCE of what was shipped,
// not when. Priority order is enforced via the sort key so sections
// render in descending priority (urgent first), matching `tsk ls`
// ordering conventions.
//
// sortKey: the bucket emitter sorts ASCENDING, so we INVERT the
// numeric priority to push higher-importance sections to the top.
// urgent=1, high=2, medium=3, low=4. Matches the Priority enum
// ordering elsewhere in the codebase (model.Priority) — just
// inverted at the bucketing layer for human-friendly section order.
func bucketByPriority(t model.Task) (string, int) {
	switch t.Priority {
	case model.PriorityUrgent:
		return "urgent", 1
	case model.PriorityHigh:
		return "high", 2
	case model.PriorityMedium:
		return "medium", 3
	case model.PriorityLow:
		return "low", 4
	}
	// Unreachable for parsed tasks (priority defaults to medium on
	// load) — kept as a safety net so a future enum variant doesn't
	// produce a silent empty key.
	return "unknown", 99
}

// bucketByFirstTag groups tasks by their FIRST tag (in declaration
// order). Useful for per-project / per-context rollups: archive
// tagged "work" lands under "work", "personal" under "personal",
// untagged lands under "untagged" so nothing is lost.
//
// Why first tag rather than every tag (which would duplicate the
// task across multiple sections)? Because the archive bucketing
// contract is one-task-one-bucket. Cross-listing would multiply
// task counts and break id uniqueness inside the archive. Picking
// the first tag is the most predictable interpretation: it's the
// PRIMARY label the user gave the task.
//
// sortKey: tags don't have an intrinsic numeric order, so we use
// 0 for all — writeBucketedArchive falls back to lexicographic
// sort on the key when the sortKeys tie. Untagged sorts after
// every tagged section via the "untagged" key vs. real names
// (writeBucketedArchive's sort is stable; "untagged" is
// alphabetically late among typical English tags but a
// deterministic sort puts the user-named tags first regardless
// of where 'u' falls).
func bucketByFirstTag(t model.Task) (string, int) {
	if len(t.Tags) == 0 {
		return "untagged", 0
	}
	return t.Tags[0], 0
}

// resolveBucketByKey turns the user-supplied --bucket-by value into
// the corresponding bucketFn. Supported keys:
//
//	""             no --bucket-by — returns nil (caller falls through
//	               to the default flat or strategy switch path)
//	"priority"     priority sections (urgent/high/medium/low/none)
//	"tag"          sections keyed off the first tag of each task
//	"tag:X"        boolean partition: tasks tagged X go in "tag:X",
//	               tasks NOT tagged X go in "other"
//	"id-range:N"   sections of N ids each ("1-N", "N+1-2N", …)
//
// Empty/whitespace is treated as the no-op no-flag path. Unknown
// keys surface a usage error with the supported list so the user
// can fix the typo quickly.
//
// "tag:X" is the SINGLE-tag boolean partition — different from
// "tag" (which sections by every first-tag value). Useful when the
// user wants to highlight ONE tag in the archive (e.g. "show me
// what I shipped tagged 'release' vs everything else") without
// generating a section per distinct tag in the batch. Tag matching
// is case-insensitive, same convention as `tsk ls --tag`.
//
// "id-range:N" is the id-axis sister of priority/tag bucketing:
// archived tasks are grouped into fixed-width id windows ("1-50",
// "51-100", "101-150", …). Useful for project-rollup archives where
// the id sequence reflects chronological order of creation — id-range
// bucketing then doubles as a coarse timeline without requiring
// completion timestamps. The bucket label sorts naturally (1-50 < 51-100)
// because we use the WINDOW START as the sort key.
//
// Range size must be positive. Zero / negative / non-numeric N is
// rejected with a usage error pointing at the expected shape.
// Tasks with id == 0 (ID-less tasks created before the model gained
// id assignment) all collapse into "id:0" so they're not lost.
//
// strictAnd flips the multi-tag CSV semantic from UNION (any-of) to
// INTERSECTION (all-of): when set, a task lands in the call-out
// bucket only if it carries ALL listed tags. Default is the
// historical union behavior so existing recipes keep working. Has
// no effect on the single-tag form (one tag has no union/
// intersection distinction). The caller validates that strictAnd
// is only set when bucketBy starts with "tag:".
func resolveBucketByKey(raw string, strictAnd bool) (bucketFn, error) {
	trimmed := strings.TrimSpace(raw)
	lower := strings.ToLower(trimmed)
	switch lower {
	case "":
		return nil, nil
	case "priority", "prio":
		return bucketByPriority, nil
	case "tag", "tags":
		return bucketByFirstTag, nil
	}
	// "id-range:N" — variable parameter, parsed below.
	if strings.HasPrefix(lower, "id-range:") {
		spec := trimmed[len("id-range:"):]
		spec = strings.TrimSpace(spec)
		if spec == "" {
			return nil, usageErrorf("--bucket-by id-range:N requires a positive window size, got %q", raw)
		}
		n, err := strconv.Atoi(spec)
		if err != nil {
			return nil, usageErrorf("--bucket-by id-range:N requires an integer N, got %q", raw)
		}
		if n <= 0 {
			return nil, usageErrorf("--bucket-by id-range:N requires N > 0, got %d", n)
		}
		return makeIDRangeBucketFn(n), nil
	}
	// "tag:X" or "tag:X,Y,Z" — single-tag boolean partition or
	// multi-tag CSV union partition. Both share the same render
	// shape ("tag:<label>" vs "other"); single-tag is just the
	// CSV variant with len(tags)==1.
	//
	// "tag:!X" — INVERSE single-tag partition. The leading "!" on
	// the payload flips the predicate: tasks NOT tagged X land in
	// the call-out bucket ("## tag:!X"), tasks tagged X land in
	// "## other". Useful when you want to call out everything
	// EXCEPT a specific tag — e.g. "show me what shipped that
	// wasn't tagged 'release'" or "scaffolding tasks vs the rest".
	// The "!" prefix is also supported in CSV form ("tag:!X,Y" or
	// "tag:!X,!Y") — each "!" applies to its own tag, so
	// "tag:!X,Y" matches tasks NOT tagged X OR tagged Y.
	if strings.HasPrefix(lower, "tag:") {
		// Slice off the "tag:" prefix from the ORIGINAL (case-
		// preserving) string so tag values keep their case in
		// the bucket label. We only normalized `lower` for the
		// prefix check; the actual tag value is parsed from
		// `trimmed`.
		tagPayload := trimmed[len("tag:"):]
		tagPayload = strings.TrimSpace(tagPayload)
		if tagPayload == "" {
			return nil, usageErrorf("--bucket-by tag:X requires a tag name, got %q", raw)
		}
		tags := splitTagFilterCSV(tagPayload)
		if len(tags) == 0 {
			return nil, usageErrorf("--bucket-by tag:X requires at least one non-empty tag name, got %q", raw)
		}
		// Parse "!" prefixes — a leading "!" on a tag inverts the
		// match for that tag. We accept the inverse form in both
		// single ("tag:!work") and multi-tag CSV ("tag:!a,!b,c")
		// shapes; the inversion is per-tag, NOT a global flip on
		// the bucket. To keep semantics crisp, ALL tags must
		// share the same inversion sense: "tag:!a,b" (mixed) is
		// rejected as a usage error so the user has to commit to
		// one direction.
		inverted, parsedTags, err := parseInversionTags(tags, raw)
		if err != nil {
			return nil, err
		}
		return makeTagFilterBucketFnInversionMode(parsedTags, inverted, strictAnd), nil
	}
	return nil, usageErrorf("unknown --bucket-by %q (supported: priority, tag, tag:X, tag:X,Y, tag:!X, id-range:N)", raw)
}

// parseInversionTags inspects the per-tag list for leading "!"
// prefixes and returns the cleaned tag names along with a single
// boolean indicating whether the partition is inverted.
//
// Rule: ALL tags in the list must share the same inversion sense.
// Mixed inputs ("tag:!a,b") are rejected — silently picking one
// side would surprise the user, and there isn't a clean intended
// semantic for "NOT a OR b" inside the boolean-partition contract
// (the call-out bucket has exactly two outcomes; mixing predicates
// breaks the symmetry). If the user really wants "NOT a OR b"
// they can express it via two separate archive calls or a future
// expression DSL.
//
// All-inverted (every tag prefixed with "!") returns (true, [a,b,…]).
// All-positive returns (false, [a,b,…]). Empty inversion sense
// (no "!" anywhere) is the original union behavior — backward-
// compatible with every existing "tag:X" and "tag:X,Y" recipe.
func parseInversionTags(tags []string, raw string) (bool, []string, error) {
	if len(tags) == 0 {
		return false, nil, usageErrorf("--bucket-by tag:X requires at least one non-empty tag name, got %q", raw)
	}
	inverted := strings.HasPrefix(tags[0], "!")
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		hasBang := strings.HasPrefix(t, "!")
		if hasBang != inverted {
			return false, nil, usageErrorf("--bucket-by tag:!X — all tags must share the same inversion sense; got mixed %q (express as two separate archive calls if you need NOT a OR b)", raw)
		}
		clean := t
		if hasBang {
			clean = strings.TrimSpace(strings.TrimPrefix(t, "!"))
			if clean == "" {
				return false, nil, usageErrorf("--bucket-by tag:!X requires a tag name after \"!\", got %q", raw)
			}
		}
		out = append(out, clean)
	}
	return inverted, out, nil
}

// makeTagFilterBucketFnInversion is the bucketFn factory for the
// inversion-aware boolean partition. When `inverted` is true, the
// predicate is flipped: tasks that DO NOT carry any of the listed
// tags land in the call-out bucket; tasks that DO land in "other".
// When false, behaves identically to makeTagFilterBucketFn (which
// is now a thin wrapper around this one for backward compat).
//
// Label preserves the user's case + ordering AND surfaces the
// inversion via a "!" prefix: "tag:!work" for single-tag inverse,
// "tag:!a,!b" for multi-tag inverse. The label is the SOURCE of
// the bucket identity, so the inverse form is visibly different
// from the positive form even in flat-text archive scans.
//
// Always emits UNION (any-of) semantics for multi-tag inputs. For
// the intersection (all-of) variant see
// makeTagFilterBucketFnInversionMode, which exposes the strictAnd
// flag.
func makeTagFilterBucketFnInversion(tags []string, inverted bool) bucketFn {
	return makeTagFilterBucketFnInversionMode(tags, inverted, false)
}

// makeTagFilterBucketFnInversionMode is the fully-parameterized
// factory: both the inversion sense AND the union/intersection
// mode are caller-controlled. When `strictAnd` is true, multi-tag
// predicates require ALL listed tags to be present on the task
// (intersection); false uses the historical union (any-of)
// behavior. Has no effect on single-tag inputs (one tag has no
// union/intersection distinction).
//
// Label decoration: the "& " marker after "tag:" announces the
// intersection mode visually so flat-text archive scans can
// distinguish "## tag:a,b" (union) from "## tag:&a,b"
// (intersection). The marker is dropped for single-tag inputs
// where the mode is moot.
//
// Why label-encode the mode? Because the same archive file can
// hold rollups from different filter passes over time — without
// a visible distinction, someone reviewing the archive months
// later can't tell whether a "tag:a,b" section was the union or
// the intersection. The label lets the bucket carry its own
// provenance.
//
// Combines cleanly with inversion: "tag:!&a,b" is the inverse
// intersection (tasks NOT carrying ALL of a AND b), and the
// label preserves the "!&" prefix shape so the four combinations
// (positive union, positive intersection, inverse union, inverse
// intersection) are all visually distinct.
func makeTagFilterBucketFnInversionMode(tags []string, inverted, strictAnd bool) bucketFn {
	wantLower := make(map[string]bool, len(tags))
	for _, t := range tags {
		wantLower[strings.ToLower(t)] = true
	}
	// The intersection marker is meaningful only for multi-tag
	// predicates; a single-tag list has no union/intersection
	// distinction, so we drop the "&" to avoid label noise.
	useAnd := strictAnd && len(tags) > 1
	var label string
	prefix := "tag:"
	switch {
	case inverted && useAnd:
		prefix = "tag:!&"
	case inverted:
		prefix = "tag:!"
		// "!" applied per-tag for the inverse multi-tag CSV
		// shape (kept for backward-compat label parity with
		// the previous inversion factory).
		bangedTags := make([]string, len(tags))
		for i, t := range tags {
			bangedTags[i] = "!" + t
		}
		label = "tag:" + strings.Join(bangedTags, ",")
		return func(t model.Task) (string, int) {
			return matchTagBucket(t, wantLower, true, false, label)
		}
	case useAnd:
		prefix = "tag:&"
	}
	label = prefix + strings.Join(tags, ",")
	return func(t model.Task) (string, int) {
		return matchTagBucket(t, wantLower, inverted, useAnd, label)
	}
}

// matchTagBucket is the per-task predicate body shared between the
// union, intersection, and inverse variants. Pulled out so the
// factory's branch logic doesn't have to repeat the same return-
// shape boilerplate for every combination.
func matchTagBucket(t model.Task, wantLower map[string]bool, inverted, strictAnd bool, label string) (string, int) {
	var matched bool
	if strictAnd {
		// Intersection: every listed tag must be present.
		// Build the set of the task's tags (lower-cased) once,
		// then check membership per listed tag.
		taskTags := make(map[string]bool, len(t.Tags))
		for _, tg := range t.Tags {
			taskTags[strings.ToLower(tg)] = true
		}
		matched = true
		for want := range wantLower {
			if !taskTags[want] {
				matched = false
				break
			}
		}
	} else {
		// Union: any listed tag matches.
		for _, tg := range t.Tags {
			if wantLower[strings.ToLower(tg)] {
				matched = true
				break
			}
		}
	}
	// Inverted predicate: NO match means the task belongs in the
	// call-out bucket. Positive: match means it does.
	inCallOut := matched
	if inverted {
		inCallOut = !matched
	}
	if inCallOut {
		return label, 1
	}
	return "other", 2
}

// splitTagFilterCSV tokenizes a `tag:X,Y,Z` CSV payload into its
// individual tag names. Whitespace around each tag is trimmed;
// empty tokens (from "tag:X,,Y") are silently dropped so the
// user's accidental double-comma doesn't surprise. Returns an
// empty slice when no usable tokens are present (caller treats
// that as a usage error).
func splitTagFilterCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// makeTagFilterBucketFn returns a bucketFn that PARTITIONS tasks
// into exactly two sections: "tag:X" (or "tag:X,Y,Z" for the multi-
// tag variant) for tasks carrying ANY of the listed tags (case-
// insensitive match, same convention as `tsk ls --tag`), and
// "other" for everything else.
//
// Why this rather than "tag" (which sections by every first-tag
// value)? Because in real archive workflows the user often wants
// to call out ONE tag — "what shipped tagged release" or "what's
// in the work bucket vs personal bucket" — without scattering the
// archive across one section per distinct tag in the batch. The
// boolean partition keeps the layout to two clean buckets, which
// is the right shape for a "highlight this one tag" review.
//
// Multi-tag CSV variant ("tag:X,Y,Z"): a task lands in the call-
// out bucket if it carries ANY of the listed tags (logical OR —
// union semantics). Mirrors the `tsk graph --highlight-tag a,b`
// CSV semantics so the two surfaces use the same mental model:
// "show me tasks tagged any of these". The bucket label keeps
// the full CSV ("tag:X,Y") so the user sees which set was the
// filter — useful when the same archive holds multiple
// rollups produced with different filters.
//
// Sort keys: the call-out bucket gets 1 so it sorts BEFORE "other"
// (the user asked for it; they want it on top). "other" gets 2.
//
// Empty tags slice is rejected at resolveBucketByKey — we never
// get here with one.
//
// makeTagFilterBucketFn is preserved as a thin wrapper over
// makeTagFilterBucketFnInversion for backward compatibility — the
// internal callers all use the inversion-aware factory now, but
// external/test callers (and anyone reading the code archeology)
// expect this name. Always produces the POSITIVE (non-inverted)
// partition: tag-matched → call-out, otherwise → "other".
func makeTagFilterBucketFn(tags []string) bucketFn {
	return makeTagFilterBucketFnInversion(tags, false)
}

// makeIDRangeBucketFn returns a bucketFn that groups tasks into
// fixed-width id windows of size n. Window labels are "1-N",
// "N+1-2N", "2N+1-3N", …. Tasks with id 0 collapse into "id:0"
// (no-id legacy bucket). The sort key is the WINDOW START so
// buckets render in ascending id order ("1-50" before "51-100").
//
// Window math:
//
//	window index   = (id - 1) / n   (integer division)
//	window start   = index * n + 1
//	window end     = (index + 1) * n
//
// Examples (n=50):
//
//	id=1   → window 0 → "1-50"     sort=1
//	id=50  → window 0 → "1-50"     sort=1
//	id=51  → window 1 → "51-100"   sort=51
//	id=200 → window 3 → "151-200"  sort=151
//
// Pulled into its own factory so the resolveBucketByKey switch
// stays compact and the closure captures n cleanly.
func makeIDRangeBucketFn(n int) bucketFn {
	return func(t model.Task) (string, int) {
		if t.ID <= 0 {
			return "id:0", 0
		}
		windowIdx := (t.ID - 1) / n
		start := windowIdx*n + 1
		end := (windowIdx + 1) * n
		return fmt.Sprintf("%d-%d", start, end), start
	}
}

// writeBucketedArchive renders the archive file with the newly-
// archived tasks grouped into per-key sections, while leaving any
// pre-existing archive content verbatim at the top.
//
// The bucket function decides the section key + sort order — see
// bucketByISOWeek / bucketByMonth for the two shipped strategies.
//
// Layout:
//
//	# tsk archive                  (header — preserved from existing arch)
//	<existing tasks: untouched, in their current order>
//
//	## <bucket-key>                (new section for THIS batch's earliest bucket)
//	- [x] task1 <!-- id:N ... -->
//	- [x] task2 ...
//
//	## <bucket-key>                (more sections, ascending by sort key)
//
//	## undated                     (tasks without a Completed timestamp)
//	- [x] taskX ...
//
// We don't try to re-bucket the existing archive contents — that would
// renumber ids, shuffle the existing layout, and break any external
// references to old archived ids. The bucketed grouping applies only
// to the BATCH being added on this call, which matches the documented
// "switching strategies just affects the next batch" contract.
//
// The whole render goes through a manual buffer (renderTask is
// store-internal) so we keep store.Save's atomic-write + .bak
// snapshot semantics for the FIRST write (initial archive creation)
// and use a manual atomic write for subsequent appends with bucketed
// sections. Both paths share store.AtomicWriteFile so the on-disk
// contract is identical to a normal Save.
func writeBucketedArchive(archivePath string, arch *store.Store, batch []model.Task, bucket bucketFn) error {
	// First, take a snapshot of the existing archive file as the .bak
	// (matches store.Save's contract so `tsk undo-last` on the
	// archive still works).
	if cur, err := os.ReadFile(archivePath); err == nil {
		_ = store.AtomicWriteFile(archivePath+".bak", cur, 0o644)
	}

	var buf bytes.Buffer
	// Header (and any existing content) first — render via store.Save's
	// machinery by walking the existing tasks plus a header. We can't
	// reuse store.render (unexported), so we replay the same rendering
	// shape: header + a tail of task lines.
	if arch.Header != "" {
		buf.WriteString(arch.Header)
		if !strings.HasSuffix(arch.Header, "\n") {
			buf.WriteByte('\n')
		}
	}
	for _, t := range arch.Tasks {
		writeArchiveTask(&buf, t)
	}

	// Group batch by bucket key.
	type bucketRow struct {
		key   string
		sort  int
		tasks []model.Task
	}
	bucketMap := map[string]*bucketRow{}
	for _, t := range batch {
		key, sortKey := bucket(t)
		b, ok := bucketMap[key]
		if !ok {
			b = &bucketRow{key: key, sort: sortKey}
			bucketMap[key] = b
		}
		b.tasks = append(b.tasks, t)
	}
	// Stable ordering: oldest bucket first; "undated" goes last
	// (sort=0 would put it first, so we move it to the tail
	// explicitly).
	buckets := make([]*bucketRow, 0, len(bucketMap))
	for _, b := range bucketMap {
		buckets = append(buckets, b)
	}
	sort.SliceStable(buckets, func(i, j int) bool {
		// undated last
		if (buckets[i].key == "undated") != (buckets[j].key == "undated") {
			return buckets[j].key == "undated"
		}
		return buckets[i].sort < buckets[j].sort
	})
	// Inside each bucket, sort by id ascending so the layout is
	// reproducible regardless of the batch's input order.
	for _, b := range buckets {
		sort.SliceStable(b.tasks, func(i, j int) bool {
			return b.tasks[i].ID < b.tasks[j].ID
		})
	}

	// Emit. Ensure there's a blank line before the first new section
	// if existing content didn't end with one.
	existing := buf.Bytes()
	if len(existing) > 0 && !bytes.HasSuffix(existing, []byte("\n\n")) {
		if !bytes.HasSuffix(existing, []byte("\n")) {
			buf.WriteByte('\n')
		}
		buf.WriteByte('\n')
	}
	for i, b := range buckets {
		if i > 0 {
			buf.WriteByte('\n')
		}
		fmt.Fprintf(&buf, "## %s\n\n", b.key)
		for _, t := range b.tasks {
			writeArchiveTask(&buf, t)
		}
	}

	if err := store.AtomicWriteFile(archivePath, buf.Bytes(), 0o644); err != nil {
		return err
	}
	// Sync arch.Tasks so callers that examine it post-save see the
	// merged set (matches arch.Save() semantics for the flat path).
	arch.ReplaceTasks(append(arch.Tasks, batch...))
	return nil
}

// writeArchiveTask renders a single task line in the same shape as
// store's canonical writer. We can't call store.renderTask (unexported)
// so we duplicate the small bit of rendering logic. Keeping it tight
// here — the archive file uses the SAME bullet/box/meta layout as
// the active file, just with section dividers in between.
func writeArchiveTask(buf *bytes.Buffer, t model.Task) {
	box := " "
	if t.Done {
		box = "x"
	}
	fmt.Fprintf(buf, "- [%s] %s", box, t.Title)
	meta := renderArchiveMeta(t)
	if meta != "" {
		fmt.Fprintf(buf, " <!-- %s -->", meta)
	}
	buf.WriteByte('\n')
	if t.Notes != "" {
		for _, line := range strings.Split(t.Notes, "\n") {
			fmt.Fprintf(buf, "%s%s\n", strings.Repeat(" ", store.NotesIndent), line)
		}
	}
}

// renderArchiveMeta mirrors store.renderMeta's field order so a
// `tsk lint` against the archive (if anyone does that) would not flag
// drift. Stays a private dup rather than exporting store.renderMeta
// because the archive layout is logically a sibling of the active
// file's writer, not a public consumer.
func renderArchiveMeta(t model.Task) string {
	parts := make([]string, 0, 8)
	if t.ID > 0 {
		parts = append(parts, fmt.Sprintf("id:%d", t.ID))
	}
	parts = append(parts, fmt.Sprintf("prio:%s", t.Priority))
	if t.Due != nil {
		parts = append(parts, fmt.Sprintf("due:%s", t.Due.Format(model.DateLayout)))
	}
	if t.WaitUntil != nil {
		parts = append(parts, fmt.Sprintf("wait:%s", t.WaitUntil.Format(model.DateLayout)))
	}
	if len(t.Tags) > 0 {
		tags := append([]string(nil), t.Tags...)
		sort.Strings(tags)
		parts = append(parts, fmt.Sprintf("tags:%s", strings.Join(tags, ",")))
	}
	if !t.Created.IsZero() {
		parts = append(parts, fmt.Sprintf("created:%s", t.Created.Format(store.TimeLayout)))
	}
	if t.Started != nil {
		parts = append(parts, fmt.Sprintf("started:%s", t.Started.Format(store.TimeLayout)))
	}
	if t.Completed != nil {
		parts = append(parts, fmt.Sprintf("completed:%s", t.Completed.Format(store.TimeLayout)))
	}
	if t.Pinned {
		parts = append(parts, "pin:true")
	}
	if len(t.DependsOn) > 0 {
		ids := make([]string, 0, len(t.DependsOn))
		for _, id := range t.DependsOn {
			ids = append(ids, fmt.Sprintf("%d", id))
		}
		parts = append(parts, fmt.Sprintf("depends:%s", strings.Join(ids, ",")))
	}
	return strings.Join(parts, " ")
}

// archivedRow is one task in the JSON archive envelope. The shape
// keeps the most useful per-task identity:
//   - active_id   : the id the task carried in the active store
//   - archive_id  : the id it received in the archive (continuing
//     the archive's max+1; differs from active_id since the
//     archive has its own monotonic sequence). For --dry-run the
//     archive_id is the SIMULATED id (what a real run would
//     assign), so the envelope shape stays stable across dry vs.
//     real runs.
//   - title       : the task's title (unchanged across the move)
//   - priority    : canonical string form (low/medium/high/urgent)
//   - bucket      : the section/key the task landed in (matches
//     the section header rendered in the archive file; empty
//     string for flat strategy where there are no sections)
type archivedRow struct {
	ActiveID  int    `json:"active_id"`
	ArchiveID int    `json:"archive_id"`
	Title     string `json:"title"`
	Priority  string `json:"priority"`
	Bucket    string `json:"bucket,omitempty"`
}

// archiveDoc is the JSON envelope for `tsk archive --json`. Stable
// schema:
//   - archive_path : the resolved file the run wrote to (or would
//     write to, in --dry-run mode)
//   - strategy     : "flat" by default, or the user's --strategy
//     value (daily/weekly/monthly/quarterly/yearly)
//   - bucket_by    : the user-supplied --bucket-by axis (priority/
//     tag/tag:X/tag:X,Y/tag:!X/id-range:N) or empty
//   - strict_and   : whether --strict-and was set (only meaningful
//     for CSV-tag bucket-by forms)
//   - dry_run      : true when --dry-run was set (so consumers can
//     tell preview from real run without re-checking flags)
//   - total_count  : len(archived) — convenience field for jq
//     `.total_count` without a `.archived | length`
//   - active_count : len(kept) — the active store's post-archive
//     task count (useful for "did this drop us below N?" CI gates)
//   - archived     : per-task rows; always emitted as an array
//     (empty array, not null, when no tasks qualified) so jq
//     iteration works on every case
type archiveDoc struct {
	ArchivePath string        `json:"archive_path"`
	Strategy    string        `json:"strategy"`
	BucketBy    string        `json:"bucket_by,omitempty"`
	StrictAnd   bool          `json:"strict_and,omitempty"`
	DryRun      bool          `json:"dry_run"`
	TotalCount  int           `json:"total_count"`
	ActiveCount int           `json:"active_count"`
	Archived    []archivedRow `json:"archived"`
}

// emitArchiveJSON renders the stable archive-run envelope. Called
// from three exit paths:
//
//  1. "no tasks to archive" — archived is empty, totals reflect
//     that, archive_path still points at the resolved target so
//     consumers see WHERE a real run would have written.
//
//  2. --dry-run — archived carries SIMULATED archive_ids (the
//     archive store is read for its current max id; nothing is
//     written). The envelope shape is identical to a real run so
//     scripted pipelines that consume the JSON behave the same
//     way across preview and real-run modes.
//
//  3. Real run — archived carries the actually-assigned archive
//     ids (the Save has already happened by the time this is
//     called).
//
// activeIDs is the parallel slice of the tasks' pre-rewrite active-
// store ids (the IDs they carried BEFORE the for-loop reassigned
// them to archive_ids). The user's mental model is "task #N
// became archive #M"; capturing both halves keeps that round-trip
// visible in the JSON envelope. len(activeIDs) must equal
// len(archived); when nil (only on the "no tasks" path) the rows
// loop just doesn't run.
//
// The bucket field is computed from the resolution path that
// matches the strategy/bucketBy combination — the same bucketFn
// the writer uses for the on-disk grouping, so the JSON and the
// on-disk file agree on which task landed in which section.
//
// For the flat strategy (no bucket axis) every row has bucket="";
// omitempty drops the field so the JSON stays minimal.
func emitArchiveJSON(w io.Writer, archivePath, strategy, bucketBy string, strictAnd, dryRun bool, bucketFunc bucketFn, kept, archived []model.Task, activeIDs []int) error {
	// Resolve the time-based strategy to its bucketFn for the JSON
	// path. The writer dispatches the same way in the switch above
	// — we mirror that dispatch here so the JSON's "bucket" field
	// agrees with the on-disk section header for every strategy.
	rows := make([]archivedRow, 0, len(archived))
	for i, t := range archived {
		row := archivedRow{
			ArchiveID: t.ID,
			Title:     t.Title,
			Priority:  t.Priority.String(),
		}
		if i < len(activeIDs) {
			row.ActiveID = activeIDs[i]
		}
		key := ""
		switch strategy {
		case "daily":
			key, _ = bucketByDay(t)
		case "weekly":
			key, _ = bucketByISOWeek(t)
		case "monthly":
			key, _ = bucketByMonth(t)
		case "quarterly":
			key, _ = bucketByQuarter(t)
		case "yearly":
			key, _ = bucketByYear(t)
		default:
			if bucketFunc != nil {
				key, _ = bucketFunc(t)
			}
		}
		row.Bucket = key
		rows = append(rows, row)
	}
	doc := archiveDoc{
		ArchivePath: archivePath,
		Strategy:    strategy,
		BucketBy:    bucketBy,
		StrictAnd:   strictAnd,
		DryRun:      dryRun,
		TotalCount:  len(archived),
		ActiveCount: len(kept),
		Archived:    rows,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}
