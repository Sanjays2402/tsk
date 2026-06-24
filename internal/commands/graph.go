package commands

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/Sanjays2402/tsk/internal/model"
	"github.com/Sanjays2402/tsk/internal/store"
)

// newGraphCmd implements `tsk graph`: render the dependency graph
// across the whole store.
//
// `tsk depend <id> --tree` walks ONE prerequisite chain rooted at a
// task. `tsk graph` is the bird's-eye view: every dependency edge in
// the store, summarized in one screen.
//
// Two formats:
//
//   - default (`--format ascii`, or no flag): an indented adjacency
//     listing — one line per task that has dependencies, with the
//     ids it depends on shown after a "->" arrow. Open tasks come
//     first (the actionable ones) then a "(done)" section so the
//     graph isn't dominated by historical completed work.
//
//   - `--format dot`: GraphViz DOT source on stdout. Pipe into
//     `dot -Tpng > graph.png` or paste into https://dreampuf.github.io/GraphvizOnline
//     for a real visual. Done tasks render as filled gray nodes;
//     open tasks render as outlined boxes; blocked tasks (open with
//     at least one open prereq) get a red border so the chokepoints
//     stand out.
//
// Filters:
//
//	--open               only include open tasks AND the open deps that
//	                     actually block them (filters out the done-history
//	                     noise; the most useful default for active work)
//	--reachable <id>     only include the subgraph reachable from <id>
//	                     via DependsOn edges (the transitive prereqs of
//	                     one root + the root itself). Pairs nicely with
//	                     `tsk depend <id> --tree`: tree shows one chain
//	                     in depth-first form, --reachable shows the full
//	                     fan-in/out subgraph in DOT layout.
//	--highlight <id>     (DOT only) wrap one node in a distinct gold
//	                     fill + bold border so the focus task stands
//	                     out on a complex graph. Useful when you're
//	                     pasting the graph into a review and want to
//	                     draw the eye to the milestone or chokepoint
//	                     under discussion.
//	--dim <ids>          (DOT only) inverse of --highlight: render the
//	                     named CSV ids in a quiet gray fill + dashed
//	                     border so the OTHER nodes stand out. Useful
//	                     when there are a few well-known tasks you want
//	                     to push to the background ("ignore the
//	                     scaffolding tasks; show me everything else").
//	                     Mutually exclusive with --highlight on the same
//	                     id: a single node can't be both spotlighted
//	                     AND backgrounded — that's contradictory intent.
//	--dim-tag <name>     (DOT only) tag selector for --dim: push every
//	                     task carrying the named tag to the background.
//	                     Sister of --highlight-tag for the inverse
//	                     verb; same case-insensitive match. Combines
//	                     with --dim ids (union); rejects overlap with
//	                     --highlight or --highlight-tag.
//
// Empty graphs (no deps anywhere) print "no dependencies" rather than
// emitting a blank DOT skeleton — both shapes are still parseable but
// the explicit message saves a "why is this empty?" diagnostic loop.
func newGraphCmd() *cobra.Command {
	var (
		format               string
		open                 bool
		reachable            int
		upstreamOf           int
		highlight            string
		highlightTag         string
		dim                  string
		dimTag               string
		asJSON               bool
		outputPath           string
		jsonCompact          bool
		jsonAppend           bool
		jsonRotate           int
		includePriority      bool
		includeTags          bool
		includeDue           bool
		includeCompleted     bool
		includeStarted       bool
		includePinned        bool
		includeAll           bool
		filterCompletedSince string
		filterStartedSince   string
	)
	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Render the dependency graph (ascii or GraphViz DOT)",
		Long: `Render the whole-store dependency graph.

Two output formats:
  --format ascii     adjacency listing (default; one line per task)
  --format dot       GraphViz DOT source for piping to ` + "`dot -Tpng`" + `
  --format svg       self-contained SVG (embedded renderer; no GraphViz dep)

Filters:
  --open                  skip done tasks and edges to done prereqs
  --reachable <id>        restrict to the subgraph reachable from <id>
                          via DependsOn (transitive prereqs + root) —
                          the DOWNSTREAM view: "what does #id depend on
                          all the way down?"
  --upstream-of <id>      restrict to the subgraph that transitively
                          DEPENDS ON <id> via DependsOn (every task
                          whose prereq chain eventually names <id>,
                          plus <id> itself). The INVERSE of
                          --reachable: where --reachable shows "what
                          must finish before #id?", --upstream-of
                          shows "what's still waiting on #id to finish?".
                          Pairs with ` + "`tsk depend <id> --upstream`" + `,
                          which only shows the direct dependents (one
                          step); --upstream-of walks the full chain in
                          the same DOT layout as the whole-store graph.
                          Mutually exclusive with --reachable: each
                          answers a different direction and combining
                          them would muddle the subgraph definition.
  --highlight <ids>       (DOT only) wrap one or more nodes in a distinct
                          gold fill + bold border so they stand out.
                          Comma-separated id list — single id "7" or
                          multi "7,3,5" both work; the # prefix is
                          tolerated.
  --highlight-tag <list>  (DOT only) spotlight every node carrying any
                          of the listed tags in the SAME gold style.
                          Comma-separated CSV — 'release' matches one
                          tag, 'release,p0' matches the UNION of both.
                          Broader than --highlight ids: useful when
                          you want to highlight a logical SLICE of
                          the graph ("show me everything tagged
                          'release'") without listing ids one by one.
                          Case-insensitive (` + "`tsk ls --tag`" + `'s rules).
                          Combines with --highlight ids — the spotlight
                          set is the UNION of both, so you can pin
                          one specific task on top of a tag-wide
                          highlight (e.g. "highlight the release tag,
                          and spotlight #42 inside it").
  --dim <ids>             (DOT only) inverse of --highlight: render the
                          named CSV ids in a quiet gray fill + dashed
                          border so the OTHER nodes stand out.
                          Useful when there are a few scaffolding
                          tasks you want to push to the background
                          and let everything else read as foreground.
                          Mutually exclusive with --highlight on the
                          same id (contradictory intent: a single
                          node can't be both spotlighted and dimmed).
  --dim-tag <list>        (DOT only) push every node carrying any of
                          the listed tags to the background in the
                          SAME dim style. Comma-separated CSV (same
                          shape as --highlight-tag). Sister of
                          --highlight-tag for the inverse verb.
                          Combines with --dim ids (union); rejected
                          for overlap with --highlight or
                          --highlight-tag the same way --dim is.

Use ` + "`tsk depend <id> --tree`" + ` instead if you want one branch in
depth-first form; this command is the bird's-eye view.
` + "`--reachable`" + ` is the in-between view: every transitive prereq of
one root, in the same DOT layout used for the whole-store graph.
` + "`--upstream-of`" + ` is the inverse view: every transitive dependent
of one root — the chain still waiting on it.

Examples:
  tsk graph                              # quick text adjacency view
  tsk graph --open                       # only show what's still blocking
  tsk graph --reachable 7                # the subgraph rooted at #7 (downstream)
  tsk graph --upstream-of 7              # the subgraph WAITING on #7 (upstream)
  tsk graph --reachable 7 --json         # scripted impact-analysis (downstream)
  tsk graph --upstream-of 7 --json       # scripted impact-analysis (upstream)
  tsk graph --reachable 7 --open         # …filtered to active work
  tsk graph --format dot | dot -Tpng -o deps.png
  tsk graph --format dot --highlight 7   # draw the eye to #7
  tsk graph --format dot --highlight 7,3,5   # spotlight a whole subset
  tsk graph --format dot --highlight-tag release  # spotlight a whole tag
  tsk graph --format dot --highlight-tag release,p0   # union of two tags
  tsk graph --format dot --highlight 42 --highlight-tag release  # union
  tsk graph --format dot --dim 1,2       # push #1 and #2 to the background
  tsk graph --format dot --dim-tag scaffold,wip   # push two tags down
  tsk graph --format dot --upstream-of 7 --highlight 7 | dot -Tsvg > impact.svg
  tsk graph --format dot --reachable 7 | dot -Tsvg > sub.svg
  tsk graph --format svg > deps.svg               # self-contained, no graphviz
  tsk graph --format svg --reachable 7 > sub.svg  # subgraph SVG, no graphviz
  tsk graph --format svg --output deps.svg        # write directly to file (no shell redirection)
  tsk graph --format dot --output deps.dot        # extension validated against --format
  tsk graph --reachable 7 --json --output impact.json   # JSON envelope -> file
  tsk graph --reachable 7 --json --compact-json         # single-line JSON (JSONL-friendly)
  tsk graph --reachable 7 --json --compact-json --output snap.json   # compact write
  tsk graph --reachable 7 --json --output snap.jsonl --append       # JSONL append (one record per call)
  tsk graph --reachable 7 --json --output snap.jsonl --append --rotate 100  # rolling buffer of last 100 snapshots
  tsk graph --reachable 7 --json --include-priority                 # JSON envelope with per-node priority
  tsk graph --reachable 7 --json --include-completed                # add per-node 'completed' RFC3339 timestamp (done tasks only)
  tsk graph --reachable 7 --json --include-started                  # add per-node 'started' RFC3339 timestamp (in-progress tasks only)
  tsk graph --reachable 7 --json --include-pinned                   # add per-node 'pinned' boolean (high-importance bookmark axis)
  tsk graph --reachable 7 --json --include-all                      # full-fat envelope: every opt-in field (priority+tags+due+completed+started+pinned)
  tsk graph --reachable 7 --json --filter-completed-since 7d        # only nodes completed in the last 7 days (recency-trimmed envelope)
  tsk graph --reachable 7 --json --filter-started-since 24h         # only nodes started in the last day (in-progress recency)
  tsk graph --reachable 7 --json --filter-completed-since 7d --filter-started-since 7d  # UNION: anything actively moving
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmtChoice, err := resolveGraphFormat(format)
			if err != nil {
				return err
			}
			if reachable > 0 && upstreamOf > 0 {
				return usageErrorf("--reachable and --upstream-of are mutually exclusive (each defines a different subgraph direction)")
			}
			if highlight != "" && fmtChoice != "dot" && fmtChoice != "svg" {
				return usageErrorf("--highlight only applies to --format dot or svg (got %s)", fmtChoice)
			}
			if highlightTag != "" && fmtChoice != "dot" && fmtChoice != "svg" {
				return usageErrorf("--highlight-tag only applies to --format dot or svg (got %s)", fmtChoice)
			}
			if dim != "" && fmtChoice != "dot" && fmtChoice != "svg" {
				return usageErrorf("--dim only applies to --format dot or svg (got %s)", fmtChoice)
			}
			if dimTag != "" && fmtChoice != "dot" && fmtChoice != "svg" {
				return usageErrorf("--dim-tag only applies to --format dot or svg (got %s)", fmtChoice)
			}
			if asJSON && reachable == 0 && upstreamOf == 0 {
				return usageErrorf("--json only applies to --reachable or --upstream-of (the per-root subgraph paths)")
			}
			if jsonCompact && !asJSON {
				return usageErrorf("--compact-json only applies to --json (the JSON envelope path)")
			}
			if includePriority && !asJSON {
				return usageErrorf("--include-priority only applies to --json (the JSON envelope path)")
			}
			if includeTags && !asJSON {
				return usageErrorf("--include-tags only applies to --json (the JSON envelope path)")
			}
			if includeDue && !asJSON {
				return usageErrorf("--include-due only applies to --json (the JSON envelope path)")
			}
			if includeCompleted && !asJSON {
				return usageErrorf("--include-completed only applies to --json (the JSON envelope path)")
			}
			if includeStarted && !asJSON {
				return usageErrorf("--include-started only applies to --json (the JSON envelope path)")
			}
			if includePinned && !asJSON {
				return usageErrorf("--include-pinned only applies to --json (the JSON envelope path)")
			}
			if includeAll && !asJSON {
				return usageErrorf("--include-all only applies to --json (the JSON envelope path)")
			}
			// --include-all is the ergonomic shortcut for "turn on
			// every opt-in node field at once". Flip every flag on
			// when set; the individual flags can still be set
			// alongside (idempotent — true OR true == true).
			if includeAll {
				includePriority = true
				includeTags = true
				includeDue = true
				includeCompleted = true
				includeStarted = true
				includePinned = true
			}
			if jsonAppend {
				if !asJSON {
					return usageErrorf("--append only applies to --json (the JSON envelope path)")
				}
				if outputPath == "" {
					return usageErrorf("--append requires --output <path> (the file to append to)")
				}
				// --append implies compact mode: each call adds
				// ONE record to a JSONL stream, and indented JSON
				// across multiple records would corrupt every
				// consumer that splits on \n. Quietly upgrading
				// rather than rejecting keeps the most common
				// invocation (`tsk graph --reachable 7 --json
				// --output snap.jsonl --append`) ergonomic.
				jsonCompact = true
			}
			if jsonRotate < 0 {
				return usageErrorf("--rotate must be >= 0 (got %d); 0 disables rotation, any positive N keeps the most-recent N records", jsonRotate)
			}
			if jsonRotate > 0 && !jsonAppend {
				// Rotation only makes sense for the streaming
				// (append) path. Overwriting --output keeps a
				// single record by definition, so capping it to
				// N is a vacuous request — surface the conflict
				// at the CLI layer so the user re-thinks rather
				// than getting silent no-op behavior.
				return usageErrorf("--rotate requires --append (rotation only applies to the JSONL streaming path)")
			}
			// --filter-completed-since trims the subgraph to
			// nodes completed within the recency window (e.g.
			// 7d, 24h). Useful for "what shipped this week in
			// this dep chain?" / completion-velocity dashboards
			// / change-impact reports that need to focus on
			// recent activity. Only meaningful on the --json
			// envelope path (the ASCII / DOT / SVG renderers
			// don't have an obvious "recently completed" filter
			// idiom). Empty value = no filter (defensive against
			// shell vars; mirrors --tag / --priority's empty
			// stance elsewhere). Negative / zero durations are
			// rejected — a zero-window filter would drop every
			// node and is almost certainly a typo.
			var filterCompletedDur time.Duration
			filterCompletedActive := false
			if strings.TrimSpace(filterCompletedSince) != "" {
				if !asJSON {
					return usageErrorf("--filter-completed-since only applies to --json (the JSON envelope path)")
				}
				d, err := parseDurationLocal(strings.TrimSpace(filterCompletedSince))
				if err != nil {
					return usageErrorf("invalid --filter-completed-since %q: %v", filterCompletedSince, err)
				}
				if d <= 0 {
					return usageErrorf("--filter-completed-since must be a positive duration, got %q", filterCompletedSince)
				}
				filterCompletedDur = d
				filterCompletedActive = true
			}
			// --filter-started-since trims the subgraph to nodes
			// whose task is currently in-progress AND was started
			// within the recency window (e.g. 7d, 24h). The OPEN
			// sister of --filter-completed-since: completed-since
			// surfaces RECENTLY-FINISHED work, started-since
			// surfaces RECENTLY-BEGUN work. Together they enable
			// "what's active in this dep chain?" reports —
			// composing both gives the UNION of recent activity
			// at either end of the work-state pair. Same
			// validation contract as --filter-completed-since
			// (requires --json, positive duration only, empty
			// value is no-op). The root id is always preserved
			// regardless of the filter, matching the symmetric
			// semantic completed-since uses.
			var filterStartedDur time.Duration
			filterStartedActive := false
			if strings.TrimSpace(filterStartedSince) != "" {
				if !asJSON {
					return usageErrorf("--filter-started-since only applies to --json (the JSON envelope path)")
				}
				d, err := parseDurationLocal(strings.TrimSpace(filterStartedSince))
				if err != nil {
					return usageErrorf("invalid --filter-started-since %q: %v", filterStartedSince, err)
				}
				if d <= 0 {
					return usageErrorf("--filter-started-since must be a positive duration, got %q", filterStartedSince)
				}
				filterStartedDur = d
				filterStartedActive = true
			}
			s, err := resolveStore(cmd, true)
			if err != nil {
				return err
			}
			highlightSet, err := parseHighlightCSV(s, highlight)
			if err != nil {
				return err
			}
			highlightSet = mergeHighlightTag(s, highlightSet, highlightTag)
			dimSet, err := parseDimCSV(s, dim)
			if err != nil {
				return err
			}
			dimSet = mergeDimTag(s, dimSet, dimTag)
			if err := rejectDimHighlightOverlap(dimSet, highlightSet); err != nil {
				return err
			}
			edges := collectGraphEdges(s, open)
			if reachable > 0 {
				if s.ByID(reachable) == nil {
					return fmt.Errorf("no task with id %d in %s", reachable, s.Path)
				}
				edges = filterReachableEdges(s, edges, reachable)
			}
			if upstreamOf > 0 {
				if s.ByID(upstreamOf) == nil {
					return fmt.Errorf("no task with id %d in %s", upstreamOf, s.Path)
				}
				edges = filterUpstreamOfEdges(s, edges, upstreamOf)
			}
			rootDisplay := reachable
			rootKind := "reachable"
			if upstreamOf > 0 {
				rootDisplay = upstreamOf
				rootKind = "upstream-of"
			}
			// --output redirects the rendered graph to a file
			// instead of stdout, with extension validation that
			// pairs the format keyword (ascii/dot/svg) to the
			// extension (.txt/.dot/.svg). Why bother validating?
			// Because a `tsk graph --format svg --output deps.dot`
			// produces a `.dot` file containing SVG bytes — a
			// silent footgun that breaks every downstream tool
			// expecting DOT (a `dot -Tpng deps.dot` would fail in
			// a confusing way). Surfacing the mismatch at the CLI
			// layer catches the typo before the file lands.
			//
			// --output now also supports --json: the subgraph
			// envelope is written directly to <path> when both
			// flags are set together. Useful for CI gates and
			// snapshot tests that want a stable filename without
			// shell redirection (e.g. `tsk graph --reachable 7
			// --json --output impact.json`). Extension validation
			// for the JSON path requires .json — same fail-fast
			// behavior as the other format/extension pairs so a
			// typo like `--json --output impact.svg` is caught
			// before the file lands containing JSON bytes under
			// the wrong name.
			//
			// We buffer the render before writing so a render
			// failure leaves NO partial file on disk (matches the
			// atomic-write contract every other tsk write path
			// follows).
			if outputPath != "" {
				if asJSON {
					if err := validateGraphOutputJSONExtension(outputPath, jsonAppend); err != nil {
						return err
					}
					var buf bytes.Buffer
					if err := emitSubgraphJSON(&buf, s, edges, rootDisplay, rootKind, open, jsonCompact, includePriority, includeTags, includeDue, includeCompleted, includeStarted, includePinned, filterCompletedActive, filterCompletedDur, filterStartedActive, filterStartedDur); err != nil {
						return err
					}
					if jsonAppend {
						// Append-mode: open the file in O_APPEND, write the
						// single compact record (already terminated with a
						// trailing newline by json.Encoder), close. If the
						// file doesn't exist we create it with the same
						// 0644 mode the truncating-write path uses, so the
						// first append creates a fresh JSONL file
						// transparently. The atomic-write contract is
						// weaker here than the rest of tsk's I/O (a
						// concurrent reader could see a partial write of
						// a single record on a non-POSIX filesystem), but
						// JSONL by design is append-friendly: most
						// filesystems guarantee atomicity for writes
						// under 4KB, which our compact envelope easily
						// fits.
						f, err := os.OpenFile(outputPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
						if err != nil {
							return fmt.Errorf("--append: open %s: %w", outputPath, err)
						}
						if _, err := f.Write(buf.Bytes()); err != nil {
							f.Close()
							return fmt.Errorf("--append: write %s: %w", outputPath, err)
						}
						if err := f.Close(); err != nil {
							return fmt.Errorf("--append: close %s: %w", outputPath, err)
						}
						// Post-append rotation: if --rotate N was
						// set, cap the file to the most-recent N
						// records by dropping the oldest lines (FIFO
						// eviction). Done AFTER the append so a
						// fresh write is always retained, even when
						// N=1 and the file was already at capacity.
						// Implemented as a full read + tail-keep +
						// atomic rename (write to .tmp, rename over
						// the target) so a crash mid-rotation leaves
						// the existing file untouched. Tradeoff:
						// O(file size) on every rotated call, which
						// is fine for the snapshot-history use case
						// (logs measured in MBs, not GBs) and avoids
						// the complexity of seek-and-truncate.
						droppedCount := 0
						if jsonRotate > 0 {
							n, err := rotateJSONLFile(outputPath, jsonRotate)
							if err != nil {
								return fmt.Errorf("--rotate: %w", err)
							}
							droppedCount = n
						}
						if droppedCount > 0 {
							pf(cmd.OutOrStdout(), "appended %d bytes to %s (format=jsonl; rotated: dropped %d oldest line(s), kept newest %d)\n", buf.Len(), outputPath, droppedCount, jsonRotate)
						} else {
							pf(cmd.OutOrStdout(), "appended %d bytes to %s (format=jsonl)\n", buf.Len(), outputPath)
						}
						return nil
					}
					if err := os.WriteFile(outputPath, buf.Bytes(), 0o644); err != nil {
						return fmt.Errorf("--output: write %s: %w", outputPath, err)
					}
					pf(cmd.OutOrStdout(), "wrote %d bytes to %s (format=json)\n", buf.Len(), outputPath)
					return nil
				}
				if err := validateGraphOutputExtension(outputPath, fmtChoice); err != nil {
					return err
				}
				var buf bytes.Buffer
				if err := emitGraph(&buf, s, edges, fmtChoice, rootDisplay, rootKind, highlightSet, dimSet); err != nil {
					return err
				}
				if err := os.WriteFile(outputPath, buf.Bytes(), 0o644); err != nil {
					return fmt.Errorf("--output: write %s: %w", outputPath, err)
				}
				pf(cmd.OutOrStdout(), "wrote %d bytes to %s (format=%s)\n", buf.Len(), outputPath, fmtChoice)
				return nil
			}
			if asJSON {
				return emitSubgraphJSON(cmd.OutOrStdout(), s, edges, rootDisplay, rootKind, open, jsonCompact, includePriority, includeTags, includeDue, includeCompleted, includeStarted, includePinned, filterCompletedActive, filterCompletedDur, filterStartedActive, filterStartedDur)
			}
			return emitGraph(cmd.OutOrStdout(), s, edges, fmtChoice, rootDisplay, rootKind, highlightSet, dimSet)
		},
	}
	cmd.Flags().StringVar(&format, "format", "ascii", "output format: ascii, dot, or svg (svg uses a self-contained embedded renderer; no GraphViz required)")
	cmd.Flags().BoolVar(&open, "open", false, "only include open tasks and the open deps that block them")
	cmd.Flags().IntVar(&reachable, "reachable", 0, "restrict to the subgraph reachable from this task id via DependsOn (downstream prereq chain)")
	cmd.Flags().IntVar(&upstreamOf, "upstream-of", 0, "restrict to the subgraph that transitively DEPENDS ON this task id (upstream dependent chain; inverse of --reachable)")
	cmd.Flags().StringVar(&highlight, "highlight", "", "(DOT/SVG) comma-separated task ids to draw with a distinct fill+border")
	cmd.Flags().StringVar(&highlightTag, "highlight-tag", "", "(DOT/SVG) comma-separated tag list; spotlight every task carrying any of them (case-insensitive)")
	cmd.Flags().StringVar(&dim, "dim", "", "(DOT/SVG) comma-separated task ids to render in a quiet gray fill+dashed border")
	cmd.Flags().StringVar(&dimTag, "dim-tag", "", "(DOT/SVG) comma-separated tag list; push every task carrying any of them to the background (case-insensitive)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "for --reachable or --upstream-of: emit a stable JSON envelope listing every node and edge in the subgraph (scripted impact-analysis)")
	cmd.Flags().BoolVar(&jsonCompact, "compact-json", false, "for --json: emit a single-line, no-indent JSON record (JSONL-friendly). Useful when appending impact-analysis snapshots to a log file where each line must be a self-contained record (`tsk graph --reachable 7 --json --compact-json --output snap.jsonl` appends one record per call).")
	cmd.Flags().BoolVar(&jsonAppend, "append", false, "for --json --output: APPEND the JSON envelope to <path> instead of overwriting (JSONL semantics). Each call adds exactly one record to the file (creating it if missing); the file builds up a history of impact-analysis snapshots over time. Implies --compact-json so the on-disk shape is true JSONL. .json and .jsonl extensions both accepted; .jsonl is the canonical streaming-JSON convention.")
	cmd.Flags().IntVar(&jsonRotate, "rotate", 0, "for --json --output --append: cap the JSONL file to N records by trimming the OLDEST lines after each append (FIFO eviction). 0 (default) = no rotation (the file grows unbounded). Useful for long-lived snapshot loops where you want a sliding window of the last N impact-analysis runs (e.g. --rotate 100 keeps a rolling buffer of the last 100 snapshots, oldest dropped as new ones arrive). Only the OLDEST lines are evicted (head-of-file truncation); the most-recent N records are always preserved. Requires --append (rotation only makes sense for the streaming JSONL path; overwriting --output keeps a single record by definition).")
	cmd.Flags().BoolVar(&includePriority, "include-priority", false, "for --json: add a per-node 'priority' field to the envelope (canonical string: low/medium/high/urgent). Useful for jq pipelines that need to filter by priority without a per-node `tsk show --json <id>` round-trip. Historical default keeps the envelope minimal (id+title+done) so existing snapshot fixtures stay byte-identical.")
	cmd.Flags().BoolVar(&includeTags, "include-tags", false, "for --json: add a per-node 'tags' field to the envelope (alphabetized array of tag strings; empty array when the task has no tags). Sister of --include-priority, same opt-in shape so existing snapshot fixtures stay byte-identical when unset. Useful for jq pipelines that need to filter by tag (e.g. `select(.tags | index(\"urgent\"))`) without a per-node `tsk show --json <id>` round-trip. Composes with --include-priority and --compact-json; dangling-edge nodes (rendered as '(missing)') omit the field via omitempty since we don't have a task to read tags from.")
	cmd.Flags().BoolVar(&includeDue, "include-due", false, "for --json: add a per-node 'due' field to the envelope (canonical YYYY-MM-DD date string; field is omitted when the task has no due date). Sister of --include-tags / --include-priority — same opt-in shape so existing snapshot fixtures stay byte-identical when unset. Useful for jq pipelines that need to flag impact-analysis chains where something is due this week (e.g. `select(.due < \"2026-07-01\")`) without a per-node `tsk show --json <id>` round-trip. Composes with all other --include-* opt-ins and --compact-json; dangling-edge nodes (rendered as '(missing)') omit the field since we don't have a task to read due from.")
	cmd.Flags().BoolVar(&includeCompleted, "include-completed", false, "for --json: add a per-node 'completed' field to the envelope (canonical RFC3339 timestamp string; field is omitted when the task isn't done). Sister of --include-due / --include-tags / --include-priority — same opt-in shape so existing snapshot fixtures stay byte-identical when unset. Useful for completion-velocity analysis (`jq '[.nodes[] | select(.completed != null)] | length / (.nodes | length)'`), recently-completed sibling detection (which prereqs finished in the last 24h?), and CI gates that gate on \"this dependency chain is N% done\". Composes with all other --include-* opt-ins and --compact-json; dangling-edge nodes (rendered as '(missing)') omit the field since we don't have a task to read completed from.")
	cmd.Flags().BoolVar(&includeStarted, "include-started", false, "for --json: add a per-node 'started' field to the envelope (canonical RFC3339 timestamp string; field is omitted when the task isn't in-progress). Sister of --include-completed for the OPEN side of the work-state pair: --include-completed surfaces finish time, --include-started surfaces work-began time. Useful for elapsed-time analysis on currently-working tasks (`jq '.nodes[] | select(.started != null) | .id'`), \"what's in flight in this dep chain?\" gates, and pomodoro/timer integrations that need to know the start instant. Composes with all other --include-* opt-ins and --compact-json; dangling-edge nodes omit the field since we don't have a task to read started from.")
	cmd.Flags().BoolVar(&includePinned, "include-pinned", false, "for --json: add a per-node 'pinned' field to the envelope (boolean: true when the task is pinned via `tsk pin`, false otherwise; field is OMITTED when the flag isn't set). Sister of --include-priority / --include-tags / --include-due / --include-completed / --include-started — same opt-in shape so existing snapshot fixtures stay byte-identical when unset. Useful for jq pipelines that need to flag pinned tasks in an impact chain (e.g. `.nodes[] | select(.pinned)`), \"is anything important still blocking this release?\" gates, and CI scripts that want to spotlight pinned dependencies. Composes with all other --include-* opt-ins and --compact-json; dangling-edge nodes omit the field since we don't have a task to read pinned from. Modeled as a *bool with omitempty so 'flag off' (field absent) and 'flag on, task not pinned' (field present and false) are distinguishable in the JSON output — same pattern --include-tags uses for its '[]' vs 'no field' distinction.")
	cmd.Flags().BoolVar(&includeAll, "include-all", false, "for --json: turn on EVERY --include-* opt-in field at once (priority, tags, due, completed, started, pinned). Ergonomic shortcut for \"give me the full-fat envelope\" use cases — pre-commit dashboards, comprehensive snapshot tests, completion-velocity + elapsed-time analyses that need every axis. Equivalent to passing --include-priority --include-tags --include-due --include-completed --include-started --include-pinned together; setting --include-all alongside the individual flags is idempotent (true OR true == true). The default stays minimal so existing snapshot fixtures and jq pipelines that don't need the extra fields keep their byte-identical historical shape. Useful when scripting `jq` queries that join multiple axes (e.g. `select(.priority == \"urgent\" and .due < \"2026-07-01\" and .completed == null and .pinned)`) without remembering each opt-in flag name.")
	cmd.Flags().StringVar(&outputPath, "output", "", "write the rendered graph to this file instead of stdout; extension must match --format (.txt/.dot/.svg). With --json also writes the subgraph envelope (.json required, or .jsonl with --append). Useful for `tsk graph --format svg --output deps.svg` or `tsk graph --reachable 7 --json --output impact.json` without shell redirection.")
	cmd.Flags().StringVar(&filterCompletedSince, "filter-completed-since", "", "for --json: trim the subgraph envelope to nodes whose task was completed within this duration window (e.g. 7d, 24h, 2w, 1h30m). The ROOT id is always kept (the consumer asked about THIS root); every other node must be done AND completed-within-window to survive. Edges touching a filtered-out node are dropped (no point linking to a node we removed). Useful for completion-velocity dashboards (\"what shipped this week in this dep chain?\"), recent-impact reports (\"which prereqs just closed?\"), and change-summary CI gates. Composes with all --include-* opt-ins, --reachable / --upstream-of (both directions), --compact-json, --append (each appended record reflects the filter at call time). Empty (default) = no filter. The envelope gains a top-level `filter_completed_since` field naming the window in canonical humanized form when active, so scripts can distinguish a filtered from un-filtered envelope.")
	cmd.Flags().StringVar(&filterStartedSince, "filter-started-since", "", "for --json: trim the subgraph envelope to nodes whose task is currently in-progress AND was started within this duration window (e.g. 7d, 24h, 2w, 1h30m). The OPEN sister of --filter-completed-since: completed-since surfaces RECENTLY-FINISHED work, started-since surfaces RECENTLY-BEGUN work. Useful for \"what's actively in flight in this dep chain?\" reports, currently-working impact analysis, and pomodoro/elapsed-time dashboards. The ROOT id is always kept (same semantic as completed-since). Edges touching a filtered-out node are dropped. When BOTH --filter-completed-since and --filter-started-since are set, the composition semantic is UNION (logical OR) — a node survives if EITHER recently-completed OR recently-started. This is the only useful composition since done and in-progress are mutually exclusive states on the same task. Composes with all --include-* opt-ins, --reachable / --upstream-of (both directions), --compact-json, --append. Empty (default) = no filter. The envelope gains a top-level `filter_started_since` field naming the window in canonical humanized form when active, so scripts can distinguish either filter (or both) by which marker fields are present.")
	return cmd
}

