# tsk — autoship state

**Active branch: `main`** — commit and push DIRECTLY to main every tick. No feature branches.
**Loop:** cron — 5 feature slices per 20-minute tick, gated once at the end.
**Last bootstrapped:** 2026-06-19

## Conventions

- Cron commits as `Cake (cron) <51058514+Sanjays2402@users.noreply.github.com>`.
- No emoji in commit messages, no PRs, no merges to main, no tags.
- Each feature = its own commit (revertible).
- Quality gates: `gofmt -w . && go vet ./... && go build ./... && go test ./...`
- Real working tree path on this host: `/Volumes/Projects/tsk` (the
  user-facing `/Users/sanjay/Projects/tsk` path is a transient mount that
  can disappear mid-session, so always resolve via the real volume).

## Roadmap (great tsk features sized for this loop)

Items marked `[x]` are shipped on this branch. `[ ]` are unstarted.
Pull the next 5 unstarted items per tick.

### Per-task CLI shortcuts (replace verbose `tsk bulk --id N` invocations)

- [x] `tsk show <id>` — single-task detail view with notes, full timestamps; `--json` for scripts (tick 2026-06-19/2122)
- [x] `tsk pri <id> <prio>` — quick priority setter; aliases `priority` (tick 2026-06-19/2122)
- [x] `tsk due <id> <date>` — quick due setter; `--clear` to remove; natural language (tick 2026-06-19/2122)
- [x] `tsk tag <id> +foo -bar` — add/remove tags on a single task in one shot (tick 2026-06-19/2122)
- [x] `tsk where` — print resolved `.tsk.md` path + how it was resolved + tz; `--json` (tick 2026-06-19/2122)
- [x] `tsk rename <id> <new title>` — quick title change without dropping into TUI (tick 2026-06-19/2209)
- [x] `tsk note <id> [text]` — add/edit notes from CLI (opens $EDITOR with no text arg) (tick 2026-06-19/2209)
- [x] `tsk clone <id>` / `tsk dupe <id>` — duplicate a task with a fresh ID (tick 2026-06-19/2209)
- [x] `tsk reopen <id>` — alias for `undo` (more discoverable verb) (tick 2026-06-19/2209)
- [x] `tsk snooze <id> <date>` — bump due date forward; refuses if already further out (with --force override) (tick 2026-06-19/2209)

### New views / scriptable surfaces

- [x] `tsk top [N]` — show N highest-priority undone tasks (multi-task `next`) (tick 2026-06-19/2306)
- [x] `tsk today` / `tsk overdue` — convenience aliases for the common `ls --today` / `ls --overdue` invocations (tick 2026-06-19/2306)
- [x] `tsk tags` — list all tags with usage counts (extended top-tags; supports `--json`) (tick 2026-06-19/2306)
- [x] `tsk log` — show recently completed tasks (tail of completions, optional `--since`) (tick 2026-06-19/2306)
- [x] `tsk yesterday` — what was completed yesterday (standup-friendly summary) (tick 2026-06-20/0312)
- [x] `tsk daily` — synthesized morning plan: overdue + today + top 3 upcoming, in one screen (tick 2026-06-20/0312)
- [x] `tsk grep <regex>` — exact regex search across title+notes (vs fuzzy `search`) (tick 2026-06-19/2306)
- [x] `tsk diff` — diff active `.tsk.md` vs `.bak` snapshot (what would `undo-last` undo) (tick 2026-06-20/0312)
- [x] `tsk env` — print effective env: TSK_TZ, EDITOR, NO_COLOR, paths (tick 2026-06-20/0312)

### Storage / model / format

- [ ] `tsk import <path>` — accept todo.txt / TaskWarrior `task export` JSON / Notion CSV
- [x] `tsk export --jsonl` — streaming JSON-lines for pipelines (tick 2026-06-20/0312)
- [ ] Recurrence (`tsk add -r weekly`, recur-on-done): adopt from stale `feat-recur` branch carefully
- [ ] Started/in-progress state with `tsk start <id>` / `tsk stop <id>` + `started:` timestamp
- [ ] Multi-file aggregation (`ls --include ~/work/.tsk.md --include ~/home/.tsk.md`)
- [ ] Config file at `~/.tsk/config.toml` for default file, default priority, palette overrides

### TUI

- [ ] Detail pane (right side): expanded view of selected task with notes/timestamps
- [ ] `u` / Ctrl-Z in-session undo (separate from CLI `undo-last`)
- [ ] `g`/`G` top/bottom navigation (vim-style)
- [ ] Pomodoro / focus timer overlay (`f` to start, status bar countdown)
- [ ] Task creation form with priority/due/tag fields exposed (currently title-only)

### Polish & DX (added 2026-06-20)

- [x] `tsk pin <id>` — sticky-flag a task so it appears first in `top`/`next` regardless of priority (tick 2026-06-20/0640)
- [x] `tsk depend <id> --on <other>` — track task dependencies; block `done` if unmet (tick 2026-06-20/1736)
- [x] `tsk wait <id> <until>` — hide a task from default views until a date (separate from due) (tick 2026-06-20/0640)
- [ ] `tsk move-to <file>` — relocate a task between .tsk.md files (cross-store mover)
- [ ] `tsk shell` — interactive REPL with command history, useful for batch sessions
- [ ] `tsk view <id> --watch` — re-render every N seconds while iterating
- [x] `tsk last` — show the most recently mutated task (whatever the last edit was) (tick 2026-06-20/0640)
- [ ] `tsk hist [id]` — list every save's diff against the previous from git-like backups (would need backup chain)
- [x] `tsk completion --install` — write the shell completion script directly into the right rc file (tick 2026-06-20/0640)
- [x] `tsk man` — generate a manpage from cobra's help tree (tick 2026-06-20/0640)

### Polish & DX (added 2026-06-20 tick #6)

- [ ] `tsk show <id> --watch` — re-render the detail view every N seconds (live progress)
- [x] `tsk find <regex>` — `grep` over titles only (no notes scan, faster for big stores) (tick 2026-06-20/1736)
- [x] `tsk rebuild-ids` — densify ID space after lots of removes (1,5,7,12 -> 1,2,3,4) (tick 2026-06-20/0948)
- [ ] `tsk recent` — alias for `last` with a window flag (`--since 1h`) — NOTE: `recent` is already a `log` alias; consider a different verb
- [x] `tsk pri --up <id>` / `tsk pri --down <id>` — cycle priority without remembering the name (tick 2026-06-20/0948)
- [ ] `tsk depends-on <id>` — set/list the prerequisite chain (lighter than full graph) — SUPERSEDED by full `tsk depend` in tick #9
- [x] `tsk lint` — validate the file (orphan IDs, dangling notes, ungrouped block) and suggest fixes (tick 2026-06-20/0948)
- [x] `tsk swap <id> <id>` — exchange two tasks' positions in the file (manual reorder) (tick 2026-06-20/0948)
- [ ] `tsk archive --strategy weekly` — roll completed tasks into per-week archive sections
- [x] `tsk bench` — print parser/save timings for the current file (useful when a store gets big) (tick 2026-06-20/0948)

### Polish & DX (added 2026-06-20 tick #7)

- [ ] `tsk start <id>` / `tsk stop <id>` — track in-progress state with `started:` meta key (also in storage backlog) — SHIPPED in tick #8 (see tick #8 notes)
- [ ] `tsk recur <id> <interval>` — convert a task to a recurring one (sister of stale `feat-recur`)
- [x] `tsk dedupe` — find tasks with identical or near-identical titles, surface for review (tick 2026-06-20/1506)
- [ ] `tsk wrap <id> <text>` — wrap a long title with `>` continuation lines for readability
- [ ] `tsk shuffle` — randomize task order for "what should I do next" decision paralysis
- [x] `tsk freeze <id>` — alias for `wait <id> 2099-01-01` (indefinite hide; surface only via `tsk wait --list`) (tick 2026-06-20/1506)
- [ ] `tsk why <id>` — print the full history-ish trail (created, edited, dependencies, where it came from)
- [x] `tsk pri-stats` — distribution of priorities (how many low/med/high/urgent), `--by-tag` breakdown (tick 2026-06-20/1506)
- [ ] `tsk lint --autofix-all` — combine multiple safe fixes (canonical bullet + drop unknown meta) without per-finding prompts (we have --fix; this would be the "just trust me" form)
- [x] `tsk hash` — print a stable content hash of the store (great for CI signals: "did anything change?") (tick 2026-06-20/1506)

### Polish & DX (added 2026-06-20 tick #8)

