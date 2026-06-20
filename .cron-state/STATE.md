# tsk — autoship state

**Active branch:** `feature/autoship` (off main)
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

- [x] `tsk show <id>` — single-task detail view with notes, full timestamps; `--json` for scripts (tick 2026-06-19/2022)
- [x] `tsk pri <id> <prio>` — quick priority setter; aliases `priority` (tick 2026-06-19/2022)
- [x] `tsk due <id> <date>` — quick due setter; `--clear` to remove; natural language (tick 2026-06-19/2022)
- [x] `tsk tag <id> +foo -bar` — add/remove tags on a single task in one shot (tick 2026-06-19/2022)
- [x] `tsk where` — print resolved `.tsk.md` path + how it was resolved + tz; `--json` (tick 2026-06-19/2022)
- [ ] `tsk rename <id> <new title>` — quick title change without dropping into TUI
- [ ] `tsk note <id> [text]` — add/edit notes from CLI (opens $EDITOR with no text arg)
- [ ] `tsk clone <id>` / `tsk dupe <id>` — duplicate a task with a fresh ID
- [ ] `tsk reopen <id>` — alias for `undo` (more discoverable verb)
- [ ] `tsk snooze <id> <date>` — bump due date forward; refuses if already further out (with --force override)

### New views / scriptable surfaces

- [ ] `tsk top [N]` — show N highest-priority undone tasks (multi-task `next`)
- [ ] `tsk today` / `tsk overdue` — convenience aliases for the common `ls --today` / `ls --overdue` invocations
- [ ] `tsk tags` — list all tags with usage counts (extended top-tags; supports `--json`)
- [ ] `tsk log` — show recently completed tasks (tail of completions, optional `--since`)
- [ ] `tsk yesterday` — what was completed yesterday (standup-friendly summary)
- [ ] `tsk daily` — synthesized morning plan: overdue + today + top 3 upcoming, in one screen
- [ ] `tsk grep <regex>` — exact regex search across title+notes (vs fuzzy `search`)
- [ ] `tsk diff` — diff active `.tsk.md` vs `.bak` snapshot (what would `undo-last` undo)
- [ ] `tsk env` — print effective env: TSK_TZ, EDITOR, NO_COLOR, paths

### Storage / model / format

- [ ] `tsk import <path>` — accept todo.txt / TaskWarrior `task export` JSON / Notion CSV
- [ ] `tsk export --jsonl` — streaming JSON-lines for pipelines
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

## Known footguns the loop has run into

- The `/Users/sanjay/Projects/tsk` user-facing path is an ephemeral mount.
  Mid-tick the volume can flip and the path stops resolving. Always work
  from `/Volumes/Projects/tsk` directly and verify `pwd` after any pause.

## Tick log

### 2026-06-19 20:22 PT (tick #1)

Bootstrapped `feature/autoship` off main. Shipped 5 per-task CLI shortcuts that
collapse common `tsk bulk --id N --set-foo X --apply` invocations into single
verbs. Also added `tsk where` for prompt/debug introspection.

- `show` — feat: `tsk show <id>` detail view (notes, timestamps, JSON)
- `pri` — feat: `tsk pri <id> <prio>` quick priority setter
- `due` — feat: `tsk due <id> <date>` quick due setter + `--clear`
- `tag` — feat: `tsk tag <id> +x -y` add/remove tag mutator
- `where` — feat: `tsk where` prints resolved file + resolution method + tz

Mid-tick footgun: the `/Users/sanjay/Projects/tsk` path remounted, dropping
in-progress files; redid work at `/Volumes/Projects/tsk`. Single commit for
bootstrap + 5 commits for features = 6 commits this tick.