// mergeHighlightTag extends the highlight set with every task in the
// store carrying any of the named tags (case-insensitive). When the
// raw input is empty, the set is returned unchanged so callers don't
// have to branch on "feature in use".
//
// The raw input is a CSV of one or more tag names (comma-separated):
//
//	mergeHighlightTag(s, set, "")           → no-op
//	mergeHighlightTag(s, set, "release")    → single tag
//	mergeHighlightTag(s, set, "release,p0") → UNION of both tags
//
// Multi-tag matches a TASK if it carries ANY of the listed tags
// (logical OR). Useful for "spotlight everything tagged release OR
// p0" without typing ids individually. Mirrors the multi-id CSV
// extension on --highlight that landed in tick #17.
//
// The result is the UNION of explicit ids (via --highlight) and
// every tag-matched id (via --highlight-tag CSV) — all render with
// the same gold-bold style, so the user sees ONE coherent spotlight
// group rather than competing decorations.
//
// Missing tag (none of the listed names match any task in the
// store) is not an error here: the spotlight is a render decoration,
// not a hard predicate. Empty/whitespace tokens in the CSV are
// quietly dropped so `--highlight-tag release,,p0` doesn't surprise.
func mergeHighlightTag(s *store.Store, highlightSet map[int]bool, tagsRaw string) map[int]bool {
	return mergeTagsIntoSet(s, highlightSet, tagsRaw)
}