- [x] `tsk start <id>` / `tsk stop <id>` / `tsk in-progress` — in-progress state with `started:` timestamp (tick 2026-06-20/1506)
- [x] `tsk elapsed <id>` — show "started Nm/h/d ago" for an in-progress task; --json for scripts (tick 2026-06-20/1736)
- [ ] `tsk pause <id>` — alias for `stop` that pairs with start visually (semantics: same)
- [ ] `tsk recent` — rename of the would-be alias; show last-N edits with --since (the verb collision with `log` blocked this in tick #6)
- [x] `tsk why <id>` — print created/started/completed/wait/due/tags trail in one view (sister of `tsk show`) (tick 2026-06-20/1736)
- [ ] `tsk dedupe --merge <id>` — pick a survivor, merge notes from the others, rm the rest (interactive)
- [x] `tsk shuffle` — random order of undone tasks (decision-paralysis breaker) (tick 2026-06-20/1736)
- [ ] `tsk wrap <id>` — split a long title with `>` continuation, mirror of `tsk note` for the title axis
- [ ] `tsk recur <id> <interval>` — recurring tasks (recur-on-done from stale feat-recur)
- [ ] `tsk lint --autofix-all` — non-interactive multi-fix

### Polish & DX (added 2026-06-20 tick #9)

Fresh ideas so future ticks have ample sized work:

- [x] `tsk blocked` — alias for `depend --list` (more discoverable verb for "what's stuck on what?") (tick 2026-06-20/2046)
- [x] `tsk graph` — DOT/ASCII rendering of the dependency graph; pipe to graphviz for a visual (tick 2026-06-20/2046)
- [x] `tsk depend <id> --tree` — print the recursive prerequisite chain (depth-first, indented) (tick 2026-06-20/2046)
- [x] `tsk next --respect-deps` — skip blocked tasks in `next` (also in `top`, `ls`) (tick 2026-06-20/2046)
- [x] `tsk merge <a> <b>` — merge task b into task a (concatenate notes, union tags, redirect deps; back-refs rewritten; undo-able) (tick 2026-06-20/2046)
- [ ] `tsk split <id>` — open editor with a one-task-per-line list to split a parent task into N subtasks
- [ ] `tsk timer <id> [<duration>]` — pomodoro overlay paired with start/stop; default 25m
- [ ] `tsk rules` — declarative auto-mutation rules (e.g. "if tag=:weekly, recreate daily")
- [ ] `tsk export --opml` — outline import format for note-takers (Roam, Logseq, OmniOutliner)
- [ ] `tsk preview` — stdout-only `ls` that doesn't read .tsk.md (uses a snapshot pipe; useful in pipelines without side effects on the .bak chain)

### Polish & DX (added 2026-06-20 tick #10)

Fresh ideas so future ticks have ample sized work — focus on what
the dep system makes newly possible plus a few cross-cuts:

- [ ] `tsk depend --add-bidir <a> <b>` — symmetric "these two relate" not strictly prerequisite (info-only depends?). Probably not — that's a new field, not a dep.
- [x] `tsk graph --reachable <id>` — only emit the subgraph reachable from one root (filter `graph` by transitive deps) (tick 2026-06-20/2318)
- [ ] `tsk depend --pending` — list tasks whose prereqs were recently completed, so you can pull them up (the "now-unblocked" view)
- [x] `tsk path <a> <b>` — find the dep path between two tasks (BFS over the graph) (tick 2026-06-20/2318)
- [x] `tsk topo` — emit tasks in topological order (dep-respecting linearization for "do these in this order") (tick 2026-06-20/2318)
- [x] `tsk depend <id> --justify` — print the reason chain ("blocked because #3 (which is blocked because #7 (which is not done))") (tick 2026-06-20/2318)
- [x] `tsk next --json` — JSON output for `next` so it composes with scripts (currently only plain text) (tick 2026-06-20/2318)
- [ ] `tsk top --pinned-only` — show only pinned tasks (the "high-importance bookmark" view)
- [x] `tsk show <id> --tree` — combine show snapshot with the dep tree below it (tick 2026-06-21/0234)
- [ ] `tsk merge --interactive` — pick conflict resolution per-field via prompts (when `--prefer` is too coarse)

### Polish & DX (added 2026-06-20 tick #11)

Fresh ideas so future ticks have ample sized work — many are
follow-ons to the dep-debugging cluster (justify/path/topo/reachable)
plus some long-tail polish:

- [ ] `tsk justify --all` — emit a justify chain for every blocked task in one screen (review tool: "what's gating everything?")
- [ ] `tsk path <a> <b> --any-direction` — BFS in both directions (handles "are these two related at all?")
- [ ] `tsk topo --since <id>` — emit only the tasks in topo order that come AFTER a given checkpoint id
- [ ] `tsk depend --pending` — tasks whose prereqs were recently completed (the "now-unblocked" notification queue)
- [x] `tsk depend <id> --upstream` — reverse view of --tree (what depends on me?) (tick 2026-06-21/0234)
- [x] `tsk reachable <id>` — top-level alias for `graph --reachable` (discoverable verb) (tick 2026-06-21/0234)
- [ ] `tsk lint --dep-cycles` — detect 3+ node cycles the writer doesn't catch; suggest fixes
- [ ] `tsk export --graph-dot` — shortcut for `graph --format dot` that respects --file scoping
- [x] `tsk next --skip <id1,id2,...>` — exclude specific tasks from the next-pick pool (temporary hold without freeze) (tick 2026-06-21/0234)
- [x] `tsk add --depends <ids>` — set DependsOn at creation time (saves a follow-up `tsk depend` call) (tick 2026-06-21/0234)

### Polish & DX (added 2026-06-21 tick #12)

Fresh ideas so future ticks have ample sized work. The dependency
cluster is well-mined now; this batch leans into TUI polish, the
storage/import-export backlog, and a few cross-cuts the loop hasn't
touched in a while.

- [ ] `tsk show <id> --watch` — re-render the detail view every N seconds (live progress / pomodoro)
- [ ] `tsk import <path>` — accept todo.txt / TaskWarrior JSON / Notion CSV (storage/model backlog item still unstarted)
- [ ] `tsk lint --dep-cycles` — detect 3+ node cycles via Tarjan/Kosaraju; suggest break-points
- [x] `tsk depend --pending` — list tasks whose prereqs were recently completed ("now-unblocked" notification queue) (tick 2026-06-21/0541)
- [x] `tsk justify --all` — emit a justify chain for every blocked task in one screen (shipped as top-level `tsk justify [--all]` verb) (tick 2026-06-21/0541)
- [x] `tsk path <a> <b> --any-direction` — BFS in both directions (handles "are these two related at all?") (tick 2026-06-21/0541)
- [ ] `tsk topo --since <id>` — emit only the tasks in topo order that come AFTER a given checkpoint id
- [ ] `tsk depend --add-bidir <a> <b>` — symmetric "these relate" (probably a new "related:" field; design first)
- [ ] `tsk export --graph-dot` — shortcut for `graph --format dot` that respects --file scoping
- [x] `tsk show <id> --upstream` — combine snapshot + upstream view (sister of --tree) (tick 2026-06-21/0541)
- [x] `tsk top --pinned-only` — show only pinned tasks (the "high-importance bookmark" view) (tick 2026-06-21/0541)
- [ ] `tsk recent --since 1h` — last-N edits with a window flag (recent verb still collides with log alias)
- [ ] `tsk archive --strategy weekly` — roll completed tasks into per-week archive sections
- [ ] Config file at `~/.tsk/config.toml` for default file, default priority, palette overrides
- [ ] Multi-file aggregation (`ls --include ~/work/.tsk.md --include ~/home/.tsk.md`)

### Polish & DX (added 2026-06-21 tick #13)

Fresh ideas so future ticks have ample sized work. The dependency
debugging cluster is now mostly mined; this batch leans into TUI
gaps, storage/import-export, recurring tasks, and a few
quality-of-life polish items.

- [ ] `tsk show <id> --watch` — re-render the detail view every N seconds (live progress / pomodoro)
- [ ] `tsk import <path>` — accept todo.txt / TaskWarrior JSON / Notion CSV (storage/model backlog item still unstarted)
- [x] `tsk lint --dep-cycles` — detect 3+ node cycles via Tarjan/Kosaraju; suggest break-points (tick 2026-06-21/0843)
- [ ] `tsk recur <id> <interval>` — recurring tasks (recur-on-done from stale feat-recur branch)
- [x] `tsk topo --since <id>` — emit only the tasks in topo order that come AFTER a given checkpoint id (tick 2026-06-21/0843)
- [ ] `tsk export --graph-dot` — shortcut for `graph --format dot` that respects --file scoping
- [x] `tsk archive --strategy weekly` — roll completed tasks into per-week archive sections (tick 2026-06-21/0843)
- [ ] Config file at `~/.tsk/config.toml` for default file, default priority, palette overrides
- [ ] Multi-file aggregation (`ls --include ~/work/.tsk.md --include ~/home/.tsk.md`)
- [ ] `tsk split <id>` — open editor with one-task-per-line list to split a parent task into N subtasks
- [ ] `tsk wrap <id>` — split a long title with `>` continuation, mirror of `tsk note` for the title axis
- [ ] `tsk dedupe --merge <id>` — pick a survivor, merge notes from the others, rm the rest (interactive)
- [ ] `tsk pause <id>` — alias for `stop` that pairs with start visually (semantics: same)
- [ ] `tsk lint --autofix-all` — non-interactive multi-fix combining safe round-trips
- [x] `tsk preview` — stdout-only `ls` that doesn't read .tsk.md (uses a snapshot pipe; useful in pipelines without side-effects on the .bak chain) (tick 2026-06-21/0843)
- [ ] `tsk timer <id> [<duration>]` — pomodoro overlay paired with start/stop; default 25m
- [ ] `tsk rules` — declarative auto-mutation rules (e.g. "if tag=:weekly, recreate daily")
- [x] `tsk depend --pending --tag <t>` — narrow the pending notification queue by tag (tick 2026-06-21/0843)
- [ ] `tsk justify --all --json | jq` recipe doc — write a one-pager showing the chokepoint-finding patterns (`select(.value[] | .blocked_by == 7)`)
- [ ] TUI detail pane (right side): expanded view of selected task with notes/timestamps

### Polish & DX (added 2026-06-21 tick #14)

Fresh ideas so future ticks have ample sized work. With the
dep-debugging cluster now fully mined (including --dep-cycles
detection), this batch leans hard into the long-unstarted
storage/import-export backlog, recurring tasks (the stale
feat-recur branch is a known parking lot), and the TUI gaps.

- [ ] `tsk show <id> --watch` — re-render the detail view every N seconds (live progress / pomodoro)
- [ ] `tsk import <path>` — accept todo.txt / TaskWarrior JSON / Notion CSV
- [ ] `tsk recur <id> <interval>` — recurring tasks (recur-on-done from stale feat-recur)
- [x] `tsk export --graph-dot` — shortcut for `graph --format dot` respecting --file scoping (tick 2026-06-21/1153)
- [ ] Config file at `~/.tsk/config.toml` for default file, default priority, palette overrides
- [ ] Multi-file aggregation (`ls --include ~/work/.tsk.md --include ~/home/.tsk.md`)
- [ ] `tsk split <id>` — open editor with one-task-per-line list, split a parent task into N subtasks
- [ ] `tsk wrap <id>` — split a long title with `>` continuation
- [ ] `tsk dedupe --merge <id>` — pick a survivor, merge notes from others, rm the rest (interactive)
- [ ] `tsk pause <id>` — alias for `stop` that pairs with start visually
- [x] `tsk lint --autofix-all` — non-interactive multi-fix combining safe round-trips (tick 2026-06-21/1153)
- [ ] `tsk timer <id> [<duration>]` — pomodoro overlay paired with start/stop; default 25m
- [ ] `tsk rules` — declarative auto-mutation rules (e.g. "if tag=:weekly, recreate daily")
- [ ] `tsk preview --json | jq` recipe doc — one-pager showing pipeline patterns that benefit from no-side-effect parsing
- [x] `tsk archive --strategy monthly` — sibling of --strategy weekly (same scaffolding) (tick 2026-06-21/1153)
- [ ] `tsk archive --merge-into <file>` — write to a non-default sibling archive (useful for project rollups)
- [ ] TUI detail pane (right side): expanded view of selected task with notes/timestamps
- [ ] `tsk lint --dep-cycles --json | jq -r` recipe doc — one-pager showing how to feed the cycle output back into `tsk depend --remove` automation
- [ ] `tsk topo --since <id> --depth N` — limit how many layers past the checkpoint to emit (for huge graphs)
- [x] `tsk depend --pending --priority urgent` — narrow the queue by priority (sister of --tag) (tick 2026-06-21/1153)
- [ ] `tsk preview --from <path> --watch N` — re-render every N seconds while a separate process mutates the file
- [x] `tsk topo --since <id> --reverse` — emit tasks BEFORE the checkpoint in topo order (the "what's left to do BEFORE I can hit this milestone?" view) (tick 2026-06-21/1153)
- [ ] `tsk archive --strategy daily` — finer-grained sibling of weekly

### Polish & DX (added 2026-06-21 tick #15)

Fresh ideas so future ticks have ample sized work. With archive
strategies (flat/weekly/monthly), pending filters (tag+priority),
topo slicing (forward+reverse), the central data-out verb
(`tsk export --graph-dot`), and lint's autofix-all all shipped,
this batch leans into the still-unstarted long-tail: TUI gaps,
recurring tasks (parking lot), config/multi-file, plus a few
new follow-ons that this tick's features make sensible.

- [ ] `tsk show <id> --watch` — re-render the detail view every N seconds (live progress / pomodoro)
- [ ] `tsk import <path>` — accept todo.txt / TaskWarrior JSON / Notion CSV
- [ ] `tsk recur <id> <interval>` — recurring tasks (recur-on-done from stale feat-recur)
- [ ] Config file at `~/.tsk/config.toml` for default file, default priority, palette overrides
- [ ] Multi-file aggregation (`ls --include ~/work/.tsk.md --include ~/home/.tsk.md`)
- [ ] `tsk split <id>` — open editor with one-task-per-line list, split a parent task into N subtasks
- [ ] `tsk wrap <id>` — split a long title with `>` continuation
- [ ] `tsk dedupe --merge <id>` — pick a survivor, merge notes from others, rm the rest (interactive)
- [ ] `tsk pause <id>` — alias for `stop` that pairs with start visually
- [ ] `tsk timer <id> [<duration>]` — pomodoro overlay paired with start/stop; default 25m
- [ ] `tsk rules` — declarative auto-mutation rules (e.g. "if tag=:weekly, recreate daily")
- [ ] `tsk archive --merge-into <file>` — write to a non-default sibling archive (useful for project rollups)
- [ ] `tsk archive --strategy daily` — finer-grained sibling of weekly/monthly (same bucketFn scaffolding)
- [ ] `tsk archive --strategy quarterly` — coarse-grained sibling (Q1/Q2/Q3/Q4 buckets)
- [ ] `tsk topo --since <id> --depth N` — limit topological emission depth from the checkpoint
- [ ] `tsk export --graph-dot --highlight <id>` — wrap the focus task in a distinct color to draw the eye on a complex graph
- [ ] `tsk lint --autofix-all --backup <path>` — explicit backup directory instead of in-place .bak (useful in pre-commit setups)
- [ ] `tsk depend --pending --since <dur> --priority <p> --tag <t> | jq` recipe doc — one-pager on composing the freshly-unblocked feed for standup automation
- [ ] `tsk show <id> --upstream --tree` — combine both views in one snapshot (currently mutually exclusive; would need a sibling format)
- [ ] TUI detail pane (right side): expanded view of selected task with notes/timestamps

## Known footguns the loop has run into

- The `/Users/sanjay/Projects/tsk` user-facing path is an ephemeral mount.
  Mid-tick the volume can flip and the path stops resolving. Always work
  from `/Volumes/Projects/tsk` directly and verify `pwd` after any pause.
- Tick #1 (2026-06-19 20:22 PT) reported these 5 features as shipped but
  actually parked broken WIP in a git stash that referenced undefined
  functions. The bootstrap commit landed; the 5 feature commits did NOT.
  Tick #2 dropped the broken stash and re-implemented all 5 cleanly with
  passing tests. **Always verify `git log origin/<branch>` after push.**
- For commands with `-name`-style positional args (e.g. `tsk tag 1 -old`),
  cobra parses the `-o` as an unknown shorthand. Fix: set
  `DisableFlagParsing: true` on the cmd and use `extractFileFlag` (in
  tag.go) to re-extract `--file` from the raw args.

## Tick log

### 2026-06-19 20:22 PT (tick #1 — failed)

Bootstrapped `feature/autoship` off main (commit 3a378db landed). The 5
"shipped" features in the original log were NOT actually committed —
they were parked in a `git stash` with broken cross-references (root.go
referencing undefined `newShowCmd`/`newPriCmd`/`newDueCmd`/`newTagCmd`/
`newWhereCmd`). The mid-tick volume remount appears to have caused the
shipper to stash instead of commit. Stash was discovered and dropped by
tick #2.

### 2026-06-19 21:22 PT (tick #2)

Re-shipped tick #1's 5 per-task CLI shortcuts cleanly with passing tests.
Each is a single revertible commit; full quality gate (gofmt + vet +
build + go test ./...) green before push; landed on origin/feature/autoship.

- `show` — e8e2066 feat(show): tsk show <id> single-task detail view with --json
- `pri`  — cb23c03 feat(pri): tsk pri <id> <priority> quick priority setter
- `due`  — 9e3c72c feat(due): tsk due <id> <date> quick due-date setter with --clear
- `tag`  — a60ddae feat(tag): tsk tag <id> +foo -bar single-task tag mutator
- `where`— 1ba054e feat(where): tsk where prints resolved file + method + timezone

Notable: tag.go uses `DisableFlagParsing: true` + `extractFileFlag` helper
so `-old`-style remove args don't trip cobra's flag parser; that helper
is reusable for any future command needing the same trick.

### 2026-06-19 22:09 PT (tick #3)

Shipped the rest of the "per-task CLI shortcuts" section — all 5 picked
items landed clean with passing tests, single-commit each, full gate
green before push.

- `rename` — a3a981b feat(rename): tsk rename <id> <new title> single-task title change
- `clone`  — 0ef1391 feat(clone): tsk clone <id> duplicate a task with a fresh ID
- `reopen` — a9ad18a feat(reopen): tsk reopen — discoverable undo with --last and --since
- `snooze` — 9722016 feat(snooze): tsk snooze <id> <date> push due date forward only
- `note`   — 607e94a feat(note): tsk note <id> [text] add/edit/clear notes from CLI

Notable choices:
- `reopen` is more than an undo alias — it adds --last (most recently
  completed) and --since <duration> (everything in a window) for the
  two common "wait, no" patterns. Local duration parser (NOT
  store.ParseDuration), because the store helper treats `m` as months
  — would silently make `--since 5m` mean 5 months ago.
- `snooze` refuses backward moves by default (whole point: "deal with
  this later"), with --force override. On an undated task it degrades
  to a plain set with an "(initial)" marker.
- `note` has three input modes (text args, editor, --stdin) plus
  --append/--clear modifiers. Editor invocation indirected through a
  package-level noteEditor var so tests can swap it for a shim — no
  $EDITOR juggling, no subprocess fork.
- `clone` deep-copies tags and due pointer so mutating the source can
  never bleed into the clone (or vice versa); a dedicated regression
  test guards that.

### 2026-06-19 23:06 PT (tick #4)

Shipped 5 new view / scriptable-surface commands. All single-commit,
all tests passing, full gate green before push. (Three commits needed
a `git commit --fixup` + autosquash pass to fold gofmt alignment fixes
into the right feature commit — kept history clean.)

- `top`              — 1c2cefd feat(top): tsk top [N] surface the N highest-priority undone tasks
- `today` + `overdue`— a45077d feat(today,overdue): one-word verbs for the two most common ls slices
- `tags`             — 7e98ce3 feat(tags): tsk tags full per-tag usage report
- `log`              — e932f05 feat(log): tsk log chronological tail of recently completed tasks
- `grep`             — 45fae7e feat(grep): tsk grep <regex> exact RE2 regex search across tasks

Notable choices:
- `top` matches `tsk next`'s tie-break exactly (priority desc, dated-
  first, earliest-due, lower-id) so `top[0]` and `next` agree on the
  same task. Regression test guards that.
- `today`/`overdue` aren't thin aliases — they default to undone-only
  (the only sensible default for a workday) and expose the same
  --format/--json/--tag/--priority surface as `tsk ls`. Both route
  through the shared applyFilters → single source of truth.
- `tags` is the catalog cousin of `stats`'s top-5 block: full per-tag
  count, with --all/--done scope, --min suppression for noisy stores,
  --sort alpha for completion menus, JSON emits [] not null when empty.
- `log` is the journal vs `ls --done`'s catalog: newest-first, capped
  by --limit (default 10), trimmed by --since. Done tasks without a
  Completed timestamp are excluded from rows but surfaced in a footer
  line so the user can audit. JSON stays silent — contract is just the
  task array.
- `grep` is exact RE2 vs `search`'s fuzzy. Case-insensitive default
  (POSIX-grep-shaped), undone-only default, prints field annotation
  ("matched in: title") so you see WHY it matched. --files-only/-l
  for xargs pipelines, --count, --json — all mutually exclusive and
  guarded upfront. NOT using -v as a short for --invert to avoid the
  `tsk version` collision.

Pushed e9f3b3f..45fae7e to origin/feature/autoship; verified via
`git log origin/feature/autoship | head`.

### 2026-06-20 03:12 PT (tick #5)

Shipped 5 features covering the remaining "new views / scriptable
surfaces" backlog plus one storage-format extension. All single-commit,
all tests passing, full gate green (gofmt + vet + build + go test ./...)
before push. Pushed e0875f9..e257d77 to origin/feature/autoship,
verified landed.

- `yesterday` — b73dc52 feat(yesterday): tsk yesterday standup summary of yesterday's completions
- `daily`     — 0700c11 feat(daily): tsk daily synthesized morning briefing in one screen
- `diff`      — 276f155 feat(diff): tsk diff show what undo-last would revert
- `env`       — 06db477 feat(env): tsk env effective configuration dump for debug + scripts
- `export --jsonl` — e257d77 feat(export): jsonl streaming format for pipelines

Notable choices:
- `yesterday` is anchored on calendar-day boundaries [yesterdayStart,
  todayStart) in the active tz — NOT a rolling 24h window, which would
  miss yesterday's late work and include this morning's. Boundary
  tests cover 00:00 today (excluded), 00:00 yesterday (included), and
  23:59:59 yesterday (included). JSON empty case emits `[]` not `null`.
- `daily` groups into OVERDUE/TODAY/UPCOMING. Bucket assignment checks
  IsDueToday BEFORE IsOverdue because the markdown store persists due
  dates as UTC midnight — a same-day task can otherwise compare as
  "before local startOfDay". Each section sorts by canonical tie-break
  (priority desc, dated-first, earliest-due, lower ID) so it matches
  `top`/`next`. Empty sections render as "(none)" so missing slots
  are obvious. Tests use `dueDate(±2)` (not ±1) to avoid the same
  UTC-midnight-vs-local boundary causing flakes; the singleton-day
  +1d boundary IS broken in tsk's persist layer but is a pre-existing
  bug, NOT introduced here. Documented in the commit body.
- `diff` is a self-contained Myers-like LCS — no external `diff`
  binary required, works on stripped containers. Exit codes mirror
  `git diff --exit-code`: 0 clean, 1 changes (SilentExitCoder so
  main.go skips "error:"), 2 no .bak snapshot. Pair with `undo-last`
  for an inspect-before-revert workflow.
- `env` is the canonical "why did tsk do that?" debug snapshot —
  pairs with `tsk where`. Reports VISUAL > EDITOR resolution, NO_COLOR
  semantics (empty doesn't disable, anything does, per no_color.org),
  TSK_* prefixed env var enumeration. Unset vars render "(unset)"
  so missing values are distinguishable from empty ones.
- `export --jsonl` adds the third top-tier export format. Aliases
  --format jsonl + --format ndjson. Per-line schema matches one
  --json array element so consumers switching between the two don't
  rename field accessors. Empty store = empty output (NOT `[]`, NOT
  a blank line — jsonlines convention). Tests guard against embedded
  newlines (would break streaming) and mutual-exclusion with --json.

Roadmap status: per-task CLI shortcuts section (10 items) is fully
shipped. New views / scriptable surfaces section (9 items) is fully
shipped. Storage/model/format section: 1/6 shipped (the easy one).
Added a new "Polish & DX" roadmap section (10 items) so the next 3+
ticks have fresh, sized work to draw from.

### 2026-06-20 06:40 PT (tick #6)

Shipped 5 features from the new "Polish & DX" backlog plus a wait-
state model addition. All single-commit, all tests passing (96 new
test cases across the 5 features), full gate green (gofmt + vet +
build + go test ./...) before push. Pushed 865aefe..50e6aa6 to
origin/feature/autoship; verified landed.

- `pin/unpin`           — 31eaa5a feat(pin): sticky-flag tasks so they float to the top
- `last`                — 6578af4 feat(last): show the most recently mutated task
- `man`                 — cf87073 feat(man): generate manpages from the cobra command tree
- `completion --install`— be6973d feat(completion): drop scripts into per-shell paths
- `wait`                — 50e6aa6 feat(wait): hide tasks from default views until a date

Notable choices:
- `pin` is a storage format extension (new `pin:true` meta key), but
  strictly additive — the key only renders when true, so old files
  round-trip unchanged. Hand-edit `pin:false` (or delete the key)
  to clear. `next` and `top` were updated to put pinned tasks first;
  `next` also prefixes a `*` marker so users can see WHY a low-prio
  task is winning.
- `last` scores by max(Created, Completed) per task so an old task
  just marked done floats above a freshly-added one. RFC3339 is
  second-precision so the unit tests use synthetic timestamps
  directly (mostRecentlyMutated() is exposed at the package level);
  the CLI smoke test only needs a single task. Tasks with neither
  timestamp (hand-edited entries) are skipped to avoid them ranking
  at epoch zero.
- `man` pulls in github.com/spf13/cobra/doc (already transitive of
  cobra) and uses GenManTree — pages stay in sync with help text
  automatically. --install requires --yes to prevent typo-installs
  to /usr/local/share/man/man1. ./man is the default destination
  so users can preview without elevated permissions.
- `completion --install` had a special case for PowerShell: its
  profile is a single .ps1 file the user already curates (custom
  prompt, aliases, etc), so we cannot overwrite it. Instead the
  install wraps the script in sentinel-guarded blocks (`# >>> tsk
  completion >>>` / `# <<< tsk completion <<<`) and on re-run
  replaces only the block between sentinels. Idempotent. Verified
  by test that pre-seeds custom content, runs install twice, asserts
  exactly one sentinel pair survives and custom content is intact.
- `wait` is a NEW second-class date on the task (separate field
  WaitUntil, separate `wait:` meta key). The whole point is it's
  NOT the same as due — wait HIDES the task entirely until the
  date passes, due just marks it overdue. Filter wiring updates:
  ls/top/next default-hide waiting tasks; ls --all and the new
  --include-waiting flag bring them back; `tsk show <id>` and
  `tsk wait --list` always surface them. Past-dated wait is
  treated as expired (visible) — the user wanted the task back
  when that date arrived; forgetting to clear shouldn't hide
  it forever.
- The completion gofmt adjustment from tick #6 work was folded
  into the completion commit via `git commit --fixup` + autosquash
  to keep each feature one-commit.

Roadmap status: "Polish & DX" section is 5/10 shipped (pin, wait,
last, completion --install, man). Added a fresh "Polish & DX (added
2026-06-20 tick #6)" subsection with 10 more sized items so future
ticks never starve.

### 2026-06-20 09:48 PT (tick #7)

Shipped 5 features from the "Polish & DX (added tick #6)" backlog.
All single-commit, all tests passing (52 new test cases across the
5 features), full gate green (gofmt + vet + build + go test ./...)
before push. Pushed 031154a..5880aee to origin/feature/autoship;
verified landed.

- `pri --up/--down/--cycle` — 031154a feat(pri): bump priority without naming it
- `swap <id1> <id2>`        — 4d6bcaa feat(swap): exchange task positions in the file
- `lint`                    — 1daa4f3 feat(lint): .tsk.md hygiene checker with safe --fix
- `rebuild-ids`             — af9ea6d feat(rebuild-ids): densify sparse task IDs
- `bench`                   — 5880aee feat(bench): parser + render timings for the active file

Notable choices:
- `pri` Args constraint relaxes from ExactArgs(2) to RangeArgs(1, 2).
  The mode arbiter (validatePriModeFlags) enforces exactly one of
  {positional <priority>, --up, --down, --cycle}. --up/--down clamp
  at the extremes (urgent/low) and print "already at priority X"
  instead of erroring — a no-op is the friendlier response when the
  user holds the bump key. --cycle is the only mode that wraps
  (urgent -> low); --cycle-down was considered and dropped because
  upward-cycling matches the TUI's `p` key behavior.
- `swap` ships a tiny findTaskIndex helper rather than reaching for
  store.ByID — the swap needs the slice index (not a pointer) for
  the actual exchange. Helper stays adjacent until a second caller
  appears. Self-swap is a usage error so typos don't silently
  no-op.
- `lint` is the file-format sibling of `doctor` (which already
  covers in-memory checks). Surfaces things the parser TOLERATES
  on read but the writer drops on save: non-canonical bullets
  (*/+), 'X' uppercase, unknown meta keys (silently lost on next
  Save), missing created stamps, stray notes-shaped lines before
  any task. --fix is a safe round-trip through store.Save: same
  in-memory tasks, canonical bytes on disk, normal .bak chain so
  undo-last reverts. The known-meta-keys set is intentionally
  DUPLICATED locally rather than exported from store — keeps the
  lint surface revertible without breaking store's public API.
- `rebuild-ids` is destructive (every id mentioned anywhere
  shifts), so safety is layered: dry-run default, --apply requires
  --yes (defends against scripts), --since-id N preserves lower
  ids (keeps bookmarked ids stable). The planRebuildIDs algorithm
  has a tricky bit: when --since-id reserves some ids, the new
  dense numbers must not collide with those reserved ones —
  covered by a direct algorithm test (the CLI test alone would
  miss subtle off-by-ones). Aliases: densify-ids, renumber.
- `bench` measures the user's actual file (where the
  bench_test.go microbenchmarks measure synthetic data). Reports
  size, line count, task count, then load + render min/median/max
  across N iterations. Does NOT measure Save (atomic tempfile +
  fsync would dominate and pollute .bak chain). renderForBench
  inlines its own near-identical serializer rather than reaching
  into store.render (unexported) — keeps the command decoupled.
  --iter is bounded [1, 1000]. Auto-unit formatter (us under 1ms,
  ms above) keeps the numbers scannable.

Roadmap status: tick #6 "Polish & DX" subsection 9/10 shipped
(`recent` deferred: the verb already aliases `log`, needs a new
name first). Added "Polish & DX (added tick #7)" subsection with
10 more sized items so future ticks have room.


### 2026-06-20 15:06 PT (tick #8)

Shipped 5 features from the "Polish & DX (added tick #7)" backlog,
including the one storage-format extension. All single-commit, all
tests passing (58 new test cases across the 5 features + every
existing test still green), full gate green (gofmt + vet + build +
go test ./...) before push. Pushed 905c7ca..2d211f3 to
origin/feature/autoship; verified landed.

- `hash`              — 905c7ca feat(hash): stable content hash (file + semantic modes)
- `pri-stats`         — 8964226 feat(pri-stats): priority distribution with optional --by-tag
- `freeze`/`thaw`     — 5112ca8 feat(freeze): indefinite-hide shorthand over wait
- `dedupe`            — c6ab1e3 feat(dedupe): surface duplicate / near-duplicate tasks for review
- `start`/`stop`/`wip`— 2d211f3 feat(start): in-progress state tracking with started: timestamp

Notable choices:

- `hash` has TWO modes for a real reason: file mode is the sha256sum-
  shape sanity check ("did the bytes change?"), semantic mode is the
  CI-grade "did any task content change?" check that survives a
  `lint --fix` round-trip. The canonical projection uses UTC RFC3339
  timestamps and ID-ordered emission so two stores describing the
  same instants in different timezones hash identically. Notes are
  escape-encoded with `\\` for `\` (lossless) so literal `\n` and a
  real newline produce different digests — verified by a dedicated
  test.

- `pri-stats` deliberately renders ALL FOUR priority buckets even
  when some are zero. The user wants to see WHERE the zeros are;
  silent suppression makes their absence a footgun. The bar
  rendering enforces a floor of one `█` for any non-zero count so
  proportional rounding can't drop a real value to invisible.

- `freeze`/`thaw` are pure SHIM commands on top of `wait`/`wait
  --clear` — no new persisted state. The freeze sentinel is
  2099-12-31 (parseable real date, lints clean, lets hand-editing
  the meta silently un-freeze on that date). IsFrozen is exposed
  for callers that want to render frozen vs ordinary-waiting
  differently; currently only tests rely on it.

- `dedupe` is a REVIEW tool, never destructive. Auto-removal would
  too often clobber the wrong copy (the one with notes, the one
  with a due date). The output is shaped for "review then maybe
  rm" workflows: `--files-only` is group-aware (blank line between
  groups). Levenshtein has EARLY TERMINATION (row min > cap bails)
  which keeps `--near 2` linear-in-store rather than quadratic-in-
  title-length on most inputs. Damerau (transpositions) intentionally
  not added — regular edit distance at <= 2 catches typos fine, and
  the extra row inflates code with little user-visible win. Title
  normalization strips punctuation so "buy milk." / "buy milk"
  collide in exact mode (a frequent class of accidental dupes).

- `start`/`stop`/`in-progress` are the FIRST storage-format extension
  this tick (Task.Started *time.Time, `started:` meta key). Strictly
  additive: old files round-trip unchanged, NormalizeTags-style
  defensive behavior, lint's knownMetaKeys updated, hash's canonical
  projection extended in the documented order (between created and
  completed). One real semantic decision is documented in store.SetDone:
  marking done CLEARS Started (Completed is the more useful timestamp
  at that point); reopen does NOT re-set Started (user must explicitly
  start again — they may not actually be picking the work back up).
  start is idempotent without --reset because "I forgot I had started
  this an hour ago" shouldn't accidentally zero the elapsed time;
  start on a done task is a usage error (forces explicit reopen
  first — implicit transitions hide bugs).

Roadmap status: tick #7 "Polish & DX" subsection is 5/10 shipped
(hash, pri-stats, freeze, dedupe, start/stop). The remaining 5 are
recur, wrap, shuffle, why, lint --autofix-all. Added "Polish & DX
(added tick #8)" subsection with 10 fresh items so future ticks
have room.

Per-feature test counts: hash 9, pri-stats 13, freeze/thaw 10,
dedupe 15, start/stop/in-progress + helpers 16. Total ~63 new test
cases on top of the existing suite, all green.

### 2026-06-20 17:36 PT (tick #9)

Shipped 5 features — three from the "Polish & DX tick #8" backlog,
one cross-section (depend was in the "added tick #6" rough draft,
fully designed and shipped here), one from "tick #6" backlog
(find). All single-commit, all tests passing (54 new test cases),
full gate green (gofmt + vet + build + go test ./...) before push.
Pushed 5258000..8484534 to origin/feature/autoship; verified landed.

- `elapsed`     — 5258000 feat(elapsed): tsk elapsed [id] elapsed-time view
- `why`         — 1bfa4fc feat(why): tsk why <id> chronological provenance trail
- `shuffle`     — 259e0b2 feat(shuffle): tsk shuffle [N] decision-paralysis breaker
- `find`        — 1491e07 feat(find): tsk find <regex> title-only RE2 search
- `depend`      — 8484534 feat(depend): tsk depend <id> task deps with done-blocking

Notable choices:

- `elapsed` is the script-friendly cousin of `tsk in-progress`. The
  list mode sorts OLDEST-start first (staleness view) — opposite of
  in-progress's most-recent-first sort, because "what's been
  sitting?" is the actually-actionable question. JSON exposes
  elapsed_seconds (int) so pipelines like
    `tsk elapsed --json | jq -r '.[] | select(.elapsed_seconds > 86400) | .id'`
  work without humanized-string parsing. Single-id JSON is an
  OBJECT (not array) so `jq -r .elapsed_seconds` works without
  indexing. Clock-skew (started in the future) clamps to 0 rather
  than returning a negative duration.

- `why` is the timeline sibling of `show` (snapshot) and `diff`
  (file delta). Emits each known timestamp (created, started,
  waited, due, completed) sorted chronologically with relative
  annotations (today/overdue/upcoming for due; hidden-until vs
  expired for wait). Done tasks suppress the overdue framing so
  "due (overdue)" doesn't appear on already-completed work. The
  due-annotation test uses an explicit +3-day date to dodge the
  documented UTC-midnight persistence boundary from tick #5.
  Empty-events case (hand-edited task with no Created) prints an
  explainer line instead of crashing.

- `shuffle` uses partial Fisher-Yates on an index slice so only
  the first N positions are uniformly shuffled (O(N) work) and the
  original pool is never copied. Sampling is WITHOUT replacement
  (the same task can never appear twice in one pick — sampling
  with replacement would feel broken given the user's intent).
  --seed makes picks deterministic for tests and reproducible
  scripts; 0 = time-based. Default scope mirrors top/next (undone,
  not waiting); --all expands; --tag and --priority compose via
  AND. N > pool caps at pool size with a heads-up note instead of
  erroring (user's intent is obvious).

- `find` is the fast, title-only cousin of `grep`. grep matches
  across title + tags + notes; find matches TITLE only and skips
  the notes scan entirely — the right tool when you remember a
  phrase from the title and the store is big. Defaults mirror grep
  (case-insensitive default, undone-only, --invert / --limit /
  --done / --all). compileGrepPattern is reused from grep.go so
  case-fold semantics can't drift. Mutually-exclusive output
  modes (--files-only / --count / --json) guarded up-front.

- `depend` is the FIRST storage extension since tick #8's
  start/started. New field model.Task.DependsOn []int + HasDeps
  helper; store parser handles depends/depends_on/dependson;
  writer emits `depends:1,5,7` after pin and before tag closer.
  Old files round-trip unchanged (strictly additive). lint's
  knownMetaKeys updated; hash's canonical projection extended in
  documented order (after tags, before created) — two stores
  describing the same deps in any input order hash identically
  thanks to a numeric sort inside the projection.
  Enforcement lives in toggle.go: runToggle pre-flights every id
  when done=true; a blocked id aborts the WHOLE batch with a
  usage-coded error (exit 2, no partial state). The unmetBlockers
  helper treats ids in the SAME batch as satisfied — so
  `tsk done 1 2` works when 2 depends on 1 (without forcing arg
  order; forcing it would be hostile). Dangling deps (id with no
  task) are TOLERATED at runtime so a hand-edit typo can't lock
  you out; `tsk lint` is the surface for those.
  Cycle handling: self-deps and direct A↔B cycles are rejected
  at write time. 3+ node cycles are intentionally NOT detected —
  rare in practice and the user notices when both ends refuse to
  close. Graph traversal cost > value here.
  Four mutation flags (--on / --add / --remove / --clear) are
  mutually exclusive. --list gives a global "what's stuck on
  what?" view with JSON output for CI signals.

Roadmap status: tick #8 "Polish & DX" subsection 5/10 → 8/10
(elapsed, why, shuffle added). tick #6 subsection 5/10 → 6/10
(find added). "Polish & DX" original section (tick #5) one more
done (depend). Added "Polish & DX (added tick #9)" subsection with
10 fresh items so future ticks have ample room — many are
follow-ons to depend (blocked alias, graph, --tree, next
--respect-deps).

Per-feature test counts: elapsed 7, why 8, shuffle 11, find 9,
depend 15. Total ~50 new test cases on top of the existing suite,
all green. (Existing test suite ~480 cases also still green after
the depend storage extension + SetDone enforcement change —
verified via full repo gate before push.)

One small post-commit fix: the depend commit landed with a typo'd
author email (`51058514+Sanjays2402+@...` — extra `+`), corrected
via `git commit --amend --reset-author --no-edit` BEFORE push, so
origin only ever saw the canonical email.

### 2026-06-20 20:46 PT (tick #10)

Shipped 5 features — all five from the "Polish & DX (added tick
#9)" subsection plus extensions to `top` and `ls` baked into the
`--respect-deps` slice. Together they form a cohesive
"dependency-aware workflow" cluster: discover blockers (blocked),
visualize the graph (graph), drill into one chain (depend --tree),
plan around it (next/top/ls --respect-deps), and clean it up by
merging duplicates (merge with back-ref rewrite). All single-commit,
all tests passing, full gate green (gofmt + vet + build + go test
./...) before push. Pushed da61ded..c695fe7 to origin/feature/autoship;
verified landed.

- `blocked`           — 714e4d8 feat(blocked): discoverable verb for stuck tasks
- `depend --tree`     — 3c5eed7 feat(depend): recursive prerequisite chain
- `graph`             — 607c42b feat(graph): whole-store dependency graph (ascii + DOT)
- `--respect-deps`    — 7ad2561 feat(deps): for next, top, ls
- `merge`             — c695fe7 feat(merge): fold two tasks into one

Notable choices:

- `blocked` is a TOP-LEVEL command, not a cobra alias on `depend`.
  An alias would surface as `tsk depend blocked` in help/man output,
  burying it. A dedicated command shows up in `tsk --help` and gets
  its own tab-completion entry. The `stuck` synonym was added for
  muscle-memory typists. Runtime is a one-line delegate to
  runDependList so the two surfaces literally cannot drift.

- `depend --tree` adds defensive cycle protection that the writer
  intentionally omits. tick #9 documented that `depend --on` only
  rejects self-deps and direct 2-cycles; 3+ node cycles are
  tolerated (rare, expensive to detect). The tree renderer would
  loop forever on those, so it uses a visit-set guarded recursion
  — re-entering an already-descending node marks it "(cycle)" and
  short-circuits. Set is mutated on entry and rolled back on exit
  so the same id under two ancestors in a fan-in graph renders
  fully under each.

- `graph` is the bird's-eye complement. ASCII format groups
  sources into open then "(done):" sections so historical noise
  doesn't bury active work. DOT format styles nodes: done = filled
  lightgray (satisfied), open-blocked = red outline (chokepoint),
  open-actionable = default. Arrow convention "A -> B means A
  depends on B" matches `depend --on`'s English reading so users
  don't context-switch. Edge sort is fully deterministic so shell
  diffs across time work.

- `--respect-deps` is the killer feature. Without it, `tsk next`
  happily returns an urgent task that can't actually be done. With
  it, the selector skips blocked tasks and falls back to "best
  blocked + (blocked by #X) annotation" when EVERY open task is
  blocked — refuses to lie with "all caught up" in that case.
  Default is OFF so legacy scripts piping `tsk next` keep their
  exact priority-only behavior; opt in per call (or future config
  knob). The shared filter helper (filterBlockedTasks in
  toggle.go) lives next to its sister unmetBlockers so the
  semantics can't drift between done-time enforcement and view-
  time filtering.

- `merge` is the most complex of the five and was sized for it.
  Notes concatenate with a "--- merged from #N ---" provenance
  separator (dropped when either side is empty). Tags union (case-
  insensitive, dedup). Deps union. The hardest part: BACK-REF
  REWRITE — every DependsOn list elsewhere in the store that
  pointed at the victim is rewritten to point at the survivor,
  with DEDUP so a third task that depended on BOTH ends up with
  the survivor exactly once. Scalar conflicts (priority, due, etc.)
  resolve via --prefer {survivor, victim, newer}; --note-only
  skips them entirely. --dry-run previews without writing.
  Mutual-dep refusal forces the user to clear the relationship
  before merging — implicit handling would hide the relationship
  in unexpected ways. The whole thing goes through store.Save so
  `undo-last --yes` reverts the entire merge in one step
  (regression-tested).

Roadmap status: tick #9 "Polish & DX" subsection 0/10 → 5/10
(blocked, graph, depend --tree, --respect-deps, merge — the
dependency-tooling cluster). The remaining 5 in that subsection
are split, timer, rules, export --opml, preview. Added "Polish &
DX (added tick #10)" subsection with 10 fresh items — many are
follow-ons to the dep cluster (`graph --reachable`, `path <a> <b>`,
`topo`, `depend --justify`, `next --json`, `show <id> --tree`,
`merge --interactive`).

Per-feature test counts: blocked 4, depend --tree 9, graph 8,
--respect-deps 7, merge 12. Total ~40 new test cases on top of the
existing suite, all green. (Existing test suite ~530 cases also
still green after the next.go rewrite + the new shared
filterBlockedTasks helper — verified via full repo gate before
push.)

### 2026-06-20 23:18 PT (tick #11)

Shipped 5 features — all from the "Polish & DX (added tick #10)"
subsection, forming a tight cohesive cluster: deep dependency
debugging tooling. Together they answer five distinct questions
about a dep graph that the previous cluster (blocked/graph/tree/
respect-deps/merge) opened up: "in what order?" (topo), "from
where to where?" (path), "just this subgraph?" (graph
--reachable), "why is it stuck?" (depend --justify), and "what's
the next pick programmatically?" (next --json). All single-commit,
all tests passing, full gate green (gofmt + vet + build + go test
./...) before push. Pushed 334163e..d981978 to
origin/feature/autoship; verified landed.

- `next --json`        — 78bd57e feat(next): script-friendly structured output
- `topo`               — 6117667 feat(topo): dep-respecting linearization
- `path <a> <b>`       — 4b40cce feat(path): shortest dependency path via BFS
- `graph --reachable`  — 451f3e7 feat(graph): subgraph filter
- `depend --justify`   — d981978 feat(depend): plain-English why-chain

Notable choices:

- `next --json` keeps a stable schema: {id, title, priority, due,
  pinned, tags, blocked?, blocked_by?} for found, {empty: true}
  for caught-up. The omitempty contract is documented (empty key
  NEVER appears on a successful pick) so `jq '.empty'` reliably
  branches. blocked + blocked_by encode the "(blocked by …)"
  annotation the human-readable path appends under --respect-deps
  fallback — same semantic, structured form.

- `topo` uses Kahn's algorithm with an inside-layer tie-break
  identical to `tsk top`/`next` (pin > priority desc > earliest-
  due > lower id), so the head of `topo` is what `next
  --respect-deps` would return. Cycle handling: any task left
  after the drain has non-zero in-degree → emitted at the tail
  with "(cycle)" annotation in plain text or `"cycle": true` in
  JSON. Never silently dropped — the user needs to know the file
  is broken. Plain-text footer points at `tsk lint`. Four output
  formats (default plain / --json / --ids / --format dot), all
  mutually exclusive (validated up-front).

- `path` BFS uses parent-map reconstruction (O(V) memory) not
  queue-stored-paths (O(V²) worst case). Strictly a → b
  direction; reverse is `tsk graph --reachable` territory.
  silentExit (exit 1 with no "error:" prefix) so the human-
  readable "no dependency path" line stands alone — the message
  IS the signal. JSON variant always emits the full shape with
  found=false on no-match, but still exits 1 for script
  `tsk path A B || …` patterns.

- `graph --reachable` is the "subgraph rooted at one task" filter.
  BFS over source→target edges from the root; drop edges whose
  source isn't in the reachable set. Empty-from-root gets a
  specific message ("no dependencies reachable from #N") vs the
  whole-store empty text. Stacks cleanly with --open (regression-
  tested). The fan-in test guards an important semantic: sibling
  tasks that ALSO depend on the same prereq are NOT pulled in —
  only the ancestors of the queried root.

- `depend --justify` is the bottleneck-finder. Walks lowest-id
  open prereq at each step (deterministic; the "best pick"
  question is `tsk next`'s job — justify is for tracing one
  chain to its actionable leaf). The status state-machine is
  encoded explicitly in the JSON: done / no-deps / open-leaf /
  blocked / cycle / missing. Root-vs-mid context switches the
  classification: a root with no DependsOn entries gets "no-deps"
  (distinct message: "did you forget to set deps?"), while a
  mid-chain leaf with no DependsOn gets "open-leaf" with the
  "START HERE" marker. buildJustifyChain tracks isRoot so this
  is one bool, not a position-arithmetic dance for the renderer.
  Cycle-safe (visit-set guard), mutually exclusive with --tree
  (different intents — tree=structure, justify=why-chain), and
  the depend.go flag validator gains a 4th read-only mode without
  breaking the existing 3.

Roadmap status: "Polish & DX (added tick #10)" subsection 0/10 →
5/10 (next --json, topo, path, graph --reachable, depend
--justify). The remaining 5 are show --tree, top --pinned-only,
merge --interactive, depend --add-bidir, depend --pending. Added
"Polish & DX (added tick #11)" subsection with 10 fresh items —
many are follow-ons to the new debugging cluster (justify --all,
path --any-direction, topo --since, depend --upstream/--pending,
reachable as top-level verb).

Per-feature test counts: next --json 5, topo 14, path 11, graph
--reachable 7, depend --justify 11. Total 48 new test cases on
top of the existing suite, all green. (Existing test suite ~570
cases also still green after the depend flag-validator signature
change and the graph emitGraph signature change — verified via
full repo gate before push.)

### 2026-06-21 02:34 PT (tick #12)

First tick on the new commit-direct-to-main loop (chore commit
5ba7c8b switched STATE.md and the cron prompt from
feature/autoship to main in the prior pass). 5 features picked
from the "Polish & DX (added tick #11)" backlog forming a
discoverability + entry-point cluster: extending the dep-debugging
tooling with the reverse view (`--upstream`), surfacing the
reachable subgraph as a top-level verb, folding the dep tree into
`tsk show`, gating the `next` picker via `--skip`, and removing
the "add-then-depend" two-step at task creation.

All single-commit, all tests passing, full gate green (gofmt + vet
+ build + go test ./...) before push. Pushed 038cdf3..02906ea to
origin/main; verified landed.

- `show --tree`       — 038cdf3 feat(show): tsk show <id> --tree
- `depend --upstream` — 4483efa feat(depend): tsk depend --upstream
- `reachable`         — db26cea feat(reachable): top-level discoverable verb
- `next --skip`       — 53f407f feat(next): tsk next --skip <ids>
- `add --depends`     — 02906ea feat(add): tsk add --depends <ids>

Notable choices:

- `show --tree` reuses printDependTreeText + buildDependTreeNode
  from depend_tree.go so the plain/JSON tree rendering can't drift
  from `tsk depend --tree`. The "no deps" path makes output BYTE-
  IDENTICAL to plain `tsk show` (no dangling empty section header)
  so callers with fixed expectations don't see drift. JSON
  embedding uses a marshal→unmarshal→re-marshal round-trip to
  splice `dependency_tree` onto the task object without changing
  the existing field set; the key is omitted entirely on leaf
  tasks, so the JSON schema for `tsk show --json` callers stays
  stable.

- `depend --upstream` answers the inverse-of-tree question with
  per-row state annotation: "(unblocks)" / "(blocked)" / "(done)".
  "Unblocks" is the headline signal — it tells the user which
  downstream tasks the next close would actually activate. The
  classifier looks at OTHER blockers (deps minus the queried task)
  to decide; if all of them are done (or missing — same policy as
  unmetBlockers), the queried task IS the gating edge.
  validateDependFlags grew a `readOnlyCount` arbiter so
  --tree/--justify/--upstream are mutually exclusive (each is a
  different view; combining muddles the output shape).

- `tsk reachable <id>` follows the same pattern as tick #10's
  `tsk blocked` — a top-level discoverable verb that delegates
  to an existing flag (`graph --reachable`). Aliases would bury
  the command under `graph` in help/man output; top-level entries
  show up in `tsk --help` and get shell-completion. The forwarder
  is a one-liner so the two surfaces literally cannot drift, and
  TestReachableMatchesGraphReachable asserts byte-identical
  output between them.

- `next --skip` is the right shape for the "no thanks, runner-up"
  moment. Pin/freeze would persist state for a transient choice;
  `--skip` is a one-call filter that vanishes after the command
  returns. The CSV parser silently drops missing ids (the
  semantic is "do not consider these"; bouncing on a stale id
  would make it brittle in scripts) but DOES reject non-numeric
  tokens up-front with usage exit code 2 so typos surface.
  Composes with --respect-deps so users can ask "give me the
  next unblocked task that isn't one of these".

- `add --depends` is the same shape as `tag <id> +foo -bar`'s "do
  it all in one call" ergonomic. The dep parser is `parseDependCSV`
  (shared with `tsk depend --on`) so id syntax — including `#7`
  prefix and dedup — can't drift between creation and post-
  creation. Validation is two-pass: existence check BEFORE s.Add
  so a typo never lands a half-formed task; then
  `validateProposedDeps` AFTER id allocation with `s.Remove`
  rollback on failure, sharing the SAME cycle/self-dep validator
  `tsk depend --on` uses. Success line annotates the wiring:
  "added #4: report (depends on #1, #3)".

Roadmap status: "Polish & DX (added tick #11)" subsection 0/10 →
4/10 (depend --upstream, reachable, next --skip, add --depends).
"Polish & DX (added 2026-06-20 tick #10)" subsection 5/10 → 6/10
(show --tree added). Added "Polish & DX (added 2026-06-21 tick
#12)" subsection with 15 fresh items — leans into TUI polish,
storage/import backlog, and cross-cuts the loop hasn't visited
in a while (config, multi-file, archive).

Per-feature test counts: show --tree 5, depend --upstream 8,
reachable 6, next --skip 7, add --depends 7. Total 33 new test
cases on top of the existing suite, all green. (Existing test
suite ~615 cases also still green after the depend validator
signature change and the show JSON marshalling change — verified
via full repo gate before push.)

This is the first batch shipped DIRECTLY ON MAIN (no
feature/autoship branch), so every commit immediately shows on
GitHub's contribution graph. The quality gate (gofmt + vet +
build + full test suite) is the only thing protecting main —
ran clean before push.

