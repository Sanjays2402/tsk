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
	)
	cmd := &cobra.Command{
		Use:   "archive",
		Short: "Move completed tasks to a sibling .tsk.archive.md file",
		Long: "Move Done tasks out of the active .tsk.md and into a sibling .tsk.archive.md.\n" +
			"Archived tasks get fresh sequential IDs in the archive file, continuing\n" +
			"from the archive's existing max ID. Active task IDs do not change.\n\n" +
			"--strategy controls how archived tasks are grouped inside the archive file:\n" +
			"  flat (default)  one growing list in the order they were archived\n" +
			"  weekly          group into '## YYYY-W##' sections by completion week (ISO),\n" +
			"                  so old archives have a scannable timeline. Tasks without a\n" +
			"                  Completed timestamp fall into '## undated' so they're not lost.\n\n" +
			"weekly is purely a layout choice — IDs, content, and round-trip semantics\n" +
			"are identical; switching strategies later just changes how the next batch is\n" +
			"placed in the file. Existing archive contents are preserved verbatim.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			strategy = strings.ToLower(strings.TrimSpace(strategy))
			switch strategy {
			case "", "flat":
				strategy = "flat"
			case "weekly":
				// ok
			default:
				return usageErrorf("unknown --strategy %q (want flat or weekly)", strategy)
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
			archivePath := filepath.Join(filepath.Dir(s.Path), archiveFileName)

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
			if strategy == "weekly" {
				if err := writeWeeklyArchive(archivePath, arch, archived); err != nil {
					return fmt.Errorf("save weekly archive: %w", err)
				}
			} else {
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
	cmd.Flags().StringVar(&strategy, "strategy", "flat", "archive layout: flat (one growing list) or weekly (group by ISO week)")
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

// writeWeeklyArchive renders the archive file with the newly-archived
// tasks grouped into per-ISO-week sections, while leaving any
// pre-existing archive content verbatim at the top.
//
// Layout:
//
//	# tsk archive                  (header — preserved from existing arch)
//	<existing tasks: untouched, in their current order>
//
//	## YYYY-W##                    (new section for THIS batch's earliest week)
//	- [x] task1 <!-- id:N ... -->
//	- [x] task2 ...
//
//	## YYYY-W##                    (more sections, ascending by week)
//
//	## undated                     (tasks without a Completed timestamp)
//	- [x] taskX ...
//
// We don't try to re-bucket the existing archive contents — that would
// renumber ids, shuffle the existing layout, and break any external
// references to old archived ids. The weekly grouping applies only to
// the BATCH being added on this call, which matches the documented
// "switching strategies just affects the next batch" contract.
//
// ISO-week formatting via time.ISOWeek matches what most tools and
// calendars use (Mon-first; weeks span year boundaries cleanly).
//
// The whole render goes through a manual buffer (renderTask is
// store-internal) so we keep store.Save's atomic-write + .bak
// snapshot semantics for the FIRST write (initial archive creation)
// and use a manual atomic write for subsequent appends with weekly
// sections. Both paths share store.AtomicWriteFile so the on-disk
// contract is identical to a normal Save.
func writeWeeklyArchive(archivePath string, arch *store.Store, batch []model.Task) error {
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

	// Group batch by ISO week of t.Completed. Tasks without a
	// Completed stamp land in "undated".
	type bucket struct {
		key   string // "YYYY-W##" or "undated"
		sort  int    // sortable: year*100 + week (0 for "undated" tail)
		tasks []model.Task
	}
	bucketMap := map[string]*bucket{}
	for _, t := range batch {
		var key string
		var sortKey int
		if t.Completed == nil {
			key = "undated"
			sortKey = 0
		} else {
			y, w := t.Completed.ISOWeek()
			key = fmt.Sprintf("%04d-W%02d", y, w)
			sortKey = y*100 + w
		}
		b, ok := bucketMap[key]
		if !ok {
			b = &bucket{key: key, sort: sortKey}
			bucketMap[key] = b
		}
		b.tasks = append(b.tasks, t)
	}
	// Stable ordering: oldest week first; "undated" goes last (sort=0
	// would put it first, so we move it to the tail explicitly).
	buckets := make([]*bucket, 0, len(bucketMap))
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