// mergeDimTag is the inverse sister: extend the DIM set with every
// task carrying any of the named tags. Same CSV semantics, same
// case-insensitive match, same "missing tag is not an error" policy.
// Shares the mergeTagsIntoSet core with mergeHighlightTag so the
// two flags behave symmetrically and a fix to one applies to both.
func mergeDimTag(s *store.Store, dimSet map[int]bool, tagsRaw string) map[int]bool {
	return mergeTagsIntoSet(s, dimSet, tagsRaw)
}

// mergeTagsIntoSet is the shared "extend an id-set by tag-CSV match"
// body behind --highlight-tag and --dim-tag. Returns the original set
// unchanged when the input is empty (the "feature not in use" branch).
// When the merged set is empty (caller-supplied set was nil AND no
// task carries any of the listed tags), returns nil so downstream
// styling branches fire the "no match" path — matches parseHighlight
// CSV's "empty returns nil" contract so the two helpers compose
// seamlessly.
//
// Tokenization: comma-split, trim whitespace, drop empties. So
// "release, p0," produces the same parse as "release,p0". Case-
// insensitive match is delegated to model.Task.HasTag (the same
// resolver `tsk ls --tag` uses).
func mergeTagsIntoSet(s *store.Store, set map[int]bool, tagsRaw string) map[int]bool {
	tags := splitTagCSV(tagsRaw)
	if len(tags) == 0 {
		return set
	}
	if set == nil {
		set = make(map[int]bool)
	}
	for _, t := range s.Tasks {
		for _, tag := range tags {
			if t.HasTag(tag) {
				set[t.ID] = true
				break
			}
		}
	}
	if len(set) == 0 {
		return nil
	}
	return set
}