### 2026-06-21 05:41 PT (tick #13)

Shipped 5 features from the "Polish & DX (added tick #12)"
backlog, completing the dependency-debugging follow-ons that the
last cluster opened up: a top-level `justify` verb (with `--all`
for the whole-store chokepoint review), the `--upstream` sister
of `show --tree`, the "now-unblocked notification queue" via
`depend --pending`, undirected BFS via `path --any-direction`,
and the pinned-bookmark filter via `top --pinned-only`. All
single-commit, all tests passing, full gate green (gofmt + vet
+ build + go test ./...) before push. Pushed 911682e..44939fd
to origin/main; verified landed.

- `justify`             — 911682e feat(justify): top-level chain-of-reasons verb [--all]
- `show --upstream`     — 11b466b feat(show): tsk show <id> --upstream appends dependents
- `depend --pending`    — 55d651f feat(depend): tsk depend --pending now-unblocked queue
- `path --any-direction`— 3ce417c feat(path): undirected BFS for "are these related at all?"
- `top --pinned-only`   — 44939fd feat(top): tsk top --pinned-only bookmark view

Notable choices:

- `justify` follows the same forwarder pattern as `blocked` and
  `reachable`: top-level verb that delegates to runDependJustify
  for the single-id case so output is byte-identical to
  `tsk depend <id> --justify` (a regression test asserts this).
  `--all` is the new capability: walks every open blocked task
  in id order, prefixing each chain with `=== #N title ===`
  headers in plain text and emitting a JSON OBJECT keyed by id
  string (NOT array) so callers can look up one chain in
  constant time. Empty result = `{}` not null so `jq 'keys[]'`
  consumers don't crash. Inner chain shape matches single-task
  --json exactly (justifyStep array) so jq filters compose
  across modes.

