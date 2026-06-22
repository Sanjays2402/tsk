package commands

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
		all       bool
		dryRun    bool
		strategy  string
		mergeInto string
	)
	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Move completed tasks to a sibling .tsk.archive.md file",
		Long: "Move Done tasks out of the active .tsk.md and into a sibling .tsk.archive.md.\n" +
			"Archived tasks get fresh sequential IDs in the archive file, continuing\n" +
			"from the archive's existing max ID. Active task IDs do not change.\n\n" +
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
			"                  retro reviews. Same '## undated' bucket policy.\n\n" +
			"daily/weekly/monthly/quarterly is purely a layout choice — IDs, content, and\n" +
			"round-trip semantics are identical; switching strategies later just changes\n" +
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
				pf(out, "no tasks to archive\n")
				return nil
			}

			if dryRun {
				pf(out, "would archive %d task(s) → %s (strategy=%s)\n", len(archived), archivePath, strategy)
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
			for i := range archived {
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
			default:
				arch.ReplaceTasks(append(arch.Tasks, archived...))
				if err := arch.Save(); err != nil {
					return fmt.Errorf("save archive: %w", err)
				}
			}

			s.ReplaceTasks(kept)
			if err := s.Save(); err != nil {
				return fmt.Errorf("save active: %w", err)
			}

			pf(out, "archived %d task(s) → %s (strategy=%s)\n", len(archived), archivePath, strategy)
			pf(out, "active tasks: %d\n", len(kept))
			return nil
		},
	}
	cmd.Flags().StringVar(&olderThan, "older-than", "30d", "only archive tasks completed more than this ago (e.g. 7d, 2w, 1m)")
	cmd.Flags().BoolVar(&all, "all", false, "archive every Done task regardless of age")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print what would be archived without changing files")
	cmd.Flags().StringVar(&strategy, "strategy", "flat", "archive layout: flat | daily | weekly | monthly | quarterly (one bucket per fiscal quarter)")
	cmd.Flags().StringVar(&mergeInto, "merge-into", "", "write to this archive file instead of the sibling .tsk.archive.md (~ expansion supported; created if missing)")
	return cmd
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