// splitTagCSV tokenizes the CSV tag input. Empty / whitespace-only
// tokens are dropped so `--highlight-tag release,,p0` (a typo) and
// `--highlight-tag release` both work without surprises. Returns
// an empty slice when the input has no usable tokens.
func splitTagCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
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

// parseHighlightCSV converts a comma-separated highlight id list to
// a set (map). Returns nil for an empty input (no highlight). Validates
// that every id is positive and exists in the store — surfaces typos
// at the flag layer with a clear error rather than silently rendering
// a graph with no spotlight.
//
// The leading "#" prefix is tolerated so `--highlight #3,#5` works the
// same as `--highlight 3,5`, matching `tsk depend --on` and friends.
// Whitespace around tokens is trimmed for forgiving shell-quoted input.
// Duplicates collapse: `--highlight 3,3,3` is the same as `--highlight 3`.
//
// Why a set rather than a slice? printGraphDOT membership lookup is
// O(1) per node — the rendering loop iterates every used id, and a
// linear scan over an N-id highlight list would balloon to O(N*M) on
// big graphs with multi-id spotlights. The set keeps it linear.
func parseHighlightCSV(s *store.Store, raw string) (map[int]bool, error) {
	return parseCSVIDSet(s, raw, "--highlight")
}

// parseDimCSV is the inverse sister of parseHighlightCSV: same CSV
// parsing rules (#-prefix tolerance, whitespace trimming, dup-
// collapse, existence validation), but produces the SET of ids the
// caller wants to dim. Returns nil on empty input so callers can
// short-circuit the "no dim" path without branching on len.
//
// Shares the CSV parsing core with parseHighlightCSV via
// parseCSVIDSet — keeps the two flags' error messages perfectly
// symmetric ("--dim: invalid task id" vs "--highlight: invalid
// task id"), and means a fix to one tightens the other. The
// previously inline parseHighlightCSV body has moved into the
// shared helper.
func parseDimCSV(s *store.Store, raw string) (map[int]bool, error) {
	return parseCSVIDSet(s, raw, "--dim")
}