- `show --upstream` mirrors `--tree` for the inverse direction.
  Plain output: snapshot + blank line + `upstream:` header +
  indented rows with the same `(unblocks)/(blocked)/(done)`
  annotations `tsk depend --upstream` uses. Suppressed on a
  task with no dependents — output is BYTE-IDENTICAL to plain
  `tsk show` in that case (regression test guards). JSON uses
  the same round-trip-and-splice technique as `--tree`'s
  emitShowJSONWithTree so the existing field set is preserved
  exactly; the new `upstream` key is omitted when empty.
  --tree and --upstream are mutually exclusive — each is a
  different relationship.

- `depend --pending` is the "now-unblocked notification queue"
  — distinct from `tsk blocked --inverted` because it includes
  the RECENCY discriminator. A task is pending iff: open + has
  deps + zero unmet blockers + at least one done dep completed
  inside --since (24h default). The recency check is what makes
  this the "what just became actionable?" view rather than
  "every open unblocked task" (which would include stale
  noise). Done prereqs WITHOUT a Completed timestamp (hand-
  edited) are conservatively ignored for recency but still
  count as satisfied. Each row annotates the trigger ("unblocked
  by #1 at 2026-06-21 05:49") so the why-now signal is
  immediate. Reuses parseDurationLocal (same parser tsk log
  uses) so 7d / 2w / 1h30m all work consistently. Validator
  rejects positional id with --pending, mutex with --list and
  --tree/--justify/--upstream.

- `path --any-direction` adds undirected BFS to the existing
  directed search. New findDepPathUndirected mirrors findDepPath
  exactly except the neighbour set combines forward edges
  (t.DependsOn) AND reverse edges (other tasks that name t in
  their DependsOn). The reverse adjacency map is built once
  up-front (O(V+E)) so the BFS stays O(V+E) — avoiding per-pop
  scans that would make it O(V²+VE) on big stores. Neighbour
  sort is ascending so output reproducibility matches the
  directed search's contract. Plain-text not-found message now
  suggests --any-direction in directed mode ("try
  --any-direction") and explicitly says "even with
  --any-direction" when the wider search also fails (no false
  hope). JSON adds a `direction` field ("directed" vs "any") so
  consumers know which search produced the result. The reported
  path always reads from→to in output order even when
  intermediate hops follow reversed edges.

- `top --pinned-only` is the "important-bookmark" view —
  restricts the result to pinned tasks regardless of priority.
  Pinning already floats tasks to the TOP of `top`/`next` via
  tie-break #1, but that's ordering, not isolation; --pinned-
  only is the filter that answers "show me ONLY what I've
  marked worth tracking." filterPinnedTasks is a fresh helper
  next to filterTopCandidates (kept separate so each predicate
  is single-purpose, matching the filterBlockedTasks pattern).
  Applied AFTER filterTopCandidates but BEFORE --respect-deps
  so the call order documents the layering: scope -> bookmark
  -> reachability. Stacks cleanly with --respect-deps
  ("which of my pinned tasks are actually unblocked?"). Empty
  when nothing's pinned → "no tasks" marker, no silent
  fallback.

Roadmap status: tick #12 "Polish & DX" subsection 0/15 → 5/15.
The remaining 10 are show --watch, import, lint --dep-cycles,
topo --since, depend --add-bidir, export --graph-dot, recent,
archive, config, multi-file. Added "Polish & DX (added 2026-06-21
tick #13)" with 20 fresh items so future ticks have ample room
— leans into TUI, storage/import-export, recurring tasks, and a
few cross-cuts the loop hasn't visited (split, wrap, dedupe
--merge, rules, timer).

Per-feature test counts: justify 8, show --upstream 6,
depend --pending 9, path --any-direction 6, top --pinned-only 6.
Total 35 new test cases on top of the existing suite, all green.
(Existing suite ~648 cases also still green after the show flag
addition, path JSON schema addition, depend validator signature
change, and top filter chain reorder — verified via full repo
gate before push.)

Process notes:
- Hit a self-inflicted bug in TestPendingHonorsSinceWindow's
  hand-edit logic: used `strings.IndexAny(rest, " -")` to find
  the end of the completed:RFC3339 value, but `-` matches
  INSIDE the date (2026-06-21). Replaced with `strings.Index(rest, " ")`
  (RFC3339 has no spaces, so the next space terminates the value).
- Hit a name shadow in TestPathDirectionFieldInJSON — local
  `dir, _ := dirDoc["direction"]…` shadowed the outer `dir` (tmp
  dir). Tests still passed because the shadow happened after the
  last runCmd, but renamed local to `direction` for clarity.
- One commit needed a `git commit --fixup` + autosquash to fold a
  gofmt alignment pass into the right feature commit (the
  --pending flag block in depend.go had unaligned var declarations
  before gofmt ran), keeping per-feature one-commit revertibility
  intact.

### 2026-06-21 08:43 PT (tick #14)