// parseCSVIDSet is the shared CSV id-set parser behind --highlight
// and --dim. flagName is the user-facing flag label used in error
// messages so a typo on `--dim` surfaces as "--dim: ..." not as a
// confusingly-attributed "--highlight: ..." message.
func parseCSVIDSet(s *store.Store, raw, flagName string) (map[int]bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	out := make(map[int]bool, 4)
	for _, tok := range strings.Split(raw, ",") {
		tok = strings.TrimSpace(tok)
		tok = strings.TrimPrefix(tok, "#")
		if tok == "" {
			continue
		}
		n, err := strconvAtoiPos(tok)
		if err != nil || n == 0 {
			return nil, usageErrorf("%s: invalid task id %q", flagName, tok)
		}
		if s.ByID(n) == nil {
			return nil, fmt.Errorf("%s: no task with id %d in %s", flagName, n, s.Path)
		}
		out[n] = true
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// rejectDimHighlightOverlap returns a usage error if any id appears
// in BOTH the dim and highlight sets. A node can't be simultaneously
// spotlighted AND backgrounded — that's contradictory intent, and
// silently letting one style win would surprise the user. Returns
// nil when either set is empty (no overlap possible) or when the
// intersection is empty.
//
// Error message lists every overlapping id (sorted ascending for
// determinism) so the user can see all the conflicts in one shot
// rather than discovering them one fix at a time.
func rejectDimHighlightOverlap(dimSet, highlightSet map[int]bool) error {
	if len(dimSet) == 0 || len(highlightSet) == 0 {
		return nil
	}
	overlap := make([]int, 0)
	for id := range dimSet {
		if highlightSet[id] {
			overlap = append(overlap, id)
		}
	}
	if len(overlap) == 0 {
		return nil
	}
	sort.Ints(overlap)
	idStrs := make([]string, len(overlap))
	for i, id := range overlap {
		idStrs[i] = fmt.Sprintf("#%d", id)
	}
	return usageErrorf("--dim and --highlight overlap on %s (a node can't be both spotlighted and dimmed)", strings.Join(idStrs, ", "))
}

// graphEdge represents a single from->to dependency arrow. Sorted
// (from asc, then to asc) before rendering so output is reproducible.
type graphEdge struct {
	from int
	to   int
}

// collectGraphEdges walks every task in the store and returns the full
// edge list. With openOnly, we skip done tasks AND any edges that
// point to done tasks (the dep is already satisfied — no longer
// constraining the graph). This is the "show me what's actively
// blocking real work" mode.
//
// Dangling refs (dep id with no task in the store) are tolerated:
// the edge is emitted in the default mode so the user can see "this
// id is referenced but missing", but dropped in openOnly mode (a
// missing dep is treated as satisfied per toggle.go's semantics).
func collectGraphEdges(s *store.Store, openOnly bool) []graphEdge {
	edges := make([]graphEdge, 0)
	for _, t := range s.Tasks {
		if openOnly && t.Done {
			continue
		}
		if !t.HasDependencies() {
			continue
		}
		for _, dep := range t.DependsOn {
			if openOnly {
				bt := s.ByID(dep)
				if bt == nil || bt.Done {
					continue
				}
			}
			edges = append(edges, graphEdge{from: t.ID, to: dep})
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].from != edges[j].from {
			return edges[i].from < edges[j].from
		}
		return edges[i].to < edges[j].to
	})
	return edges
}

// filterReachableEdges keeps only the edges that participate in the
// subgraph reachable from `root` via DependsOn. Algorithm:
//
//  1. BFS from `root` over the source->target edges to compute the
//     set of every transitively-reachable node (the root itself
//     plus every prereq, every prereq's prereq, etc).
//  2. Drop every edge whose source is NOT in that set.
//
// Note: this is the "downstream" reachability (where `root` is the
// source). It answers "what does #X transitively depend on?" — the
// matching question for the user typing `--reachable 7`. The reverse
// ("what transitively depends on #X?") is `filterUpstreamOfEdges`
// (powering `--upstream-of`).
//
// Edges already sorted by collectGraphEdges; we preserve that order
// so DOT/ASCII output stays deterministic regardless of which root
// the user picks.
func filterReachableEdges(s *store.Store, edges []graphEdge, root int) []graphEdge {
	// Build outgoing adjacency from the edge list.
	out := make(map[int][]int)
	for _, e := range edges {
		out[e.from] = append(out[e.from], e.to)
	}
	// BFS to find the reachable node set.
	visited := map[int]bool{root: true}
	queue := []int{root}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for _, next := range out[curr] {
			if visited[next] {
				continue
			}
			visited[next] = true
			queue = append(queue, next)
		}
	}
	// Filter the edge list — keep only edges whose source is in the
	// reachable set. (Target reachability is implied: if from is
	// reachable and we kept the edge, the BFS already added the
	// target to visited.)
	filtered := make([]graphEdge, 0, len(edges))
	for _, e := range edges {
		if visited[e.from] {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// filterUpstreamOfEdges is the inverse of filterReachableEdges. It
// keeps only the edges that participate in the UPSTREAM subgraph of
// `root` — every task whose transitive prereq chain eventually names
// `root`, plus `root` itself.
//
// Algorithm:
//
//  1. Build the INCOMING adjacency from the edge list (target -> [sources]).
//     In tsk's DependsOn convention, an edge A->B means "A depends on B",
//     so B's incoming sources are the tasks that point at B in their
//     prereqs. The reverse BFS from root over incoming gives every
//     transitive dependent.
//  2. BFS from `root` over the incoming adjacency to compute the
//     set of every transitively-upstream node.
//  3. Keep every edge whose source AND target both land in the upstream
//     set — i.e. the edge participates in the upstream subgraph.
//     Edges where only one endpoint is upstream (e.g. a dependent's
//     OTHER prereq that's unrelated to root) are dropped, so the
//     rendered subgraph stays focused on the chain leading to root.
//
// Why "source AND target both upstream" vs "source upstream"? Because
// upstream nodes typically have additional prereqs OUTSIDE the
// upstream chain (e.g. "ship feature" depends on "deploy" depends on
// root, but "ship feature" might also depend on "write release notes"
// which is unrelated to root). Including those off-chain edges would
// pollute the impact-analysis view. By restricting to source-AND-
// target the rendered subgraph is purely the "what's blocked by root"
// chain — the answer to the user's actual question.
//
// Pairs with `tsk depend <id> --upstream`, which only shows the
// direct dependents (one step). --upstream-of walks the full chain
// in the same DOT layout used for the whole-store graph.
func filterUpstreamOfEdges(s *store.Store, edges []graphEdge, root int) []graphEdge {
	// Build incoming adjacency from the edge list: target -> sources.
	in := make(map[int][]int)
	for _, e := range edges {
		in[e.to] = append(in[e.to], e.from)
	}
	// BFS from root over incoming → every transitively-upstream node.
	visited := map[int]bool{root: true}
	queue := []int{root}
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for _, next := range in[curr] {
			if visited[next] {
				continue
			}
			visited[next] = true
			queue = append(queue, next)
		}
	}
	// Keep only edges where BOTH endpoints are in the upstream set —
	// see the doc comment for the rationale (off-chain prereqs would
	// dilute the impact-analysis view).
	filtered := make([]graphEdge, 0, len(edges))
	for _, e := range edges {
		if visited[e.from] && visited[e.to] {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// resolveGraphFormat normalizes --format to the canonical lowercase
// keyword. Empty defaults to ascii. Unknown values are rejected
// up-front (usage-coded so main.go exits 2).
//
// Supported formats:
//   - "ascii" (default; aliases "text", "txt"): adjacency listing
//   - "dot"   (alias "graphviz"):              GraphViz DOT source
//   - "svg":                                    self-contained SVG
//     emitted directly by a tiny embedded layered-layout renderer
//     (no GraphViz dependency). Useful for `tsk graph --format svg
//     > deps.svg` without piping through `dot -Tsvg`. The renderer
//     is intentionally simple — for production-quality layouts the
//     DOT format piped to GraphViz still wins, but the embedded
//     path is great for quick visual inspection in environments
//     without graphviz installed (CI containers, fresh dev boxes).
func resolveGraphFormat(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "ascii", "text", "txt":
		return "ascii", nil
	case "dot", "graphviz":
		return "dot", nil
	case "svg":
		return "svg", nil
	}
	return "", usageErrorf("unknown --format %q (want ascii, dot, or svg)", raw)
}

// validateGraphOutputExtension surfaces a clear usage error when the
// caller passes --output with an extension that doesn't match the
// resolved --format. Catches the silent-footgun case where a user
// types `--format svg --output deps.dot`: without this check, the
// file lands containing SVG bytes under a `.dot` filename, breaking
// every downstream tool (`dot -Tpng deps.dot` would fail in a
// confusing way; an SVG viewer ignoring the extension would still
// open it; a content-type-sensitive web server would mislabel it).
//
// Extension rules:
//
//	ascii  -> .txt or no extension (text files are commonly extensionless)
//	dot    -> .dot or .gv (GraphViz's two canonical extensions)
//	svg    -> .svg only (the W3C standard extension)
//
// Case-insensitive on the extension so `.SVG`/`.Dot` pass. An empty
// path is rejected at the caller layer (we only reach here when
// outputPath != ""). Paths with multiple dots resolve via
// filepath.Ext which returns the last extension, matching the
// "deps.tagged.svg" pattern correctly.
//
// Why ascii + no-extension OK? Because `--format ascii --output graph`
// is a perfectly reasonable invocation (text adjacency listings are
// often dumped to extensionless names like `notes` or `summary`).
// The other two formats have a real on-disk convention worth
// enforcing; ascii is the most permissive of the three.
//
// Future formats slot in by adding a case here; the helper is the
// single source of truth for the format <-> extension mapping so
// drift between flag-help and validation can't sneak in.
func validateGraphOutputExtension(path, format string) error {
	ext := strings.ToLower(filepath.Ext(path))
	switch format {
	case "ascii":
		if ext == "" || ext == ".txt" {
			return nil
		}
		return usageErrorf("--format ascii expects --output ending in .txt or no extension, got %q", ext)
	case "dot":
		if ext == ".dot" || ext == ".gv" {
			return nil
		}
		return usageErrorf("--format dot expects --output ending in .dot or .gv, got %q", ext)
	case "svg":
		if ext == ".svg" {
			return nil
		}
		return usageErrorf("--format svg expects --output ending in .svg, got %q", ext)
	}
	// Defensive: a future format keyword we haven't added a case
	// for shouldn't fall through silently — surface the gap.
	return usageErrorf("--output extension validation has no rule for --format %q", format)
}

// validateGraphOutputJSONExtension surfaces a clear usage error
// when --json + --output point at a path whose extension isn't
// .json (or, in --append mode, also .jsonl). Catches the silent-
// footgun case where a user types `--reachable 7 --json --output
// impact.svg`: without this check, the file lands containing JSON
// bytes under a `.svg` name, which breaks every downstream tool
// inspecting it by extension (an SVG viewer would refuse to render
// it; a JSON-typed pipeline would skip it).
//
// Case-insensitive on the extension so `.JSON` / `.JSONL` pass.
// Bare paths (no extension) are REJECTED — JSON / JSONL have a real
// on-disk convention worth enforcing, and extensionless dumps usually
// mean a typo (the user almost certainly meant `--output impact.json`
// or `--output snap.jsonl`). This is stricter than ASCII's
// "extensionless OK" because the JSON content has no inherent
// text-fallback identity.
//
// appendMode flips the matrix: when set, .jsonl is the CANONICAL
// extension (JSONL = "JSON Lines", one record per line — every
// streaming JSON pipeline uses this convention). .json is still
// accepted in append mode for users who already named their file
// that way — the on-disk shape is functionally JSONL either
// extension, but .jsonl is the more honest label.
//
// Future format gains slot in via validateGraphOutputExtension's
// switch + a sibling helper here; the helpers stay narrow so
// drift between flag-help and validation can't sneak in.
func validateGraphOutputJSONExtension(path string, appendMode bool) error {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".json" {
		return nil
	}
	if appendMode && ext == ".jsonl" {
		return nil
	}
	if appendMode {
		return usageErrorf("--json --output --append expects path ending in .json or .jsonl, got %q", ext)
	}
	return usageErrorf("--json --output expects path ending in .json, got %q", ext)
}

// rotateJSONLFile caps a JSONL file at <path> to keep the most-recent
// keepN lines, dropping the oldest. Returns the number of lines
// dropped (0 if no rotation was needed; positive when trimming
// occurred). Caller has already validated keepN > 0 — passing 0 or
// negative is a no-op (returns 0, nil) for defensive ergonomics.
//
// Algorithm: read the whole file, split on \n, count non-empty
// lines (since the encoder always terminates with \n there's a
// trailing empty token to discard), keep only the last keepN, then
// atomically replace the file via write-to-.tmp + rename. The
// atomic-rename pattern matches the rest of tsk's I/O contract: a
// crash mid-rotation leaves the previous (unrotated) file
// untouched, never a partially-rotated mess. Tradeoff: O(file
// size) memory + O(file size) write on every rotated call, which
// is fine for the snapshot-history use case (kilobytes to single-
// digit megabytes typical), and avoids the complexity of
// seek+truncate-from-middle that file systems don't natively
// support.
//
// Files smaller than keepN+1 records are left untouched (returns
// 0 lines dropped). This is the common-case fast path for
// callers using --rotate as a safety cap rather than as a
// constantly-active eviction policy.
//
// Empty/missing files are not an error (callers may set --rotate
// before any append has populated the file; bail cleanly so the
// first append simply creates the file and the rotation pass
// finds nothing to do).
func rotateJSONLFile(path string, keepN int) (int, error) {
	if keepN <= 0 {
		return 0, nil
	}
	body, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("read %s: %w", path, err)
	}
	// Split and filter empties (trailing newline produces one).
	rawLines := strings.Split(string(body), "\n")
	lines := rawLines[:0]
	for _, l := range rawLines {
		if l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) <= keepN {
		return 0, nil
	}
	dropped := len(lines) - keepN
	keep := lines[dropped:]
	// Rebuild file body: each retained line followed by \n,
	// matching the on-disk JSONL convention json.Encoder produces.
	var sb strings.Builder
	for _, l := range keep {
		sb.WriteString(l)
		sb.WriteString("\n")
	}
	// Atomic replace: write to a sibling .tmp, then rename. On
	// POSIX rename is atomic within a filesystem, so a crash
	// mid-write leaves the original file intact.
	tmp := path + ".rotate.tmp"
	if err := os.WriteFile(tmp, []byte(sb.String()), 0o644); err != nil {
		return 0, fmt.Errorf("write tmp %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		// Try to clean up the orphan tmp so future runs don't
		// accumulate them. Best-effort: a remove failure isn't
		// the user's problem here, the original file is intact.
		_ = os.Remove(tmp)
		return 0, fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return dropped, nil
}

// emitGraph dispatches based on the resolved format. When rootID
// is set (>0) and the filter produced zero edges, the message is
// more specific so the user understands "the root has no prereqs"
// vs the whole store being empty. rootKind tells the empty branch
// which direction was asked — "reachable" gets "no dependencies
// reachable from #N" (downstream), "upstream-of" gets "no tasks
// depend on #N" (upstream). highlightSet is the optional focus-id
// set (only meaningful for DOT format); nil/empty means no
// highlight. Multi-id sets render every member with the same
// spotlight style, useful for "show me this whole subset" on a
// complex graph. dimSet is the inverse — ids to render in a quiet
// gray fill + dashed border to push them to the background; nil/
// empty means no dim. Overlap with highlightSet is the caller's
// responsibility to reject (see rejectDimHighlightOverlap).
func emitGraph(w io.Writer, s *store.Store, edges []graphEdge, format string, rootID int, rootKind string, highlightSet, dimSet map[int]bool) error {
	if len(edges) == 0 {
		if rootID > 0 {
			switch rootKind {
			case "upstream-of":
				pf(w, "no tasks depend on #%d\n", rootID)
			default:
				pf(w, "no dependencies reachable from #%d\n", rootID)
			}
			return nil
		}
		pln(w, "no dependencies")
		return nil
	}
	if format == "dot" {
		return printGraphDOT(w, s, edges, highlightSet, dimSet)
	}
	if format == "svg" {
		return printGraphSVG(w, s, edges, highlightSet, dimSet)
	}
	return printGraphASCII(w, s, edges)
}

// printGraphASCII prints an adjacency listing — one line per source
// task with all its dep ids and a short title for the source.
//
// Layout:
//
//	#3 -> #1, #2    research the API (3 prereqs)
//	#5 -> #3        ship the feature
//
// Open tasks come first (the actionable ones); done tasks land in a
// "(done)" section so the active work isn't visually buried.
func printGraphASCII(w io.Writer, s *store.Store, edges []graphEdge) error {
	bySource := groupEdgesBySource(edges)
	openSources := make([]int, 0)
	doneSources := make([]int, 0)
	for from := range bySource {
		t := s.ByID(from)
		if t != nil && t.Done {
			doneSources = append(doneSources, from)
		} else {
			openSources = append(openSources, from)
		}
	}
	sort.Ints(openSources)
	sort.Ints(doneSources)
	for _, from := range openSources {
		printGraphRow(w, s, from, bySource[from])
	}
	if len(doneSources) > 0 {
		if len(openSources) > 0 {
			pln(w, "")
		}
		pln(w, "(done):")
		for _, from := range doneSources {
			printGraphRow(w, s, from, bySource[from])
		}
	}
	return nil
}

// groupEdgesBySource collapses a flat edge list into a map keyed by
// the source id. The dep-id slices stay sorted (edges came in sorted).
func groupEdgesBySource(edges []graphEdge) map[int][]int {
	out := make(map[int][]int)
	for _, e := range edges {
		out[e.from] = append(out[e.from], e.to)
	}
	return out
}

// printGraphRow renders one "#N -> #X, #Y  title" line. The title is
// included so the user doesn't have to cross-reference ids to know
// what the line is about.
func printGraphRow(w io.Writer, s *store.Store, from int, deps []int) {
	t := s.ByID(from)
	depStrs := make([]string, len(deps))
	for i, d := range deps {
		depStrs[i] = fmt.Sprintf("#%d", d)
	}
	title := ""
	if t != nil {
		title = "  " + t.Title
	}
	pf(w, "#%d -> %s%s\n", from, strings.Join(depStrs, ", "), title)
}

// printGraphDOT emits GraphViz DOT syntax. The directed-graph
// convention here is "A -> B means A depends on B" — i.e. the arrow
// points TOWARDS the prerequisite, matching how `tsk depend <id>
// --on X` reads ("id depends on X").
//
// Node styling:
//   - done tasks: filled gray (the dep is satisfied)
//   - open with at least one open prereq (blocked): red outline
//   - open with no open prereqs (actionable): default outline
//   - dim target (when id is in dimSet): light-gray fill + dashed
//     gray border + gray font. Reads as "background scaffolding"
//     so the foreground nodes stand out. Lower-priority than
//     highlight: callers must reject overlap up-front (a node
//     can't be both spotlighted and backgrounded). Higher-
//     priority than the blocked-red and done-gray defaults: the
//     point of --dim is to suppress those decorations so the
//     dimmed node really does recede.
//   - highlight target (when id is in highlightSet): gold fill + bold black
//     border, OVERRIDES every other style. Picked because gold/amber
//     reads as the "this one's important" color in DOT renders
//     without colliding with the red "blocked" outline. The bold
//     border keeps it readable even if the renderer ignores fill
//     (some SVG viewers downsample colors aggressively).
//
// Long titles are truncated to 40 chars at the node level so the
// rendered graph stays readable.
//
// highlightSet=nil/empty means no highlight; any id present in the
// set wraps that node (whether it's a source, target, or both) in
// the focus style. Multi-id sets render every member with the SAME
// spotlight style — useful for "show me this whole subset" reviews
// where several related tasks all deserve the eye. Ids missing
// from the rendered graph (e.g. filtered out by --reachable rooted
// elsewhere) are silently ignored — at the command-flag layer we
// already validated existence in the store, so a missing match
// here is just "you asked to spotlight something not in this
// subgraph". The graph still renders cleanly in that case.
//
// dimSet=nil/empty means no dim; any id present is rendered in the
// quiet background style. Same "ids missing from this subgraph are
// silently ignored" policy as highlight — the validation was done
// at the flag layer.
func printGraphDOT(w io.Writer, s *store.Store, edges []graphEdge, highlightSet, dimSet map[int]bool) error {
	pln(w, "digraph tsk {")
	pln(w, "  rankdir=LR;")
	pln(w, `  node [shape=box, fontname="Helvetica", fontsize=10];`)
	// Compute the set of unique node ids we'll emit (sources + targets).
	used := make(map[int]bool)
	for _, e := range edges {
		used[e.from] = true
		used[e.to] = true
	}
	ids := make([]int, 0, len(used))
	for id := range used {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	// Pre-compute blocked-set for styling (open task with at least one open prereq).
	blocked := make(map[int]bool)
	for _, e := range edges {
		from := s.ByID(e.from)
		to := s.ByID(e.to)
		if from == nil || from.Done {
			continue
		}
		if to == nil || !to.Done {
			blocked[e.from] = true
		}
	}
	for _, id := range ids {
		t := s.ByID(id)
		label := fmt.Sprintf("#%d", id)
		style := ""
		if t == nil {
			label = fmt.Sprintf("#%d (missing)", id)
			style = ` color="gray", style="dashed", fontcolor="gray"`
		} else {
			label = fmt.Sprintf("#%d %s", id, truncateForDOT(t.Title, 40))
			switch {
			case t.Done:
				style = ` style="filled", fillcolor="lightgray"`
			case blocked[id]:
				style = ` color="red"`
			}
		}
		// Dim sits BETWEEN the default styles and the highlight
		// override: it replaces the done/blocked/actionable
		// decoration so the node visually recedes, but the
		// highlight check below still wins on overlap (which
		// the caller has already rejected at the flag layer
		// anyway).
		if dimSet[id] {
			style = ` style="filled,dashed", fillcolor="lightgray", color="gray", fontcolor="gray"`
		}
		// Highlight overrides every other style so the focus task(s)
		// always read as the focus regardless of done/blocked
		// state. Gold/amber + bold border picks the "look here"
		// signal cleanly without clashing with the red blocked
		// outline. Same style for every id in the set so a multi-
		// task subset reads as ONE highlighted group rather than
		// per-node noise.
		if highlightSet[id] {
			style = ` style="filled,bold", fillcolor="gold", color="black", penwidth=2`
		}
		pf(w, "  %d [label=%q%s];\n", id, label, style)
	}
	for _, e := range edges {
		pf(w, "  %d -> %d;\n", e.from, e.to)
	}
	pln(w, "}")
	return nil
}

// truncateForDOT shortens a title to max runes with an ellipsis, and
// escapes the few characters DOT's quoted-string syntax cares about.
// fmt's %q already handles \\ and \" so we only need length capping.
func truncateForDOT(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-1]) + "…"
}

// subgraphNode is one node in the JSON subgraph envelope. Stable
// schema — id + title + done flag, all a script needs to fan out
// per-task lookups. Matches the field shape used by `tsk show
// --json` and `tsk wip --json` so users with existing jq pipelines
// can reuse selector patterns.
//
// Priority is OPT-IN via the --include-priority flag. The historical
// default keeps the envelope minimal (id+title+done) so existing
// snapshot fixtures and jq pipelines stay stable. When opted in,
// every node gains a "priority" field carrying the canonical string
// form ("low"/"medium"/"high"/"urgent") — same shape `tsk show --json`
// uses, so a jq selector like `.nodes[] | select(.priority == "urgent")`
// works identically across the two surfaces. Dangling-edge nodes
// (rendered as "(missing)") get priority="" since we don't have a
// task to read from — consumers can filter those out with
// `select(.priority != "")`.
//
// The `omitempty` tag keeps the default envelope byte-identical to
// the historical shape (no "priority":"" appearing when the flag
// isn't set). When the flag IS set, every real task gets a
// non-empty priority value (model.Priority always serializes to one
// of the four named values), so omitempty doesn't suppress the
// field for actual tasks — only for the "(missing)" placeholder.
//
// Tags is OPT-IN via the --include-tags flag, same shape:
// when set, every real task's node gains a "tags" field carrying
// the alphabetized tag array (empty array []string{} for tasks
// with no tags so jq's `.tags | length` works without null-
// crashing). Modeled as a *[]string pointer instead of a plain
// []string so we can DISTINGUISH "field intentionally omitted
// (flag off)" — nil pointer drops the field via omitempty — from
// "field present and empty" — non-nil pointer to []string{} which
// serializes as `[]`. A plain []string with omitempty would drop
// the empty case (collapsing both meanings into "no field"), which
// is the opposite of what jq pipelines need. Dangling-edge
// "(missing)" nodes leave the pointer nil — there's no task to
// read tags from, so omitting the field is the honest answer.
// Useful for downstream filters like `jq '.nodes[] | select(.tags
// | index("urgent"))'` that need tags without falling back to a
// `tsk show --json <id>` round-trip per node.
//
// Due is OPT-IN via the --include-due flag, third opt-in field:
// when set, real-task nodes with a due date gain a "due" field
// carrying the canonical YYYY-MM-DD string (matches model.DateLayout,
// same shape `tsk show` uses). Tasks WITHOUT a due date leave the
// field absent entirely — semantically "no due" is meaningfully
// different from "due on date X", and a downstream jq comparison
// like `select(.due < "2026-07-01")` should naturally skip nodes
// with no due field rather than tripping a type error. Stored as a
// plain string with omitempty (a missing-due task gets ""; the
// omitempty tag drops the field). Dangling-edge "(missing)" nodes
// leave the field absent for the same reason — there's no task to
// read due from. Useful for impact-analysis chains where you want
// to know "what depends on something due this week?" without a
// per-node `tsk show --json <id>` round-trip.
type subgraphNode struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Done      bool      `json:"done"`
	Priority  string    `json:"priority,omitempty"`
	Tags      *[]string `json:"tags,omitempty"`
	Due       string    `json:"due,omitempty"`
	Completed string    `json:"completed,omitempty"`
	Started   string    `json:"started,omitempty"`
	Pinned    *bool     `json:"pinned,omitempty"`
}

// subgraphEdge is one directed dep edge in the JSON subgraph
// envelope. from depends on to (i.e. from -> to means "from is
// blocked on to"), matching the tsk DependsOn convention.
type subgraphEdge struct {
	From int `json:"from"`
	To   int `json:"to"`
}

// subgraphDoc is the JSON envelope for `tsk graph --reachable <id>
// --json` and `tsk graph --upstream-of <id> --json`. Stable schema:
//   - root_id     : the id the subgraph was rooted at
//   - direction   : "reachable" (downstream) or "upstream-of" (upstream)
//   - nodes       : every task that appears in the subgraph, sorted
//     asc by id for determinism
//   - edges       : every dep edge in the subgraph, sorted by
//     (from asc, then to asc) for determinism
//   - filter      : "open" when --open was passed (so a consumer can
//     tell whether done-task noise was filtered) or
//     omitted otherwise
//
// Empty nodes/edges are emitted as empty arrays (not null) so jq
// pipelines iterating them don't crash. The root node ALWAYS
// appears in nodes even when there are no edges (the user asked
// about THIS root; \"nothing depends on it\" is itself a useful
// answer for the impact-analysis use case).
type subgraphDoc struct {
	RootID               int            `json:"root_id"`
	Direction            string         `json:"direction"`
	Nodes                []subgraphNode `json:"nodes"`
	Edges                []subgraphEdge `json:"edges"`
	Filter               string         `json:"filter,omitempty"`
	FilterCompletedSince string         `json:"filter_completed_since,omitempty"`
	FilterStartedSince   string         `json:"filter_started_since,omitempty"`
}

// emitSubgraphJSON renders the stable JSON envelope for the
// --reachable/--upstream-of subgraph extractors. Designed for
// scripted impact-analysis: a single command yields the complete
// "what depends on X" or "what does X depend on" answer in a
// machine-readable shape, so pre-commit hooks, CI gates, and
// release-impact tools don't have to parse DOT or ASCII output.
//
// The root node is always included, even when edges is empty —
// the user asked about this specific root; the answer "no other
// tasks are involved" is itself useful (empty nodes would be a
// confusing degenerate case). Sorting is deterministic: nodes by
// id ascending, edges by (from, to) ascending — matches the
// ordering printGraphASCII uses.
//
// rootKind is the subgraph direction ("reachable" or "upstream-of")
// — it surfaces as direction in the envelope so a downstream
// consumer can tell which question the preview answers without
// having to know which CLI flag was passed.
//
// compact=true switches the encoder from the default two-space
// indented form to a single-line "no whitespace, no indent" form
// — useful for JSONL pipelines where each line must be a
// self-contained record (a multi-line indented JSON would corrupt
// every consumer that splits on \n). The contract: indent mode
// produces the historical bytes (regression test guard via
// TestGraphJSONOutputBytesMatchStdout) so existing fixtures still
// pass; compact mode is strictly opt-in via --compact-json.
//
// includePriority=true adds the per-task "priority" field to every
// node in the envelope. The default (false) keeps the historical
// minimal shape (id+title+done) so existing snapshot fixtures and
// jq pipelines stay byte-identical. When opted in, every real
// task's node gains a "priority" carrying the canonical string
// form ("low"/"medium"/"high"/"urgent"); dangling-edge nodes
// rendered as "(missing)" get priority="" since we don't have a
// task to read from. Useful for downstream filters like `jq
// '.nodes[] | select(.priority == "urgent")'` that need priority
// without falling back to a `tsk show --json <id>` round-trip per
// node.
//
// includeTags=true is the sister flag: adds the per-task "tags"
// field to every real-task node (alphabetized; empty `[]` for
// tasks with no tags so `.tags | length` works without null-
// crashing). Dangling-edge "(missing)" nodes leave the field
// omitted entirely — we don't have a task to read tags from, and
// guessing would mislead. Composes cleanly with --include-priority
// (both modifiers are independent opt-ins) and --compact-json
// (same single-line shape with both fields inline).
//
// includeDue=true is the third opt-in: adds a per-task "due"
// field carrying the canonical YYYY-MM-DD string when set. Tasks
// without a due date leave the field absent (omitempty drops the
// empty string) — semantically "no due" differs from "due on date
// X" and jq comparisons like `select(.due < "2026-07-01")` should
// naturally skip nodes with no due field. Composes with all other
// --include-* opt-ins (each is an independent boolean modifier).
//
// includeCompleted=true is the fourth opt-in: adds a per-task
// "completed" field carrying the RFC3339 timestamp when the task
// is done. Tasks not yet done leave the field absent (omitempty
// drops the empty string) — the "still open" case is naturally
// distinguishable from "completed at time X". Useful for
// completion-velocity analysis, "what finished in the last 24h"
// gates, and CI hooks that need to know how much of a dep chain
// has shipped. Composes with all other --include-* opt-ins.
//
// includeStarted=true is the fifth opt-in: adds a per-task
// "started" field carrying the RFC3339 timestamp when the task
// is in-progress. Sister of includeCompleted for the OPEN side
// of the work-state pair: completed surfaces finish time,
// started surfaces work-began time. Useful for elapsed-time
// analysis on currently-working tasks and "what's in flight in
// this chain?" gates. Composes with all other --include-* opt-ins.
func emitSubgraphJSON(w io.Writer, s *store.Store, edges []graphEdge, rootID int, rootKind string, open, compact, includePriority, includeTags, includeDue, includeCompleted, includeStarted, includePinned bool, filterCompletedActive bool, filterCompletedDur time.Duration, filterStartedActive bool, filterStartedDur time.Duration) error {
	// Collect every node that appears (sources + targets), plus
	// the root itself (so the empty-edges case still yields a
	// useful one-node response).
	used := make(map[int]bool)
	used[rootID] = true
	for _, e := range edges {
		used[e.from] = true
		used[e.to] = true
	}
	ids := make([]int, 0, len(used))
	for id := range used {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	// --filter-completed-since / --filter-started-since filtering:
	// each recency filter trims to nodes matching its predicate
	// (done+recently-completed for completed-since, in-progress+
	// recently-started for started-since). The ROOT id is ALWAYS
	// kept regardless of either filter — the consumer asked about
	// THIS root; the answer "your root isn't itself recently-active
	// but here's the recently-active subset around it" is more
	// useful than "your root disappeared from its own subgraph".
	// Dangling-edge nodes (no task) are dropped under any active
	// filter (they have no timestamps to evaluate).
	//
	// Composition semantic when BOTH filters are active: UNION
	// (logical OR). A node survives if EITHER recently-completed
	// OR recently-started. This matches the "what's actively
	// moving?" use case the two filters were designed for — done
	// and in-progress are mutually exclusive states on the same
	// task (tsk done clears Started; tsk start requires not-done),
	// so an AND interpretation would always yield zero nodes
	// (no single task satisfies both at once). UNION is the only
	// composition that produces a useful result.
	kept := make(map[int]bool, len(ids))
	if filterCompletedActive || filterStartedActive {
		now := time.Now()
		completedCutoff := now.Add(-filterCompletedDur)
		startedCutoff := now.Add(-filterStartedDur)
		for _, id := range ids {
			if id == rootID {
				kept[id] = true
				continue
			}
			t := s.ByID(id)
			if t == nil {
				continue
			}
			// Apply each active filter; node survives if it
			// matches AT LEAST ONE active predicate (UNION).
			matched := false
			if filterCompletedActive && t.Done && t.Completed != nil && !t.Completed.Before(completedCutoff) {
				matched = true
			}
			if !matched && filterStartedActive && t.Started != nil && !t.Started.Before(startedCutoff) {
				matched = true
			}
			if matched {
				kept[id] = true
			}
		}
		// Trim the ids slice to the kept set so the node-render
		// loop below only emits surviving nodes.
		filteredIDs := ids[:0]
		for _, id := range ids {
			if kept[id] {
				filteredIDs = append(filteredIDs, id)
			}
		}
		ids = filteredIDs
	}
	nodes := make([]subgraphNode, 0, len(ids))
	for _, id := range ids {
		t := s.ByID(id)
		if t == nil {
			// Dangling edge target/source — emit the node so the
			// consumer sees that the id is referenced but missing,
			// matching the ASCII path's "missing" labeling. We
			// leave Priority unset (empty string) since there's
			// no task to read from; omitempty drops it from the
			// rendered JSON so the dangling node shape stays
			// minimal regardless of --include-priority.
			nodes = append(nodes, subgraphNode{ID: id, Title: "(missing)", Done: false})
			continue
		}
		node := subgraphNode{ID: id, Title: t.Title, Done: t.Done}
		if includePriority {
			node.Priority = t.Priority.String()
		}
		if includeTags {
			// Alphabetized copy of the task's tags so the JSON
			// output is deterministic across calls regardless of
			// the on-disk tag order. Empty/no-tag tasks get a
			// non-nil empty slice via a fresh allocation so the
			// JSON encoder emits `[]` (not omitted, not null) —
			// jq pipelines that do `.tags | length` would crash
			// on null but cleanly return 0 on `[]`.
			tags := make([]string, len(t.Tags))
			copy(tags, t.Tags)
			sort.Strings(tags)
			node.Tags = &tags
		}
		if includeDue && t.Due != nil {
			// Canonical YYYY-MM-DD form. Same DateLayout `tsk
			// show` and `tsk due` use, so jq pipelines that
			// compare dates lexicographically (the ISO format
			// makes string comparison equivalent to date
			// comparison) work without parsing.
			node.Due = t.Due.Format(model.DateLayout)
		}
		if includeCompleted && t.Completed != nil {
			// Canonical RFC3339 timestamp — same format
			// `tsk show --json` uses for Completed, so a jq
			// pipeline can correlate the graph envelope's
			// nodes with per-task lookups without parsing
			// a different shape. Only set when the task is
			// actually completed (the IsDone gate would also
			// work here, but checking the pointer directly
			// mirrors the existing includeDue pattern).
			node.Completed = t.Completed.Format(time.RFC3339)
		}
		if includeStarted && t.Started != nil {
			// Canonical RFC3339 timestamp — same format
			// `tsk show --json` uses for Started. Only set
			// when the task has a started: stamp (the
			// in-progress signal). Tasks that have never
			// been started leave the field absent.
			node.Started = t.Started.Format(time.RFC3339)
		}
		if includePinned {
			// Pinned is the sixth opt-in field, the "high-
			// importance bookmark" axis. Modeled as a *bool so
			// the JSON encoder can DISTINGUISH "field
			// intentionally omitted (flag off)" — nil pointer
			// drops the field via omitempty — from "field
			// present and false" (flag on, task not pinned) —
			// non-nil pointer to false. A plain bool with
			// omitempty would drop the false case (collapsing
			// both meanings into "no field"), which would hide
			// the fact that the flag is active for unpinned
			// tasks. The pattern matches Tags's *[]string
			// modeling that landed in tick #27 for the same
			// reason. Useful for jq pipelines that need to
			// flag pinned tasks in an impact chain (`.nodes[]
			// | select(.pinned)`) without a per-node
			// `tsk show --json <id>` round-trip.
			pinned := t.Pinned
			node.Pinned = &pinned
		}
		nodes = append(nodes, node)
	}
	jsonEdges := make([]subgraphEdge, 0, len(edges))
	for _, e := range edges {
		// Drop edges whose endpoints didn't survive the
		// completed-since / started-since filter. When neither
		// filter is active, kept is empty AND both flags are
		// false, so we accept every edge. When either is active,
		// both endpoints must be in the kept set (otherwise the
		// edge points at a non-rendered node and would be
		// meaningless).
		if filterCompletedActive || filterStartedActive {
			if !kept[e.from] || !kept[e.to] {
				continue
			}
		}
		jsonEdges = append(jsonEdges, subgraphEdge{From: e.from, To: e.to})
	}
	doc := subgraphDoc{
		RootID:    rootID,
		Direction: rootKind,
		Nodes:     nodes,
		Edges:     jsonEdges,
	}
	if open {
		doc.Filter = "open"
	}
	if filterCompletedActive {
		// Canonical form: "completed<=Nm" / "completed<=Ndh" so
		// scripts watching the envelope can distinguish an
		// active recency-filter from the un-filtered "open"
		// case. The duration is rendered via humanizeDuration
		// for consistency with --since reporting elsewhere
		// (depend --pending, wip --stale).
		doc.FilterCompletedSince = humanizeDuration(filterCompletedDur)
	}
	if filterStartedActive {
		// Sister marker for --filter-started-since. Same
		// canonical humanized form as FilterCompletedSince so
		// scripts can detect either / both filters and
		// distinguish them by which field is present.
		doc.FilterStartedSince = humanizeDuration(filterStartedDur)
	}
	enc := json.NewEncoder(w)
	if !compact {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(doc)
}