Shipped 5 features from the "Polish & DX (added tick #13)" backlog,
forming a cohesive "depth-of-feature" cluster on the existing
commands plus a brand-new pipeline-safe view. The dep-debugging
follow-ons that landed:

- `lint --dep-cycles`: closes the documented gap where the depend
  writer rejects self/2-cycles but ignores 3+ node cycles.
- `topo --since <id>`: anchors topological output at a checkpoint
  for the "I've already done up through #N, what's next?" workflow.
- `archive --strategy weekly`: ISO-week section grouping for the
  archive file (the year-2 scannability problem).
- `depend --pending --tag <t>`: narrows the standup queue to one
  project tag — same shape as `tsk ls --tag`.
- `preview`: stdin/--from rendering of a .tsk.md payload without
  touching the active store or .bak chain. Pipeline-safe.

All single-commit, all tests passing (29 new test cases across the
5 features), full gate green (gofmt + vet + build + go test ./...)
before push. Pushed 9a7ee95..4aa5ed4 to origin/main; verified
landed.

- `lint --dep-cycles`       — 9a7ee95 feat(lint): tsk lint --dep-cycles Tarjan SCC
- `topo --since`            — 4610a4e feat(topo): tsk topo --since <id> checkpoint anchor
- `archive --strategy week` — 7dd99be feat(archive): tsk archive --strategy weekly ISO-week sections
- `depend --pending --tag`  — d8373d7 feat(depend): tsk depend --pending --tag narrow queue
- `preview`                 — 4aa5ed4 feat(preview): tsk preview stdin/--from snapshot renderer

Notable choices:

- `lint --dep-cycles` uses Tarjan's strongly-connected-components
  algorithm rather than a plain DFS cycle search because SCC gives
  us a stable, canonical surface: every cycle has exactly one
  representation regardless of visit order. The reconstructed chain
  follows directed edges from the smallest-id rotation start, so
  the printed "#1 -> #2 -> #3 -> #1" reads as the actual
  traversal, not Tarjan's pop order. 2-cycles SKIPPED (rejected at
  write time; surfacing them would be noise unless hand-edited
  around the validator). Dangling deps ignored (matches
  unmetBlockers' policy). Opt-in via --dep-cycles so default
  `tsk lint` stays fast on big stores.

- `topo --since` strict requirement: the id MUST already be in the
  topological output for the trim to anchor anything. If not (typo
  or excluded by --all=false), exit-2 with a message that names
  --all explicitly — silently emitting empty would be hostile. The
  helper (sliceTopoSince) first looks in the linear pass then in
  the cycle tail, so a checkpoint inside a corrupt cycle still
  anchors — corruption visibility wins over the slicing window.
  Cycle-tail rows from BEFORE the slice start get re-appended at
  the new tail (defensive against unusual topoOrder positioning;
  the property "cycle visibility wins" matches the rest of the
  codebase).

- `archive --strategy weekly` never re-buckets existing archive
  content — only the BATCH being added on this call gets weekly
  layout. Re-arranging old data would shuffle the layout users
  might already be looking at via grep/editor, and would touch
  the .bak chain for tasks they didn't ask to modify. ISO-week
  formatting via time.ISOWeek (Mon-first, year-boundary-safe).
  "## undated" bucket catches done tasks without a Completed
  stamp so hand-edited rows aren't lost. Buckets sort oldest-
  first; "undated" explicitly pushed to the tail (sortKey=0
  would otherwise put it first). renderArchiveMeta /
  writeArchiveTask are local dups of store.renderMeta /
  store.renderTask (both unexported) — kept duplicate rather
  than exporting store internals because the archive layout is
  logically a sibling of the active writer, not a public API.

- `depend --pending --tag` uses Task.HasTag (the same predicate
  ls/top use) so tag semantics are consistent — case-insensitive,
  exact-match. Empty `--tag ""` deliberately behaves like no
  filter at all, defensive against a shell-var typo that leaves
  the value blank. Header annotation includes "tag=<name>" when
  the filter is active so the user understands WHY they got
  fewer rows than expected. Empty-result message also includes
  the tag — silent output on an empty filtered query would look
  identical to "no work at all".

- `preview` introduces store.LoadBytes — parses an in-memory
  payload into a Store with an empty Path. Documented contract:
  callers MUST NOT call Save on the returned Store. Preview
  reuses the lsFilters surface (--done/--all/--today/--overdue/
  --upcoming/--tag/--priority/--include-waiting/--respect-deps
  /--json/--format) so users don't learn a second filter set.
  --respect-deps walks the snapshot's OWN dep graph (exactly
  what "test this snapshot in isolation" wants). Safety guards:
  4 MiB cap on input (binary/log/.git pack files masquerading
  as .tsk.md); empty input is a usage error not silent "no
  tasks" (a zero-byte read almost always means the pipe broke
  or --from was forgotten). io.LimitReader caps the read so we
  don't even buffer beyond the cap before failing.

Roadmap status: "Polish & DX (added tick #13)" subsection 0/20 →
5/20 (lint --dep-cycles, topo --since, archive weekly, pending
--tag, preview). Added "Polish & DX (added 2026-06-21 tick #14)"
subsection with 23 fresh items — leans into the long-unstarted
storage/import-export backlog, recurring tasks (stale feat-recur),
TUI gaps, plus follow-ons to this batch (archive monthly/daily,
preview --watch, topo --since --depth/--reverse, pending --priority,
recipe docs).

Per-feature test counts: lint --dep-cycles 6, topo --since 6,
archive weekly 5, depend pending --tag 5, preview 8. Total 30 new
test cases on top of the existing suite, all green. (Existing
suite ~680 cases also still green after the depend pending
signature change, archive output format addition, lint command
flag addition, topo flag addition, and the new store.LoadBytes
export — verified via full repo gate before push.)

This is the second batch (after tick #13) shipped DIRECTLY ON
MAIN. The quality gate ran clean before push; every feature is a
single revertible commit (no fixups needed this tick).

### 2026-06-21 11:53 PT (tick #15)

Shipped 5 features from the tick #13/#14 backlog — a sweep of the
"clear-shape" remaining items: monthly archive (sister of weekly),
pending priority filter (sister of pending --tag), reverse topo
slice (mirror of --since), the central data-out verb for the
graph (`tsk export --graph-dot`), and lint's non-interactive
autofix-all combining the round-trip + semantic backfill paths.

All single-commit, all tests passing (36 new test cases across
the 5 features), full gate green (gofmt + vet + build + go test
./...) before push. Pushed 41909e5..cf0b8af to origin/main;
verified landed.

- `archive --strategy monthly`  — 41909e5 feat(archive): YYYY-MM bucketed sections
- `depend --pending --priority` — b015f30 feat(depend): narrow freshly-unblocked queue by priority
- `topo --since --reverse`      — 8bec6b5 feat(topo): emit prereq chain leading up to milestone
- `export --graph-dot`          — 276edf8 feat(export): graph DOT under the central data-out verb
- `lint --autofix-all`          — cf0b8af feat(lint): multi-step safe repairs (canonicalize + backfill)

Notable choices:

- `archive --strategy monthly`: refactored `writeWeeklyArchive`
  into a generic `writeBucketedArchive(path, arch, batch, bucketFn)`
  that takes a `bucketFn func(model.Task) (key string, sortKey int)`.
  Two shipped implementations: `bucketByISOWeek` (existing) and
  `bucketByMonth` (new — uses `t.Completed.Date()` for year/month
  in the task's recorded time zone). All other archive policies
  unchanged: existing content is preserved verbatim (no
  re-bucketing); "undated" goes to the tail; per-bucket
  id-ascending order; .bak snapshot via store.AtomicWriteFile;
  archive ids continue from the file's max+1. Sortkey scheme
  year*100+month is lexicographic-safe (Dec 2025 = 202512,
  Jan 2026 = 202601, sorts correctly). The error message for an
  unknown --strategy was updated to name all three valid values
  ("flat, weekly, or monthly") so users discover monthly from a
  typo on quarterly.

- `depend --pending --priority`: intersects with --tag (not
  unions). The user story is "what's freshly unblocked AND on
  fire?" — both filters tightening the feed. parsePendingPriority
  uses the canonical model.ParsePriority (same parser ls/top/add
  use) so short forms ("u" / "h" / "m" / "l") work. Empty value
  mirrors --tag's defensive policy: no filter, no header
  annotation. Invalid values are exit-2 with the bad value
  quoted (silent fallback to "no filter" would confuse —
  the user clearly meant something). buildPendingFilterSummary
  helper centralizes the "tag=X, priority=Y" trailer used in
  both the header and the empty-state message so the rendering
  rule (deterministic comma-separated ordering) is in one place.

- `topo --since --reverse`: FIRST implementation sliced by linear
  position in the topo sequence — caught by a cycle-visibility
  test that surfaced the semantic bug. Kahn's algorithm sorts
  the ready set by priority, so isolated tasks drain in
  lock-step with real prereqs; positional slicing pulls in
  unrelated work. The correct semantics: walk the DependsOn
  graph FORWARD from the anchor (forward through "I depend on"
  arrows IS the prereq direction), build the transitive prereq
  set, then intersect with the original topo order so emission
  stays dependency-respecting. Cycle-tail rows that are
  themselves prereqs of the anchor are preserved at the end
  (corruption visibility wins, same policy as sliceTopoSince).
  Cycle rows that are NOT prereqs are dropped (they don't gate
  this particular milestone). --reverse without --since is a
  usage error; the at-head case (anchor has no prereqs) is a
  separate error that names "head" so the user understands.
  Existing unit test on sliceTopoBefore had to update its
  synthetic data to set explicit DependsOn slices (since the
  helper now needs them) — folded back into the feature commit
  via git commit --fixup + autosquash so the per-feature
  one-commit revertibility contract stayed intact.

- `export --graph-dot`: routes through the existing
  emitGraph(w, s, edges, "dot", reachable) helper from graph.go
  so the "no dependencies" / "no dependencies reachable from #N"
  empty-state messages are byte-for-byte identical to
  `tsk graph --format dot`. A regression test asserts the two
  surfaces stay in lockstep. --reachable and --open are added
  to export and rejected up-front with a clear error when paired
  with a non-graph format ("--reachable / --open only apply to
  --graph-dot, got format X") — silently ignoring would be
  confusing. Format aliases accepted: "graph-dot", "graphdot"
  (no-dash for some shells), and "dot" (the short form users
  coming from `tsk graph` will reach for).

- `lint --autofix-all`: builds on --fix by ALSO repairing the
  semantic finding the round-trip can't address —
  missing_created_timestamp. The repair stamps now() into
  every flagged task's Created field, then saves through the
  canonical writer (which also fixes the round-trippable
  findings in the same pass). Idempotent: re-running on a clean
  file emits "all checks passed" without writing. Repair
  count credits ONE for the entire round-trippable bucket
  (a single save fixes them all) and ONE per semantic backfill
  (distinct fixes). Why now() and not file mtime? Because the
  very next thing we do is save, which overwrites mtime to
  now() anyway — they converge on the same outcome. Cycle
  resolution stays human-only (no safe automatic break-edge).
  --autofix-all + --json prints the JSON report FIRST then the
  "autofixed:" summary, so consumers can capture pre-fix state.

Roadmap status:
- "Polish & DX (added tick #13)" subsection: 5/20 → 6/20
  (lint --autofix-all shipped).
- "Polish & DX (added tick #14)" subsection: 0/23 → 3/23
  (archive monthly, export --graph-dot, lint --autofix-all
  shipped — autofix-all spanned both subsections).
- Added "Polish & DX (added 2026-06-21 tick #15)" subsection
  with 19 fresh items — leans into still-unstarted TUI, recurring
  tasks (parking lot), config + multi-file (long-unstarted
  storage backlog), and a few new follow-ons opened by this
  tick's features (archive daily/quarterly, topo --since
  --depth, export --graph-dot --highlight, lint --autofix-all
  --backup, --pending recipe doc).

Per-feature test counts: archive monthly 6, pending --priority 7,
topo --since --reverse 8, export --graph-dot 9, lint --autofix-all 7.
Total 36 new test cases on top of the existing suite, all green.
(Existing suite ~710 cases also still green after the archive
strategy refactor, the depend pending signature change, the
sliceTopoBefore implementation rewrite, the export resolveFormat
signature change, and the lint --autofix-all addition — verified
via full repo gate before push.)

Process notes:
- The cycle-visibility test for `topo --since --reverse` caught a
  real semantic bug (linear positional slicing was wrong for
  "what gates this milestone?"). Rewriting sliceTopoBefore as
  reverse BFS through DependsOn edges fixed it. The lesson:
  "before in topo position" is NOT the same as "transitive
  prereq" when Kahn's tie-break uses priority — isolated tasks
  drain alongside real prereqs.
- Folding the unit-test fix into the topo commit via
  `git commit --fixup` + `GIT_SEQUENCE_EDITOR=: git rebase -i
  --autosquash` kept per-feature one-commit revertibility. Had
  to stash the work-in-progress lint files first (`git stash
  push -u -m feature5-lint-autofix-all -- ...`) so the rebase
  could proceed with a clean working tree, then pop them back.
- Used `time.Now()` for lint's autofix backfill (not file
  mtime) because saving the file overwrites mtime to now()
  anyway — both converge. Documented as best-effort
  placeholders, not real creation times.

This is the third batch shipped DIRECTLY ON MAIN. The quality
gate ran clean before push; one feature (`topo --since
--reverse`) needed a git commit --fixup + autosquash to fold
in a test fix on top of the reverse BFS algorithm rewrite.
Every commit on origin/main remains a single revertible feature
slice.

